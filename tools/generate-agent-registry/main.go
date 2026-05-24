// generate-agent-registry emits agent-registry.json — the agent-facing
// counterpart to registry.json that inlines each CLI's tools-manifest
// so consumers don't fan out N+1 HTTP requests to assemble the catalog.
//
// Per-entry sources: library/<cat>/<slug>/.printing-press.json (identity,
// install, auth, MCP) and library/<cat>/<slug>/tools-manifest.json (the
// tool list, passed through verbatim). CLIs without a tools-manifest
// emit tools=null with tools_source="agent-context"; consumers capture
// the live tree via `<binary> agent-context` after install.
//
// Usage:
//
//	go run ./tools/generate-agent-registry             # write agent-registry.json
//	go run ./tools/generate-agent-registry --check     # exit non-zero if drift detected
//	go run ./tools/generate-agent-registry --print     # print to stdout, do not write
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
)

const (
	libraryDir          = "library"
	outputPath          = "agent-registry.json"
	schemaVersion       = 1
	toolsSourceManifest = "tools-manifest"
	toolsSourceAgentCtx = "agent-context"
)

type AgentRegistry struct {
	SchemaVersion int          `json:"schema_version"`
	Entries       []AgentEntry `json:"entries"`
}

type AgentEntry struct {
	Name          string    `json:"name"`
	API           string    `json:"api"`
	Category      string    `json:"category"`
	Description   string    `json:"description"`
	InstallModule string    `json:"install_module"`
	Skill         string    `json:"skill"`
	Auth          AuthBlock `json:"auth"`
	MCP           *MCPBlock `json:"mcp,omitempty"`
	// ToolsSource is "tools-manifest" when Tools is the verbatim
	// upstream array, "agent-context" when Tools is null and consumers
	// must capture the live tree via `<binary> agent-context`.
	ToolsSource string            `json:"tools_source"`
	Tools       []json.RawMessage `json:"tools"`
}

type AuthBlock struct {
	// Consumers should treat unknown Type values as "credentials
	// required" rather than hard-failing — new auth types may land
	// before consumer updates.
	Type    string   `json:"type"`
	EnvVars []string `json:"env_vars"`
}

type MCPBlock struct {
	Binary          string   `json:"binary"`
	Transports      []string `json:"transports"`
	ToolCount       int      `json:"tool_count"`
	PublicToolCount int      `json:"public_tool_count"`
	EnvVars         []string `json:"env_vars"`
	MCPReady        string   `json:"mcp_ready,omitempty"`
}

type printingPressManifest struct {
	APIName            string   `json:"api_name"`
	DisplayName        string   `json:"display_name"`
	Description        string   `json:"description"`
	AuthType           string   `json:"auth_type"`
	AuthEnvVars        []string `json:"auth_env_vars"`
	MCPBinary          string   `json:"mcp_binary"`
	MCPToolCount       int      `json:"mcp_tool_count"`
	MCPPublicToolCount *int     `json:"mcp_public_tool_count"`
	MCPReady           string   `json:"mcp_ready"`
}

// toolsManifest is the read-subset of tools-manifest.json: the auth
// block (runtime-authoritative when .printing-press.json's auth_type
// is empty) and the tools array (passed through as json.RawMessage
// so we don't strip fields the manifest carries that we don't model).
type toolsManifest struct {
	Auth  *toolsManifestAuth `json:"auth"`
	Tools []json.RawMessage  `json:"tools"`
}

type toolsManifestAuth struct {
	Type string `json:"type"`
}

func main() {
	check := flag.Bool("check", false, "exit non-zero if generated output differs from on-disk agent-registry.json")
	printOnly := flag.Bool("print", false, "print generated registry to stdout instead of writing")
	flag.Parse()

	entries, err := buildEntries(libraryDir)
	if err != nil {
		log.Fatalf("building entries: %v", err)
	}

	registry := AgentRegistry{
		SchemaVersion: schemaVersion,
		Entries:       entries,
	}

	out, err := marshalRegistry(registry)
	if err != nil {
		log.Fatalf("marshaling: %v", err)
	}

	if *printOnly {
		if _, err := os.Stdout.Write(out); err != nil {
			log.Fatalf("writing to stdout: %v", err)
		}
		return
	}

	if *check {
		current, err := os.ReadFile(outputPath)
		if err != nil {
			log.Fatalf("reading %s for check: %v", outputPath, err)
		}
		if !bytes.Equal(current, out) {
			fmt.Fprintf(os.Stderr, "drift detected in %s\nRun `go run ./tools/generate-agent-registry` and commit the result.\n", outputPath)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "%s is in sync with library/\n", outputPath)
		return
	}

	if err := os.WriteFile(outputPath, out, 0o644); err != nil {
		log.Fatalf("writing %s: %v", outputPath, err)
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d entries)\n", outputPath, len(entries))
}

func buildEntries(root string) ([]AgentEntry, error) {
	categories, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("reading library dir: %w", err)
	}
	var entries []AgentEntry
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
			entry, err := buildEntry(cliDir, cat.Name(), slug.Name())
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

// buildEntry returns (nil, nil) when the directory carries no
// .printing-press.json — the gate for "is this an actual CLI directory?"
// that lets unrelated scratch dirs coexist under library/.
func buildEntry(dir, category, slug string) (*AgentEntry, error) {
	pp, err := readPrintingPressManifest(filepath.Join(dir, ".printing-press.json"))
	if err != nil {
		return nil, err
	}
	if pp == nil {
		return nil, nil
	}
	tm, err := readToolsManifest(filepath.Join(dir, "tools-manifest.json"))
	if err != nil {
		return nil, err
	}

	pathSlash := filepath.ToSlash(dir)
	entry := AgentEntry{
		Name:          slug,
		API:           pickAPIName(pp, slug),
		Category:      category,
		Description:   pp.Description,
		InstallModule: installModulePath(pathSlash, slug),
		Skill:         "pp-" + slug,
		Auth: AuthBlock{
			Type:    pickAuthType(pp, tm),
			EnvVars: nonNilStrings(pp.AuthEnvVars),
		},
	}

	if pp.MCPBinary != "" {
		// EnvVars sources from pp.AuthEnvVars: by convention the MCP
		// server and the CLI share credentials, matching the source
		// generate-registry uses for registry.json's mcp.env_vars.
		mcp := &MCPBlock{
			Binary:     pp.MCPBinary,
			Transports: detectMCPTransports(dir, pp.MCPBinary),
			ToolCount:  pp.MCPToolCount,
			EnvVars:    nonNilStrings(pp.AuthEnvVars),
			MCPReady:   pp.MCPReady,
		}
		if pp.MCPPublicToolCount != nil {
			mcp.PublicToolCount = *pp.MCPPublicToolCount
		}
		entry.MCP = mcp
	}

	if tm != nil {
		entry.ToolsSource = toolsSourceManifest
		// A manifest with a null/absent tools field still means
		// "manifest-sourced": emit [] not null, so tools:null stays
		// reserved for the agent-context case (the README invariant).
		entry.Tools = nonNilRawMessages(tm.Tools)
	} else {
		entry.ToolsSource = toolsSourceAgentCtx
	}

	return &entry, nil
}

// pickAuthType resolves the auth.type field through three tiers:
// the press manifest (authoritative when populated), the tools-manifest
// (runtime-authoritative — what the binary actually consumes), and
// finally "none" so consumers see a stable non-empty string.
func pickAuthType(pp *printingPressManifest, tm *toolsManifest) string {
	if pp.AuthType != "" {
		return pp.AuthType
	}
	if tm != nil && tm.Auth != nil && tm.Auth.Type != "" {
		return tm.Auth.Type
	}
	return "none"
}

// readPrintingPressManifest returns (nil, nil) when the file is absent;
// any other read or parse error is fatal.
func readPrintingPressManifest(path string) (*printingPressManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var pp printingPressManifest
	if err := json.Unmarshal(data, &pp); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &pp, nil
}

// readToolsManifest returns (nil, nil) when tools-manifest.json is
// absent (docs-driven / sniff-driven CLIs ship none).
func readToolsManifest(path string) (*toolsManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	var tm toolsManifest
	if err := json.Unmarshal(data, &tm); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return &tm, nil
}

// pickAPIName resolves the api display name: display_name > api_name > slug.
// Reproducible from sources only — unlike generate-registry's equivalent,
// we don't curate from a prior agent-registry.json.
func pickAPIName(pp *printingPressManifest, slug string) string {
	if pp.DisplayName != "" {
		return pp.DisplayName
	}
	if pp.APIName != "" {
		return pp.APIName
	}
	return slug
}

// detectMCPTransports source-greps the MCP binary's main.go: stdio is
// always linked; streamable-HTTP is reported only when
// NewStreamableHTTPServer is referenced. Kept in lockstep with
// generate-registry's identical helper — the two tools live in
// separate modules and don't share code yet.
func detectMCPTransports(cliDir, binary string) []string {
	transports := []string{"stdio"}
	if binary == "" {
		return transports
	}
	mainPath := filepath.Join(cliDir, "cmd", binary, "main.go")
	data, err := os.ReadFile(mainPath)
	if err != nil {
		return transports
	}
	if bytes.Contains(data, []byte("NewStreamableHTTPServer")) {
		transports = append(transports, "http")
	}
	return transports
}

// installModulePath duplicates generate-registry's helper; see the note
// on detectMCPTransports above.
func installModulePath(path, slug string) string {
	if path == "" || slug == "" {
		return ""
	}
	return fmt.Sprintf("github.com/mvanhorn/printing-press-library/%s/cmd/%s-pp-cli", path, slug)
}

// nonNilStrings ensures JSON encodes empty arrays as [] not null.
func nonNilStrings(src []string) []string {
	if src == nil {
		return []string{}
	}
	return src
}

// nonNilRawMessages is nonNilStrings for the tools array — keeps
// manifest-sourced entries emitting [] rather than null.
func nonNilRawMessages(src []json.RawMessage) []json.RawMessage {
	if src == nil {
		return []json.RawMessage{}
	}
	return src
}

// marshalRegistry produces the canonical encoding: 2-space indent, no
// HTML escaping, trailing newline.
func marshalRegistry(r AgentRegistry) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
