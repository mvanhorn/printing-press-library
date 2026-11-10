package mcp

// PATCH(dialer-initiate-call-contract-pin-930): the MCP runtime path for dialer.initiate-call (board #930). The binary registers the
// generic conduyt-crm_execute tool; for a POST the params that are not path/query bindings become the JSON body. Drive that real
// path with every request-schema key and require POST /dialer/call with exactly those keys in the body. The list is hard-coded
// (see internal/cli/dialer_initiate_call_contract_test.go, which also compares it to spec.yaml and upstream/openapi.yaml).

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func TestCodeOrchInitiateCallSendsExactlySchemaKeys(t *testing.T) {
	want := []string{"autoDialer", "callerIdStrategy", "contactId", "dncOverride", "to", "vendorBridgeNonce", "vendorBridgeable"}
	params := map[string]any{
		"to": "+15555550100", "contactId": "11111111-1111-4111-8111-111111111111", "dncOverride": true, "autoDialer": false,
		"callerIdStrategy": "iphone", "vendorBridgeNonce": "nonce-sentinel", "vendorBridgeable": true,
	}
	var hits int
	var method, path, rawQuery, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		method, path, rawQuery = r.Method, r.URL.Path, r.URL.RawQuery
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{},"meta":{}}`))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("CONDUYT_CRM_BASE_URL", srv.URL+"/api/v1")
	t.Setenv("CONDUYT_CRM_BEARER_AUTH", "test-token")
	t.Setenv("CONDUYT_CRM_CONFIG", filepath.Join(t.TempDir(), "config.toml"))
	res, err := codeOrchTestInvoke(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{
		Arguments: map[string]any{"endpoint_id": "dialer.initiate-call", "params": params},
	}})
	if err != nil || res.IsError || hits != 1 {
		t.Fatalf("err=%v isError=%v hits=%d", err, res != nil && res.IsError, hits)
	}
	if method != "POST" || path != "/api/v1/dialer/call" || rawQuery != "" {
		t.Errorf("on the wire: %s %s ?%s, want POST /api/v1/dialer/call with no query", method, path, rawQuery)
	}
	var sent map[string]any
	if err := json.Unmarshal([]byte(body), &sent); err != nil {
		t.Fatalf("body %q: %v", body, err)
	}
	var keys []string
	for k := range sent {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if !reflect.DeepEqual(keys, want) {
		t.Errorf("MCP body keys %v != schema properties %v", keys, want)
	}
	if !reflect.DeepEqual(sent, params) {
		t.Errorf("body = %v, want each key with its own value %v", sent, params)
	}
}

func TestCodeOrchInitiateCallRoute(t *testing.T) {
	for _, ep := range codeOrchEndpoints {
		if ep.ID == "dialer.initiate-call" {
			if ep.Method != "POST" || ep.Path != "/dialer/call" || len(ep.QueryParams) != 0 || len(ep.TemplateParams) != 0 {
				t.Errorf("route = %s %s query=%v template=%v", ep.Method, ep.Path, ep.QueryParams, ep.TemplateParams)
			}
			return
		}
	}
	t.Fatal("dialer.initiate-call missing from codeOrchEndpoints")
}
