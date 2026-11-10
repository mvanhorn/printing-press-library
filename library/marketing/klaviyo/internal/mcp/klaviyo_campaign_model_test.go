// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestMCPCampaignGetID404IncludesMigrationHintWithoutFallback(t *testing.T) {
	session := raMCPSession(t)
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	const id = "01J0000000MESSAGE000000001"
	calls := 0
	http.DefaultTransport = raTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.Method != "GET" || r.URL.Path != "/api/campaigns/"+id || r.Header.Get("revision") != "2026-10-15" {
			t.Errorf("generated MCP request changed: %s %s", r.Method, r.URL)
		}
		recorder := httptest.NewRecorder()
		recorder.Header().Set("Content-Type", "application/json")
		recorder.WriteHeader(404)
		fmt.Fprint(recorder, `{"errors":[{"detail":"not found"}]}`)
		return recorder.Result(), nil
	})
	result, err := handleCodeOrchRead(platform.ContextWithSession(context.Background(), session), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "klaviyo_read", Arguments: map[string]any{"endpoint_id": "campaigns.get-id", "params": map[string]any{"id": id}}}})
	if err != nil || result == nil || !result.IsError || calls != 1 {
		t.Fatalf("result=%+v error=%v calls=%d", result, err, calls)
	}
	const hint = "IDs from before revision 2026-10-15 are campaign message IDs now: try `campaign-messages get-id <id>` or `campaigns get-id <container id>`"
	found := false
	for _, block := range result.Content {
		if content, ok := mcplib.AsTextContent(block); ok && strings.Contains(content.Text, hint) {
			found = true
		}
	}
	if !found {
		t.Fatalf("MCP error omitted migration hint: %+v", result)
	}
}
