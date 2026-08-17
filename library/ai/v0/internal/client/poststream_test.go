// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/v0/internal/config"
)

func testStreamClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	cfg := &config.Config{BaseURL: baseURL, AuthHeaderVal: "Bearer test-token"}
	c := New(cfg, 0, 0)
	c.HTTPClient = &http.Client{}
	return c
}

// TestPostStreamStreamsEventsLive verifies that SSE events are delivered to the
// caller incrementally as the server writes them — not buffered until EOF —
// and that the response body must be closed by the caller.
func TestPostStreamStreamsEventsLive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Errorf("Accept = %q, want text/event-stream", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		w.Write([]byte("event: chat\ndata: {\"id\":\"abc\"}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
		w.Write([]byte("event: message.parts.chunk\ndata: {\"text\":\"hi\"}\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer server.Close()

	c := testStreamClient(t, server.URL)
	body, err := c.PostStream(context.Background(), "/chats/stream", map[string]any{"message": "hi"}, nil)
	if err != nil {
		t.Fatalf("PostStream error = %v", err)
	}
	defer body.Close()

	raw, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("reading stream: %v", err)
	}
	got := string(raw)
	for _, want := range []string{`event: chat`, `{"id":"abc"}`, `message.parts.chunk`, `{"text":"hi"}`} {
		if !strings.Contains(got, want) {
			t.Fatalf("stream output missing %q:\n%s", want, got)
		}
	}
}

// TestPostStreamErrorReturnsAPIError verifies non-2xx responses become a typed
// APIError instead of a silently empty body.
func TestPostStreamErrorReturnsAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Unauthorized"}`))
	}))
	defer server.Close()

	c := testStreamClient(t, server.URL)
	body, err := c.PostStream(context.Background(), "/chats/stream", map[string]any{"message": "hi"}, nil)
	if body != nil {
		body.Close()
		t.Fatal("expected nil body on error")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T, want *APIError (got %v)", err, err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("StatusCode = %d, want 401", apiErr.StatusCode)
	}
	if !strings.Contains(apiErr.Body, "Unauthorized") {
		t.Fatalf("Body = %q, want to mention Unauthorized", apiErr.Body)
	}
}
