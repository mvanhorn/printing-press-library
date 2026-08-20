// Copyright 2026 Dave Morin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeAuthLogoutConfig writes a config.toml with a stored fm_api_key, clears
// both auth env vars, and returns the config path. Individual tests re-export
// whichever env var they want to assert about.
func writeAuthLogoutConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	body := `base_url = 'https://api.setlist.fm/rest'
fm_api_key = 'stored-key'
`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	t.Setenv("SETLIST_FM_CONFIG", path)
	t.Setenv("SETLISTFM_API_KEY", "")
	t.Setenv("SETLIST_FM_API_KEY", "")
	return path
}

// runAuthLogout invokes the auth logout command in JSON mode and returns the
// parsed envelope plus the captured combined stdout/stderr.
func runAuthLogout(t *testing.T, configPath string, jsonMode bool) (map[string]any, string) {
	t.Helper()
	flags := &rootFlags{configPath: configPath, asJSON: jsonMode}
	cmd := newAuthLogoutCmd(flags)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("logout Execute: %v", err)
	}
	if !jsonMode {
		return nil, buf.String()
	}
	var env map[string]any
	if err := json.Unmarshal(buf.Bytes(), &env); err != nil {
		t.Fatalf("parse logout JSON: %v\nbody=%s", err, buf.String())
	}
	return env, buf.String()
}

func TestAuthLogoutNotesSetlistfmApiKeyWhenOnlyHigherPriorityEnvSet(t *testing.T) {
	path := writeAuthLogoutConfig(t)
	t.Setenv("SETLISTFM_API_KEY", "from-env")

	env, _ := runAuthLogout(t, path, true)
	note, _ := env["note"].(string)
	if note != "SETLISTFM_API_KEY env var is still set" {
		t.Fatalf("note: got %q, want %q", note, "SETLISTFM_API_KEY env var is still set")
	}

	_, prose := runAuthLogout(t, path, false)
	if !strings.Contains(prose, "SETLISTFM_API_KEY env var is still set") {
		t.Fatalf("human prose should name SETLISTFM_API_KEY, got:\n%s", prose)
	}
}

func TestAuthLogoutNotesSetlistFmApiKeyWhenOnlyLowerPriorityEnvSet(t *testing.T) {
	path := writeAuthLogoutConfig(t)
	t.Setenv("SETLIST_FM_API_KEY", "from-env")

	env, _ := runAuthLogout(t, path, true)
	note, _ := env["note"].(string)
	if note != "SETLIST_FM_API_KEY env var is still set" {
		t.Fatalf("note: got %q, want %q", note, "SETLIST_FM_API_KEY env var is still set")
	}

	_, prose := runAuthLogout(t, path, false)
	if !strings.Contains(prose, "SETLIST_FM_API_KEY env var is still set") {
		t.Fatalf("human prose should name SETLIST_FM_API_KEY, got:\n%s", prose)
	}
}

func TestAuthLogoutPrefersHigherPriorityEnvWhenBothSet(t *testing.T) {
	path := writeAuthLogoutConfig(t)
	t.Setenv("SETLISTFM_API_KEY", "modern")
	t.Setenv("SETLIST_FM_API_KEY", "legacy")

	env, _ := runAuthLogout(t, path, true)
	note, _ := env["note"].(string)
	if note != "SETLISTFM_API_KEY env var is still set" {
		t.Fatalf("note should name the higher-priority var; got %q", note)
	}
}

func TestAuthLogoutOmitsNoteWhenNoEnvSet(t *testing.T) {
	path := writeAuthLogoutConfig(t)

	env, _ := runAuthLogout(t, path, true)
	if _, ok := env["note"]; ok {
		t.Fatalf("note should be omitted when no env var is set, envelope=%v", env)
	}
	cleared, _ := env["cleared"].(bool)
	if !cleared {
		t.Fatalf("cleared should be true; envelope=%v", env)
	}

	_, prose := runAuthLogout(t, path, false)
	if !strings.Contains(prose, "Logged out. Credentials cleared.") {
		t.Fatalf("human prose should match the bare-clear copy, got:\n%s", prose)
	}
	if strings.Contains(prose, "env var is still set") {
		t.Fatalf("human prose should not mention env var when none set, got:\n%s", prose)
	}
}
