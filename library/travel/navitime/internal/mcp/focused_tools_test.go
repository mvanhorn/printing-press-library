package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/travel/navitime/internal/mcp/cobratree"
)

func TestFocusedRegistrationKeepsDomainToolsAndOmitsUnsupportedSyncTools(t *testing.T) {
	s := server.NewMCPServer("navitime-focused", "test")
	RegisterFocusedTools(s)
	tools := s.ListTools()
	if len(tools) != 7 {
		t.Fatalf("focused registration has %d tools, want six workflows plus context", len(tools))
	}
	if tools["context"] == nil {
		t.Fatal("context tool is missing")
	}
	for _, name := range []string{"search", "sql"} {
		if tools[name] != nil {
			t.Fatalf("unsupported synced-data tool %q is exposed", name)
		}
	}
	root := cli.RootCmd()
	for _, command := range []string{"capabilities", "places search", "passes list", "routes search", "routes compare", "routes show"} {
		name := cobratree.ToolNameForCommand(s, root, command)
		if name == "" || tools[name] == nil {
			t.Fatalf("focused registration lost domain command %q", command)
		}
	}
	result, err := tools["context"].Handler(context.Background(), mcplib.CallToolRequest{})
	if err != nil || result == nil || result.IsError {
		t.Fatalf("focused context failed: %v", err)
	}
	text := mcpTextContent(t, result)
	var payload struct {
		ToolCount int `json:"tool_count"`
		Tools     []struct {
			Name string `json:"name"`
		} `json:"tools"`
		Guidance map[string]string `json:"guidance"`
	}
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.ToolCount != len(tools) || len(payload.Tools) != len(tools) {
		t.Fatal("context count does not reflect focused registration")
	}
	for _, item := range payload.Tools {
		if tools[item.Name] == nil {
			t.Fatalf("context advertises absent tool %q", item.Name)
		}
	}
	for _, falseClaim := range []string{"syncable", "searchable", "cursor", "after paging", "default100", "Requires sync", "\"sql\"", "\"search\""} {
		if strings.Contains(text, falseClaim) {
			t.Fatalf("focused context contains unsupported claim %q", falseClaim)
		}
	}
	for _, key := range []string{"verification", "places", "time", "availability", "fares", "pagination", "snapshots", "refresh", "no_cache"} {
		if payload.Guidance[key] == "" {
			t.Fatalf("focused context lacks %s guidance", key)
		}
	}
}
