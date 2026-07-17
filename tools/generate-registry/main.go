// generate-registry walks library/<category>/<slug>/ and emits the
// top-level registry.json from each CLI's .printing-press.json,
// manifest.json, and .goreleaser.yaml. Existing registry.json values are
// preserved as legacy curated catalog copy unless a CLI explicitly opts into
// source-authored catalog copy with .printing-press.json catalog_description.
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
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	API         string   `json:"api"`
	Description string   `json:"description"`
	SearchTerms []string `json:"search_terms,omitempty"`
	Path        string   `json:"path"`
	Release     *Release `json:"release,omitempty"`
	sourceAPI   string
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
	PrinterName string `json:"printer_name,omitempty"`
	// Creator is the permanent original author (handle + display name),
	// sourced from .printing-press.json's `creator` object. It supersedes
	// the legacy printer/printer_name fields, which are still emitted during
	// the dual-write transition window so older consumers keep working.
	Creator *Person `json:"creator,omitempty"`
	// Contributors accrue as others improve a CLI (reprinter first), sourced
	// from .printing-press.json's `contributors` array.
	Contributors []Person  `json:"contributors,omitempty"`
	MCP          *MCPBlock `json:"mcp,omitempty"`
}

// Person is one credited human (creator or contributor): a slug-safe GitHub
// @handle plus a prose display name. Mirrors the generator's spec.Person and
// the .printing-press.json shape.
type Person struct {
	Handle string `json:"handle,omitempty"`
	Name   string `json:"name,omitempty"`
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

// Release is the catalog-facing subset of a CLI's
// .printing-press-release.json. It lets search/list JSON consumers compare an
// installed binary's --version output with the current catalog version without
// falling back to go module build metadata or repo inspection.
type Release struct {
	CLIName      string `json:"cli_name"`
	Version      string `json:"version"`
	ReleasedAt   string `json:"released_at"`
	SourceCommit string `json:"source_commit"`
}

// printingPressManifest captures the subset of .printing-press.json fields
// the registry needs. The on-disk shape carries many other fields
// (scorecard_total, run_id, etc.); we ignore them so a future generator
// version that adds fields doesn't break this consumer.
type printingPressManifest struct {
	APIName            string   `json:"api_name"`
	DisplayName        string   `json:"display_name"`
	CatalogDisplayName string   `json:"catalog_display_name"`
	CatalogDescription string   `json:"catalog_description"`
	Description        string   `json:"description"`
	Creator            *Person  `json:"creator"`
	Contributors       []Person `json:"contributors"`
	Printer            string   `json:"printer"`
	PrinterName        string   `json:"printer_name"`
	CLIName            string   `json:"cli_name"`
	AuthDescription    string   `json:"auth_description"`
	MCPBinary          string   `json:"mcp_binary"`
	MCPToolCount       int      `json:"mcp_tool_count"`
	MCPPublicToolCount *int     `json:"mcp_public_tool_count"`
	MCPReady           string   `json:"mcp_ready"`
	AuthType           string   `json:"auth_type"`
	AuthEnvVars        []string `json:"auth_env_vars"`
	SpecFormat         string   `json:"spec_format"`
	NovelFeatures      []struct {
		Name        string `json:"name"`
		Command     string `json:"command"`
		Description string `json:"description"`
		Rationale   string `json:"rationale"`
	} `json:"novel_features"`
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
	validate := flag.Bool("validate", false, "exit non-zero if any entry would have an empty required field or duplicate display label after fallback resolution (sources only — ignores prior registry.json curated values). Designed for the PR-time CI gate.")
	flag.Parse()

	// --validate runs before the normal flow so it never depends on the
	// current on-disk registry.json. It builds entries from sources alone
	// (empty existing map) and fails when any required field would land
	// empty — catching the lawhub-shape regression where a curated value
	// in registry.json masks a missing source description.
	//
	// Positional args restrict validation to a set of CLI slugs. The
	// PR-time CI gate passes the slugs whose registry-source files the PR
	// actually adds or modifies, so a stale PR is never failed for an
	// unrelated CLI that is already correct on main (merging the PR won't
	// change that CLI). With no args, every entry is validated — the mode
	// the post-merge full-library check uses.
	if *validate {
		sourceEntries, err := buildEntries(libraryDir, map[string]RegistryEntry{})
		if err != nil {
			log.Fatalf("building entries for validation: %v", err)
		}
		allSourceEntries := sourceEntries
		restrict := flag.Args()
		if len(restrict) > 0 {
			sourceEntries = filterEntriesBySlug(sourceEntries, restrict)
		}
		errs := validateEntries(sourceEntries)
		errs = append(errs, validateUniqueAPIDisplayNames(allSourceEntries, restrict)...)
		if len(errs) > 0 {
			fmt.Fprintln(os.Stderr, "Registry validation failed:")
			for _, e := range errs {
				fmt.Fprintln(os.Stderr, "  - "+e)
			}
			fmt.Fprintln(os.Stderr, "\nFix the source files:")
			fmt.Fprintln(os.Stderr, "  - description: populate .printing-press.json's `description` or the `.goreleaser.yaml` brews `description` for the affected CLI(s).")
			fmt.Fprintln(os.Stderr, "  - api:         give each CLI a unique .printing-press.json `display_name` so catalog labels do not collide.")
			fmt.Fprintln(os.Stderr, "  - mcp.*:       populate .printing-press.json's `mcp_binary`, `auth_type`, and related fields for any CLI advertising an MCP block.")
			os.Exit(2)
		}
		fmt.Fprintf(os.Stderr, "Registry validation passed (%d entries).\n", len(sourceEntries))
		return
	}

	existing := loadExistingEntries(registryPath)

	entries, err := buildEntries(libraryDir, existing)
	if err != nil {
		log.Fatalf("building entries: %v", err)
	}
	if errs := validateUniqueAPIDisplayNames(entries, nil); len(errs) > 0 {
		fmt.Fprintln(os.Stderr, "Registry generation failed:")
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, "  - "+e)
		}
		fmt.Fprintln(os.Stderr, "\nFix .printing-press.json `display_name` values so catalog labels do not collide.")
		os.Exit(2)
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
// can't yet be reliably derived from disk:
//
//   - description: legacy fallback only when source metadata has no
//     description yet. Modern per-CLI metadata is authoritative to avoid
//     generated registry.json accumulating hand-maintained drift.
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
	repairDuplicateAPIDisplayNames(entries)
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
		sourceAPI: strings.TrimSpace(firstNonEmpty(
			pp.CatalogDisplayName,
			pp.DisplayName,
			pp.APIName,
			slug,
		)),
		// Printer attribution: always derive from the manifest. Do not
		// honor a curated prior.Printer value — the manifest is the
		// only source of truth, and a curated map would re-introduce
		// the multi-author retrofit footgun the cliAuthorByAPIName map
		// in tools/sweep-canonical/ exists to manage carefully.
		// Existing CLIs without a printer field in their manifest will
		// emit registry entries with omitempty (no printer key).
		Printer:     pp.Printer,
		PrinterName: pp.PrinterName,
		// Creator + contributors: same manifest-is-source-of-truth rule.
		// Emitted alongside the legacy printer fields during the dual-write
		// transition; both come straight from .printing-press.json.
		Creator:      pp.Creator,
		Contributors: pp.Contributors,
	}

	// Description preference: explicit .printing-press.json catalog_description
	// (modern per-CLI catalog copy) > existing registry value (legacy curated
	// copy) > .printing-press.json / goreleaser fallback for entries that never
	// had curated registry copy. This lets individual CLIs move website copy into
	// source metadata without accidentally rewriting the whole catalog.
	entry.Description = registryDescription(
		prior.Description,
		readGoreleaserDescription(filepath.Join(dir, ".goreleaser.yaml")),
		pp.Description,
		pp.CatalogDescription,
	)
	entry.SearchTerms = searchTerms(pp)
	if release, err := readRelease(filepath.Join(dir, ".printing-press-release.json")); err != nil {
		return nil, err
	} else if release != nil && !isUnreleasedSkeleton(release) {
		// Skip an unreleased skeleton (version/released_at/source_commit all
		// blank, before the post-merge release workflow stamps them). Emitting
		// it would put a release block with empty required fields into
		// registry.json, which the npm installer's parseRegistryEntry rejects as
		// malformed — skipping the whole CLI. Omitting it keeps the generated
		// registry, the --validate gate, and the npm parser consistent: a
		// release block is present only once the CLI is actually released.
		entry.Release = release
	}

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

// isUnreleasedSkeleton reports whether a release ledger is a well-formed
// freshly-printed skeleton not yet stamped by the post-merge release workflow:
// cli_name is present (written at print time) while version, released_at, and
// source_commit are all blank. source_commit is the merge commit and cannot
// exist while a publish PR is open, so an unreleased skeleton is the normal
// pre-merge state and is treated as "no release block" for the catalog — the
// registry omits it rather than emitting empty required fields that the npm
// installer's parseRegistryEntry would reject.
//
// cli_name is required for the skeleton classification so a malformed ledger
// with a blank cli_name is NOT silently omitted: it stays a non-nil release
// block and validateEntries still flags the empty cli_name, preserving the
// pre-existing gate against a printer-workflow misfire.
func isUnreleasedSkeleton(r *Release) bool {
	return r != nil &&
		strings.TrimSpace(r.CLIName) != "" &&
		strings.TrimSpace(r.Version) == "" &&
		strings.TrimSpace(r.ReleasedAt) == "" &&
		strings.TrimSpace(r.SourceCommit) == ""
}

func readRelease(path string) (*Release, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var release Release
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &release, nil
}

func repairDuplicateAPIDisplayNames(entries []RegistryEntry) {
	groups := make(map[string][]int)
	for i, entry := range entries {
		label := strings.TrimSpace(entry.API)
		if label == "" {
			continue
		}
		groups[strings.ToLower(label)] = append(groups[strings.ToLower(label)], i)
	}

	for _, indexes := range groups {
		if len(indexes) < 2 {
			continue
		}
		for _, i := range indexes {
			source := strings.TrimSpace(entries[i].sourceAPI)
			if source != "" && !strings.EqualFold(source, strings.TrimSpace(entries[i].API)) {
				entries[i].API = source
			}
		}
	}
}

// registryDescription picks the final description for a registry entry. Explicit
// per-CLI catalog_description wins, then prior registry copy, then
// source fallbacks. The explicit field is the migration path away from
// hand-curated registry copy without broad catalog churn.
// Bare Markdown headings from the legacy "# Introduction" bug are not treated
// as valid copy; everything else is preserved until a CLI opts in explicitly.
func registryDescription(prior, goreleaser, ppDescription, catalogDescription string) string {
	if catalogDescription != "" {
		return catalogDescription
	}
	if prior != "" && !isBareMarkdownHeading(prior) {
		return prior
	}
	if source := firstNonEmpty(ppDescription, goreleaser); source != "" {
		return source
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func searchTerms(pp printingPressManifest) []string {
	var terms []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			terms = append(terms, value)
		}
	}

	add(pp.APIName)
	add(pp.DisplayName)
	add(pp.CatalogDisplayName)
	add(pp.CLIName)
	add(pp.Description)
	add(pp.AuthDescription)
	for _, feature := range pp.NovelFeatures {
		add(feature.Name)
		add(feature.Command)
		add(feature.Description)
		add(feature.Rationale)
	}
	return dedupeStrings(terms)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	var out []string
	for _, value := range values {
		key := strings.ToLower(strings.TrimSpace(value))
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}

// validateEntries returns one human-readable error per missing required
// field across the given entries. The required-field set mirrors the npm
// installer's parseRegistry contract (every field it calls requiredString
// on, plus the MCP-block fields it calls requiredString / requiredStringArray
// on when the mcp object is present). A registry whose generation passes this
// check round-trips through the npm parser without per-entry errors.
//
// Returns an empty slice when every entry validates. Caller decides how to
// surface the result (the --validate flag prints them and exits 2).
func validateEntries(entries []RegistryEntry) []string {
	// isBlank matches the npm installer's requiredString semantics
	// (`.trim() === ""`). Using == "" here would let an all-whitespace
	// value pass validation but still throw inside parseRegistryEntry,
	// defeating the gate. Centralizing the check keeps the Go and TS
	// acceptance criteria byte-for-byte aligned.
	isBlank := func(s string) bool {
		return strings.TrimSpace(s) == ""
	}

	var errs []string
	for _, e := range entries {
		slug := strings.TrimSpace(e.Name)
		if slug == "" {
			slug = "(unnamed)"
		}
		if isBlank(e.Name) {
			errs = append(errs, fmt.Sprintf("%s: name is empty", slug))
		}
		if isBlank(e.Category) {
			errs = append(errs, fmt.Sprintf("%s: category is empty", slug))
		}
		if isBlank(e.API) {
			errs = append(errs, fmt.Sprintf("%s: api is empty", slug))
		}
		if isBlank(e.Path) {
			errs = append(errs, fmt.Sprintf("%s: path is empty", slug))
		}
		if isBlank(e.Description) {
			// Source order mirrors the resolution chain in registryDescription:
			// .printing-press.json description is checked before goreleaser brews.
			errs = append(errs, fmt.Sprintf("%s: description is empty (sources checked: .printing-press.json description, .goreleaser.yaml brews description)", slug))
		}
		if e.MCP != nil {
			if isBlank(e.MCP.Binary) {
				errs = append(errs, fmt.Sprintf("%s: mcp.binary is empty", slug))
			}
			if len(e.MCP.Transports) == 0 {
				errs = append(errs, fmt.Sprintf("%s: mcp.transports is empty", slug))
			}
			if isBlank(e.MCP.AuthType) {
				errs = append(errs, fmt.Sprintf("%s: mcp.auth_type is empty", slug))
			}
		}
		if e.Release != nil {
			if isBlank(e.Release.CLIName) {
				errs = append(errs, fmt.Sprintf("%s: release.cli_name is empty", slug))
			}
			if isBlank(e.Release.Version) {
				errs = append(errs, fmt.Sprintf("%s: release.version is empty", slug))
			}
			if isBlank(e.Release.ReleasedAt) {
				errs = append(errs, fmt.Sprintf("%s: release.released_at is empty", slug))
			}
			if isBlank(e.Release.SourceCommit) {
				errs = append(errs, fmt.Sprintf("%s: release.source_commit is empty", slug))
			}
		}
	}
	return errs
}

// validateUniqueAPIDisplayNames rejects duplicate human-facing catalog labels.
// When scopedSlugs is non-empty, it reports only duplicate groups involving at
// least one scoped entry; this lets PR-time validation catch a touched CLI that
// collides with an unchanged sibling without making unrelated baseline issues
// block stale branches.
func validateUniqueAPIDisplayNames(entries []RegistryEntry, scopedSlugs []string) []string {
	scoped := make(map[string]bool, len(scopedSlugs))
	for _, slug := range scopedSlugs {
		scoped[strings.TrimSpace(slug)] = true
	}

	type apiGroup struct {
		label string
		names []string
	}

	groups := make(map[string]*apiGroup)
	for _, e := range entries {
		label := strings.TrimSpace(e.API)
		if label == "" {
			continue
		}
		key := strings.ToLower(label)
		if groups[key] == nil {
			groups[key] = &apiGroup{label: label}
		}
		groups[key].names = append(groups[key].names, e.Name)
	}

	var errs []string
	for _, group := range groups {
		if len(group.names) < 2 {
			continue
		}
		if len(scoped) > 0 {
			inScope := false
			for _, name := range group.names {
				if scoped[name] {
					inScope = true
					break
				}
			}
			if !inScope {
				continue
			}
		}
		sort.Strings(group.names)
		errs = append(errs, fmt.Sprintf("api display name %q is used by multiple entries: %s", group.label, strings.Join(group.names, ", ")))
	}
	sort.Strings(errs)
	return errs
}

// filterEntriesBySlug returns the subset of entries whose Name is in slugs.
// Used by --validate to scope the PR-time gate to the CLIs a PR actually
// touched: validation runs against the PR's whole tree, but a stale PR that
// doesn't modify an already-correct CLI shouldn't be failed for it. Entry
// Name is the directory basename (see buildEntries), which matches the slug
// a caller derives from a changed library/<cat>/<slug>/ path. Slugs that
// match no entry are ignored — they describe a deleted or renamed CLI, which
// has nothing left to validate.
func filterEntriesBySlug(entries []RegistryEntry, slugs []string) []RegistryEntry {
	want := make(map[string]bool, len(slugs))
	for _, s := range slugs {
		want[strings.TrimSpace(s)] = true
	}
	var out []RegistryEntry
	for _, e := range entries {
		if want[e.Name] {
			out = append(out, e)
		}
	}
	return out
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
//  1. .printing-press.json's catalog_display_name when explicitly set. This
//     is the source-authored catalog label and intentionally supersedes a
//     previously curated registry value.
//  2. .printing-press.json's display_name when the prior registry value
//     matches a known stale generated shape: bare slug echo, naive title-cased
//     slug ("Setlist Fm"), a long description accidentally stored in `api`, or
//     a generic suffix such as "Pricebook" when the manifest has the parent
//     product ("ServiceTitan Pricebook").
//  3. The current registry.json's existing `api` value, when it differs
//     from the slug — registry api values are hand-curated (e.g.,
//     "PokéAPI", "Cal.com", "Product Hunt") and frequently better than
//     what .printing-press.json's display_name auto-derives. Treating
//     prior == slug as "not curated" lets the generator replace bare
//     slug echoes with a proper display name when one shows up.
//  4. .printing-press.json's display_name (modern-generator best guess).
//  5. .printing-press.json's api_name (machine slug fallback).
//  6. The slug itself, last resort.
//
// Choosing prior over pp.DisplayName here is deliberate. Several
// existing registry entries have curated names (PokéAPI, Product Hunt)
// that pp's auto-derivation produces less faithfully (Pokeapi,
// Producthunt). The cost is: when a CLI's display_name *is* improved
// upstream, the registry won't pick it up automatically — but the
// curated value also won't regress. A future cleanup could lift
// curated api values back into .printing-press.json explicitly.
func apiDisplayName(pp printingPressManifest, prior RegistryEntry, slug string) string {
	if label := strings.TrimSpace(pp.CatalogDisplayName); label != "" {
		return label
	}
	if pp.DisplayName != "" && isStaleAPIValue(prior.API, pp.DisplayName, slug) {
		return pp.DisplayName
	}
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

func isStaleAPIValue(prior, displayName, slug string) bool {
	prior = strings.TrimSpace(prior)
	displayName = strings.TrimSpace(displayName)
	if prior == "" || prior == slug || displayName == "" || prior == displayName {
		return false
	}
	if len(prior) > 80 {
		return true
	}
	if prior == titleCaseSlug(slug) {
		return true
	}
	// A "PP "-prefixed prior is a printing-press infix artifact when the
	// prior is exactly "PP " + the tail of the corrected display_name
	// (e.g. prior="PP Clarity", display="Microsoft Clarity", tail="Clarity").
	// Scoping to that structural match supersedes the leaked prefix while
	// leaving a legitimately curated brand that merely starts with "PP "
	// (e.g. "PP Labs") untouched when the display_name doesn't share its tail.
	if strings.HasPrefix(prior, "PP ") && !strings.HasPrefix(displayName, "PP ") &&
		strings.HasSuffix(displayName, strings.TrimPrefix(prior, "PP ")) {
		return true
	}
	if strings.HasSuffix(displayName, " "+prior) && !strings.Contains(prior, " ") {
		return true
	}
	return false
}

func titleCaseSlug(slug string) string {
	parts := strings.FieldsFunc(slug, func(r rune) bool {
		return r == '-' || r == '_' || r == ' '
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		runes := []rune(part)
		parts[i] = strings.ToUpper(string(runes[:1])) + strings.ToLower(string(runes[1:]))
	}
	return strings.Join(parts, " ")
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
// printer in the catalog row's description cell. Renders the prose
// display name when one is set and links it to the GitHub handle stored
// in Printer; falls back to `@handle` when no display name is present.
// Folded into the description cell rather than added as a new column to
// avoid widening the existing 4-column table (every entry has a
// description; not every entry has a printer until the backfill
// follow-up issue ships).
func printerSuffix(e RegistryEntry) string {
	if e.Printer == "" {
		return ""
	}
	label := strings.TrimSpace(e.PrinterName)
	if label == "" {
		label = "@" + e.Printer
	}
	return fmt.Sprintf("<br><sub>Printed by [%s](https://github.com/%s)</sub>", label, e.Printer)
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
