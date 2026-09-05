// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeJarSession writes a cookies.json under home holding the named session
// value, mirroring the layout the persistent cookie jar (and the auth-refresh
// sidecar) produces.
func writeJarSession(t *testing.T, home, value string) {
	t.Helper()
	dir := filepath.Join(home, ".local", "share", "artistly-pp-cli")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir jar dir: %v", err)
	}
	body := `[{"name":"artistly_session","value":"` + value + `","domain":"app.artistly.ai","path":"/","secure":true}]`
	if err := os.WriteFile(filepath.Join(dir, "cookies.json"), []byte(body), 0o600); err != nil {
		t.Fatalf("write cookies.json: %v", err)
	}
}

// runAuthStatus runs `auth status` with HOME pointed at a temp dir and no
// config token, returning combined output and the command error.
func runAuthStatus(t *testing.T, home string) (string, error) {
	t.Helper()
	t.Setenv("HOME", home)
	// Ensure the env-cookie path doesn't mask the jar behavior under test.
	t.Setenv("ARTISTLY_SESSION_COOKIE", "")
	t.Setenv("ARTISTLY_CONFIG", filepath.Join(home, "no-such-config.toml"))

	flags := &rootFlags{}
	cmd := newAuthStatusCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.RunE(cmd, nil)
	return out.String(), err
}

// A jar session written by the sidecar (no config token) must report
// authenticated — the regression this fix addresses.
func TestAuthStatus_JarSessionReportsAuthenticated(t *testing.T) {
	home := t.TempDir()
	writeJarSession(t, home, "live-session-value")

	out, err := runAuthStatus(t, home)
	if err != nil {
		t.Fatalf("auth status returned error with jar session present: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "Authenticated") {
		t.Fatalf("output missing \"Authenticated\":\n%s", out)
	}
	if !strings.Contains(out, "browser cookie jar") {
		t.Fatalf("output missing jar source label:\n%s", out)
	}
}

// No config token and no jar session must still report not authenticated.
func TestAuthStatus_NoCredentialsReportsNotAuthenticated(t *testing.T) {
	home := t.TempDir()

	out, err := runAuthStatus(t, home)
	if err == nil {
		t.Fatalf("auth status returned nil error with no credentials; output:\n%s", out)
	}
	if !strings.Contains(out, "Not authenticated") {
		t.Fatalf("output missing \"Not authenticated\":\n%s", out)
	}
}

// An empty artistly_session value is not a credential.
func TestAuthStatus_EmptyJarSessionReportsNotAuthenticated(t *testing.T) {
	home := t.TempDir()
	writeJarSession(t, home, "")

	out, err := runAuthStatus(t, home)
	if err == nil {
		t.Fatalf("auth status returned nil error with empty jar session; output:\n%s", out)
	}
	if !strings.Contains(out, "Not authenticated") {
		t.Fatalf("output missing \"Not authenticated\":\n%s", out)
	}
}
