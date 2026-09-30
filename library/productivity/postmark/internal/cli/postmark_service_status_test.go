// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil/testenv"
)

func withStatusServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	prevBase, prevAttempts := serviceStatusBaseURL, serviceStatusMaxAttempts
	serviceStatusBaseURL, serviceStatusMaxAttempts = srv.URL, 1
	t.Cleanup(func() { serviceStatusBaseURL, serviceStatusMaxAttempts = prevBase, prevAttempts })
}

func runServiceStatus(t *testing.T) (string, error) {
	t.Helper()
	testenv.Isolate(t)
	t.Setenv("PRINTING_PRESS_DOGFOOD", "")
	t.Setenv("PRINTING_PRESS_VERIFY", "")
	cmd := RootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs([]string{"service-status", "--json", "--no-learn"})
	err := cmd.Execute()
	return out.String(), err
}

func TestServiceStatusReportsDegradedComponentsAndNotices(t *testing.T) {
	var paths []string
	withStatusServer(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		switch {
		case r.URL.Path == "/api/v1/status":
			writeJSON(w, 200, map[string]any{"page": map[string]any{"name": "Postmark Status", "state": "degraded", "state_text": "Some systems are affected", "url": "https://status.postmarkapp.com", "updated_at": "2026-09-30T10:00:00Z"}})
		case r.URL.Path == "/api/v1/components" && r.URL.Query().Get("page") == "":
			writeJSON(w, 200, map[string]any{
				"components": []map[string]any{{"id": 1, "name": "API", "state": "operational", "parent_id": nil, "position": 1}},
				"meta":       map[string]any{"next_page": "/api/v1/components?page=2"},
			})
		case r.URL.Path == "/api/v1/components":
			writeJSON(w, 200, map[string]any{
				"components": []map[string]any{{"id": 2, "name": "Sending", "state": "degraded", "parent_id": 1, "position": 2}},
				"meta":       map[string]any{"next_page": nil},
			})
		case r.URL.Path == "/api/v1/notices":
			if r.URL.Query().Get("filter[timeline_state_eq]") != "present" {
				t.Errorf("notices must be filtered to present, got %s", r.URL.RawQuery)
			}
			writeJSON(w, 200, map[string]any{
				"notices": []map[string]any{
					{"id": 10, "type": "unplanned", "state": "investigating", "subject": "Delayed delivery", "url": "https://status.postmarkapp.com/notices/10", "began_at": "2026-09-30T09:00:00Z",
						"latest_update": map[string]any{"state": "investigating", "content": "Looking into it", "created_at": "2026-09-30T09:05:00Z"}},
					{"id": 11, "type": "planned", "state": "underway", "subject": "DB maintenance", "begins_at": "2026-09-30T08:00:00Z", "ends_at": "2026-09-30T12:00:00Z"},
				},
				"meta": map[string]any{"next_page": nil},
			})
		default:
			w.WriteHeader(404)
		}
	})
	out, err := runServiceStatus(t)
	if err != nil {
		t.Fatalf("service-status: %v\n%s", err, out)
	}
	var view serviceStatusView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if view.Operational || view.State != "degraded" || len(view.Components) != 2 {
		t.Fatalf("view = %+v", view)
	}
	if len(view.DegradedComponents) != 1 || view.DegradedComponents[0].Name != "Sending" || view.DegradedComponents[0].Parent != "API" {
		t.Fatalf("degraded = %+v", view.DegradedComponents)
	}
	if len(view.OpenIncidents) != 1 || view.OpenIncidents[0].LatestUpdate != "Looking into it" {
		t.Fatalf("incidents = %+v", view.OpenIncidents)
	}
	if len(view.MaintenanceUnderway) != 1 || view.MaintenanceUnderway[0].BeganAt != "2026-09-30T08:00:00Z" {
		t.Fatalf("maintenance = %+v", view.MaintenanceUnderway)
	}
	if len(paths) != 4 {
		t.Fatalf("expected status, 2 component pages, notices; got %v", paths)
	}
}

func TestServiceStatusSurfacesHTTPFailures(t *testing.T) {
	withStatusServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(404)
		_, _ = w.Write([]byte("<!doctype html><html>not found</html>"))
	})
	out, err := runServiceStatus(t)
	if err == nil || ExitCode(err) != 5 || !strings.Contains(err.Error(), "HTTP 404") {
		t.Fatalf("404 must be an API error naming the status, got %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("no fake report on failure, got %s", out)
	}
}

func TestServiceStatusRateLimitIsNotEmptyResult(t *testing.T) {
	withStatusServer(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	out, err := runServiceStatus(t)
	if err == nil || ExitCode(err) != 7 {
		t.Fatalf("429 must exit 7, got %v", err)
	}
	var rl *cliutil.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("error should wrap *cliutil.RateLimitError: %v", err)
	}
	if strings.TrimSpace(out) != "" {
		t.Fatalf("throttled check must not print a status report: %s", out)
	}
}

func TestNextStatusPath(t *testing.T) {
	s := func(v string) *string { return &v }
	cases := []struct {
		in   *string
		want string
	}{
		{nil, ""},
		{s(""), ""},
		{s("/api/v1/components?page=2"), "/api/v1/components?page=2"},
		{s("https://status.postmarkapp.com/api/v1/notices?page=3"), "/api/v1/notices?page=3"},
		{s("https://evil.example.com/api/v1/notices?page=3"), ""},
		{s("/elsewhere"), ""},
	}
	for _, tc := range cases {
		if got := nextStatusPath(tc.in); got != tc.want {
			t.Errorf("nextStatusPath(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildServiceStatusViewOperational(t *testing.T) {
	var page statusPageResponse
	page.Page.State = "operational"
	view := buildServiceStatusView(page, []statusComponent{{ID: 1, Name: "API", State: "operational"}}, nil, time.Unix(0, 0))
	if !view.Operational || len(view.DegradedComponents) != 0 || view.OpenIncidents == nil || view.MaintenanceUnderway == nil {
		t.Fatalf("view = %+v", view)
	}
}
