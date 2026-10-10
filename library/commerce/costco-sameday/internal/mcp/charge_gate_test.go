// Copyright 2026 DashLabsDev and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: MCP raw endpoint tools cannot reach FinalizeCheckout.

package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func TestMCPEndpointToolCannotFinalizeCheckout(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"finalizeCheckout":{"legacyOrderId":"1"}}}`))
	}))
	defer srv.Close()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(dir, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))
	t.Setenv("COSTCO_SAMEDAY_CONFIG", filepath.Join(dir, "config.toml"))
	t.Setenv("COSTCO_SAMEDAY_BASE_URL", srv.URL)
	t.Setenv("PRINTING_PRESS_VERIFY", "")

	// A mutating GraphQL endpoint tool (e.g. checkout.updatecheckout) passes
	// unknown args straight into the POST body, so an agent can try to name
	// FinalizeCheckout itself, even claiming yes/confirm_charge in args.
	handler := makeAPIHandler("POST", "/graphql", false, false, nil, mcpPageConfig{}, nil, nil)
	argSets := []map[string]any{
		{"operationName": "FinalizeCheckout", "variables": map[string]any{"checkoutSessionId": "s"}},
		{"operationName": "UpdateCheckout", "query": "mutation { finalizeCheckout { legacyOrderId } }"},
		{"operationName": "FinalizeCheckout", "yes": true, "confirm_charge": true},
	}
	for i, args := range argSets {
		result, err := handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: args}})
		refused := err != nil || (result != nil && result.IsError)
		if !refused {
			t.Fatalf("case %d: MCP tool should refuse FinalizeCheckout", i)
		}
	}
	if n := atomic.LoadInt32(&hits); n != 0 {
		t.Fatalf("FinalizeCheckout reached the server via MCP (%d hits)", n)
	}
	// Control: the same tool does reach the mock for a non-charge op, so the
	// refusals above come from the charge gate, not a broken harness.
	if _, err := handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"operationName": "UpdateCheckout"}}}); err != nil {
		t.Fatalf("control call errored: %v", err)
	}
	if n := atomic.LoadInt32(&hits); n != 1 {
		t.Fatalf("control call should reach mock once, hits=%d", n)
	}
}
