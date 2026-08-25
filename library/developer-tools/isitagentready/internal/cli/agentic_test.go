// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/isitagentready/internal/cliutil"
)

// problem404 is the literal upstream application/problem+json body for a
// never-scanned domain (verified, do not alter).
const problem404 = `{"type":"https://is-agentic.com/docs#report-not-found","title":"Completed report not found",
 "status":404,"detail":"No completed Is Agentic report is stored for this URL.",
 "instance":"...","code":"report_not_found",
 "resolution":"Start a scan at https://is-agentic.com/scan/mvanhorn.com, wait for it to complete, and retry this request.",
 "documentation_url":"https://is-agentic.com/docs#errors"}`

// problem400 is the literal upstream application/problem+json body for an
// invalid URL.
const problem400 = `{"type":"https://is-agentic.com/docs#invalid-url","title":"Invalid public URL","status":400,
 "detail":"Enter a URL to scan.","instance":"...","code":"invalid_url",
 "resolution":"Pass one public HTTP or HTTPS URL in the required url query parameter.",
 "documentation_url":"https://is-agentic.com/docs#errors"}`

func TestClassifyAgenticError(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		body     string
		wantCode int
		wantSub  string // a substring that must appear verbatim in the message
	}{
		{"404 report_not_found", 404, problem404, 3, "Start a scan at https://is-agentic.com/scan/mvanhorn.com, wait for it to complete, and retry this request."},
		{"400 invalid_url", 400, problem400, 2, "Pass one public HTTP or HTTPS URL in the required url query parameter."},
		{"429 rate limit", 429, `{"type":"x","title":"Too Many Requests","status":429,"detail":"Rate limit exceeded","code":"rate_limited"}`, 7, "Rate limit exceeded"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			apiErr := &client.APIError{Method: "GET", Path: "/api/v1/report", StatusCode: tc.status, Body: tc.body}
			err := classifyAgenticError(apiErr, nil)
			if got := ExitCode(err); got != tc.wantCode {
				t.Fatalf("ExitCode = %d, want %d (err=%v)", got, tc.wantCode, err)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Fatalf("message %q does not contain upstream substring %q verbatim", err.Error(), tc.wantSub)
			}
		})
	}
}

// TestClassifyAgenticErrorFallback covers the non-problem+json fallback:
// when the body does not parse, classifyAgenticError hands off to
// classifyAPIError so the generic HTTP classification (and its exit code)
// still applies instead of silently returning a no-data result.
func TestClassifyAgenticErrorFallback(t *testing.T) {
	apiErr := &client.APIError{Method: "GET", Path: "/api/v1/report", StatusCode: 500, Body: "not json at all"}
	err := classifyAgenticError(apiErr, nil)
	// 500 with unparseable body falls through to classifyAPIError -> apiErr.
	if got := ExitCode(err); got != 5 {
		t.Fatalf("ExitCode = %d, want 5 (apiErr fallback), err=%v", got, err)
	}
}

// TestAgenticClientIsProcessWideSingleton is the regression test for the
// fan-out rate-limit bug: newAgenticClient used to construct a fresh
// client.Client per call, and client.New builds a fresh AdaptiveLimiter each
// time. Each of the 4 compare/batch fan-out workers therefore got its OWN
// limiter and all 4 could fire at once against the host's advertised 2 req/s
// public-report budget.
//
// One client == one limiter, so asserting pointer identity across concurrent
// callers asserts a single shared budget.
func TestAgenticClientIsProcessWideSingleton(t *testing.T) {
	seedStore(t)
	resetAgenticClient()
	t.Cleanup(resetAgenticClient)

	flags := &rootFlags{timeout: 5 * time.Second, noCache: true}

	const workers = 8
	got := make([]*client.Client, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c, err := flags.newAgenticClient()
			if err != nil {
				t.Errorf("newAgenticClient: %v", err)
				return
			}
			got[i] = c
		}(i)
	}
	wg.Wait()

	for i, c := range got {
		if c == nil {
			t.Fatalf("worker %d got no client", i)
		}
		if c != got[0] {
			t.Fatalf("worker %d got a different client than worker 0: each client carries its own limiter, so the fan-out would not share the 2 req/s budget", i)
		}
	}
	if rate := got[0].RateLimit(); rate != agenticRateLimit {
		t.Fatalf("shared client rate = %v, want the host budget %v", rate, agenticRateLimit)
	}
}

// TestAgenticFanoutSharesOneRateLimit proves the shared limiter actually
// throttles: a concurrent fan-out of 3 fetchAgenticReport calls must arrive at
// the host spaced by the 2 req/s budget (500ms apart), not all at once.
func TestAgenticFanoutSharesOneRateLimit(t *testing.T) {
	seedStore(t)
	resetAgenticClient()
	t.Cleanup(resetAgenticClient)

	var mu sync.Mutex
	var arrivals []time.Time
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		arrivals = append(arrivals, time.Now())
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"target":%q,"score":50,"score_label":"Partial","scanned_at":"2026-08-20T09:00:00Z"}`,
			r.URL.Query().Get("url"))
	}))
	defer srv.Close()

	flags := &rootFlags{timeout: 30 * time.Second, noCache: true}
	// Retarget the memoized client at the test server BEFORE any goroutine
	// starts, so the concurrent fetches below reuse this same client (and its
	// limiter) rather than racing on the field.
	c, err := flags.newAgenticClient()
	if err != nil {
		t.Fatalf("newAgenticClient: %v", err)
	}
	c.BaseURL = srv.URL

	urls := []string{
		"https://a.example",
		"https://b.example",
		"https://c.example",
	}
	start := time.Now()
	results, ferrs := cliutil.FanoutRun(context.Background(), urls,
		func(u string) string { return u },
		func(ctx context.Context, u string) (json.RawMessage, error) {
			return fetchAgenticReport(ctx, flags, u)
		})
	elapsed := time.Since(start)

	if len(ferrs) != 0 {
		t.Fatalf("fan-out errors: %v", ferrs)
	}
	if len(results) != len(urls) {
		t.Fatalf("got %d results, want %d", len(results), len(urls))
	}

	mu.Lock()
	defer mu.Unlock()
	if len(arrivals) != len(urls) {
		t.Fatalf("host saw %d requests, want %d", len(arrivals), len(urls))
	}
	sort.Slice(arrivals, func(i, j int) bool { return arrivals[i].Before(arrivals[j]) })

	// 2 req/s => 500ms between requests. Allow generous scheduler slack; the
	// bug being guarded against produced gaps near zero, not near 400ms.
	const minGap = 350 * time.Millisecond
	for i := 1; i < len(arrivals); i++ {
		if gap := arrivals[i].Sub(arrivals[i-1]); gap < minGap {
			t.Fatalf("requests %d and %d arrived %v apart (want >= %v): the fan-out workers are not sharing one limiter",
				i-1, i, gap, minGap)
		}
	}
	// Two gaps at 500ms => ~1s total; a per-worker limiter finishes near 0s.
	if elapsed < 2*minGap {
		t.Fatalf("fan-out finished in %v, too fast for a shared %v req/s budget", elapsed, agenticRateLimit)
	}
}
