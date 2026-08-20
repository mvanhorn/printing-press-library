// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const labelsFixture = `{
	"labels": [
		{"id":"Label_5","name":"Vendors","type":"user"},
		{"id":"INBOX","name":"INBOX","type":"system"},
		{"id":"Label_1","name":"Newsletters","type":"user"},
		{"id":"SPAM","name":"SPAM","type":"system"},
		{"id":"DRAFT","name":"DRAFT","type":"system"}
	]
}`

func newLabelsFakeServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/users/me/labels") {
			_, _ = w.Write([]byte(labelsFixture))
			return
		}
		http.Error(w, "wrong path", 404)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestLabelsList_JSON_AllLabelsReturnedSorted asserts the envelope contains
// every label with system-first ordering preserved.
func TestLabelsList_JSON_AllLabelsReturnedSorted(t *testing.T) {
	srv := newLabelsFakeServer(t)
	withGmailBaseURL(t, srv.URL)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "labels", "list")
	if err != nil {
		t.Fatalf("labels list --json: %v", err)
	}
	var env map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &env); jerr != nil {
		t.Fatalf("parse envelope: %v\n%s", jerr, stdout)
	}
	if env["count"].(float64) != 5 {
		t.Fatalf("count = %v want 5", env["count"])
	}
	data, _ := env["data"].([]any)
	if len(data) != 5 {
		t.Fatalf("data len = %d want 5", len(data))
	}
	// First three entries should be system (INBOX, SPAM, DRAFT in fixture
	// order, since the gmail package preserves Gmail's order for systems).
	for i := 0; i < 3; i++ {
		m := data[i].(map[string]any)
		if m["type"] != "system" {
			t.Fatalf("expected system at index %d, got %v", i, m)
		}
	}
	// Last two should be user, alphabetical (Newsletters before Vendors).
	prev := ""
	for _, row := range data[3:] {
		m := row.(map[string]any)
		if m["type"] != "user" {
			t.Fatalf("expected user labels in trailing slice, got %v", m)
		}
		name := m["name"].(string)
		if prev != "" && prev > name {
			t.Fatalf("user labels not alphabetical: %q before %q", prev, name)
		}
		prev = name
	}
}

// TestLabelsList_SystemOnly filters to type=system.
func TestLabelsList_SystemOnly(t *testing.T) {
	srv := newLabelsFakeServer(t)
	withGmailBaseURL(t, srv.URL)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "labels", "list", "--system-only")
	if err != nil {
		t.Fatalf("labels list --system-only: %v", err)
	}
	var env map[string]any
	_ = json.Unmarshal([]byte(stdout), &env)
	if env["count"].(float64) != 3 {
		t.Fatalf("count = %v want 3 system labels", env["count"])
	}
	data, _ := env["data"].([]any)
	for _, row := range data {
		m := row.(map[string]any)
		if m["type"] != "system" {
			t.Fatalf("--system-only returned non-system: %v", m)
		}
	}
}

// TestLabelsList_UserOnly filters to type=user.
func TestLabelsList_UserOnly(t *testing.T) {
	srv := newLabelsFakeServer(t)
	withGmailBaseURL(t, srv.URL)

	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	stdout, _, err := executeCmd(t, "--config", configPath, "--json", "labels", "list", "--user-only")
	if err != nil {
		t.Fatalf("labels list --user-only: %v", err)
	}
	var env map[string]any
	_ = json.Unmarshal([]byte(stdout), &env)
	if env["count"].(float64) != 2 {
		t.Fatalf("count = %v want 2 user labels", env["count"])
	}
}

// TestLabelsList_BothFlagsConflict surfaces a usage error.
func TestLabelsList_BothFlagsConflict(t *testing.T) {
	configPath, tokenStorePath := withConfigPath(t)
	seedSendStore(t, tokenStorePath, "user@example.com", "gid-001")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")

	_, _, err := executeCmd(t, "--config", configPath, "labels", "list", "--system-only", "--user-only")
	if err == nil {
		t.Fatalf("expected usage error for conflicting flags")
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("exit code = %d want 2", got)
	}
}
