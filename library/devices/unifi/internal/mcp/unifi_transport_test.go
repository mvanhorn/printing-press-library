// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func TestCodeOrchRoutesThroughConsoleTransport(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"offset":0,"limit":25,"count":0,"totalCount":0,"data":[]}`))
	}))
	defer srv.Close()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", home+"/state")
	t.Setenv("XDG_CONFIG_HOME", home+"/config")
	t.Setenv("XDG_CACHE_HOME", home+"/cache")
	t.Setenv("XDG_DATA_HOME", home+"/data")
	t.Setenv("UNIFI_BASE_URL", srv.URL)
	t.Setenv("UNIFI_API_KEY", "fixture-cloud-key-aaaa")
	t.Setenv("UNIFI_CONSOLE_ID", "MCP-CONSOLE")

	call := func(fn func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error), args map[string]any) *mcplib.CallToolResult {
		t.Helper()
		var req mcplib.CallToolRequest
		req.Params.Arguments = args
		res, err := fn(context.Background(), req)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	res := call(handleCodeOrchRead, map[string]any{"endpoint_id": "network.list-sites"})
	if res.IsError {
		t.Fatalf("read failed: %+v", res.Content)
	}
	mu.Lock()
	got := append([]string(nil), seen...)
	mu.Unlock()
	if len(got) != 1 || got[0] != "GET /v1/connector/consoles/MCP-CONSOLE/proxy/network/integration/v1/sites" {
		t.Fatalf("MCP read did not use the connector mapping: %v", got)
	}

	// Execute previews until confirm is boolean true, and sends nothing meanwhile.
	params := map[string]any{"siteId": "s1", "aclRuleId": "a1"}
	res = call(handleCodeOrchExecute, map[string]any{"endpoint_id": "network.delete-acl-rule", "params": params})
	text := ""
	for _, c := range res.Content {
		if tc, ok := c.(mcplib.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "confirmation_required") {
		t.Fatalf("execute without confirm did not preview: %s", text)
	}
	mu.Lock()
	n := len(seen)
	mu.Unlock()
	if n != 1 {
		t.Fatalf("unconfirmed execute reached the API: %v", seen)
	}

	// UNIFI_READ_ONLY blocks a confirmed execute at the client choke point.
	t.Setenv("UNIFI_READ_ONLY", "1")
	res = call(handleCodeOrchExecute, map[string]any{"endpoint_id": "network.delete-acl-rule", "params": params, "confirm": true})
	if !res.IsError {
		t.Fatalf("read-only mode let an execute through: %+v", res.Content)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 1 {
		t.Fatalf("read-only execute reached the API: %v", seen)
	}
}
