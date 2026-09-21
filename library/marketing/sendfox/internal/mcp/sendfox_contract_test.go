package mcp

import (
	"context"
	"encoding/json"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/contract"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func request(args map[string]any) mcplib.CallToolRequest {
	return mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: args}}
}
func TestSendfoxSchemasCoverEveryOperation(t *testing.T) {
	if len(codeOrchEndpoints) != 60 {
		t.Fatal(len(codeOrchEndpoints))
	}
	for i := range codeOrchEndpoints {
		ep := &codeOrchEndpoints[i]
		op := contract.Find(ep.ID)
		if op == nil {
			t.Fatal(ep.ID)
		}
		meta := codeOrchEndpointMetadata(ep)
		if _, ok := meta["body_schema"]; !ok {
			t.Fatal(ep.ID)
		}
		res, e := handleCodeOrchGet(context.Background(), request(map[string]any{"endpoint_id": ep.ID}))
		if e != nil || res.IsError {
			t.Fatalf("metadata %s: %v %#v", ep.ID, e, res)
		}
	}
}
func TestSendfoxMCPPreviewAndApproval(t *testing.T) {
	testenv.Isolate(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":12}`))
	}))
	defer server.Close()
	t.Setenv("SENDFOX_BASE_URL", server.URL)
	t.Setenv("SENDFOX_API_TOKEN", "unit-fixture-value")
	for _, tc := range []struct {
		name      string
		args      map[string]any
		wantCalls int
		wantError bool
	}{{"preview", map[string]any{"endpoint_id": "contacts.create", "params": map[string]any{"email": "reader@example.com"}}, 0, false}, {"unapproved explicit apply", map[string]any{"endpoint_id": "contacts.create", "params": map[string]any{"email": "reader@example.com"}, "dry_run": false}, 0, true}, {"approved create", map[string]any{"endpoint_id": "contacts.create", "params": map[string]any{"email": "reader@example.com"}, "confirm": true}, 1, false}, {"unapproved send", map[string]any{"endpoint_id": "campaigns.send", "params": map[string]any{"id": 12}, "confirm": true}, 1, true}, {"invalid enum", map[string]any{"endpoint_id": "campaigns.resend", "params": map[string]any{"id": 12, "audience": "everyone"}}, 1, true}} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := handleCodeOrchExecute(context.Background(), request(tc.args))
			if err != nil || res.IsError != tc.wantError || calls != tc.wantCalls {
				b, _ := json.Marshal(res)
				t.Fatalf("calls=%d err=%v result=%s", calls, err, b)
			}
		})
	}
}

func TestSearchIsThinWhileGetRetainsSchemas(t *testing.T) {
	r, e := handleCodeOrchSearch(context.Background(), request(map[string]any{"query": "campaign", "limit": 3}))
	if e != nil || r.IsError {
		t.Fatal(e, r)
	}
	b, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(string(b), "body_schema") || strings.Contains(string(b), "response_schema") {
		t.Fatal("search returned full endpoint schemas")
	}
	r, e = handleCodeOrchGet(context.Background(), request(map[string]any{"endpoint_id": "campaigns.create"}))
	if e != nil || r.IsError {
		t.Fatal(e, r)
	}
	b, _ = json.Marshal(r)
	if !strings.Contains(string(b), "body_schema") {
		t.Fatal("get missing schema")
	}
}
