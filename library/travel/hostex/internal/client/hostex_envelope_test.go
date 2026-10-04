// Copyright 2026 bust011r and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostex/internal/config"
)

// PATCH(hostex-error-code-envelope): Hostex answers HTTP 200 for failures, so
// the response envelope's error_code is the only reliable outcome.
func TestHostexErrorCodeEnvelope(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		body     string
		wantCode int // 0 means success
	}{
		{"success 200", `{"request_id":"r","error_code":200,"error_msg":"Done.","data":{"ok":true}}`, 0},
		{"legacy success 0", `{"error_code":0,"data":{}}`, 0},
		{"non envelope body", `{"ok":true}`, 0},
		{"validation 422", `{"error_code":422,"error_msg":"bad date","data":null}`, 422},
		{"auth 401", `{"error_code":401,"error_msg":"Invalid access token."}`, 401},
		{"subscription 420", `{"error_code":420,"error_msg":"plan"}`, 420},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			t.Cleanup(server.Close)

			c := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
			c.HTTPClient = server.Client()
			c.NoCache = true

			_, err := c.Get(context.Background(), "/reservations", nil)
			if tc.wantCode == 0 {
				if err != nil {
					t.Fatalf("Get returned error for success body: %v", err)
				}
				return
			}
			var apiErr *APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != tc.wantCode {
				t.Fatalf("Get error = %v, want APIError with status %d", err, tc.wantCode)
			}
		})
	}
}

// A 5xx error_code on a write must not be replayed: the first attempt may
// already have been accepted.
func TestHostexEnvelope5xxDoesNotReplayWrites(t *testing.T) {
	t.Parallel()

	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"error_code":502,"error_msg":"upstream"}`))
	}))
	t.Cleanup(server.Close)

	c := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	c.HTTPClient = server.Client()
	c.NoCache = true

	_, _, err := c.Post(context.Background(), "/listings/prices", map[string]any{"listing_id": "1"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 502 {
		t.Fatalf("Post error = %v, want APIError 502", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("write was sent %d times, want exactly 1", got)
	}
}
