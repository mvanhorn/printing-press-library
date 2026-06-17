package hn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// TestBuildURLDisablesTypoTolerance is a fast unit check on the URL helper.
func TestBuildURLDisablesTypoTolerance(t *testing.T) {
	for _, endpoint := range []string{"search", "search_by_date"} {
		q := url.Values{}
		q.Set("query", "pwcommunications")
		q.Set("tags", "story")
		got := buildURL(hnBaseURL, endpoint, q)
		if !strings.Contains(got, "typoTolerance=false") {
			t.Fatalf("%s: typo tolerance not disabled: %s", endpoint, got)
		}
		if !strings.HasPrefix(got, hnBaseURL+endpoint+"?") {
			t.Fatalf("%s: unexpected base/endpoint: %s", endpoint, got)
		}
	}
}

// TestSearchMethodsDisableTypoTolerance drives every public search method through
// a fake server and asserts the REAL outgoing request carries typoTolerance=false.
// This guards the call sites, not just the helper: a refactor that bypassed
// buildURL would fail here. It is the regression guard for the company
// domain-token false-positive bug, where a long token like "pwcommunications"
// (16 chars) fuzzy-matched the ordinary word "communications" under Algolia's
// default two-typo allowance and surfaced an unrelated story.
func TestSearchMethodsDisableTypoTolerance(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"hits":[],"nbHits":0}`))
	}))
	defer srv.Close()

	c := &Client{HTTP: srv.Client(), baseURL: srv.URL + "/"}
	ctx := context.Background()

	cases := []struct {
		name string
		call func() error
	}{
		{"SearchAll", func() error { _, err := c.SearchAll(ctx, "pwcommunications", 5); return err }},
		{"SearchShowHN", func() error { _, err := c.SearchShowHN(ctx, "pwcommunications", 5); return err }},
		{"SearchByDate", func() error { _, err := c.SearchByDate(ctx, "pwcommunications", 5); return err }},
	}
	for _, tc := range cases {
		got = nil
		if err := tc.call(); err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if got.Get("typoTolerance") != "false" {
			t.Fatalf("%s: outgoing request missing typoTolerance=false (got %q)", tc.name, got.Get("typoTolerance"))
		}
		if got.Get("query") != "pwcommunications" {
			t.Fatalf("%s: query not preserved: %q", tc.name, got.Get("query"))
		}
	}
}
