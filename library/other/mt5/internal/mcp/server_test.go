package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestLooksLikeSQLWrite(t *testing.T) {
	cases := map[string]bool{
		"SELECT * FROM deals":          false,
		"select 1":                     false,
		"  with cte as (select 1) ...": false,
		"explain query plan select 1":  false,
		"INSERT INTO foo VALUES (1)":   true,
		"insert into foo":              true,
		"UPDATE foo SET":               true,
		"DELETE FROM foo":              true,
		"DROP TABLE foo":               true,
		"ALTER TABLE foo":              true,
		"CREATE INDEX":                 true,
		// PRAGMA is intentionally NOT flagged anymore — read-only PRAGMAs
		// (table_info, schema_version, database_list) are legitimate
		// diagnostics for the agent. Write PRAGMAs fail at the engine on
		// the RO connection mt5_sql uses.
		"  PRAGMA journal_mode = wal": false,
		"pragma table_info(deals)":    false,
		"replace into foo values":     true,
	}
	for in, want := range cases {
		if got := looksLikeSQLWrite(in); got != want {
			t.Errorf("looksLikeSQLWrite(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestExitName(t *testing.T) {
	cases := map[int]string{
		0: "ok", 2: "usage", 3: "not_found", 4: "auth",
		5: "broker_rejected", 6: "safety_rejected", 7: "rate_limited",
		10: "config", 11: "terminal_down", 99: "error",
	}
	for code, want := range cases {
		if got := exitName(code); got != want {
			t.Errorf("exitName(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestAuditStderrWarningFiltersBenignStderr(t *testing.T) {
	if got := auditStderrWarning("python debug line\nbridge trace\n"); got != "" {
		t.Fatalf("benign stderr should be hidden on success, got %q", got)
	}

	in := "python debug line\nAUDIT failed to record confirmed write\n  reconcile from broker history\nlater noise\n"
	want := "AUDIT failed to record confirmed write\n  reconcile from broker history"
	if got := auditStderrWarning(in); got != want {
		t.Fatalf("auditStderrWarning() = %q, want %q", got, want)
	}
}

func TestToolRegistrySizeAndUniqueness(t *testing.T) {
	regs := tools()
	if len(regs) < 15 {
		t.Errorf("expected at least 15 tools, got %d", len(regs))
	}
	seen := map[string]bool{}
	for _, r := range regs {
		if seen[r.tool.Name] {
			t.Errorf("duplicate tool name %q", r.tool.Name)
		}
		seen[r.tool.Name] = true
		if !strings.HasPrefix(r.tool.Name, "mt5_") {
			t.Errorf("tool %q missing mt5_ prefix", r.tool.Name)
		}
		if r.handler == nil {
			t.Errorf("tool %q has nil handler", r.tool.Name)
		}
		if r.tool.Description == "" {
			t.Errorf("tool %q has empty description", r.tool.Name)
		}
	}
}

func TestListToolNamesCoversRegistry(t *testing.T) {
	names := ListToolNames()
	if len(names) != len(tools()) {
		t.Fatalf("ListToolNames: %d != tools(): %d", len(names), len(tools()))
	}
	// Smoke check: well-known tool names are present.
	want := []string{"mt5_doctor", "mt5_quote", "mt5_order_send", "mt5_close_all", "mt5_sql", "mt5_audit_tail"}
	have := map[string]bool{}
	for _, n := range names {
		have[n] = true
	}
	for _, w := range want {
		if !have[w] {
			t.Errorf("expected %q in ListToolNames(), not found", w)
		}
	}
}

func TestMt5SqlBlocksWrites(t *testing.T) {
	// Find the mt5_sql handler.
	var h mcp.Tool
	var fn func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error)
	for _, r := range tools() {
		if r.tool.Name == "mt5_sql" {
			h = r.tool
			fn = r.handler
			break
		}
	}
	if fn == nil {
		t.Fatal("mt5_sql tool not found")
	}
	_ = h

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"query": "INSERT INTO deals VALUES (1)",
	}
	res, err := fn(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned err: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError=true on INSERT")
	}
	// Check the error mentions read-only.
	if len(res.Content) == 0 {
		t.Fatal("error content empty")
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", res.Content[0])
	}
	if !strings.Contains(strings.ToLower(tc.Text), "read-only") {
		t.Errorf("error message %q should mention read-only", tc.Text)
	}
}

func TestArgHelpers(t *testing.T) {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"s":     "value",
		"empty": "",
		"n":     float64(42),
		"b":     true,
	}

	if got := stringArg(req, "s", "def"); got != "value" {
		t.Errorf("stringArg s = %q, want value", got)
	}
	if got := stringArg(req, "empty", "fallback"); got != "fallback" {
		t.Errorf("stringArg empty-string should fall back: got %q", got)
	}
	if got := stringArg(req, "missing", "def"); got != "def" {
		t.Errorf("stringArg missing = %q, want def", got)
	}
	if got, err := requireString(req, "s"); err != nil || got != "value" {
		t.Errorf("requireString s err=%v got=%q", err, got)
	}
	if _, err := requireString(req, "missing"); err == nil {
		t.Error("requireString missing should error")
	}
	if _, err := requireString(req, "empty"); err == nil {
		t.Error("requireString empty-string should error")
	}
	if got := floatArg(req, "n", 0); got != 42 {
		t.Errorf("floatArg n = %v, want 42", got)
	}
	if got := floatArg(req, "missing", 7); got != 7 {
		t.Errorf("floatArg missing default = %v, want 7", got)
	}
	if got, ok := optFloat(req, "n"); !ok || got != 42 {
		t.Errorf("optFloat n = %v, ok=%v", got, ok)
	}
	if _, ok := optFloat(req, "missing"); ok {
		t.Error("optFloat missing should return false")
	}
	if got, err := requireFloat(req, "n"); err != nil || got != 42 {
		t.Errorf("requireFloat n err=%v got=%v", err, got)
	}
	if _, err := requireFloat(req, "s"); err == nil {
		t.Error("requireFloat on string should error")
	}
	if !boolArg(req, "b", false) {
		t.Error("boolArg b should be true")
	}
	if boolArg(req, "missing", false) {
		t.Error("boolArg missing default false")
	}
}
