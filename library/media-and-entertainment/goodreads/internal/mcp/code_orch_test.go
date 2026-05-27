// Copyright 2026 zaydiscold. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

func TestCodeOrchSearchFindsNotesPublicizeRoute(t *testing.T) {
	result, err := handleCodeOrchSearch(context.Background(), mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Arguments: map[string]any{
				"query": "publicize notes highlights",
				"limit": float64(5),
			},
		},
	})
	if err != nil {
		t.Fatalf("handleCodeOrchSearch returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("handleCodeOrchSearch returned MCP error: %#v", result.Content)
	}

	text, ok := result.Content[0].(mcplib.TextContent)
	if !ok {
		t.Fatalf("first result content = %T, want TextContent", result.Content[0])
	}
	var payload struct {
		Results []struct {
			EndpointID string `json:"endpoint_id"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(text.Text), &payload); err != nil {
		t.Fatalf("search payload was not JSON: %v\n%s", err, text.Text)
	}
	for _, got := range payload.Results {
		if got.EndpointID == "notes_share_update" {
			return
		}
	}
	t.Fatalf("notes_share_update missing from search results: %#v", payload.Results)
}

func TestCodeOrchSearchFindsSitemapDiscoveredRoutes(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		{query: "ask the author questions", want: "ask-the-author_list"},
		{query: "similar books readers also enjoyed", want: "book_get-similar"},
		{query: "choice awards", want: "choiceawards_list"},
	}

	for _, tt := range tests {
		result, err := handleCodeOrchSearch(context.Background(), mcplib.CallToolRequest{
			Params: mcplib.CallToolParams{
				Arguments: map[string]any{
					"query": tt.query,
					"limit": float64(5),
				},
			},
		})
		if err != nil {
			t.Fatalf("handleCodeOrchSearch(%q) returned error: %v", tt.query, err)
		}
		if result.IsError {
			t.Fatalf("handleCodeOrchSearch(%q) returned MCP error: %#v", tt.query, result.Content)
		}

		text, ok := result.Content[0].(mcplib.TextContent)
		if !ok {
			t.Fatalf("first result content = %T, want TextContent", result.Content[0])
		}
		var payload struct {
			Results []struct {
				EndpointID string `json:"endpoint_id"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(text.Text), &payload); err != nil {
			t.Fatalf("search payload was not JSON: %v\n%s", err, text.Text)
		}
		found := false
		for _, got := range payload.Results {
			if got.EndpointID == tt.want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("%s missing from search results for %q: %#v", tt.want, tt.query, payload.Results)
		}
	}
}

func TestCodeOrchRegistryKeepsCriticalDynamicRoutes(t *testing.T) {
	want := map[string]bool{
		"ask-the-author_list":                    false,
		"book_get-similar":                       false,
		"choiceawards_list":                      false,
		"friend_list-requests":                   false,
		"group_get":                              false,
		"message_list-markallasread":             false,
		"notes_load-more_get":                    false,
		"notes_note_create":                      false,
		"notes_share_update":                     false,
		"questions_get":                          false,
		"review_get":                             false,
		"review_get-current":                     false,
		"shelf_create":                           false,
		"shelf_create-movebatch-2":               false,
		"user_get-yearinbooks":                   false,
		"user-following_list":                    false,
		"user-shelves_create":                    false,
		"amazon-purchases_list":                  false,
		"goodreads-web-undocumented-search_list": false,
	}
	seen := map[string]bool{}
	for _, ep := range codeOrchEndpoints {
		if seen[ep.ID] {
			t.Fatalf("duplicate code-orch endpoint id %q", ep.ID)
		}
		seen[ep.ID] = true
		if _, ok := want[ep.ID]; ok {
			want[ep.ID] = true
		}
	}
	for id, ok := range want {
		if !ok {
			t.Fatalf("critical Goodreads endpoint %q missing from code-orch registry", id)
		}
	}
}

func TestRegisterToolsDefaultUsesCodeOrchSurface(t *testing.T) {
	t.Setenv("GOODREADS_MCP_ENDPOINT_MIRRORS", "")
	s := mcpserver.NewMCPServer("test", "0.0.0", mcpserver.WithToolCapabilities(false))
	RegisterTools(s)

	resp := s.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`))
	respData, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("could not marshal tools/list response: %v", err)
	}
	var payload struct {
		Result struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.Unmarshal(respData, &payload); err != nil {
		t.Fatalf("tools/list response was not JSON: %v\n%s", err, string(respData))
	}
	if payload.Error != nil {
		t.Fatalf("tools/list returned error: %#v", payload.Error)
	}

	names := map[string]bool{}
	for _, tool := range payload.Result.Tools {
		names[tool.Name] = true
	}
	for _, name := range []string{"goodreads_search", "goodreads_execute", "search", "sql", "context"} {
		if !names[name] {
			t.Fatalf("expected default MCP tool %q in tools/list; got %#v", name, names)
		}
	}
	for _, rawMirror := range []string{"friend_list", "notes_share_update", "shelf_create"} {
		if names[rawMirror] {
			t.Fatalf("raw endpoint mirror %q was registered in default code-orch mode", rawMirror)
		}
	}
}
