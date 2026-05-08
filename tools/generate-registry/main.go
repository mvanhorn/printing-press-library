// generate-registry walks library/<category>/<slug>/ and emits the
// top-level registry.json from each CLI's .printing-press.json,
// manifest.json, and .goreleaser.yaml. Every field except `description`
// is fully derived from disk; `description` is preserved from the
// existing registry.json (or falls back to the .goreleaser.yaml brews
// description) so curated copy is not clobbered.
//
// This tool is the source of truth for registry.json. It runs in CI on
// push to main against library/** changes (see
// .github/workflows/generate-registry.yml) and commits the regenerated
// registry, matching the same generated-artifact pattern this repo
// already uses for cli-skills/.
//
// Usage:
//
//	go run ./tools/generate-registry             # write registry.json
//	go run ./tools/generate-registry --check     # exit non-zero if drift detected
//	go run ./tools/generate-registry --print     # print to stdout, do not write
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	libraryDir    = "library"
	registryPath  = "registry.json"
	readmePath    = "README.md"
	schemaVersion = 2
	// stdioTransport / httpTransport are the registry-side names for the
	// MCP transports an emitted binary can serve. Detection of which
	// transports a CLI actually supports happens in detectMCPTransports
	// by inspecting cmd/<binary>/main.go: every server links ServeStdio,
	// only the streamable-HTTP-capable ones reference NewStreamableHTTPServer.
	stdioTransport = "stdio"
	httpTransport  = "http"

	// README sentinel markers. The generator only rewrites bytes
	// between matching begin/end markers; surrounding prose stays
	// hand-editable. Same drift-prevention pattern applied to the
	// catalog table that registry.json regen applies to itself.
	catalogTableBegin  = "<!-- catalog:begin -->"
	catalogTableEnd    = "<!-- catalog:end -->"
	catalogCountsBegin = "<!-- catalog-counts:begin -->"
	catalogCountsEnd   = "<!-- catalog-counts:end -->"

	// Per-CLI release tags follow `<entry-name>-current`. Confirmed
	// against the live release list (espn-current, dominos-current,
	// tiktok-shop-current, agent-capture-current, etc.).
	releaseTagURLBase = "https://github.com/mvanhorn/printing-press-library/releases/tag/"
)

type Registry struct {
	SchemaVersion int             `json:"schema_version"`
	Entries       []RegistryEntry `json:"entries"`
}

type RegistryEntry struct {
	Name        string `json:"name"`
	Category    string `json:"category"`
	API         string `json:"api"`
	Description string `json:"description"`
	Path        string `json:"path"`
	// Printer is the GitHub @handle of the human who originally ran the
	// press for this CLI. Sourced verbatim from .printing-press.json's
	// `printer` field; never derived from operator git config or curated
	// from a prior registry value. Manifest is the only source of
	// truth so attribution survives across regenerations and across
	// operator changes.
	Printer string `json:"printer,omitempty"`
	// PrinterName is the prose-shaped display name of the printer.
	// Sourced from .printing-press.json's `printer_name` field. Empty
	// values are valid; the per-CLI README byline renders without a
	// parenthetical and the catalog row renders only the @handle.
	PrinterName string    `json:"printer_name,omitempty"`
	MCP         *MCPBlock `json:"mcp,omitempty"`
}

// MCPBlock matches the on-disk shape of registry.json's mcp object.
// Field ordering is the documented surface — keeping it stable across
// regenerations means the only diffs in regenerated registry.json
// reflect actual content changes, not field-order churn.
//
// env_vars and public_tool_count are emitted unconditionally (even
// when empty/zero) because that matches the historical hand-edited
// shape; tool_count and tool_count's siblings (public_tool_count,
// env_vars: []) all appear together for every MCP-shipping entry.
// AuthType/MCPReady/SpecFormat use omitempty because some legacy
// entries genuinely lack those fields and synthesizing empty strings
// would be misleading.
type MCPBlock struct {
	Binary          string   `json:"binary"`
	Transports      []string `json:"transports"`
	ToolCount       int      `json:"tool_count"`
	PublicToolCount int      `json:"public_tool_count"`
	AuthType        string   `json:"auth_type,omitempty"`
	EnvVars         []string `json:"env_vars"`
	MCPReady        string   `json:"mcp_ready,omitempty"`
	SpecFormat      string   `json:"spec_format,omitempty"`
}

// printingPressManifest captures the subset of .printing-press.json fields
// the registry needs. The on-disk shape carries many other fields
// (scorecard_total, run_id, etc.); we ignore them so a future generator
// version that adds fields doesn't break this consumer.
type printingPressManifest struct {
	APIName            string   `json:"api_name"`
	DisplayName        string   `json:"display_name"`
	Printer            string   `json:"printer"`
	PrinterName        string   `json:"printer_name"`
	MCPBinary          string   `json:"mcp_binary"`
	MCPToolCount       int      `json:"mcp_tool_count"`
	MCPPublicToolCount *int     `json:"mcp_public_tool_count"`
	MCPReady           string   `json:"mcp_ready"`
	AuthType           string   `json:"auth_type"`
	AuthEnvVars        []string `json:"auth_env_vars"`
	SpecFormat         string   `json:"spec_format"`
}

// brewsDescriptionRE matches a `description:` line nested under `brews:` in
// .goreleaser.yaml. We avoid pulling in a YAML parser dep (the existing
// generate-skills tool stays stdlib-only, and this generator follows that
// constraint so `go run ./tools/generate-registry/main.go` works the same
// way `go run ./tools/generate-skills/main.go` does in CI). The regex
// matches the typical 4-space indentation goreleaser configs use, with
// optional surrounding double quotes around the value.
var brewsDescriptionRE = regexp.MustCompile(`^\s+description:\s*"?(.*?)"?\s*$`)

func main() {
	check := flag.Bool("check", false, "exit non-zero if generated outputs differ from on-disk registry.json or README.md sentinel regions")
	printOnly := flag.Bool("print", false, "print generated registry to stdout instead of writing")
	flag.Parse()

	existing := loadExistingEntries(registryPath)

	entries, err := buildEntries(libraryDir, existing)
	if err != nil {
		log.Fatalf("building entries: %v", err)
	}

	registry := Registry{
		SchemaVersion: schemaVersion,
		Entries:       entries,
	}

	registryOut, err := marshalRegistry(registry)
	if err != nil {
		log.Fatalf("marshaling registry: %v", err)
	}

	if *printOnly {
		os.Stdout.Write(registryOut)
		return
	}

	currentReadme, err := os.ReadFile(readmePath)
	if err != nil {
		log.Fatalf("reading %s: %v", readmePath, err)
	}
	newReadme, err := updateReadme(currentReadme, entries)
	if err != nil {
		log.Fatalf("updating %s: %v", readmePath, err)
	}

	if *check {
		var drift []string
		if currentRegistry, err := os.ReadFile(registryPath); err != nil {
			log.Fatalf("reading %s for check: %v", registryPath, err)
		} else if !bytes.Equal(currentRegistry, registryOut) {
			drift = append(drift, registryPath)
		}
		if !bytes.Equal(currentReadme, newReadme) {
			drift = append(drift, readmePath)
		}
		if len(drift) > 0 {
			fmt.Fprintf(os.Stderr, "drift detected in: %s\nRun `go run ./tools/generate-registry` and commit the result.\n", strings.Join(drift, ", "))
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, "registry.json and README.md are in sync with library/")
		return
	}

	if err := os.WriteFile(registryPath, registryOut, 0o644); err != nil {
		log.Fatalf("writing %s: %v", registryPath, err)
	}
	if err := os.WriteFile(readmePath, newReadme, 0o644); err != nil {
		log.Fatalf("writing %s: %v", readmePath, err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s and %s (%d entries)\n", registryPath, readmePath, len(entries))
}

// loadExistingEntries reads the current registry.json and returns a
// slug → entry map. Used by the entry builder to preserve fields that
// can't be reliably derived from disk:
//
//   - description: hand-curated copy (29/42 entries don't match the
//     .goreleaser.yaml brews description; the registry copy is what's
//     authoritative).
//   - mcp block: legacy CLIs (archive-is, linear, slack, steam-web,
//     trigger-dev) ship MCP source under cmd/<slug>-pp-mcp/
//     but their pre-v2 .printing-press.json doesn't declare mcp_binary
//     or tool_count. We carry their existing registry mcp block forward
//     until they're regen'd upstream and the .printing-press.json
//     catches up.
//
// Returns an empty map when the file is missing or unparseable so
// first-time runs and corrupted-file recovery both work.
func loadExistingEntries(path string) map[string]RegistryEntry {
	out := make(map[string]RegistryEntry)
	data, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	var r Registry
	if err := json.Unmarshal(data, &r); err != nil {
		return out
	}
	for _, e := range r.Entries {
		out[e.Name] = e
	}
	return out
}

// buildEntries walks libraryDir for <category>/<slug>/ pairs and builds
// one RegistryEntry per CLI. Errors out only on filesystem/JSON parsing
// failures; missing optional files (manifest.json, .goreleaser.yaml)
// degrade gracefully so partial CLIs still register.
func buildEntries(root string, existing map[string]RegistryEntry) ([]RegistryEntry, error) {
	categories, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("reading library dir: %w", err)
	}
	var entries []RegistryEntry
	for _, cat := range categories {
		if !cat.IsDir() {
			continue
		}
		catPath := filepath.Join(root, cat.Name())
		slugs, err := os.ReadDir(catPath)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", catPath, err)
		}
		for _, slug := range slugs {
			if !slug.IsDir() {
				continue
			}
			cliDir := filepath.Join(catPath, slug.Name())
			entry, err := buildEntry(cliDir, cat.Name(), slug.Name(), existing)
			if err != nil {
				return nil, fmt.Errorf("building entry for %s: %w", cliDir, err)
			}
			if entry == nil {
				continue
			}
			entries = append(entries, *entry)
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

// buildEntry constructs a single RegistryEntry from one CLI's directory.
// Returns (nil, nil) when the directory is missing .printing-press.json
// — that's the gate for "is this an actual CLI directory?" because every
// printed CLI ships one. Pre-printing-press top-level dirs (like build/
// or experimental scratch) are silently skipped.
func buildEntry(dir, category, slug string, existing map[string]RegistryEntry) (*RegistryEntry, error) {
	ppPath := filepath.Join(dir, ".printing-press.json")
	ppData, err := os.ReadFile(ppPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", ppPath, err)
	}
	var pp printingPressManifest
	if err := json.Unmarshal(ppData, &pp); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", ppPath, err)
	}

	prior := existing[slug]

	entry := RegistryEntry{
		Name:     slug,
		Category: category,
		API:      apiDisplayName(pp, prior, slug),
		Path:     filepath.ToSlash(dir),
		// Printer attribution: always derive from the manifest. Do not
		// honor a curated prior.Printer value — the manifest is the
		// only source of truth, and a curated map would re-introduce
		// the multi-author retrofit footgun the cliAuthorByAPIName map
		// in tools/sweep-canonical/ exists to manage carefully.
		// Existing CLIs without a printer field in their manifest will
		// emit registry entries with omitempty (no printer key).
		Printer:     pp.Printer,
		PrinterName: pp.PrinterName,
	}

	// Description preference: existing registry value (curated) > goreleaser
	// brew description (homebrew tap one-liner) > empty. Curated descriptions
	// in registry.json are the documented surface and shouldn't be clobbered.
	// Exception: an old generator bug let bare Markdown headings like
	// "# Introduction" land as descriptions. Those are not real curated copy,
	// so allow the source one-liner to repair them on the next regen.
	entry.Description = registryDescription(prior.Description, readGoreleaserDescription(filepath.Join(dir, ".goreleaser.yaml")))

	// MCP block preference: derive from .printing-press.json when it
	// declares mcp_binary (the modern, authoritative source) > preserve
	// existing block when the prior registry advertised one (covers
	// legacy CLIs whose .printing-press.json predates MCP metadata
	// fields but whose source still ships an MCP server) > omit.
	//
	// Within the modern path we also fall back to prior values for
	// fields that .printing-press.json may legitimately omit
	// (mcp_public_tool_count was added in a later schema version).
	// This avoids regressing accurate registry values to 0/empty when
	// only some fields drift forward.
	if pp.MCPBinary != "" {
		entry.MCP = buildMCPBlock(pp, prior.MCP, dir)
	} else if prior.MCP != nil {
		preserved := *prior.MCP
		preserved.Transports = detectMCPTransports(dir, preserved.Binary)
		entry.MCP = &preserved
	}

	return &entry, nil
}

func registryDescription(prior, fallback string) string {
	if prior != "" && !isBareMarkdownHeading(prior) {
		return prior
	}
	return fallback
}

func isBareMarkdownHeading(s string) bool {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "#") {
		return false
	}
	trimmed = strings.TrimLeft(trimmed, "#")
	return strings.TrimSpace(trimmed) != "" && !strings.ContainsAny(trimmed, "\r\n.")
}

// apiDisplayName picks the best human-facing name for the registry's
// `api` field. Preference order:
//
//  1. The current registry.json's existing `api` value, when it differs
//     from the slug — registry api values are hand-curated (e.g.,
//     "PokéAPI", "Cal.com", "Product Hunt") and frequently better than
//     what .printing-press.json's display_name auto-derives. Treating
//     prior == slug as "not curated" lets the generator replace bare
//     slug echoes with a proper display name when one shows up.
//  2. .printing-press.json's display_name (modern-generator best guess).
//  3. .printing-press.json's api_name (machine slug fallback).
//  4. The slug itself, last resort.
//
// Choosing prior over pp.DisplayName here is deliberate. Several
// existing registry entries have curated names (PokéAPI, Product Hunt)
// that pp's auto-derivation produces less faithfully (Pokeapi,
// Producthunt). The cost is: when a CLI's display_name *is* improved
// upstream, the registry won't pick it up automatically — but the
// curated value also won't regress. A future cleanup could lift
// curated api values back into .printing-press.json explicitly.
func apiDisplayName(pp printingPressManifest, prior RegistryEntry, slug string) string {
	if prior.API != "" && prior.API != slug {
		return prior.API
	}
	if pp.DisplayName != "" {
		return pp.DisplayName
	}
	if pp.APIName != "" {
		return pp.APIName
	}
	return slug
}

// buildMCPBlock constructs an MCP block from a CLI's .printing-press.json
// values, falling back to prior (existing registry) values for fields
// the manifest legitimately omits. This is what keeps small schema gaps
// from causing regressions: a CLI that was generated before
// mcp_public_tool_count was added doesn't lose its public_tool_count
// just because we regenerated.
//
// Field-level fallbacks deliberately mix authoritative (pp) and
// preserved (prior) signals; full-block preservation for legacy CLIs
// happens upstream in buildEntry.
func buildMCPBlock(pp printingPressManifest, prior *MCPBlock, cliDir string) *MCPBlock {
	mcp := &MCPBlock{
		Binary:     pp.MCPBinary,
		Transports: detectMCPTransports(cliDir, pp.MCPBinary),
		ToolCount:  pp.MCPToolCount,
		// EnvVars must be a non-nil slice so JSON encodes as `[]`
		// rather than `null`; this matches the historical hand-edited
		// registry shape where every MCP entry has an env_vars array
		// regardless of whether it's populated.
		EnvVars: append([]string{}, pp.AuthEnvVars...),
	}
	switch {
	case pp.MCPPublicToolCount != nil:
		mcp.PublicToolCount = *pp.MCPPublicToolCount
	case prior != nil:
		mcp.PublicToolCount = prior.PublicToolCount
	}
	if pp.AuthType != "" {
		mcp.AuthType = pp.AuthType
	} else if prior != nil {
		mcp.AuthType = prior.AuthType
	}
	if pp.MCPReady != "" {
		mcp.MCPReady = pp.MCPReady
	} else if prior != nil {
		mcp.MCPReady = prior.MCPReady
	}
	if pp.SpecFormat != "" {
		mcp.SpecFormat = pp.SpecFormat
	} else if prior != nil {
		mcp.SpecFormat = prior.SpecFormat
	}
	return mcp
}

// detectMCPTransports inspects a CLI's MCP binary main.go to determine
// which MCP transports the compiled server can serve. Every emitted MCP
// binary links ServeStdio so stdio is always reported; streamable HTTP
// is reported only when main.go references NewStreamableHTTPServer
// (the streamable-HTTP entry point from mark3labs/mcp-go).
//
// Detection by source-grep matches the runtime truth: the transport
// switch in cmd/<binary>/main.go is the only place that wires either
// ServeStdio or NewStreamableHTTPServer. If the file is missing
// (e.g., a legacy CLI whose MCP source was never copied here), we
// degrade to ["stdio"] — the historical default registry value.
//
// The returned slice is always non-nil so callers can rely on it
// encoding as a real JSON array rather than null.
func detectMCPTransports(cliDir, binary string) []string {
	transports := []string{stdioTransport}
	if binary == "" {
		return transports
	}
	mainPath := filepath.Join(cliDir, "cmd", binary, "main.go")
	data, err := os.ReadFile(mainPath)
	if err != nil {
		return transports
	}
	if bytes.Contains(data, []byte("NewStreamableHTTPServer")) {
		transports = append(transports, httpTransport)
	}
	return transports
}

// readGoreleaserDescription returns the first non-empty `description`
// field nested under `brews:` in .goreleaser.yaml. Returns "" on any
// failure (file missing, no brews block, no description) — the caller
// treats that as "no fallback available."
//
// Implementation: scan line-by-line for the brews: section, then return
// the first description: line within. We deliberately avoid a YAML
// dependency to keep this tool stdlib-only and compatible with the same
// `go run ./tools/<name>/main.go` invocation pattern generate-skills uses.
func readGoreleaserDescription(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	inBrews := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "brews:" {
			inBrews = true
			continue
		}
		// A new top-level YAML key (no leading whitespace, ends in :)
		// closes the brews block.
		if inBrews && len(line) > 0 && line[0] != ' ' && line[0] != '\t' && strings.HasSuffix(trimmed, ":") {
			break
		}
		if !inBrews {
			continue
		}
		m := brewsDescriptionRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		if d := strings.TrimSpace(m[1]); d != "" {
			return d
		}
	}
	return ""
}

// marshalRegistry produces the canonical on-disk byte representation:
// 2-space indent, no HTML escaping (so > stays as `>` rather than
// `>`), trailing newline. Matches the format the existing
// registry.json was hand-edited in so a re-run on a synced repo is a
// byte-level no-op.
func marshalRegistry(r Registry) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// updateReadme returns README bytes with the catalog table and count
// callout sentinel regions replaced by freshly-rendered content. The
// rest of the document is byte-preserved. Errors when either pair of
// sentinels is missing — the README is expected to opt in by adding
// the markers, and silently no-oping would let drift sneak back.
func updateReadme(readme []byte, entries []RegistryEntry) ([]byte, error) {
	updated, err := replaceSentinelRegion(readme, catalogTableBegin, catalogTableEnd, renderCatalogTable(entries))
	if err != nil {
		return nil, fmt.Errorf("catalog table: %w", err)
	}
	updated, err = replaceSentinelRegion(updated, catalogCountsBegin, catalogCountsEnd, renderCatalogCounts(entries))
	if err != nil {
		return nil, fmt.Errorf("catalog counts: %w", err)
	}
	return updated, nil
}

// replaceSentinelRegion finds a single begin/end marker pair in src
// and replaces the bytes between them (markers preserved) with body.
// body is rendered as a standalone block: the markers stay on their
// own lines and body sits between them, so the structure on disk is:
//
//	<begin>
//	<body...>
//	<end>
//
// Errors if the markers are missing or out of order so callers can
// surface "README needs to opt in" cleanly.
func replaceSentinelRegion(src []byte, begin, end, body string) ([]byte, error) {
	beginIdx := bytes.Index(src, []byte(begin))
	if beginIdx < 0 {
		return nil, fmt.Errorf("missing begin sentinel %q", begin)
	}
	endIdx := bytes.Index(src, []byte(end))
	if endIdx < 0 {
		return nil, fmt.Errorf("missing end sentinel %q", end)
	}
	if endIdx < beginIdx {
		return nil, fmt.Errorf("end sentinel %q precedes begin sentinel %q", end, begin)
	}
	beforeEnd := beginIdx + len(begin)
	var buf bytes.Buffer
	buf.Write(src[:beforeEnd])
	buf.WriteByte('\n')
	if body != "" {
		buf.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			buf.WriteByte('\n')
		}
	}
	buf.Write(src[endIdx:])
	return buf.Bytes(), nil
}

// renderCatalogTable returns the README catalog table body that goes
// between the catalog:begin and catalog:end sentinels. Format matches
// what was previously hand-edited:
//
//	| Name | Skill | Release | What it does |
//	|------|-------|---------|--------------|
//	| [`name`](path/) | [`/pp-name`](cli-skills/pp-name/SKILL.md) | [latest](release-url) | description. |
//
// The descriptive note about generation lives just inside the begin
// marker so anyone viewing rendered markdown sees it before the table.
func renderCatalogTable(entries []RegistryEntry) string {
	var buf strings.Builder
	buf.WriteString("<!-- this section is generated by tools/generate-registry; do not hand-edit -->\n")
	buf.WriteString("| Name | Skill | Release | What it does |\n")
	buf.WriteString("|------|-------|---------|--------------|\n")
	for _, e := range entries {
		fmt.Fprintf(&buf,
			"| [`%s`](%s/) | [`/pp-%s`](cli-skills/pp-%s/SKILL.md) | [latest](%s%s-current) | %s%s |\n",
			e.Name, e.Path, e.Name, e.Name, releaseTagURLBase, e.Name, formatDescription(e.Description), printerSuffix(e),
		)
	}
	return buf.String()
}

// printerSuffix returns the markdown suffix that visibly credits the
// printer in the catalog row's description cell. Renders as
// `<br><sub>Printed by @handle</sub>` when a Printer is set; empty
// otherwise. Folded into the description cell rather than added as a
// new column to avoid widening the existing 4-column table (every
// entry has a description; not every entry has a printer until the
// backfill follow-up issue ships).
func printerSuffix(e RegistryEntry) string {
	if e.Printer == "" {
		return ""
	}
	return fmt.Sprintf("<br><sub>Printed by [@%s](https://github.com/%s)</sub>", e.Printer, e.Printer)
}

// renderCatalogCounts returns the "N CLIs across M categories." line
// that goes between catalog-counts:begin and catalog-counts:end.
// Pluralization handled for the degenerate single-CLI / single-category
// cases so the rendered prose reads correctly at any size.
func renderCatalogCounts(entries []RegistryEntry) string {
	cats := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		cats[e.Category] = struct{}{}
	}
	cliWord := "CLIs"
	if len(entries) == 1 {
		cliWord = "CLI"
	}
	catWord := "categories"
	if len(cats) == 1 {
		catWord = "category"
	}
	return fmt.Sprintf("<!-- this line is generated by tools/generate-registry; do not hand-edit -->\n%d %s across %d %s.",
		len(entries), cliWord, len(cats), catWord)
}

// formatDescription normalizes a description for the table cell:
// trims whitespace, collapses internal newlines (a description can't
// span multiple table rows), and ensures it ends with a period to
// match the historical hand-edited shape of the README catalog.
//
// The newline collapse is deliberately conservative: registry.json
// descriptions today are single lines, but any CLI whose description
// gets a stray newline (e.g., from a multiline YAML scalar in a
// goreleaser brews block) shouldn't break table rendering.
func formatDescription(d string) string {
	d = strings.TrimSpace(d)
	d = strings.ReplaceAll(d, "\r\n", " ")
	d = strings.ReplaceAll(d, "\n", " ")
	if d == "" {
		return ""
	}
	if !strings.HasSuffix(d, ".") && !strings.HasSuffix(d, "!") && !strings.HasSuffix(d, "?") {
		d += "."
	}
	return d
}
