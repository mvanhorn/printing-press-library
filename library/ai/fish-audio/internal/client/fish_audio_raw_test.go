// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/fish-audio/internal/config"
)

// PostRaw carries billed work (TTS, voice design, model create, ASR). An
// ambiguous failure must not replay the request, or the user pays twice.
func TestPostRawDoesNotReplayAfterAmbiguousFailure(t *testing.T) {
	for _, mode := range []string{"500", "503", "drop"} {
		t.Run(mode, func(t *testing.T) {
			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				hits.Add(1)
				switch mode {
				case "500":
					http.Error(w, "boom", http.StatusInternalServerError)
				case "503":
					http.Error(w, "busy", http.StatusServiceUnavailable)
				case "drop":
					hj, ok := w.(http.Hijacker)
					if !ok {
						t.Fatal("hijack unsupported")
					}
					conn, _, _ := hj.Hijack()
					_ = conn.(*net.TCPConn).SetLinger(0)
					_ = conn.Close()
				}
			}))
			defer srv.Close()

			c := New(&config.Config{BaseURL: srv.URL}, 5*time.Second, 0)
			c.NoCache = true
			_, _, err := c.PostRaw(context.Background(), "/v1/tts", []byte(`{"text":"hi"}`), nil)
			if err == nil {
				t.Fatal("expected an error")
			}
			if got := hits.Load(); got != 1 {
				t.Fatalf("POST sent %d times, want exactly 1 (no replay of billed work)", got)
			}
		})
	}
}

// A 429 is an explicit refusal (nothing was generated), so it is still retried.
func TestPostRawRetriesRateLimit(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			http.Error(w, "slow down", http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte("audio-bytes"))
	}))
	defer srv.Close()

	c := New(&config.Config{BaseURL: srv.URL}, 5*time.Second, 0)
	c.NoCache = true
	data, status, err := c.PostRaw(context.Background(), "/v1/tts", []byte(`{}`), nil)
	if err != nil {
		t.Fatalf("PostRaw: %v", err)
	}
	if status != http.StatusOK || string(data) != "audio-bytes" {
		t.Fatalf("got %d %q", status, data)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("hits = %d, want 2 (one 429 then success)", got)
	}
}
