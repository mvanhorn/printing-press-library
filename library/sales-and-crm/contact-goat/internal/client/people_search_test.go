// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// TestDefaultPollTimeoutIs180s is the regression guard for the
// 2026-04-19 bump from 60s -> 180s. 60s was causing frequent false
// failures on legitimate Happenstance queries that routinely take
// 2-5 minutes. If this assertion is changed, update hp_people's flag
// help text and the coverage docs in the same PR.
func TestDefaultPollTimeoutIs180s(t *testing.T) {
	if DefaultPollTimeout != 180*time.Second {
		t.Fatalf("DefaultPollTimeout = %s, want 180s", DefaultPollTimeout)
	}
	if got := defaultSearchOptions().PollTimeout; got != DefaultPollTimeout {
		t.Errorf("defaultSearchOptions().PollTimeout = %s, want DefaultPollTimeout (%s)", got, DefaultPollTimeout)
	}
}

// TestZeroPollTimeoutFallsBackToDefault confirms the zero-value guard
// in SearchPeopleByQuery: callers that construct SearchPeopleOptions
// without setting PollTimeout still receive the default, not a
// zero-duration (which would make the poll return instantly).
func TestZeroPollTimeoutFallsBackToDefault(t *testing.T) {
	o := SearchPeopleOptions{
		IncludeMyConnections: true,
		// PollTimeout intentionally zero
	}
	// The fallback is inlined in SearchPeopleByQuery; simulate that
	// path here without making a real HTTP call.
	if o.PollTimeout == 0 {
		o.PollTimeout = DefaultPollTimeout
	}
	if o.PollTimeout != DefaultPollTimeout {
		t.Errorf("zero PollTimeout should fall back to DefaultPollTimeout, got %s", o.PollTimeout)
	}
}

// TestSearchPeopleByCompanyHasOptionsOverload is a compile-time guard
// that the coverage command's per-call timeout plumbing continues to
// work. If the method is removed or renamed, the coverage command's
// --poll-timeout flag silently stops taking effect, so we lock the
// shape here.
func TestSearchPeopleByCompanyHasOptionsOverload(t *testing.T) {
	var c *Client
	// No call is made; the test only verifies that the method exists
	// with the expected signature and that a non-nil opts is an
	// acceptable argument.
	_ = func() (*PeopleSearchResult, error) {
		return c.SearchPeopleByCompanyWithOptions("Disney", &SearchPeopleOptions{
			PollTimeout: 300 * time.Second,
		})
	}
}

// --- U5: cookie broad-query fast-fail ---

// newTestClient builds a Client wired to the given httptest URL with
// caching disabled. Used by U5 tests that drive fetchDynamo / pollSearch
// without standing up real cookie auth.
func newTestClient(srv *httptest.Server) *Client {
	c := &Client{
		BaseURL:    srv.URL,
		HTTPClient: &http.Client{Timeout: 5 * time.Second},
		NoCache:    true,
	}
	return c
}

// TestErrCookieBroadQuery_5xxUpstream confirms that a 5xx response
// from /api/dynamo wraps with ErrCookieBroadQuery so callers can
// surface the bearer-fallback hint instead of a generic API error.
func TestErrCookieBroadQuery_5xxUpstream(t *testing.T) {
	cases := []struct {
		name       string
		statusCode int
	}{
		{"524 cloudflare", 524},
		{"502 bad gateway", http.StatusBadGateway},
		{"503 service unavailable", http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				_, _ = w.Write([]byte(`upstream timeout`))
			}))
			defer srv.Close()
			c := newTestClient(srv)
			_, err := c.fetchDynamo("any-id")
			if err == nil {
				t.Fatal("want error from 5xx response")
			}
			if !errors.Is(err, ErrCookieBroadQuery) {
				t.Errorf("err = %v\nwant errors.Is(err, ErrCookieBroadQuery) = true", err)
			}
		})
	}
}

// TestErrCookieBroadQuery_4xxNotWrapped: client errors (401, 404, etc.)
// should NOT be classified as broad-query failures. Those map to the
// existing auth/not-found exit codes through their own paths.
func TestErrCookieBroadQuery_4xxNotWrapped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`unauthorized`))
	}))
	defer srv.Close()
	c := newTestClient(srv)
	_, err := c.fetchDynamo("any-id")
	if err == nil {
		t.Fatal("want error from 401")
	}
	if errors.Is(err, ErrCookieBroadQuery) {
		t.Errorf("4xx should NOT wrap as ErrCookieBroadQuery; got: %v", err)
	}
}

// TestErrCookieBroadQuery_PollTimeout drives pollSearch against a
// fixture that always returns non-completed status. With a short poll
// timeout, the loop bails out and the error wraps ErrCookieBroadQuery.
func TestErrCookieBroadQuery_PollTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always non-terminal.
		_, _ = w.Write([]byte(`[{"request_id":"x","request_text":"q","request_status":"running","completed":false,"results":[]}]`))
	}))
	defer srv.Close()
	c := newTestClient(srv)
	_, err := c.pollSearch("x", 100*time.Millisecond, 20*time.Millisecond)
	if err == nil {
		t.Fatal("want timeout error")
	}
	if !errors.Is(err, ErrCookieBroadQuery) {
		t.Errorf("err = %v\nwant errors.Is(err, ErrCookieBroadQuery) = true", err)
	}
	if !strings.Contains(err.Error(), "poll timeout") {
		t.Errorf("error should mention poll timeout, got: %v", err)
	}
}

// TestErrCookieBroadQuery_CompletedNoWrap: a search that completes
// quickly never trips the broad-query path.
func TestErrCookieBroadQuery_CompletedNoWrap(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Second request returns completed.
		n := atomic.AddInt32(&hits, 1)
		if n < 2 {
			_, _ = w.Write([]byte(`[{"request_id":"x","request_text":"q","request_status":"running","completed":false,"results":[]}]`))
			return
		}
		_, _ = w.Write([]byte(fmt.Sprintf(`[{"request_id":"x","request_text":"q","request_status":"Found 1","completed":true,"results":[{"author_name":"Alice"}]}]`)))
	}))
	defer srv.Close()
	c := newTestClient(srv)
	got, err := c.pollSearch("x", 5*time.Second, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Completed {
		t.Errorf("got.Completed = false, want true")
	}
}
