// Copyright 2026 twidtwid. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestMessagesSendBoolFlagsAsIntegerOne pins the contract for the
// `messages send` CLI: --html / --monospace / --encrypted are declared
// as cobra BoolVars (their natural shape on the CLI), but Pushover's
// /1/messages.json encodes the same fields as integer 1, not JSON
// true. Sending the Go bool unmodified would have made every
// `messages send --html` silently fall back to plain-text formatting
// on the device, and `--encrypted` would have made the recipient
// device treat an encrypted payload as plaintext. The novel `notify`
// command already does this correctly; this test locks the same shape
// in for the direct command.
func TestMessagesSendBoolFlagsAsIntegerOne(t *testing.T) {
	var captured []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		captured = b
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":1,"request":"req-1"}`))
	}))
	defer srv.Close()
	t.Setenv("PUSHOVER_BASE_URL", srv.URL)

	cmd := newRootCmd(&rootFlags{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{
		"messages", "send",
		"--app-token", "app",
		"--user-key", "user",
		"--message", "hello",
		"--html",
		"--monospace",
		"--encrypted",
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("messages send execute: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(captured, &decoded); err != nil {
		t.Fatalf("captured body is not JSON: %v\nbody=%q", err, string(captured))
	}
	for _, key := range []string{"html", "monospace", "encrypted"} {
		v, ok := decoded[key]
		if !ok {
			t.Fatalf("body missing %q; decoded=%#v\nraw=%q", key, decoded, string(captured))
		}
		if _, isBool := v.(bool); isBool {
			t.Fatalf("%q encoded as JSON bool %v; want JSON integer 1\nraw=%q", key, v, string(captured))
		}
		// encoding/json decodes JSON numbers into float64; 1 should come back as 1.0.
		if n, ok := v.(float64); !ok || n != 1 {
			t.Fatalf("%q = %v (%T); want 1\nraw=%q", key, v, v, string(captured))
		}
	}
}
