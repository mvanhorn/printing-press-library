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

// TestRemindersCancel_BackendCallShape verifies the null-write shape
// against the user-data-write endpoint -- same pattern as drafts_discard.
func TestRemindersCancel_BackendCallShape(t *testing.T) {
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

	_, _, err := executeCmd(t, "--config", configPath, "--json",
		"reminders", "cancel",
		"--thread-id", "19e2dc46a8b281fe",
	)
	if err != nil {
		t.Fatalf("reminders cancel --json: %v", err)
	}

	wantPath := "users/gid-001/threads/19e2dc46a8b281fe/reminder"
	if observedPath != wantPath {
		t.Fatalf("path = %q want %q", observedPath, wantPath)
	}

	writes := observedBody["writes"].([]any)
	first := writes[0].(map[string]any)
	if first["value"] != nil {
		t.Fatalf("value must be null to delete, got %v", first["value"])
	}

	// Ensure no snake_case keys leak
	if _, has := observedBody["thread_id"]; has {
		t.Errorf("body should not contain snake_case thread_id")
	}
	if _, has := observedBody["reminder_id"]; has {
		t.Errorf("body should not contain snake_case reminder_id")
	}
}

// TestRemindersCancel_ReminderIDAliasesThreadID confirms the deprecated
// --reminder-id flag still drives a cancel by mapping into --thread-id.
func TestRemindersCancel_ReminderIDAliasesThreadID(t *testing.T) {
	var observedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var parsed map[string]any
		_ = json.Unmarshal(body, &parsed)
		if writes, ok := parsed["writes"].([]any); ok && len(writes) > 0 {
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

	_, _, err := executeCmd(t, "--config", configPath, "--json",
		"reminders", "cancel",
		"--reminder-id", "legacycallerthreadid",
	)
	if err != nil {
		t.Fatalf("reminders cancel via --reminder-id alias: %v", err)
	}
	wantPath := "users/gid-001/threads/legacycallerthreadid/reminder"
	if observedPath != wantPath {
		t.Fatalf("path = %q want %q", observedPath, wantPath)
	}
}

// TestRemindersCancel_MissingThreadID returns a non-zero exit code when
// neither --thread-id nor --reminder-id is provided.
func TestRemindersCancel_MissingThreadID(t *testing.T) {
	configPath, _ := withConfigPath(t)
	_, _, err := executeCmd(t, "--config", configPath, "reminders", "cancel")
	if err == nil {
		t.Fatalf("expected error when no thread id provided")
	}
}
