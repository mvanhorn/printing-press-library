// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/japan-theme-parks/internal/store"
)

// tools-manifest.json is what diagnostic tools read instead of the binary,
// so every manifest tool must match the runtime tools/list entry, including
// parameter descriptions (a compacted description once lost "285 Legoland").
func TestToolsManifestMatchesRuntime(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "tools-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			Params      []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Required    bool   `json:"required"`
			} `json:"params"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	s := server.NewMCPServer("japan-theme-parks", "test")
	RegisterTools(s)
	if len(manifest.Tools) == 0 {
		t.Fatal("manifest lists no tools")
	}
	for _, mt := range manifest.Tools {
		rt := s.GetTool(mt.Name)
		if rt == nil {
			t.Errorf("manifest tool %q is not registered at runtime", mt.Name)
			continue
		}
		if rt.Tool.Description != mt.Description {
			t.Errorf("%s description:\n manifest %q\n runtime  %q", mt.Name, mt.Description, rt.Tool.Description)
		}
		if len(rt.Tool.InputSchema.Properties) != len(mt.Params) {
			t.Errorf("%s: runtime has %d params, manifest %d", mt.Name, len(rt.Tool.InputSchema.Properties), len(mt.Params))
		}
		for _, p := range mt.Params {
			prop, ok := rt.Tool.InputSchema.Properties[p.Name].(map[string]any)
			if !ok {
				t.Errorf("%s: manifest param %q missing at runtime", mt.Name, p.Name)
				continue
			}
			if got, _ := prop["description"].(string); got != p.Description {
				t.Errorf("%s.%s description:\n manifest %q\n runtime  %q", mt.Name, p.Name, p.Description, got)
			}
			if strings.Count(p.Description, "(") != strings.Count(p.Description, ")") {
				t.Errorf("%s.%s description has unbalanced parentheses: %q", mt.Name, p.Name, p.Description)
			}
		}
	}
}

// This CLI has no sync or search command. The agent-facing sql and context
// text must not send agents to tools that do not exist.
func TestMCPTextNamesOnlyRegisteredTools(t *testing.T) {
	s := server.NewMCPServer("japan-theme-parks", "test")
	RegisterTools(s)
	for _, name := range []string{"sync", "search"} {
		if s.GetTool(name) != nil {
			t.Skipf("%s tool is registered; this guard no longer applies", name)
		}
	}
	for _, name := range []string{"snapshot", "typical", "waits", "sql"} {
		if s.GetTool(name) == nil {
			t.Fatalf("tool %q referenced by MCP text is not registered", name)
		}
	}
	sqlTool := s.GetTool("sql").Tool
	texts := []string{sqlTool.Description, mcpEmptyStoreNextStep()}
	if q, ok := sqlTool.InputSchema.Properties["query"].(map[string]any); ok {
		d, _ := q["description"].(string)
		texts = append(texts, d)
	}
	result, err := handleContext(s)(context.Background(), mcplib.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		QueryTips []string `json:"query_tips"`
	}
	if err := json.Unmarshal([]byte(mcpTextContent(t, result)), &payload); err != nil {
		t.Fatal(err)
	}
	texts = append(texts, payload.QueryTips...)
	for _, text := range texts {
		low := strings.ToLower(text)
		for _, bad := range []string{"run sync", "requires sync", "search tool", "cursor-based paging"} {
			if strings.Contains(low, bad) {
				t.Errorf("MCP text refers to a missing capability (%q): %q", bad, text)
			}
		}
	}
	if !strings.Contains(texts[2], "wait_snapshots") {
		t.Errorf("sql query description does not name wait_snapshots: %q", texts[2])
	}
}

// Rows recorded by snapshot live in wait_snapshots, not resources, so a
// store holding only wait history must not report store_status "empty".
func TestMCPStoreStatusCountsWaitHistory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if st, err := mcpStoreStatus(db); err != nil || st != mcpStoreStatusEmpty {
		t.Fatalf("fresh store = %q, %v; want empty", st, err)
	}
	now := time.Now().UTC()
	if _, err := db.InsertWaitRows(context.Background(), []store.WaitRow{{ParkID: 275, RideID: 1, RideName: "A", SourceUpdatedAt: now, FetchedAt: now}}); err != nil {
		t.Fatal(err)
	}
	if st, err := mcpStoreStatus(db); err != nil || st != mcpStoreStatusReady {
		t.Fatalf("store with wait history = %q, %v; want ready", st, err)
	}
}
