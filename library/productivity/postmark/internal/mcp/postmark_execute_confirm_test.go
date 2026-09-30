// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func executeRequest(args map[string]any) mcplib.CallToolRequest {
	return mcplib.CallToolRequest{Params: mcplib.CallToolParams{Name: "postmark_execute", Arguments: args}}
}

func resultText(r *mcplib.CallToolResult) string {
	var b strings.Builder
	for _, c := range r.Content {
		if tc, ok := c.(mcplib.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func TestExecuteConfirmPolicyCoversDeletesAndNamedEndpoints(t *testing.T) {
	for id := range postmarkConfirmEndpoints {
		if findCodeOrchEndpoint(id) == nil {
			t.Errorf("confirm policy names %q, which is not in the endpoint registry", id)
		}
	}
	deletes := 0
	for i := range codeOrchEndpoints {
		ep := &codeOrchEndpoints[i]
		_, needs := postmarkExecuteConfirmReason(ep)
		switch {
		case ep.Method == "DELETE":
			deletes++
			if !needs {
				t.Errorf("DELETE endpoint %s runs without confirmation", ep.ID)
			}
		case ep.Method == "GET" && needs:
			t.Errorf("read endpoint %s should not need confirmation", ep.ID)
		}
	}
	if deletes == 0 {
		t.Fatal("registry has no DELETE endpoints; the policy test is not exercising anything")
	}
}

func TestExecuteDestructiveCallsNeedLiteralConfirm(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ErrorCode":0,"Message":"OK"}`))
	}))
	defer srv.Close()
	t.Setenv("POSTMARK_HOME", t.TempDir())
	t.Setenv("POSTMARK_BASE_URL", srv.URL)
	t.Setenv("POSTMARK_SERVER_TOKEN", "test-server-token")
	t.Setenv("POSTMARK_ACCOUNT_TOKEN", "test-account-token")

	params := map[string]any{"templateIdOrAlias": "password-reset"}
	unconfirmed := []map[string]any{
		{"endpoint_id": "templates.delete", "params": params},
		{"endpoint_id": "templates.delete", "params": params, "confirm": false},
		{"endpoint_id": "templates.delete", "params": params, "confirm": "true"},
		{"endpoint_id": "templates.delete", "params": map[string]any{"templateIdOrAlias": "password-reset", "confirm": true}},
		{"endpoint_id": "data_removals.create", "params": map[string]any{"RequestedBy": "privacy@example.com", "RequestedFor": "jane@example.com"}},
		{"endpoint_id": "servers.delete", "params": map[string]any{"serverid": "7"}},
	}
	for i, args := range unconfirmed {
		res, err := handleCodeOrchExecute(context.Background(), executeRequest(args))
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if !res.IsError || !strings.Contains(resultText(res), "confirmation_required") {
			t.Fatalf("case %d should return a confirmation preview, got %q", i, resultText(res))
		}
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("unconfirmed calls reached the API %d times", n)
	}

	res, err := handleCodeOrchExecute(context.Background(), executeRequest(map[string]any{
		"endpoint_id": "templates.delete", "params": map[string]any{"templateIdOrAlias": "password-reset"}, "confirm": true,
	}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(resultText(res), "confirmation_required") {
		t.Fatalf("confirm: true was still gated: %q", resultText(res))
	}
	if n := hits.Load(); n != 1 {
		t.Fatalf("confirmed delete reached the API %d times, want 1", n)
	}
}

func TestExecuteResolvedPathGate(t *testing.T) {
	patterns := map[string]bool{}
	for _, cp := range confirmPaths() {
		patterns[cp.method+" "+cp.re.String()] = true
	}
	for _, want := range []string{
		"POST (?i)^/data-removals$",
		"POST (?i)^/message-streams/[^/]+/archive$",
		"POST (?i)^/message-streams/[^/]+/suppressions/delete$",
		"PUT (?i)^/templates/push$",
	} {
		if !patterns[want] {
			t.Errorf("missing policy pattern %q (have %v)", want, patterns)
		}
	}

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ErrorCode":0,"Message":"OK"}`))
	}))
	defer srv.Close()
	t.Setenv("POSTMARK_HOME", t.TempDir())
	t.Setenv("POSTMARK_BASE_URL", srv.URL)
	t.Setenv("POSTMARK_SERVER_TOKEN", "test-server-token")
	t.Setenv("POSTMARK_ACCOUNT_TOKEN", "test-account-token")

	// templates.update with the alias "push" resolves to PUT /templates/push.
	push := map[string]any{"templateIdOrAlias": "push", "SourceServerID": 1, "DestinationServerID": 2, "PerformChanges": true}
	res, err := handleCodeOrchExecute(context.Background(), executeRequest(map[string]any{"endpoint_id": "templates.update", "params": push}))
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError || !strings.Contains(resultText(res), "confirmation_required") || hits.Load() != 0 {
		t.Fatalf("templates.update onto /templates/push ran without confirm (hits=%d): %q", hits.Load(), resultText(res))
	}
	if !strings.Contains(resultText(res), `"resolved_path":"/templates/push"`) {
		t.Errorf("preview should show the resolved path: %q", resultText(res))
	}
	upper := map[string]any{"templateIdOrAlias": "PUSH", "SourceServerID": 1, "DestinationServerID": 2}
	if res, _ := handleCodeOrchExecute(context.Background(), executeRequest(map[string]any{"endpoint_id": "templates.update", "params": upper})); !strings.Contains(resultText(res), "confirmation_required") || hits.Load() != 0 {
		t.Fatalf("alias PUSH should be gated like push (hits=%d): %q", hits.Load(), resultText(res))
	}
	// An ordinary template update is not gated.
	res, err = handleCodeOrchExecute(context.Background(), executeRequest(map[string]any{
		"endpoint_id": "templates.update", "params": map[string]any{"templateIdOrAlias": "password-reset", "Subject": "Reset"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(resultText(res), "confirmation_required") || hits.Load() != 1 {
		t.Fatalf("ordinary template update was gated or not sent (hits=%d): %q", hits.Load(), resultText(res))
	}
}
