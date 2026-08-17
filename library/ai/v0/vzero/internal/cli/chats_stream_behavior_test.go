// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestChatsStreamRendersSSEEvents verifies the command renders each SSE event
// from the live body (chat, message.usage) and records chat id attribution.
func TestChatsStreamRendersSSEEvents(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: chat\ndata: {\"id\":\"chat-123\"}\n\n"))
		w.Write([]byte("event: message.usage\ndata: {\"tokens\":{\"total\":10}}\n\n"))
	}))
	defer server.Close()

	t.Setenv("V0_BASE_URL", server.URL)
	t.Setenv("V0_API_KEY", "test-key")

	rootCmd := RootCmd()
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"chats", "stream", "Say hello", "--json"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("chats stream error = %v (stderr=%q)", err, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{`"event": "chat"`, `"chat-123"`, `"event": "message.usage"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("stream output missing %q:\n%s", want, out)
		}
	}
}

// TestChatsStreamErrorEventExitsNonZero verifies an SSE error event surfaces as
// a non-zero exit rather than silent success.
func TestChatsStreamErrorEventExitsNonZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: error\ndata: {\"message\":\"boom\"}\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}))
	defer server.Close()

	t.Setenv("V0_BASE_URL", server.URL)
	t.Setenv("V0_API_KEY", "test-key")

	rootCmd := RootCmd()
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"chats", "stream", "Say hello"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error on SSE error event, got nil")
	}
	if !strings.Contains(err.Error(), "error event") {
		t.Fatalf("error = %v, want mention of error event", err)
	}
}
