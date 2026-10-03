package mcp

import (
	"context"
	"encoding/json"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/platform"
	"github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/store"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestActualMCPCacheReadsRejectOpenWALAndHardLinks(t *testing.T) {
	real := filepath.Join(t.TempDir(), "data.db")
	writer, err := store.OpenWithContext(context.Background(), real)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Upsert("planning_snapshot", "base", json.RawMessage(`{"name":"baselineword"}`)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	writer, err = store.OpenWithContext(context.Background(), real)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Upsert("planning_snapshot", "new", json.RawMessage(`{"name":"walneedleword"}`)); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "alias.db")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	run := func(path string, wantError bool) {
		t.Helper()
		ctx := platform.ContextWithSession(context.Background(), &platform.Session{GateOutcome: platform.GateVerified, Paths: platform.Paths{DataFile: path}})
		for _, tc := range []struct {
			f func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error)
			q string
		}{{handleSearch, "walneedleword"}, {handleSQL, "SELECT data FROM resources WHERE resource_type='planning_snapshot' ORDER BY id"}} {
			result, err := tc.f(ctx, mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"query": tc.q}}})
			if err != nil || result == nil || result.IsError != wantError {
				t.Fatalf("MCP error=%v result=%#v", err, result)
			}
			text := result.Content[0].(mcplib.TextContent).Text
			if wantError && !strings.Contains(text, "cache_visibility_unavailable") {
				t.Fatal("unsafe MCP read returned another result", text)
			}
			if !wantError && !strings.Contains(text, "walneedleword") {
				t.Fatal("MCP missed latest committed observation", text)
			}
		}
	}
	run(real, true)
	run(alias, true)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	run(real, false)
	run(alias, false)
	hard := filepath.Join(t.TempDir(), "hard.db")
	if err := os.Link(real, hard); err != nil {
		t.Fatal(err)
	}
	run(real, true)
	run(hard, true)
}

func TestActualMCPRejectsPercentDecodedCacheAlias(t *testing.T) {
	selected := filepath.Join(t.TempDir(), "target%41.db")
	// Empty initialized source is sufficient: no encoded sibling may be opened.
	db, err := store.OpenWithContext(context.Background(), selected)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	ctx := platform.ContextWithSession(context.Background(), &platform.Session{GateOutcome: platform.GateVerified, Paths: platform.Paths{DataFile: selected}})
	for _, handler := range []func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error){handleSearch, handleSQL} {
		result, err := handler(ctx, mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"query": "SELECT data FROM resources"}}})
		if err != nil || result == nil || !result.IsError || !strings.Contains(result.Content[0].(mcplib.TextContent).Text, "cache_visibility_unavailable") {
			t.Fatal("MCP percent alias was allowed", err, result)
		}
	}
}
