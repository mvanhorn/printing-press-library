// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/v0/internal/cliutil"
)

// TestMessagesTailTimeoutExitsNonZero verifies that when --timeout expires with
// the newest assistant message still unfinished, the command exits non-zero
// (apiErr, exit 5) instead of silently reporting success.
func TestMessagesTailTimeoutExitsNonZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Serve a page whose newest assistant message is still running so the
		// command keeps polling until its --timeout expires.
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"messages":[{"id":"m1","role":"assistant","finishReason":null,"content":"","parts":[]}]}`))
	}))
	defer server.Close()

	t.Setenv("V0_BASE_URL", server.URL)
	t.Setenv("V0_API_KEY", "test-key")

	rootCmd := RootCmd()
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"messages", "tail", "chat-1", "--interval", "100ms", "--timeout", "300ms"})
	err := rootCmd.Execute()

	var codeErr *cliError
	if !As(err, &codeErr) || codeErr.code != 5 {
		t.Fatalf("expected typed exit code 5 (timeout), got err=%v", err)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("timed out")) {
		t.Fatalf("stderr missing timeout message: %q", stderr.String())
	}
}

// TestMessagesTailTimeoutJSONExitsNonZero verifies the same non-zero exit when
// the caller asked for JSON, with a valid JSON envelope still emitted.
func TestMessagesTailTimeoutJSONExitsNonZero(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"messages":[{"id":"m1","role":"assistant","finishReason":null,"content":"","parts":[]}]}`))
	}))
	defer server.Close()

	t.Setenv("V0_BASE_URL", server.URL)
	t.Setenv("V0_API_KEY", "test-key")

	rootCmd := RootCmd()
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs([]string{"messages", "tail", "chat-1", "--interval", "100ms", "--timeout", "300ms", "--json"})
	err := rootCmd.Execute()

	var codeErr *cliError
	if !As(err, &codeErr) || codeErr.code != 5 {
		t.Fatalf("expected typed exit code 5 (timeout), got err=%v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"status": "timeout"`)) {
		t.Fatalf("stdout missing timeout envelope: %q", stdout.String())
	}
}

// TestMessagesTailFollowDedupesFinishedMessage verifies that --follow does not
// re-render an unchanged finished message on subsequent polls.
func TestMessagesTailFollowDedupesFinishedMessage(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		// Always return the same finished message: follow mode must emit it
		// exactly once.
		w.Write([]byte(`{"messages":[{"id":"m1","role":"assistant","finishReason":"stop","content":"done","parts":[{"type":"text"}]}]}`))
	}))
	defer server.Close()

	t.Setenv("V0_BASE_URL", server.URL)
	t.Setenv("V0_API_KEY", "test-key")

	rootCmd := RootCmd()
	var stdout, stderr bytes.Buffer
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	// --follow with a short overall cap; context timeout ends the loop.
	rootCmd.SetArgs([]string{"messages", "tail", "chat-1", "--interval", "100ms", "--timeout", "2s", "--follow", "--json"})
	_ = rootCmd.Execute()

	// Only one emission of message m1 should be present despite multiple polls.
	count := bytes.Count(stdout.Bytes(), []byte(`"message_id": "m1"`))
	if count != 1 {
		t.Fatalf("expected exactly 1 emission of m1, got %d (requests=%d, stdout=%q)", count, requests, stdout.String())
	}
	if requests < 2 {
		t.Fatalf("expected follow mode to keep polling (requests=%d), dedupe test not meaningful", requests)
	}
}

var _ = cliutil.IsVerifyEnv
