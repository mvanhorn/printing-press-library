// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/soccer-goat/internal/config"
)

// newFailoverClient builds a client over an ordered candidate list with the
// response cache disabled, so tests exercise the wire path deterministically
// and never touch the real user cache dir.
func newFailoverClient(bases ...string) *Client {
	c := New(&config.Config{BaseURLs: bases}, 2*time.Second, 0)
	c.NoCache = true
	return c
}

// TestFailover_5xxThenSuccess: a primary that 5xxes after retries hands off to
// the next candidate, whose 200 body is returned.
func TestFailover_5xxThenSuccess(t *testing.T) {
	t.Setenv(cliutil.DogfoodEnvVar, "1") // 0 retries -> fast, deterministic failover

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer down.Close()

	var upHits int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upHits, 1)
		_, _ = w.Write([]byte(`{"results":[{"id":"1"}]}`))
	}))
	defer up.Close()

	c := newFailoverClient(down.URL, up.URL)
	body, err := c.Get(context.Background(), "/players/search/x", nil)
	if err != nil {
		t.Fatalf("expected failover success, got error: %v", err)
	}
	var parsed struct {
		Results []struct {
			ID string `json:"id"`
		} `json:"results"`
	}
	if jErr := json.Unmarshal(body, &parsed); jErr != nil || len(parsed.Results) != 1 {
		t.Fatalf("unexpected body %s (err %v)", body, jErr)
	}
	if got := atomic.LoadInt32(&upHits); got != 1 {
		t.Fatalf("secondary hits = %d, want 1", got)
	}
}

// TestFailover_4xxDoesNotFailOver: a 4xx is the source answering, not a source
// failure. It must return immediately without probing the next candidate — a
// "player not found" must never trigger a mirror sweep.
func TestFailover_4xxDoesNotFailOver(t *testing.T) {
	t.Setenv(cliutil.DogfoodEnvVar, "1")

	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"not found"}`))
	}))
	defer first.Close()

	var secondHits int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&secondHits, 1)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer second.Close()

	c := newFailoverClient(first.URL, second.URL)
	_, err := c.Get(context.Background(), "/players/search/x", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusNotFound {
		t.Fatalf("want APIError 404, got %v", err)
	}
	if got := atomic.LoadInt32(&secondHits); got != 0 {
		t.Fatalf("secondary must not be hit on 4xx, hits = %d", got)
	}
}

// TestFailover_200EmptyDoesNotFailOver: a 200 with an empty result set is a
// working source answering, not a failure — return it, do not fail over.
func TestFailover_200EmptyDoesNotFailOver(t *testing.T) {
	t.Setenv(cliutil.DogfoodEnvVar, "1")

	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer first.Close()

	var secondHits int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&secondHits, 1)
		_, _ = w.Write([]byte(`{"results":[{"id":"9"}]}`))
	}))
	defer second.Close()

	c := newFailoverClient(first.URL, second.URL)
	body, err := c.Get(context.Background(), "/players/search/x", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != `{"results":[]}` {
		t.Fatalf("want primary empty body, got %s", body)
	}
	if got := atomic.LoadInt32(&secondHits); got != 0 {
		t.Fatalf("secondary must not be hit on empty 200, hits = %d", got)
	}
}

// TestFailover_TransportErrorThenSuccess: a candidate that refuses connections
// is a source failure — fail over to the next candidate.
func TestFailover_TransportErrorThenSuccess(t *testing.T) {
	t.Setenv(cliutil.DogfoodEnvVar, "1")

	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close() // now refuses connections -> transport error

	var upHits int32
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upHits, 1)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer up.Close()

	c := newFailoverClient(deadURL, up.URL)
	if _, err := c.Get(context.Background(), "/x", nil); err != nil {
		t.Fatalf("expected failover past transport error, got %v", err)
	}
	if got := atomic.LoadInt32(&upHits); got != 1 {
		t.Fatalf("secondary hits = %d, want 1", got)
	}
}

// TestFailover_AllSourcesDown: when every candidate 5xxes, the last error is
// returned as a >=500 APIError (which the CLI layer maps to the friendly hint).
func TestFailover_AllSourcesDown(t *testing.T) {
	t.Setenv(cliutil.DogfoodEnvVar, "1")

	mk := func() *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
		}))
	}
	s1, s2 := mk(), mk()
	defer s1.Close()
	defer s2.Close()

	c := newFailoverClient(s1.URL, s2.URL)
	_, err := c.Get(context.Background(), "/x", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode < 500 {
		t.Fatalf("want >=500 APIError after exhausting sources, got %v", err)
	}
}

// TestFailover_AllTransportDown: when every candidate refuses connections, the
// exhausted error is a *SourceUnavailableError (not a bare dial error), so the
// CLI layer can surface the same outage hint it gives for all-5xx.
func TestFailover_AllTransportDown(t *testing.T) {
	t.Setenv(cliutil.DogfoodEnvVar, "1")

	mkDead := func() string {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		u := s.URL
		s.Close() // refuses connections
		return u
	}
	c := newFailoverClient(mkDead(), mkDead())
	_, err := c.Get(context.Background(), "/x", nil)
	var srcErr *SourceUnavailableError
	if !errors.As(err, &srcErr) {
		t.Fatalf("want *SourceUnavailableError after all transport failures, got %T: %v", err, err)
	}
}

// TestFailover_SingleSource: a one-element list is a plain single-source client
// (an override); it is hit exactly once under dogfood and never fans out.
func TestFailover_SingleSource(t *testing.T) {
	t.Setenv(cliutil.DogfoodEnvVar, "1")

	var hits int32
	only := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer only.Close()

	c := newFailoverClient(only.URL)
	_, err := c.Get(context.Background(), "/x", nil)
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want 500, got %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("single source hit count = %d, want 1 (0 retries under dogfood)", got)
	}
}

func TestFailover_CrossOriginStripsSensitiveHeaders(t *testing.T) {
	t.Setenv(cliutil.DogfoodEnvVar, "1")

	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer primary.Close()

	received := make(chan http.Header, 1)
	mirror := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Header.Clone()
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer mirror.Close()

	c := New(&config.Config{
		BaseURLs:      []string{primary.URL, mirror.URL},
		AuthHeaderVal: "Bearer primary-secret",
		Headers:       map[string]string{"X-Config-Secret": "config-secret"},
	}, 2*time.Second, 0)
	c.NoCache = true
	_, _, err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, map[string]string{"X-Endpoint-Secret": "endpoint-secret"})
	if err != nil {
		t.Fatalf("expected cross-origin failover success, got %v", err)
	}

	headers := <-received
	for _, name := range []string{"Authorization", "X-Config-Secret", "X-Endpoint-Secret"} {
		if got := headers.Get(name); got != "" {
			t.Fatalf("cross-origin mirror received %s=%q", name, got)
		}
	}
}

func TestFailover_SameOriginPreservesConfiguredHeaders(t *testing.T) {
	t.Setenv(cliutil.DogfoodEnvVar, "1")

	received := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/primary/x" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		received <- r.Header.Clone()
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	c := New(&config.Config{
		BaseURLs:      []string{server.URL + "/primary", server.URL + "/mirror"},
		AuthHeaderVal: "Bearer trusted-secret",
		Headers:       map[string]string{"X-Config-Value": "config-value"},
	}, 2*time.Second, 0)
	c.NoCache = true
	_, _, err := c.do(context.Background(), http.MethodGet, "/x", nil, nil, map[string]string{"X-Endpoint-Value": "endpoint-value"})
	if err != nil {
		t.Fatalf("expected same-origin failover success, got %v", err)
	}

	headers := <-received
	for name, want := range map[string]string{
		"Authorization":    "Bearer trusted-secret",
		"X-Config-Value":   "config-value",
		"X-Endpoint-Value": "endpoint-value",
	} {
		if got := headers.Get(name); got != want {
			t.Fatalf("same-origin mirror %s=%q, want %q", name, got, want)
		}
	}
}
