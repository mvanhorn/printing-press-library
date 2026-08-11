// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"
)

func writeContactListConfig(t *testing.T, baseURL string) string {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.toml")
	cfg := fmt.Sprintf("base_url = %q\nauth_header = %q\n", baseURL, "Bearer test-token")
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return cfgPath
}

// TestContactList_AllPaginates asserts that `contact list --all` walks the
// Respond.io integer cursor: page 1 returns pagination.next=2, the loop must
// re-request with cursorId=2 in the query string (the API ignores a body
// cursorId) and accumulate page 2, producing both items in the output. It also
// pins the required wildcard body the API rejects the request without.
func TestContactList_AllPaginates(t *testing.T) {
	var requests []map[string]any
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		requests = append(requests, body)
		queries = append(queries, r.URL.RawQuery)
		if len(requests) == 1 {
			w.Write([]byte(`{"items":[{"id":1}],"pagination":{"next":2}}`))
			return
		}
		w.Write([]byte(`{"items":[{"id":2}]}`))
	}))
	defer srv.Close()

	cfgPath := writeContactListConfig(t, srv.URL)

	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"contact", "list", "--all", "--config", cfgPath, "--agent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput=%s", err, out.String())
	}

	// Parse the --agent result envelope and assert both accumulated pages
	// are present regardless of JSON spacing/indentation.
	var parsed map[string]any
	if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
		t.Fatalf("parse output JSON: %v\noutput=%s", err, out.String())
	}
	results, ok := parsed["results"].(map[string]any)
	if !ok {
		t.Fatalf("no results envelope in output: %s", out.String())
	}
	items, ok := results["items"].([]any)
	if !ok || len(items) != 2 {
		t.Fatalf("items = %v, want 2 accumulated items", results["items"])
	}
	idSet := map[string]bool{}
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			idSet[fmt.Sprintf("%v", m["id"])] = true
		}
	}
	if !idSet["1"] || !idSet["2"] {
		t.Fatalf("output missing both accumulated ids (have %v):\n%s", idSet, out.String())
	}
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2\noutput=%s", len(requests), out.String())
	}
	// The cursor advances through the query string; Respond.io silently
	// ignores a body cursorId, so a body-threaded cursor loops on page one.
	q, err := url.ParseQuery(queries[1])
	if err != nil {
		t.Fatalf("parse second request query %q: %v", queries[1], err)
	}
	if got := q.Get("cursorId"); got != "2" {
		t.Fatalf("second request cursorId query param = %q, want %q (query=%q)", got, "2", queries[1])
	}
	if _, ok := requests[1]["cursorId"]; ok {
		t.Fatalf("second request must not carry a body cursorId: %v", requests[1])
	}
	// First request must NOT carry a cursorId at all.
	if q0, err := url.ParseQuery(queries[0]); err != nil {
		t.Fatalf("parse first request query %q: %v", queries[0], err)
	} else if q0.Has("cursorId") {
		t.Fatalf("first request carried an unexpected cursorId query param: %q", queries[0])
	}
	if _, ok := requests[0]["cursorId"]; ok {
		t.Fatalf("first request carried an unexpected cursorId: %v", requests[0])
	}
	// Every request must carry the wildcard search/filter/timezone trio the
	// API rejects the call without.
	for i, body := range requests {
		for _, key := range []string{"search", "filter", "timezone"} {
			if _, ok := body[key]; !ok {
				t.Fatalf("request %d body missing required %q: %v", i, key, body)
			}
		}
	}
}

// TestContactList_AllPaginates_URLValuedNextCursor covers the live Respond.io
// shape where pagination.next is a full URL rather than a bare integer: the
// loop must pull cursorId out of that URL's query string and keep paging.
func TestContactList_AllPaginates_URLValuedNextCursor(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.RawQuery)
		if len(queries) == 1 {
			w.Write([]byte(`{"items":[{"id":1}],"pagination":{"next":"https://api.respond.io/v2/contact/list?limit=100&cursorId=77"}}`))
			return
		}
		w.Write([]byte(`{"items":[{"id":2}],"pagination":{"next":null}}`))
	}))
	defer srv.Close()

	cfgPath := writeContactListConfig(t, srv.URL)

	cmd := RootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"contact", "list", "--all", "--config", cfgPath, "--agent"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v\noutput=%s", err, out.String())
	}

	if len(queries) != 2 {
		t.Fatalf("requests = %d, want 2 (URL-valued cursor must advance the loop)\noutput=%s", len(queries), out.String())
	}
	q, err := url.ParseQuery(queries[1])
	if err != nil {
		t.Fatalf("parse second request query %q: %v", queries[1], err)
	}
	if got := q.Get("cursorId"); got != "77" {
		t.Fatalf("second request cursorId = %q, want %q (query=%q)", got, "77", queries[1])
	}
}

// TestContactListNextCursor covers the cursor shapes Respond.io returns in
// pagination.next, including the terminal values that must stop the loop.
func TestContactListNextCursor(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"integer", `5`, "5"},
		{"zero terminates", `0`, ""},
		{"negative terminates", `-1`, ""},
		{"null terminates", `null`, ""},
		{"absent terminates", ``, ""},
		{"empty string terminates", `""`, ""},
		{"absolute url", `"https://api.respond.io/v2/contact/list?limit=100&cursorId=42"`, "42"},
		{"relative url", `"/v2/contact/list?cursorId=9"`, "9"},
		{"url without cursor terminates", `"https://api.respond.io/v2/contact/list?limit=100"`, ""},
		{"numeric string", `"12"`, "12"},
		{"opaque token terminates", `"abc"`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := contactListNextCursor(json.RawMessage(tc.raw))
			if got != tc.want {
				t.Fatalf("contactListNextCursor(%s) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// TestContactList_AllDryRun_MatchesSingleShot asserts that adding --all to a
// dry run does not change the output: --all must not enter the accumulation
// loop under --dry-run (which would replace the dry-run sentinel with
// {"items":null}) and must not issue any real request.
func TestContactList_AllDryRun_MatchesSingleShot(t *testing.T) {
	var reqs int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&reqs, 1)
		w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()
	cfgPath := writeContactListConfig(t, srv.URL)

	dataOf := func(extra ...string) []byte {
		t.Helper()
		cmd := RootCmd()
		var out bytes.Buffer
		var eout bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&eout)
		args := []string{"contact", "list", "--config", cfgPath, "--json", "--dry-run"}
		args = append(args, extra...)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute %v: %v\n%s", args, err, eout.String())
		}
		var parsed map[string]any
		if err := json.Unmarshal(out.Bytes(), &parsed); err != nil {
			t.Fatalf("parse %v output: %v\n%s", args, err, out.String())
		}
		data, ok := parsed["data"]
		if !ok {
			t.Fatalf("no data key in %v output: %s", args, out.String())
		}
		b, _ := json.Marshal(data)
		return b
	}

	base := dataOf()
	withAll := dataOf("--all")
	if !bytes.Equal(base, withAll) {
		t.Fatalf("--all --dry-run data %s != single-shot data %s", withAll, base)
	}
	if !bytes.Contains(base, []byte(`"dry_run":true`)) {
		t.Fatalf("expected dry_run sentinel in data, got %s", base)
	}
	if got := atomic.LoadInt32(&reqs); got != 0 {
		t.Fatalf("dry run issued %d real request(s), want 0", got)
	}
}

// endlessContactPager always returns a non-zero next cursor so a bounded loop
// necessarily exhausts its page cap.
type endlessContactPager struct{ n int }

func (p *endlessContactPager) PostQueryWithParams(_ context.Context, _ string, _ map[string]string, _ any) (json.RawMessage, int, error) {
	p.n++
	return json.RawMessage(fmt.Sprintf(`{"items":[{"id":%d}],"pagination":{"next":%d}}`, p.n, p.n+1)), 200, nil
}

// TestContactList_All_CapHitEmitsTruncationWarning drives fetchAllContactPages
// past its page ceiling (the API keeps returning a next cursor) and asserts a
// truncation warning is emitted instead of silently dropping pages.
func TestContactList_All_CapHitEmitsTruncationWarning(t *testing.T) {
	pager := &endlessContactPager{}
	cmd := &cobra.Command{}
	cmd.SetOut(io.Discard)
	flags := &rootFlags{}
	var stderr bytes.Buffer
	const maxPages = 3

	if _, _, _, err := fetchAllContactPages(pager, cmd, flags, &stderr, "/contact/list", map[string]string{}, map[string]any{}, maxPages); err != nil {
		t.Fatalf("fetchAllContactPages: %v", err)
	}
	if pager.n != maxPages {
		t.Fatalf("pages fetched = %d, want %d (loop should exhaust the cap)", pager.n, maxPages)
	}
	if !strings.Contains(stderr.String(), `"reason":"pagination_cap_hit"`) {
		t.Fatalf("truncation warning not emitted:\n%s", stderr.String())
	}
}

// TestContactList_All_PartialFailure_NonZeroExit asserts that a partial failure
// reported on a non-final page still drives a non-zero exit after all pages are
// fetched (it must not be silently downgraded to exit 0), the warning is not
// printed twice, and the loop still continues past the failing page.
func TestContactList_All_PartialFailure_NonZeroExit(t *testing.T) {
	var reqs int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&reqs, 1)
		if n == 1 {
			w.Write([]byte(`{"items":[{"id":1}],"pagination":{"next":2},"partialFailureError":{"message":"some ops failed","code":1}}`))
			return
		}
		w.Write([]byte(`{"items":[{"id":2}]}`))
	}))
	defer srv.Close()
	cfgPath := writeContactListConfig(t, srv.URL)

	cmd := RootCmd()
	var out bytes.Buffer
	var eout bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&eout)
	cmd.SetArgs([]string{"contact", "list", "--all", "--config", cfgPath, "--agent"})
	err := cmd.Execute()
	if err == nil {
		t.Fatalf("expected non-zero partial-failure outcome, got nil (reqs=%d)\nout=%s\nerr=%s", atomic.LoadInt32(&reqs), out.String(), eout.String())
	}
	if !strings.Contains(err.Error(), "partial failure") {
		t.Fatalf("exit error %q lacks partial-failure marker", err.Error())
	}
	if got := strings.Count(eout.String(), "partial failure detected"); got != 1 {
		t.Fatalf("partial-failure warning printed %d times, want 1:\n%s", got, eout.String())
	}
	if got := atomic.LoadInt32(&reqs); got != 2 {
		t.Fatalf("requests = %d, want 2 (loop must continue past the failing page)", got)
	}
}
