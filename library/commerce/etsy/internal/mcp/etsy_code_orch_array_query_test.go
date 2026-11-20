package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestCodeOrchArrayBindingsCommaJoin(t *testing.T) {
	count := 0
	for _, ep := range codeOrchEndpoints {
		for _, q := range ep.QueryParams {
			if !q.QueryArray {
				continue
			}
			count++
			t.Run(strings.NewReplacer(".", "_", "-", "_", "/", "_").Replace(ep.ID)+"_"+q.WireName, func(t *testing.T) {
				for _, tc := range []struct {
					input       any
					want, token string
				}{
					{"1000000001,1000000002,1000000003", "1000000001,1000000002,1000000003", "ARRAYQ_CO_CSV_VALUE"},
					{[]any{"1000000001", "1000000002"}, "1000000001,1000000002", "ARRAYQ_CO_JSON_VALUE"},
					{`["1000000001","1000000002"]`, "1000000001,1000000002", "ARRAYQ_CO_JSONSTR_VALUE"},
				} {
					path := codeOrchSplitQuery(ep.Path, ep.QueryParams, map[string]any{q.PublicName: tc.input})
					_, raw, _ := strings.Cut(path, "?")
					parsed, err := url.ParseQuery(raw)
					if err != nil {
						t.Fatal(err)
					}
					wire := q.WireName
					if strings.Count(path, wire+"=") != 1 {
						t.Errorf("ARRAYQ_CO_NO_REPEAT: %s", path)
					}
					values := parsed[wire]
					if len(values) != 1 {
						t.Errorf("ARRAYQ_CO_SINGLE_KEY: %s = %q", wire, values)
					}
					if len(values) != 1 || values[0] != tc.want {
						t.Errorf("%s: got %q want %q", tc.token, values, tc.want)
					}
				}
			})
		}
	}
	if count != 9 {
		t.Fatalf("ARRAYQ_CO_COUNT: got %d want 9", count)
	}
}

func TestCodeOrchEtsyReadListingsGetByIdsCommaJoinsOnWire(t *testing.T) {
	resetMCPPathEnv(t)
	for key, value := range map[string]string{
		"ETSY_API_KEY": "synthetic-key:synthetic-secret", "ETSY_SHARED_SECRET": "",
		"ETSY_OAUTH2": "synthetic-token", "ETSY_SHOP_ID": "9000001", "ETSY_NO_LEARN": "true",
		"ETSY_MCP_READ_ONLY": "", "ETSY_VERIFY": "", "ETSY_DOGFOOD": "", "ETSY_CONFIG": "",
	} {
		t.Setenv(key, value)
	}
	queries := make(chan string, 8)
	fake := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/listings/batch" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		queries <- r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":0,"results":[]}`))
	}))
	defer fake.Close()
	t.Setenv("ETSY_BASE_URL", fake.URL)
	s := server.NewMCPServer("etsy", "test")
	RegisterTools(s)
	tool := s.ListTools()["etsy_read"]
	if tool == nil {
		t.Fatal("ARRAYQ_READ_ERROR: missing etsy_read")
	}
	result, err := tool.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{
		"endpoint_id": "listings.get-by-ids", "params": map[string]any{"listing_ids": "1000000001,1000000002,1000000003"},
	}}})
	if err != nil {
		t.Fatalf("ARRAYQ_READ_ERROR: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("ARRAYQ_READ_ERROR: %+v", result)
	}
	select {
	case raw := <-queries:
		parsed, err := url.ParseQuery(raw)
		if err != nil {
			t.Fatal(err)
		}
		values := parsed["listing_ids"]
		if len(values) != 1 || values[0] != "1000000001,1000000002,1000000003" || strings.Count(raw, "listing_ids=") != 1 {
			t.Errorf("ARRAYQ_READ_VALUE: %s", raw)
		}
		if !strings.Contains(raw, "listing_ids=1000000001%2C1000000002%2C1000000003") {
			t.Errorf("ARRAYQ_READ_RAW: %s", raw)
		}
	default:
		t.Fatal("ARRAYQ_READ_NO_REQUEST")
	}
}
