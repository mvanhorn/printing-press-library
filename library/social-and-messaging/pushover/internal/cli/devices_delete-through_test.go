// Copyright 2026 twidtwid. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDevicesDeleteThroughSendsIntegerMessageID locks in the contract for
// pushover-pp-cli's standalone update_highest_message.json wrapper:
// --message-id is declared as a StringVar (cobra has no Int64Var that pairs
// cleanly with the dual --message-id / hidden --message alias here), so the
// RunE has to ParseInt the value before assembling the body. Without that,
// `body["message"]` carries a Go string and json.Marshal emits
// "message":"789" — which Pushover's type-strict endpoint rejects with a
// 400. This is the same bug class fixed for inbox-sync --delete-through in
// PR #511; the standalone command was missed on the first pass.
func TestDevicesDeleteThroughSendsIntegerMessageID(t *testing.T) {
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
		"devices", "delete-through", "device-1",
		"--client-secret", "sec",
		"--message-id", "789",
		"--json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("delete-through execute: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(captured, &decoded); err != nil {
		t.Fatalf("captured body is not JSON: %v\nbody=%q", err, string(captured))
	}
	msg, ok := decoded["message"]
	if !ok {
		t.Fatalf("body missing \"message\" key; decoded=%#v\nraw=%q", decoded, string(captured))
	}
	// encoding/json decodes JSON numbers into float64. A JSON string would
	// decode into a Go string; that's the bug shape we're guarding against.
	if _, isString := msg.(string); isString {
		t.Fatalf("message is JSON-encoded as string %q; want JSON integer\nraw=%q", msg, string(captured))
	}
	if n, ok := msg.(float64); !ok || n != 789 {
		t.Fatalf("message = %v (%T); want 789\nraw=%q", msg, msg, string(captured))
	}
}

// TestDevicesDeleteThroughRejectsNonNumericMessageID covers the usage-error
// path: a non-numeric --message-id (e.g. a typo) must fail at the flag
// boundary, not silently send a malformed body. Without the ParseInt gate
// the request would hit the API and surface as a remote 400 in the noise
// of all the other failure modes, which is worse for an agent caller than
// a deterministic local error.
func TestDevicesDeleteThroughRejectsNonNumericMessageID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be reached on invalid --message-id")
	}))
	defer srv.Close()
	t.Setenv("PUSHOVER_BASE_URL", srv.URL)

	cmd := newRootCmd(&rootFlags{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{
		"devices", "delete-through", "device-1",
		"--client-secret", "sec",
		"--message-id", "not-a-number",
		"--json",
	})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected --message-id parse error, got nil")
	}
	if !strings.Contains(err.Error(), "message-id") {
		t.Fatalf("error does not mention --message-id: %v", err)
	}
}
