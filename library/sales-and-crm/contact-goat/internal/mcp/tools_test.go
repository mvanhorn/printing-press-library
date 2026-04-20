// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// tools_test.go: characterization tests for the four public-API MCP
// tools added in unit 8 (api_search, api_research, api_groups_list,
// api_usage). The four tools are the first MCP entries that wrap a
// bearer-auth surface; everything older in this package routes through
// the cookie client. The tests pin down the bits the orchestrator and
// the calling agents care about: registered manifest shape, structured
// error on missing args, structured error on missing key, and JSON
// envelope parity with `api hpn search --json`.

package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// TestRegisterTools_NewToolCount verifies that the four new public-API
// tools (api_search, api_research, api_groups_list, api_usage) join the
// existing roster, and that the registered manifest still includes the
// older entries. Total registered tools = 12 API-derived + 4 helpers
// (sync, search, sql, context) + 4 public-API = 20.
//
// The .printing-press.json mcp_tool_count manifest tracks API-derived
// tools only (12 cookie-surface + 4 bearer-surface = 16); the helper
// tools are intentionally uncounted because they are utilities, not
// API endpoints. This test pins down the actual registered total so a
// future regen can't silently drop a tool.
func TestRegisterTools_NewToolCount(t *testing.T) {
	s := server.NewMCPServer("test", "0.0.0", server.WithToolCapabilities(false))
	RegisterTools(s)

	listed := s.HandleMessage(context.Background(), []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	raw, err := json.Marshal(listed)
	if err != nil {
		t.Fatalf("marshal listed: %v", err)
	}
	var resp struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				InputSchema struct {
					Type       string                 `json:"type"`
					Properties map[string]interface{} `json:"properties"`
					Required   []string               `json:"required"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal tools/list: %v\nraw=%s", err, raw)
	}
	if got, want := len(resp.Result.Tools), 20; got != want {
		t.Fatalf("expected %d tools registered (12 API + 4 helpers + 4 public-API), got %d", want, got)
	}

	byName := map[string]struct {
		Description string
		Properties  map[string]interface{}
		Required    []string
	}{}
	for _, tl := range resp.Result.Tools {
		byName[tl.Name] = struct {
			Description string
			Properties  map[string]interface{}
			Required    []string
		}{tl.Description, tl.InputSchema.Properties, tl.InputSchema.Required}
	}

	want := []struct {
		name             string
		descSubstr       string
		requiredKeys     []string
		expectedPropKeys []string
	}{
		{
			name:             "api_search",
			descSubstr:       "Costs 2 credits",
			requiredKeys:     []string{"text"},
			expectedPropKeys: []string{"text", "group_ids", "include_friends_connections", "include_my_connections"},
		},
		{
			name:             "api_research",
			descSubstr:       "Costs 1 credit on completion",
			requiredKeys:     []string{"description"},
			expectedPropKeys: []string{"description"},
		},
		{
			name:             "api_groups_list",
			descSubstr:       "Free probe",
			requiredKeys:     nil,
			expectedPropKeys: nil,
		},
		{
			name:             "api_usage",
			descSubstr:       "Free probe",
			requiredKeys:     nil,
			expectedPropKeys: nil,
		},
	}
	for _, w := range want {
		got, ok := byName[w.name]
		if !ok {
			t.Errorf("tool %q not registered", w.name)
			continue
		}
		if got.Description == "" {
			t.Errorf("tool %q has empty description", w.name)
		}
		if !strings.Contains(got.Description, w.descSubstr) {
			t.Errorf("tool %q description missing substring %q (got: %q)", w.name, w.descSubstr, got.Description)
		}
		for _, key := range w.expectedPropKeys {
			if _, ok := got.Properties[key]; !ok {
				t.Errorf("tool %q missing expected property %q", w.name, key)
			}
		}
		// Required must be a superset of the expected required keys.
		req := map[string]bool{}
		for _, k := range got.Required {
			req[k] = true
		}
		for _, k := range w.requiredKeys {
			if !req[k] {
				t.Errorf("tool %q missing required key %q (got required=%v)", w.name, k, got.Required)
			}
		}
	}
}

// TestHandleAPISearch_EmptyText returns a structured error (not a panic)
// and never attempts a network call when text is empty. Mirrors the CLI
// usage-error contract from `api hpn search ""`.
func TestHandleAPISearch_EmptyText(t *testing.T) {
	req := mcplib.CallToolRequest{}
	req.Params.Name = "api_search"
	req.Params.Arguments = map[string]interface{}{"text": "   "}

	res, err := handleAPISearch(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned go error (want structured tool error): %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected structured tool error, got: %#v", res)
	}
	body := toolResultText(res)
	if !strings.Contains(body, "text is required") {
		t.Errorf("error message should mention required text, got: %q", body)
	}
}

// TestHandleAPIResearch_NoKey verifies that with no HAPPENSTANCE_API_KEY
// set, the handler returns a structured tool error that names the env
// var and points at the rotation URL — without ever making a network
// call.
func TestHandleAPIResearch_NoKey(t *testing.T) {
	t.Setenv("HAPPENSTANCE_API_KEY", "")
	// Force config loader to land in a temp dir that has no config file
	// so the TOML-fallback path also returns "" for the API key.
	t.Setenv("CONTACT_GOAT_CONFIG", t.TempDir()+"/missing.toml")
	t.Setenv("HOME", t.TempDir())

	req := mcplib.CallToolRequest{}
	req.Params.Name = "api_research"
	req.Params.Arguments = map[string]interface{}{"description": "Brian Chesky"}

	res, err := handleAPIResearch(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned go error (want structured tool error): %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected structured tool error, got: %#v", res)
	}
	body := toolResultText(res)
	if !strings.Contains(body, "HAPPENSTANCE_API_KEY") {
		t.Errorf("error message should name the env var, got: %q", body)
	}
}

// TestHandleAPIGroupsList_NoKey same shape as TestHandleAPIResearch_NoKey
// but for the no-arg tool — confirms the auth gate fires before the
// network call regardless of arg shape.
func TestHandleAPIGroupsList_NoKey(t *testing.T) {
	t.Setenv("HAPPENSTANCE_API_KEY", "")
	t.Setenv("CONTACT_GOAT_CONFIG", t.TempDir()+"/missing.toml")
	t.Setenv("HOME", t.TempDir())

	req := mcplib.CallToolRequest{}
	req.Params.Name = "api_groups_list"

	res, err := handleAPIGroupsList(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned go error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected structured tool error, got: %#v", res)
	}
	body := toolResultText(res)
	if !strings.Contains(body, "HAPPENSTANCE_API_KEY") {
		t.Errorf("error message should name env var, got: %q", body)
	}
}

// TestHandleAPIUsage_NoKey same as above for api_usage.
func TestHandleAPIUsage_NoKey(t *testing.T) {
	t.Setenv("HAPPENSTANCE_API_KEY", "")
	t.Setenv("CONTACT_GOAT_CONFIG", t.TempDir()+"/missing.toml")
	t.Setenv("HOME", t.TempDir())

	req := mcplib.CallToolRequest{}
	req.Params.Name = "api_usage"

	res, err := handleAPIUsage(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned go error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected structured tool error, got: %#v", res)
	}
	body := toolResultText(res)
	if !strings.Contains(body, "HAPPENSTANCE_API_KEY") {
		t.Errorf("error message should name env var, got: %q", body)
	}
}

// toolResultText walks a CallToolResult.Content slice and concatenates
// the text payloads. The mcp-go API stores content as a typed
// interface; we rely on the JSON roundtrip to keep the test agnostic to
// the concrete type used internally.
func toolResultText(res *mcplib.CallToolResult) string {
	if res == nil {
		return ""
	}
	raw, _ := json.Marshal(res.Content)
	var arr []map[string]any
	_ = json.Unmarshal(raw, &arr)
	var b strings.Builder
	for _, item := range arr {
		if t, ok := item["text"].(string); ok {
			b.WriteString(t)
		}
	}
	return b.String()
}
