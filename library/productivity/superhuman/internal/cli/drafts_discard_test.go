// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestDraftsDiscard_BackendCallShape verifies the writes-with-null-value
// shape and the user-data-write path.
func TestDraftsDiscard_BackendCallShape(t *testing.T) {
	var observedPath string
	var observedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/v3/userdata.write") {
			http.Error(w, "wrong path: "+r.URL.Path, 404)
			return
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &observedBody)
		if writes, ok := observedBody["writes"].([]any); ok && len(writes) > 0 {
			if first, ok := writes[0].(map[string]any); ok {
				if p, ok := first["path"].(string); ok {
					observedPath = p
				}
			}
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, srv.URL, "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "drafts", "discard", "draft0012ab34cd56ef")
	if err != nil {
		t.Fatalf("drafts discard --json: %v", err)
	}
	want := "users/gid-001/threads/draft0012ab34cd56ef/messages/draft0012ab34cd56ef/draft"
	if observedPath != want {
		t.Fatalf("path = %q want %q", observedPath, want)
	}
	writes, _ := observedBody["writes"].([]any)
	first := writes[0].(map[string]any)
	if first["value"] != nil {
		t.Fatalf("value should be null, got %v", first["value"])
	}
	if !strings.Contains(stdout, "drafts.discard") {
		t.Fatalf("envelope missing action: %s", stdout)
	}
}

// TestDraftsDiscard_RequiresYesOnNonTTY ensures the confirmation gate
// fires for piped stdin.
func TestDraftsDiscard_RequiresYesOnNonTTY(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "drafts", "discard", "draft0012ab")
	if err == nil {
		t.Fatalf("expected usage error without --yes on non-TTY stdin")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
}

// TestDraftsDiscard_MissingArg surfaces a usage error.
func TestDraftsDiscard_MissingArg(t *testing.T) {
	configPath, _ := withConfigPath(t)
	_, _, err := executeCmd(t, "--config", configPath, "drafts", "discard")
	if err == nil {
		t.Fatalf("expected usage error without draft id")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
}
