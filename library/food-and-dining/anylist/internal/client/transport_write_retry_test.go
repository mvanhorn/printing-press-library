// Copyright 2026 Jeeves and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/anylist/internal/config"
)

func TestWritesAreNotReplayedAfterAmbiguousFailures(t *testing.T) {
	methods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, method := range methods {
		for _, failure := range []string{"server-error", "dropped-connection"} {
			t.Run(method+"/"+failure, func(t *testing.T) {
				var calls atomic.Int32
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls.Add(1)
					if failure == "server-error" {
						http.Error(w, "bad gateway", http.StatusBadGateway)
						return
					}
					connection, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Errorf("hijack: %v", err)
						return
					}
					if tcp, ok := connection.(*net.TCPConn); ok {
						_ = tcp.SetLinger(0)
					}
					_ = connection.Close()
				}))
				defer server.Close()
				client := New(&config.Config{BaseURL: server.URL}, 5*time.Second, 0)
				client.NoCache = true
				_, _, err := client.do(method, "/test", nil, map[string]any{"value": 1}, nil)
				if err == nil {
					t.Fatal("expected request failure")
				}
				if got := calls.Load(); got != 1 {
					t.Fatalf("%s sent %d times after %s, want 1", method, got, failure)
				}
			})
		}
	}
}

func TestReadStillRetriesAfterAmbiguousFailures(t *testing.T) {
	for _, failure := range []string{"server-error", "dropped-connection"} {
		t.Run(failure, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if calls.Add(1) == 1 {
					if failure == "server-error" {
						http.Error(w, "bad gateway", http.StatusBadGateway)
						return
					}
					connection, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Errorf("hijack: %v", err)
						return
					}
					if tcp, ok := connection.(*net.TCPConn); ok {
						_ = tcp.SetLinger(0)
					}
					_ = connection.Close()
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()
			client := New(&config.Config{BaseURL: server.URL}, 5*time.Second, 0)
			client.NoCache = true
			if _, err := client.Get("/test", nil); err != nil {
				t.Fatalf("GET: %v", err)
			}
			if got := calls.Load(); got != 2 {
				t.Fatalf("GET sent %d times after %s, want 2", got, failure)
			}
		})
	}
}
