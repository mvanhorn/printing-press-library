package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var focusedPlanningTools = []string{
	"venues_search", "venues_get", "cuisines_list", "courses_list",
	"courses_get", "availability_check", "availability_scan", "booking_url",
}

func TestFocusedPlanningMCPKeepsEightReadOnlyMirrors(t *testing.T) {
	s := server.NewMCPServer("tablecheck", "test")
	RegisterTools(s)
	tools := s.ListTools()
	for _, name := range focusedPlanningTools {
		tool, exists := tools[name]
		if !exists {
			t.Fatalf("planning mirror %q missing", name)
		}
		if tool.Tool.Annotations.ReadOnlyHint == nil || !*tool.Tool.Annotations.ReadOnlyHint {
			t.Fatalf("planning mirror %q lost read-only hint", name)
		}
		if tool.Tool.Meta == nil || tool.Tool.Meta.AdditionalFields["pp:tenant-gate"] != "child-cli" {
			t.Fatalf("planning tool %q is not the companion CLI mirror", name)
		}
	}
	for name := range tools {
		for _, unsupported := range []string{"search", "sql", "sync", "import", "export", "workflow"} {
			if name == unsupported || strings.HasPrefix(name, unsupported+"_") {
				t.Fatalf("unsupported generic route remains exposed: %s", name)
			}
		}
	}
}

func TestFocusedPlanningMCPContextDescribesActualBoundsAndWindows(t *testing.T) {
	resetMCPPathEnv(t)
	s := server.NewMCPServer("tablecheck", "test")
	RegisterTools(s)
	result, err := handleContext(s)(context.Background(), mcplib.CallToolRequest{})
	if err != nil || result == nil || result.IsError {
		t.Fatalf("context failed: result=%v error=%v", result, err)
	}
	var payload struct {
		Resources []map[string]any `json:"resources"`
		QueryTips []string         `json:"query_tips"`
	}
	if err := json.Unmarshal([]byte(mcpTextContent(t, result)), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Resources) != 1 || payload.Resources[0]["name"] != "source" || payload.Resources[0]["syncable"] != false || payload.Resources[0]["searchable"] != false {
		t.Fatalf("context advertises unsupported local-data capability: %v", payload.Resources)
	}
	tips := strings.Join(payload.QueryTips, "\n")
	for _, name := range focusedPlanningTools {
		if !strings.Contains(tips, name) {
			t.Fatalf("context omits planning tool %q: %s", name, tips)
		}
	}
	for _, fact := range []string{"limit 10", "maximum 50", "next_cursor", "cursor", "18:00", "time window", "not a full day", "--select", "MCP error"} {
		if !strings.Contains(tips, fact) {
			t.Fatalf("context omits %q: %s", fact, tips)
		}
	}
	for _, obsolete := range []string{"default 100", "sync first", "sql tool", "search tool", "synced"} {
		if strings.Contains(strings.ToLower(tips), obsolete) {
			t.Fatalf("context contains unsupported guidance %q: %s", obsolete, tips)
		}
	}
}
