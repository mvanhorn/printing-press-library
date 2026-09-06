package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestIntentToolsDeclareReadOnlyAnnotations(t *testing.T) {
	s := server.NewMCPServer("test", "test")
	RegisterIntents(s)
	tools := s.ListTools()
	for _, name := range []string{"find_best_award", "explore_from_airport", "program_calendar_scan"} {
		tool, ok := tools[name]
		if !ok {
			t.Fatalf("missing tool %s", name)
		}
		a := tool.Tool.Annotations
		if a.ReadOnlyHint == nil || !*a.ReadOnlyHint || a.DestructiveHint == nil || *a.DestructiveHint || a.IdempotentHint == nil || !*a.IdempotentHint || a.OpenWorldHint == nil || !*a.OpenWorldHint {
			t.Fatalf("%s annotations=%+v", name, a)
		}
	}
	if strings.Contains(tools["find_best_award"].Tool.Description, "trip detail") {
		t.Fatalf("stale description=%q", tools["find_best_award"].Tool.Description)
	}
}

func TestFindBestAwardHandlerSendsDefaultBusinessCabin(t *testing.T) {
	var cabins string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cabins = r.URL.Query().Get("cabins")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()
	t.Setenv("SEATS_AERO_BASE_URL", srv.URL)
	t.Setenv("SEATS_AERO_API_KEY", "test")
	result, err := handleFindBestAward(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"origin": "JFK", "destination": "NRT"}}})
	if err != nil || result == nil || result.IsError || cabins != "business" {
		t.Fatalf("result=%+v err=%v cabins=%q", result, err, cabins)
	}
}

func TestFindBestAwardDefaultsCabinToBusiness(t *testing.T) {
	input := map[string]any{}
	applyFindBestAwardDefaults(input)
	if input["cabin"] != "business" {
		t.Fatalf("cabin=%v, want business", input["cabin"])
	}
}
