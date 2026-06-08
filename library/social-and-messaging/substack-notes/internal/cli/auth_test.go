// Copyright 2026 Peter Yang and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/substack-notes/internal/config"
	_ "modernc.org/sqlite"
)

func TestDiscoverSubstackCookieHeaderFromPlainCookieDB(t *testing.T) {
	t.Parallel()
	dbPath := writeCookieDB(t, map[string]string{
		"session_cookie": "fake-session",
		"theme":          "light",
	})
	header, count, err := discoverSubstackCookieHeader("chrome", "Default", dbPath)
	if err != nil {
		t.Fatalf("discoverSubstackCookieHeader() error = %v", err)
	}
	if count != 2 {
		t.Fatalf("cookie count = %d, want 2", count)
	}
	if !strings.Contains(header, "session_cookie=fake-session") || !strings.Contains(header, "theme=light") {
		t.Fatalf("header = %q", header)
	}
}

func TestDiscoverSubstackCookieHeaderRejectsMissingSessionCookie(t *testing.T) {
	t.Parallel()
	dbPath := writeCookieDB(t, map[string]string{"theme": "light"})
	_, _, err := discoverSubstackCookieHeader("chrome", "Default", dbPath)
	if err == nil || !strings.Contains(err.Error(), "no session-like cookie") {
		t.Fatalf("discoverSubstackCookieHeader() error = %v", err)
	}
}

func TestAuthLoginSavesCookieWithoutPrintingSecret(t *testing.T) {
	t.Parallel()
	dbPath := writeCookieDB(t, map[string]string{"session_cookie": "fake-session"})
	configPath := filepath.Join(t.TempDir(), "config.toml")
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--config", configPath, "--json", "auth", "login", "--cookie-db", dbPath, "--browser", "chrome"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("auth login error = %v", err)
	}
	if strings.Contains(stdout.String(), "fake-session") {
		t.Fatalf("auth login printed secret: %s", stdout.String())
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if got := cfg.AuthHeader(); got != "session_cookie=fake-session" {
		t.Fatalf("saved auth header = %q", got)
	}
	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config permissions = %o, want 600", got)
	}
}

func writeCookieDB(t *testing.T, cookies map[string]string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "Cookies")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE cookies (host_key TEXT, name TEXT, value TEXT, encrypted_value BLOB)`); err != nil {
		t.Fatal(err)
	}
	for name, value := range cookies {
		if _, err := db.Exec(`INSERT INTO cookies (host_key, name, value, encrypted_value) VALUES (?, ?, ?, ?)`, ".substack.com", name, value, []byte{}); err != nil {
			t.Fatal(err)
		}
	}
	return path
}
