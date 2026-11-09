package mcp

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestRXMCPWritesKeepQueryArgumentsOutsideBody(t *testing.T) {
	for _, mode := range []string{"recorder", "server"} {
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch} {
			t.Run(mode+"/"+method, func(t *testing.T) {
				session := raMCPSession(t)
				wantQuery := url.Values{
					"page_cursor": {"next +/&=?"}, "page[cursor]": {"cursor-2"}, "page[size]": {"10"},
					"fields[campaign-values-report]": {"results"}, "include": {"campaign"},
					"filter": {`equals(channel,"email")`}, "sort": {"-created"},
				}
				const wantBody = `{"data":{"attributes":{"statistics":["recipients"],"timeframe":{"key":"last_7_days"}},"type":"campaign-values-report"}}`
				calls := 0
				serve := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					if r.Method != method || r.URL.Path != "/api/campaign-values-reports" || !reflect.DeepEqual(r.URL.Query(), wantQuery) {
						t.Errorf("request = %s %s; want %s /api/campaign-values-reports with query %v", r.Method, r.URL, method, wantQuery)
					}
					for key, want := range map[string]string{
						"revision": "2026-04-15", "Authorization": "Klaviyo-API-Key fixture-key", "Content-Type": "application/json",
					} {
						if got := r.Header.Get(key); got != want {
							t.Errorf("%s = %q, want %q", key, got, want)
						}
					}
					if body, err := io.ReadAll(r.Body); err != nil || string(body) != wantBody {
						t.Errorf("body = %s, error = %v; want %s", body, err, wantBody)
					}
					w.Header().Set("Content-Type", "application/vnd.api+json")
					_, _ = io.WriteString(w, `{"data":{"attributes":{"results":[]}}}`)
				})
				original := http.DefaultTransport
				t.Cleanup(func() { http.DefaultTransport = original })
				http.DefaultTransport = raTransport(func(r *http.Request) (*http.Response, error) {
					response := httptest.NewRecorder()
					serve.ServeHTTP(response, r)
					return response.Result(), nil
				})
				if mode == "server" {
					mock := httptest.NewServer(serve)
					defer mock.Close()
					target, err := url.Parse(mock.URL)
					if err != nil {
						t.Fatal(err)
					}
					http.DefaultTransport = raTransport(func(r *http.Request) (*http.Response, error) {
						// Every request is redirected to httptest, including unexpected requests.
						r = r.Clone(r.Context())
						r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
						return original.RoundTrip(r)
					})
				}
				s := server.NewMCPServer("review-rx", "test")
				RegisterTools(s)
				const name = "campaign-values-reports_query-campaign-values"
				handler := s.ListTools()[name].Handler
				if method != http.MethodPost {
					handler = makeAPIHandler(method, "/api/campaign-values-reports", nil)
				}
				args := map[string]any{"data": map[string]any{
					"type": "campaign-values-report", "attributes": map[string]any{
						"statistics": []string{"recipients"}, "timeframe": map[string]any{"key": "last_7_days"},
					},
				}}
				for key, values := range wantQuery {
					args[key] = values[0]
				}
				result, err := handler(platform.ContextWithSession(context.Background(), session), mcplib.CallToolRequest{
					Params: mcplib.CallToolParams{Name: name, Arguments: args},
				})
				if err != nil || result == nil || result.IsError || calls != 1 {
					t.Fatalf("result = %#v, error = %v, calls = %d; want one successful report request", result, err, calls)
				}
			})
		}
	}
}
