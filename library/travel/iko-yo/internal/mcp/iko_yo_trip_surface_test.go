// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package mcp

import (
	"context"
	"encoding/json"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"strings"
	"testing"
)

func TestTripContextAndRecipeMetadata(t *testing.T) {
	s := server.NewMCPServer("test", "test")
	RegisterTools(s)
	RegisterTripSurface(s)
	e := s.GetTool("context")
	r, err := e.Handler(context.Background(), mcplib.CallToolRequest{})
	if err != nil || r.IsError || len(r.Content) != 1 {
		t.Fatalf("context: %+v %v", r, err)
	}
	text, ok := r.Content[0].(mcplib.TextContent)
	if !ok {
		t.Fatal("context is not text JSON")
	}
	var v map[string]any
	if err = json.Unmarshal([]byte(text.Text), &v); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"cursor-based", "default 100", "Run sync first", "search tool for full-text"} {
		if strings.Contains(text.Text, bad) {
			t.Errorf("context advertises unsupported %s", bad)
		}
	}
	for _, want := range []string{"Iko-yo Trip", "page=N", "unknown", "max-scan-records", "archived"} {
		if !strings.Contains(text.Text, want) {
			t.Errorf("context lacks %s", want)
		}
	}
	for _, name := range []string{"context", "published_family_facts", "trip_discover", "trip_inspect", "trip_compare", "trip_cached"} {
		tool := s.GetTool(name)
		if tool == nil || tool.Tool.Annotations.ReadOnlyHint == nil || !*tool.Tool.Annotations.ReadOnlyHint {
			t.Errorf("%s lacks read-only hint", name)
		}
	}
	if strings.Contains(s.GetTool("sql").Tool.Description, "Requires sync first") {
		t.Fatal("SQL claims unsupported sync")
	}
}
