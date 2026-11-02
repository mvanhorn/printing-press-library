// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/monitoring/sentry/internal/store"
)

// recordRequests points the CLI at a local server and returns the requests it received.
func recordRequests(t *testing.T, response string) func() []*http.Request {
	t.Helper()
	var mu sync.Mutex
	var seen []*http.Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Clone(r.Context()))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("SENTRY_CONFIG", filepath.Join(dir, "missing.toml"))
	t.Setenv("SENTRY_AUTH_TOKEN", "test-token")
	t.Setenv("SENTRY_BASE_URL", srv.URL)
	return func() []*http.Request {
		mu.Lock()
		defer mu.Unlock()
		return append([]*http.Request(nil), seen...)
	}
}

func runCLI(t *testing.T, args ...string) {
	t.Helper()
	cmd := RootCmd()
	cmd.SetArgs(args)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("%v: %v", args, err)
	}
}

func TestExploreSendsEveryRepeatedArrayValue(t *testing.T) {
	requests := recordRequests(t, `{"data":[]}`)
	runCLI(t, "organizations", "events", "query-explore-in-table-format", "test-org",
		"--dataset", "errors", "--field", "project", "--field", "count_if(a,equals,b)", "--field", "count()",
		"--project", "4512244350713856", "--project", "2", "--environment", "production",
		"--stats-period", "14d", "--no-cache", "--data-source", "live", "--json")
	got := requests()
	if len(got) != 1 {
		t.Fatalf("requests = %d, want 1", len(got))
	}
	q := got[0].URL.Query()
	want := url.Values{
		"field":       {"project", "count_if(a,equals,b)", "count()"},
		"project":     {"4512244350713856", "2"},
		"environment": {"production"},
	}
	for key, values := range want {
		if !reflect.DeepEqual(q[key], values) {
			t.Errorf("%s = %q, want %q (raw query %q)", key, q[key], values, got[0].URL.RawQuery)
		}
	}
	if q.Get("dataset") != "errors" || q.Get("statsPeriod") != "14d" {
		t.Errorf("scalar params lost: %q", got[0].URL.RawQuery)
	}
}

func TestBulkRemoveSendsItsIDsAndQuery(t *testing.T) {
	requests := recordRequests(t, `{}`)
	runCLI(t, "organizations", "issues", "bulk-remove-an-organization-s", "test-org",
		"--id", "10", "--id", "11", "--query", "is:resolved", "--yes", "--no-input", "--json")
	got := requests()
	if len(got) != 1 || got[0].Method != http.MethodDelete {
		t.Fatalf("requests = %d (first method %v), want one DELETE", len(got), got)
	}
	q := got[0].URL.Query()
	if !reflect.DeepEqual(q["id"], []string{"10", "11"}) || q.Get("query") != "is:resolved" {
		t.Errorf("bulk remove filters = %q, want id=10&id=11&query=is:resolved", got[0].URL.RawQuery)
	}
	if _, ok := q["limit"]; ok {
		t.Errorf("unset --limit was sent: %q", got[0].URL.RawQuery)
	}
}

func TestArrayFlagAcceptsJSONArrayWithExactNumbers(t *testing.T) {
	path := appendArrayQueryParam("/x/", "project", []string{`[4512244350713856, "b"]`, "c"})
	parsed, err := url.Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.Query()["project"]; !reflect.DeepEqual(got, []string{"4512244350713856", "b", "c"}) {
		t.Errorf("project = %q, want [4512244350713856 b c]", got)
	}
}

func TestProjectBulkCommandsPreserveRepeatedIDs(t *testing.T) {
	for _, command := range []string{"bulk-remove-a-list-of", "bulk-mutate-a-list-of"} {
		t.Run(command, func(t *testing.T) {
			requests := recordRequests(t, `{}`)
			runCLI(t, "projects", "issues", command, "test-org", "test-project",
				"--id", "10", "--id", "11", "--yes", "--no-input", "--json")
			got := requests()
			if len(got) != 1 || !reflect.DeepEqual(got[0].URL.Query()["id"], []string{"10", "11"}) {
				t.Fatalf("project bulk request lost IDs: %v", got)
			}
		})
	}
}

func TestBulkRemovePreservesExplicitEmptyQuery(t *testing.T) {
	requests := recordRequests(t, `{}`)
	runCLI(t, "organizations", "issues", "bulk-remove-an-organization-s", "test-org",
		"--query", "", "--yes", "--no-input", "--json")
	got := requests()
	if len(got) != 1 || !reflect.DeepEqual(got[0].URL.Query()["query"], []string{""}) {
		t.Fatalf("explicit empty query was lost: %v", got)
	}
}

func TestArrayQueryPreservesExplicitEmptyValue(t *testing.T) {
	if got := appendArrayQueryParam("/x/", "environment", []string{""}); got != "/x/?environment=" {
		t.Fatalf("path = %q, want explicit empty environment", got)
	}
}

func TestLocalDetailReadIgnoresQueryWhenResolvingID(t *testing.T) {
	recordRequests(t, `{}`)
	db, err := store.Open(defaultDBPath("sentry-pp-cli"))
	if err != nil {
		t.Fatal(err)
	}
	want := json.RawMessage(`{"id":"test-monitor","name":"saved"}`)
	if err := db.Upsert("monitors", "test-monitor", want); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, reason := range []string{"user_requested", "api_unreachable"} {
		got, prov, err := resolveLocal(context.Background(), "monitors", false,
			"/api/0/organizations/test-org/monitors/test-monitor/?environment=production", nil, reason)
		if err != nil || string(got) != string(want) || prov.Reason != reason {
			t.Fatalf("reason %s: got %s, provenance %+v, err %v", reason, got, prov, err)
		}
	}
	runCLI(t, "organizations", "monitors", "retrieve-a", "test-org", "test-monitor",
		"--environment", "production", "--data-source", "local", "--json")
}
