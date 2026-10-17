// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// A key supplied by a caller does not prove the upstream server deduplicates
// a write whose first response is ambiguous. An explicit 429 is different:
// the server rejected the operation, so a bounded retry remains safe.
func TestKeyedWriteRetriesOnlyExplicitRateLimitRejection(t *testing.T) {
	for _, status := range []int{http.StatusBadGateway, http.StatusTooManyRequests} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if calls.Add(1) == 1 {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(status)
					return
				}
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()
			client, _ := platformRateLimitTestClient(t, server)
			_, _, err := client.PostWithHeaders(context.Background(), "/mutation", map[string]any{"synthetic": true}, map[string]string{"Idempotency-Key": "synthetic-key"})
			if status == http.StatusBadGateway {
				if err == nil || calls.Load() != 1 {
					t.Fatalf("ambiguous write err=%v calls=%d; want failure without replay", err, calls.Load())
				}
			} else if err != nil || calls.Load() != 2 {
				t.Fatalf("rejected write err=%v calls=%d; want bounded retry and success", err, calls.Load())
			}
		})
	}
}

func TestKeyedWriteStopsAtRateLimitAttemptBound(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	client, _ := platformRateLimitTestClient(t, server)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := client.PostWithHeaders(ctx, "/mutation", map[string]any{"synthetic": true}, map[string]string{"Idempotency-Key": "synthetic-key"})
	if err == nil || calls.Load() != 2 {
		t.Fatalf("persistent rejection err=%v calls=%d; want error after exactly two attempts", err, calls.Load())
	}
}
