package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildEntry_WithToolsManifest(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, ".printing-press.json"), map[string]any{
		"api_name":              "hackernews",
		"display_name":          "Hacker News",
		"description":           "HN from the terminal.",
		"auth_type":             "none",
		"auth_env_vars":         []string{},
		"mcp_binary":            "hackernews-pp-mcp",
		"mcp_tool_count":        8,
		"mcp_public_tool_count": 6,
		"mcp_ready":             "full",
	})
	writeJSON(t, filepath.Join(dir, "tools-manifest.json"), map[string]any{
		"auth": map[string]any{"type": "none"},
		"tools": []map[string]any{
			{
				"name":        "items_get",
				"description": "Get an item.",
				"params": []map[string]any{
					{"name": "itemId", "type": "string", "location": "path", "required": true},
				},
			},
		},
	})

	entry, err := buildEntry(dir, "media", "hackernews")
	if err != nil {
		t.Fatalf("buildEntry: %v", err)
	}
	if entry == nil {
		t.Fatal("buildEntry returned nil entry; expected populated entry")
	}

	if entry.Name != "hackernews" || entry.API != "Hacker News" || entry.Category != "media" {
		t.Errorf("identity mismatch: got name=%q api=%q category=%q", entry.Name, entry.API, entry.Category)
	}
	if entry.Skill != "pp-hackernews" {
		t.Errorf("skill = %q, want pp-hackernews", entry.Skill)
	}
	wantInstall := "github.com/mvanhorn/printing-press-library/" + filepath.ToSlash(dir) + "/cmd/hackernews-pp-cli"
	if entry.InstallModule != wantInstall {
		t.Errorf("install_module = %q, want %q", entry.InstallModule, wantInstall)
	}
	if entry.Auth.Type != "none" {
		t.Errorf("auth.type = %q, want none", entry.Auth.Type)
	}
	if entry.MCP == nil {
		t.Fatal("mcp block missing for entry with mcp_binary")
	}
	if entry.MCP.Binary != "hackernews-pp-mcp" {
		t.Errorf("mcp.binary = %q, want hackernews-pp-mcp", entry.MCP.Binary)
	}
	if entry.MCP.ToolCount != 8 || entry.MCP.PublicToolCount != 6 {
		t.Errorf("mcp tool counts wrong: tool=%d public=%d", entry.MCP.ToolCount, entry.MCP.PublicToolCount)
	}
	if entry.MCP.MCPReady != "full" {
		t.Errorf("mcp.mcp_ready = %q, want full", entry.MCP.MCPReady)
	}
	// No cmd/<binary>/main.go in the fixture → transports falls back to ["stdio"].
	if len(entry.MCP.Transports) != 1 || entry.MCP.Transports[0] != "stdio" {
		t.Errorf("mcp.transports = %v, want [stdio] when main.go is absent", entry.MCP.Transports)
	}
	if entry.ToolsSource != toolsSourceManifest {
		t.Errorf("tools_source = %q, want %q", entry.ToolsSource, toolsSourceManifest)
	}
	if len(entry.Tools) != 1 {
		t.Fatalf("tools len = %d, want 1", len(entry.Tools))
	}
	var tool map[string]any
	if err := json.Unmarshal(entry.Tools[0], &tool); err != nil {
		t.Fatalf("unmarshal tool: %v", err)
	}
	if tool["name"] != "items_get" {
		t.Errorf("tool.name = %v, want items_get", tool["name"])
	}
}

// CLIs without tools-manifest.json should still produce an entry —
// identity + auth + install path are usable; tools=null signals the
// consumer to call agent-context after install.
func TestBuildEntry_NoToolsManifest(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, ".printing-press.json"), map[string]any{
		"api_name":      "airframe",
		"display_name":  "airframe",
		"description":   "Aircraft forensics.",
		"auth_type":     "none",
		"auth_env_vars": []string{},
	})

	entry, err := buildEntry(dir, "developer-tools", "airframe")
	if err != nil {
		t.Fatalf("buildEntry: %v", err)
	}
	if entry == nil {
		t.Fatal("buildEntry returned nil; expected entry even without manifest")
	}
	if entry.ToolsSource != toolsSourceAgentCtx {
		t.Errorf("tools_source = %q, want %q", entry.ToolsSource, toolsSourceAgentCtx)
	}
	if entry.Tools != nil {
		t.Errorf("tools = %v, want nil", entry.Tools)
	}
	if entry.MCP != nil {
		t.Errorf("mcp = %+v, want nil for CLI without mcp_binary", entry.MCP)
	}
}

// Directories without .printing-press.json should skip cleanly so
// scratch dirs under library/ don't break the walk.
func TestBuildEntry_MissingManifestSkipped(t *testing.T) {
	dir := t.TempDir()
	entry, err := buildEntry(dir, "tools", "scratch")
	if err != nil {
		t.Fatalf("buildEntry on dir without .printing-press.json should not error: %v", err)
	}
	if entry != nil {
		t.Errorf("buildEntry returned %+v, want nil for non-CLI directory", entry)
	}
}

// When .printing-press.json omits auth_type, the tools-manifest.json
// auth.type (runtime-authoritative) should fill in.
func TestBuildEntry_AuthFallback(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, ".printing-press.json"), map[string]any{
		"api_name":    "legacy",
		"description": "Pre-auth_type CLI.",
	})
	writeJSON(t, filepath.Join(dir, "tools-manifest.json"), map[string]any{
		"auth":  map[string]any{"type": "api_key"},
		"tools": []map[string]any{},
	})

	entry, err := buildEntry(dir, "tools", "legacy")
	if err != nil {
		t.Fatalf("buildEntry: %v", err)
	}
	if entry.Auth.Type != "api_key" {
		t.Errorf("auth.type = %q, want api_key (from tools-manifest fallback)", entry.Auth.Type)
	}
}

// When no source declares an auth.type, the entry should default to
// "none" rather than an empty string (consumer contract: always non-empty).
func TestBuildEntry_DefaultsAuthToNone(t *testing.T) {
	dir := t.TempDir()
	writeJSON(t, filepath.Join(dir, ".printing-press.json"), map[string]any{
		"api_name":    "unknown-auth",
		"description": "CLI with no declared auth.",
	})

	entry, err := buildEntry(dir, "tools", "unknown-auth")
	if err != nil {
		t.Fatalf("buildEntry: %v", err)
	}
	if entry.Auth.Type != "none" {
		t.Errorf("auth.type = %q, want none (default)", entry.Auth.Type)
	}
}

func TestDetectMCPTransports(t *testing.T) {
	dir := t.TempDir()
	cmdDir := filepath.Join(dir, "cmd", "x-pp-mcp")
	if err := os.MkdirAll(cmdDir, 0o755); err != nil {
		t.Fatalf("mkdir cmd dir: %v", err)
	}

	// stdio-only main.go (no streamable-HTTP reference).
	stdioOnly := []byte("package main\nfunc main() { ServeStdio() }\n")
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), stdioOnly, 0o644); err != nil {
		t.Fatalf("write stdio-only main.go: %v", err)
	}
	if got := detectMCPTransports(dir, "x-pp-mcp"); !equalSlices(got, []string{"stdio"}) {
		t.Errorf("stdio-only: detectMCPTransports = %v, want [stdio]", got)
	}

	// Same dir, swap in a main.go that references streamable-HTTP.
	stdioPlusHTTP := []byte("package main\nfunc main() { server.NewStreamableHTTPServer(...) }\n")
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), stdioPlusHTTP, 0o644); err != nil {
		t.Fatalf("write dual-transport main.go: %v", err)
	}
	if got := detectMCPTransports(dir, "x-pp-mcp"); !equalSlices(got, []string{"stdio", "http"}) {
		t.Errorf("stdio+http: detectMCPTransports = %v, want [stdio http]", got)
	}

	// Missing binary name short-circuits to the stdio default — guards
	// the CLI-only path (no MCP) without erroring.
	if got := detectMCPTransports(dir, ""); !equalSlices(got, []string{"stdio"}) {
		t.Errorf("empty binary: detectMCPTransports = %v, want [stdio]", got)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Cross-check against generate-registry's helper: both must produce
// the same module path for the same inputs.
func TestInstallModulePath(t *testing.T) {
	got := installModulePath("library/dev/github", "github")
	want := "github.com/mvanhorn/printing-press-library/library/dev/github/cmd/github-pp-cli"
	if got != want {
		t.Errorf("installModulePath = %q, want %q", got, want)
	}
}

func TestMarshalRegistry_StableShape(t *testing.T) {
	reg := AgentRegistry{
		SchemaVersion: 1,
		Entries: []AgentEntry{
			{
				Name:          "x",
				API:           "X",
				Category:      "tools",
				Description:   "Test.",
				InstallModule: "github.com/example/x/cmd/x-pp-cli",
				Skill:         "pp-x",
				Auth:          AuthBlock{Type: "none", EnvVars: []string{}},
				ToolsSource:   toolsSourceAgentCtx,
				Tools:         nil,
			},
		},
	}
	out, err := marshalRegistry(reg)
	if err != nil {
		t.Fatalf("marshalRegistry: %v", err)
	}
	if !strings.HasSuffix(string(out), "\n") {
		t.Error("output must end with a newline")
	}
	if !strings.Contains(string(out), "  \"schema_version\": 1") {
		t.Errorf("expected 2-space indent for schema_version; got:\n%s", out)
	}
	if strings.Contains(string(out), `&gt;`) {
		t.Errorf("HTML escaping leaked into output:\n%s", out)
	}
}

func writeJSON(t *testing.T, path string, body any) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
