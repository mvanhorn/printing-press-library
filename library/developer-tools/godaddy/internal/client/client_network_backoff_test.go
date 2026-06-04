// Copyright 2026 zaydiscold. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/godaddy/internal/config"
)

// flakyRoundTripper returns a transport error for the first failCount
// calls, then a 200 response. Used to pin the network-error retry
// backoff path in do().
type flakyRoundTripper struct {
	calls     int
	failCount int
}

func (f *flakyRoundTripper) RoundTrip(_ *http.Request) (*http.Response, error) {
	f.calls++
	if f.calls <= f.failCount {
		return nil, errors.New("simulated transport failure")
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader([]byte("{}"))),
		Header:     http.Header{},
	}, nil
}

// TestClient_NetworkErrorRetriesWithBackoff asserts that a transport-level
// failure is retried (so transient outages recover) and that the retry
// applies a backoff delay rather than hammering the endpoint immediately.
// Before the fix the network-error branch `continue`d with no sleep,
// producing up to four rapid-fire retries.
func TestClient_NetworkErrorRetriesWithBackoff(t *testing.T) {
	rt := &flakyRoundTripper{failCount: 1}
	cfg := &config.Config{BaseURL: "http://example.test"}
	c := New(cfg, 5*time.Second, 0)
	c.HTTPClient = &http.Client{Transport: rt}
	c.NoCache = true

	start := time.Now()
	_, err := c.Get(context.Background(), "/v1/ping", nil)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("expected success after one transient failure, got: %v", err)
	}
	if rt.calls != 2 {
		t.Fatalf("expected 2 transport calls (1 fail + 1 retry), got %d", rt.calls)
	}
	// First retry uses 2^0 = 1s backoff. Allow slack for slow CI but
	// require a non-trivial delay so a no-backoff regression fails here.
	if elapsed < 900*time.Millisecond {
		t.Fatalf("expected backoff delay >= ~1s before retry, elapsed=%s", elapsed)
	}
}

// TestClient_NetworkErrorExhaustsRetries asserts that when every attempt
// fails at the transport layer the client gives up and returns the
// wrapped error (not a panic or infinite loop). The context cancels
// promptly so cumulative backoffs don't stretch the test.
func TestClient_NetworkErrorExhaustsRetries(t *testing.T) {
	rt := &flakyRoundTripper{failCount: 100}
	cfg := &config.Config{BaseURL: "http://example.test"}
	c := New(cfg, 5*time.Second, 0)
	c.HTTPClient = &http.Client{Transport: rt}
	c.NoCache = true

	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()

	_, err := c.Get(ctx, "/v1/ping", nil)
	if err == nil {
		t.Fatal("expected an error when all attempts fail at the transport layer")
	}
	if rt.calls == 0 {
		t.Fatal("expected at least one transport call")
	}
}
