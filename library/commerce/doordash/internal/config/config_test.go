package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDoorDashSessionEnvOverrides(t *testing.T) {
	t.Setenv("DOORDASH_COOKIE", "a=b")
	t.Setenv("DOORDASH_CSRF_TOKEN", "csrf")
	t.Setenv("DOORDASH_SESSION_FILE", "~/custom-session.json")
	t.Setenv("DOORDASH_USER_AGENT", "ua")
	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CookieHeader != "a=b" || cfg.CSRFToken != "csrf" || cfg.UserAgent != "ua" {
		t.Fatalf("env overrides not applied: %#v", cfg)
	}
	if cfg.AuthSource != "env:DOORDASH_COOKIE" {
		t.Fatalf("auth source = %q", cfg.AuthSource)
	}
	home, _ := os.UserHomeDir()
	if cfg.SessionFile != filepath.Join(home, "custom-session.json") {
		t.Fatalf("session file = %q", cfg.SessionFile)
	}
}
