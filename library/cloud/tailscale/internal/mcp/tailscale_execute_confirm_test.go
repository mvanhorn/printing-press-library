// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type executeRecorder struct {
	mu   sync.Mutex
	reqs []string
}

func (r *executeRecorder) seen() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.reqs...)
}

func newExecuteTestServer(t *testing.T) *executeRecorder {
	t.Helper()
	resetMCPPathEnv(t)
	rec := &executeRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		rec.mu.Lock()
		rec.reqs = append(rec.reqs, r.Method+" "+r.URL.Path+" "+string(body))
		rec.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("TAILSCALE_BASE_URL", srv.URL)
	t.Setenv("TAILSCALE_API_KEY", "test-token-must-not-appear-in-plans")
	t.Setenv("TAILSCALE_OAUTH_CLIENT_ID", "")
	t.Setenv("TAILSCALE_OAUTH_CLIENT_SECRET", "")
	return rec
}

func callExecute(t *testing.T, args map[string]any) string {
	t.Helper()
	req := mcplib.CallToolRequest{}
	req.Params.Name = "tailscale_execute"
	req.Params.Arguments = args
	res, err := handleCodeOrchExecute(context.Background(), req)
	if err != nil {
		t.Fatalf("handleCodeOrchExecute: %v", err)
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(mcplib.TextContent); ok {
			text += tc.Text
		}
	}
	if res.IsError {
		t.Fatalf("tool returned an error: %s", text)
	}
	return text
}

func TestExecuteWriteWithoutConfirmReturnsPlan(t *testing.T) {
	rec := newExecuteTestServer(t)
	out := callExecute(t, map[string]any{
		"endpoint_id": "device.routes.set",
		"params":      map[string]any{"deviceId": "node-1", "routes": []any{"10.0.0.0/24"}},
	})
	if got := rec.seen(); len(got) != 0 {
		t.Fatalf("unconfirmed write reached the API: %v", got)
	}
	var plan codeOrchWritePlan
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatalf("plan is not JSON: %v\n%s", err, out)
	}
	if !plan.PlanOnly || plan.Method != "POST" || plan.Path != "/device/node-1/routes" {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	body, _ := json.Marshal(plan.Body)
	if !strings.Contains(string(body), "10.0.0.0/24") {
		t.Fatalf("plan body lost the routes: %s", body)
	}
	if strings.Contains(out, "test-token-must-not-appear-in-plans") || strings.Contains(strings.ToLower(out), "authorization") {
		t.Fatalf("plan exposes credentials: %s", out)
	}
}

func TestExecuteWriteWithConfirmSends(t *testing.T) {
	rec := newExecuteTestServer(t)
	callExecute(t, map[string]any{
		"endpoint_id": "device.routes.set",
		"params":      map[string]any{"deviceId": "node-1", "routes": []any{"10.0.0.0/24"}},
		"confirm":     true,
	})
	got := rec.seen()
	if len(got) != 1 || !strings.HasPrefix(got[0], "POST /device/node-1/routes ") || !strings.Contains(got[0], "10.0.0.0/24") {
		t.Fatalf("confirmed write not sent as expected: %v", got)
	}
	if strings.Contains(got[0], "confirm") {
		t.Fatalf("confirm leaked into the request: %v", got)
	}
}

func TestExecuteDeleteAndNonBoolConfirmStayPlanOnly(t *testing.T) {
	rec := newExecuteTestServer(t)
	out := callExecute(t, map[string]any{
		"endpoint_id": "device.delete",
		"params":      map[string]any{"deviceId": "node-1"},
		"confirm":     "true",
	})
	if got := rec.seen(); len(got) != 0 {
		t.Fatalf("delete without boolean confirm reached the API: %v", got)
	}
	if !strings.Contains(out, `"plan_only": true`) || !strings.Contains(out, `"method": "DELETE"`) {
		t.Fatalf("expected a DELETE plan, got: %s", out)
	}
}

func TestExecuteReadsAndReadOnlyPostsNeedNoConfirm(t *testing.T) {
	rec := newExecuteTestServer(t)
	callExecute(t, map[string]any{"endpoint_id": "tailnet.devices.list"})
	callExecute(t, map[string]any{
		"endpoint_id": "tailnet.acl.validate",
		"params":      map[string]any{"acls": []any{}},
	})
	got := rec.seen()
	if len(got) != 2 || !strings.HasPrefix(got[0], "GET ") || !strings.HasPrefix(got[1], "POST ") || !strings.Contains(got[1], "/acl/validate") {
		t.Fatalf("reads and validate should be sent without confirm: %v", got)
	}
}

func TestReadOnlyPostAllowlistMatchesRegistry(t *testing.T) {
	for id := range codeOrchReadOnlyPosts {
		ep := findCodeOrchEndpoint(id)
		if ep == nil {
			t.Fatalf("read-only allowlist names unknown endpoint %q", id)
		}
		if ep.Method != "POST" {
			t.Fatalf("read-only allowlist entry %q is %s, not POST", id, ep.Method)
		}
	}
}

// Every MCP tool that is not read-only must either write only to the local
// store or be one of the confirm-gated tools. A new remote writer exposed
// through the Cobra mirror (which shells out without --agent) fails here.
func TestMCPWriteSurfaceIsGatedOrLocal(t *testing.T) {
	s := server.NewMCPServer("tailscale", "test")
	RegisterTools(s)
	allowed := map[string]string{
		"tailscale_execute":                   "plan-only without confirm=true",
		"turn_a_mac_into_an_exit_node_safely": "routes approve recipe; --yes is blocked over MCP",
		"sync":                                "local store",
		"workflow_archive":                    "local store",
		"teach":                               "local store",
		"teach_lookup":                        "local store",
		"teach_pattern":                       "local store",
		"teach_playbook":                      "local store",
		"playbook_amend":                      "local store",
		"learnings_confirm":                   "local store",
		"learnings_forget":                    "local store",
		"learnings_reject":                    "local store",
	}
	tools := s.ListTools()
	if _, ok := tools["import"]; ok {
		t.Fatal("import is exposed over MCP; it POSTs every record without confirmation")
	}
	for name, tool := range tools {
		ro := tool.Tool.Annotations.ReadOnlyHint
		if ro != nil && *ro {
			continue
		}
		if _, ok := allowed[name]; !ok {
			t.Errorf("MCP tool %q may write and is not a known local-store or confirm-gated tool", name)
		}
	}
}
