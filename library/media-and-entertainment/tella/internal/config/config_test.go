// Copyright 2026 gregce. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// PATCH(library): regression coverage for TELLA_SESSION_COOKIE persistence.
// The env var is an ephemeral browser-cookie override; token saves must not
// silently write it into config.toml.
func TestSaveTokensDoesNotPersistEnvSessionCookie(t *testing.T) {
	t.Setenv("TELLA_SESSION_COOKIE", "Cookie: session=env-secret")
	path := filepath.Join(t.TempDir(), "config.toml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SessionCookie != "session=env-secret" {
		t.Fatalf("SessionCookie = %q, want env cookie", cfg.SessionCookie)
	}
	if err := cfg.SaveTokens("", "", "access-token", "", time.Time{}); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if strings.Contains(string(data), "env-secret") {
		t.Fatalf("env session cookie was persisted: %s", data)
	}
}

func TestSaveTokensPreservesDiskSessionCookieWhenEnvOverrides(t *testing.T) {
	t.Setenv("TELLA_SESSION_COOKIE", "Cookie: session=env-secret")
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("session_cookie = \"session=disk-secret\"\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.SessionCookie != "session=env-secret" {
		t.Fatalf("SessionCookie = %q, want env override", cfg.SessionCookie)
	}
	if err := cfg.SaveTokens("", "", "access-token", "", time.Time{}); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "env-secret") {
		t.Fatalf("env session cookie was persisted: %s", got)
	}
	if !strings.Contains(got, "disk-secret") {
		t.Fatalf("disk session cookie was not preserved: %s", got)
	}
}
