package ga4

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRunReportPostsTypedRequest(t *testing.T) {
	var seenPath, seenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenPath = r.URL.Path
		seenAuth = r.Header.Get("Authorization")
		var req RunReportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Metrics[0].Name != "sessions" {
			t.Fatalf("bad request: %#v", req)
		}
		_, _ = w.Write([]byte(`{"dimensionHeaders":[{"name":"date"}],"metricHeaders":[{"name":"sessions"}],"rows":[{"dimensionValues":[{"value":"20260612"}],"metricValues":[{"value":"42"}]}]}`))
	}))
	defer srv.Close()
	c := NewClient("tok", time.Second)
	c.DataBase = srv.URL
	out, st, err := c.RunReport(context.Background(), "280199692", RunReportRequest{Metrics: []Metric{{Name: "sessions"}}})
	if err != nil || st != 200 {
		t.Fatalf("%d %v", st, err)
	}
	if !strings.Contains(seenPath, "/properties/280199692:runReport") || seenAuth != "Bearer tok" {
		t.Fatalf("path/auth %q %q", seenPath, seenAuth)
	}
	if len(out.Rows) != 1 {
		t.Fatalf("bad rows: %#v", out)
	}
}
func TestAPIErrorIncludesStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "nope", http.StatusForbidden) }))
	defer srv.Close()
	c := NewClient("tok", time.Second)
	c.AdminBase = srv.URL
	_, st, err := c.AccountSummaries(context.Background())
	if st != 403 || err == nil {
		t.Fatalf("want 403 err, got %d %v", st, err)
	}
	if _, ok := err.(APIError); !ok {
		t.Fatalf("want APIError, got %T", err)
	}
}
