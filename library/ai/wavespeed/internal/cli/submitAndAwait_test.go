// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/wavespeed/internal/config"
)

// newTestClient builds a client pointed at an httptest server with caching off
// so each request hits the test transport.
func newTestClient(baseURL string) *client.Client {
	c := client.New(&config.Config{BaseURL: baseURL, AccessToken: "test-token"}, 5*time.Second, 0)
	c.NoCache = true
	return c
}

// captureStdio runs fn with os.Stdout/os.Stderr redirected to a pipe and
// returns whatever was written. submitAndAwait must write nothing.
func captureStdio(t *testing.T, fn func()) string {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = w, w
	defer func() { os.Stdout, os.Stderr = origOut, origErr }()
	done := make(chan string, 1)
	go func() {
		buf := make([]byte, 4096)
		n, _ := r.Read(buf)
		done <- string(buf[:n])
	}()
	fn()
	w.Close()
	select {
	case s := <-done:
		return s
	case <-time.After(time.Second):
		return ""
	}
}

func TestSubmitAndAwaitReturnsStructuredDataNoPrints(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"task-1","status":"created"}}`))
	}))
	defer ts.Close()
	c := newTestClient(ts.URL)

	var res submitResult
	var err error
	out := captureStdio(t, func() {
		res, err = submitAndAwait(context.Background(), c, submitRequest{
			modelID: "wavespeed-ai/flux-dev",
			inputs:  map[string]any{"prompt": "hi"},
		})
	})
	if err != nil {
		t.Fatalf("submitAndAwait error: %v", err)
	}
	if out != "" {
		t.Fatalf("submitAndAwait wrote to stdio: %q", out)
	}
	if len(res.Result) == 0 {
		t.Fatalf("expected a result payload")
	}
	if res.Failed {
		t.Fatalf("status 'created' should not be a failure")
	}
}

func TestSubmitAndAwaitPollsToTerminal(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/predictions/task-1/result":
			_, _ = w.Write([]byte(`{"data":{"id":"task-1","status":"completed","outputs":["https://x/out.png"]}}`))
		default:
			_, _ = w.Write([]byte(`{"data":{"id":"task-1","status":"created"}}`))
		}
	}))
	defer ts.Close()
	c := newTestClient(ts.URL)

	res, err := submitAndAwait(context.Background(), c, submitRequest{
		modelID:     "wavespeed-ai/flux-dev",
		inputs:      map[string]any{"prompt": "hi"},
		wait:        true,
		waitTimeout: 5 * time.Second,
		pollInitial: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("submitAndAwait error: %v", err)
	}
	if res.Status != "completed" {
		t.Fatalf("status = %q, want completed", res.Status)
	}
	if res.Failed {
		t.Fatalf("completed prediction should not be Failed")
	}
}

// TestSubmitAndAwaitFailedPredictionIsNotError proves partial-failure
// classification: a prediction that reaches a failed terminal status surfaces
// via res.Failed, NOT as a transport error, so the attempt can still be
// recorded.
func TestSubmitAndAwaitFailedPredictionIsNotError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/predictions/task-1/result" {
			_, _ = w.Write([]byte(`{"data":{"id":"task-1","status":"failed","error":"bad prompt"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"id":"task-1","status":"created"}}`))
	}))
	defer ts.Close()
	c := newTestClient(ts.URL)

	res, err := submitAndAwait(context.Background(), c, submitRequest{
		modelID:     "wavespeed-ai/flux-dev",
		inputs:      map[string]any{"prompt": "hi"},
		wait:        true,
		waitTimeout: 5 * time.Second,
		pollInitial: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("failed prediction must not be a transport error, got: %v", err)
	}
	if !res.Failed {
		t.Fatalf("expected res.Failed for status 'failed'")
	}
	if res.Status != "failed" {
		t.Fatalf("status = %q, want failed", res.Status)
	}
}

func TestSubmitAndAwaitEstimatesPrice(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/model/pricing" {
			_, _ = w.Write([]byte(`{"data":{"price":0.42}}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":{"id":"task-1","status":"created"}}`))
	}))
	defer ts.Close()
	c := newTestClient(ts.URL)

	res, err := submitAndAwait(context.Background(), c, submitRequest{
		modelID:       "wavespeed-ai/flux-dev",
		inputs:        map[string]any{"prompt": "hi"},
		estimatePrice: true,
	})
	if err != nil {
		t.Fatalf("submitAndAwait error: %v", err)
	}
	if got := extractCostFromPricing(res.Pricing); got != 0.42 {
		t.Fatalf("price = %v, want 0.42", got)
	}
}

// TestSubmitAndAwaitPriceBestEffort proves a pricing-endpoint failure does NOT
// abort generation when priceBestEffort is set (producers), but does surface
// the error when it is not (run --price).
func TestSubmitAndAwaitPriceBestEffort(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/model/pricing" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":"task-1","status":"created"}}`))
	}))
	defer ts.Close()
	c := newTestClient(ts.URL)

	// Best-effort: pricing 5xx is swallowed, generation proceeds.
	res, err := submitAndAwait(context.Background(), c, submitRequest{
		modelID: "m", inputs: map[string]any{"prompt": "x"},
		estimatePrice: true, priceBestEffort: true,
	})
	if err != nil {
		t.Fatalf("best-effort pricing failure should not abort generation: %v", err)
	}
	if len(res.Result) == 0 {
		t.Fatalf("expected a generation result")
	}

	// Non-best-effort: the pricing failure is fatal (run --price semantics).
	if _, err := submitAndAwait(context.Background(), c, submitRequest{
		modelID: "m", inputs: map[string]any{"prompt": "x"}, estimatePrice: true,
	}); err == nil {
		t.Fatalf("expected pricing failure to surface when priceBestEffort is false")
	}
}

// TestRecordRunGenerationFailureIsReturnedNotPanicked proves a library write
// failure is surfaced as a returned error (which run logs and continues on),
// never a panic or command abort.
func TestRecordRunGenerationFailureIsReturnedNotPanicked(t *testing.T) {
	// Point the library DB at a path that cannot be created (a file used as a
	// directory component) so OpenLibrary fails.
	bad := t.TempDir() + "/not-a-dir"
	if err := os.WriteFile(bad, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WAVESPEED_LIBRARY_DB", bad+"/library.db")

	err := recordRunGeneration("wavespeed-ai/flux-dev", map[string]any{"prompt": "hi"}, submitResult{
		Result: []byte(`{"status":"completed"}`),
		Status: "completed",
	})
	if err == nil {
		t.Fatalf("expected a library record error for an unwritable path")
	}
}
