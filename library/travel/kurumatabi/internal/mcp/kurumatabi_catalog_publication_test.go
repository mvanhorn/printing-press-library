// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package mcp

import (
	"context"
	"encoding/json"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/cliutil/testenv"
	"strings"
	"testing"
)

func TestActualToolCatalogReflectsDurableLocalEffects(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("KURUMATABI_CLI_PATH", writeRecipeIntentRecorder(t))
	s := server.NewMCPServer("test", "0")
	RegisterTools(s)
	tools := s.ListTools()
	for _, name := range []string{"context", "sql", "parks_filters", "parks_near", "parks_match"} {
		registered, ok := tools[name]
		if !ok {
			t.Fatalf("missing %s", name)
		}
		raw, _ := json.Marshal(registered.Tool)
		var tool map[string]any
		_ = json.Unmarshal(raw, &tool)
		annotations := tool["annotations"].(map[string]any)
		if strings.HasPrefix(name, "parks_") {
			properties := tool["inputSchema"].(map[string]any)["properties"].(map[string]any)
			if _, exists := properties["receipt"]; exists {
				t.Fatalf("readonly %s advertises receipt writes", name)
			}
			called, err := registered.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"receipt": true}}})
			if err != nil || !called.IsError || !strings.Contains(called.Content[0].(mcplib.TextContent).Text, "unknown MCP parameter") {
				t.Fatalf("readonly %s accepted receipt: %+v %v", name, called, err)
			}
		}
		if annotations["readOnlyHint"] != true || annotations["destructiveHint"] != false {
			t.Fatalf("readonly %s hints=%+v", name, annotations)
		}
	}
	for _, name := range []string{"parks_search", "parks_detail", "parks_fit", "parks_compare", "parks_audit", "parks_handoff", "park_comparison", "recall", "learnings_list", "learnings_stats", "learnings_candidates", "playbook_list", "workflow_status"} {
		registered, ok := tools[name]
		if !ok {
			t.Fatalf("missing %s", name)
		}
		raw, _ := json.Marshal(registered.Tool)
		var tool map[string]any
		_ = json.Unmarshal(raw, &tool)
		annotations := tool["annotations"].(map[string]any)
		if name != "park_comparison" {
			properties := tool["inputSchema"].(map[string]any)["properties"].(map[string]any)
			if _, exists := properties["receipt"]; !exists {
				t.Fatalf("local-state tool %s lost explicit receipt semantics", name)
			}
		}
		if annotations["readOnlyHint"] != false || annotations["destructiveHint"] != false || !strings.Contains(registered.Tool.Description, "local") {
			t.Fatalf("local-write %s hints=%+v description=%s", name, annotations, registered.Tool.Description)
		}
	}
	for _, name := range []string{"source_catalog", "source_page"} {
		if _, ok := tools[name]; ok {
			t.Fatalf("raw source diagnostic exposed: %s", name)
		}
	}
}
