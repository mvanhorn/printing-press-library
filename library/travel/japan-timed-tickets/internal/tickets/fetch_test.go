// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package tickets

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/japan-timed-tickets/internal/cliutil"
)

// testFetcher keeps the real HTTP client but turns off pacing and waits so
// the tests run fast.
func testFetcher(maxRequests int) *Fetcher {
	f := NewFetcher(5*time.Second, MaxPerHostRate, maxRequests)
	f.noPacing = true
	f.backoff = func(int) time.Duration { return time.Millisecond }
	f.maxRetryWait = time.Millisecond
	return f
}

func TestFetcherSurfacesRateLimit(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, "japan-timed-tickets-pp-cli/") || strings.Contains(ua, "Mozilla") {
			t.Errorf("User-Agent = %q, want the honest CLI token", ua)
		}
		w.Header().Set("Retry-After", "1")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()
	f := testFetcher(10)
	_, err := f.GetJSON(context.Background(), srv.URL+"/api/v1/configurations")
	var rl *cliutil.RateLimitError
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v, want *cliutil.RateLimitError (not an empty result)", err)
	}
	if hits != maxAttempts {
		t.Errorf("attempts = %d, want %d", hits, maxAttempts)
	}
}

func TestFetcherRefusesWaitingRoom(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://waiting.example.test/?c=webket", http.StatusFound)
	}))
	defer srv.Close()
	f := testFetcher(10)
	_, err := f.GetHTML(context.Background(), srv.URL)
	if !errors.Is(err, ErrWaitingRoom) {
		t.Fatalf("err = %v, want ErrWaitingRoom (never follow into a queue)", err)
	}
}

func TestFetcherRequestBudget(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(`{}`)) }))
	defer srv.Close()
	f := testFetcher(2)
	for i := 0; i < 2; i++ {
		if _, err := f.GetJSON(context.Background(), srv.URL); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.GetJSON(context.Background(), srv.URL); err == nil || !strings.Contains(err.Error(), "budget") {
		t.Errorf("third request err = %v, want budget error", err)
	}
	if n, b := f.Stats(); n != 2 || b != 4 {
		t.Errorf("stats = %d requests, %d bytes", n, b)
	}
}

func TestFetcherRefusesCrossHostRedirect(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("elsewhere")) }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/same" {
			_, _ = w.Write([]byte("ok"))
			return
		}
		if r.URL.Path == "/hop" {
			http.Redirect(w, r, "/same", http.StatusFound)
			return
		}
		http.Redirect(w, r, strings.Replace(other.URL, "127.0.0.1", "localhost", 1), http.StatusFound)
	}))
	defer srv.Close()
	f := testFetcher(10)
	if body, err := f.GetHTML(context.Background(), srv.URL+"/hop"); err != nil || string(body) != "ok" {
		t.Fatalf("same-host redirect: %q, %v", body, err)
	}
	if _, err := f.GetHTML(context.Background(), srv.URL+"/away"); err == nil || !strings.Contains(err.Error(), "cross-host") {
		t.Fatalf("cross-host redirect err = %v", err)
	}
}

func TestFetcherRejectsUntrustedTLS(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("{}")) }))
	defer srv.Close()
	f := NewFetcher(5*time.Second, 50, 10)
	if _, err := f.GetJSON(context.Background(), srv.URL); err == nil {
		t.Fatal("self-signed TLS server must be rejected")
	}
}

func TestStripControl(t *testing.T) {
	if got := StripControl("a\x1b[31mb\x07c\u0085d\te\n"); got != "a[31mbcd\te\n" {
		t.Errorf("StripControl = %q", got)
	}
}

func TestFetcherRedirectLoopIsFinal(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer srv.Close()
	f := testFetcher(20)
	if _, err := f.GetHTML(context.Background(), srv.URL); !errors.Is(err, errRedirectRefused) {
		t.Fatalf("err = %v, want errRedirectRefused", err)
	}
	if hits > maxRedirects+1 {
		t.Errorf("hits = %d: a redirect loop must not be retried", hits)
	}
}
