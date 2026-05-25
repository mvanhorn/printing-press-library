// Copyright 2026 dave-morin. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTempConfig(t *testing.T) *Config {
	t.Helper()
	dir := t.TempDir()
	return &Config{
		BaseURL: "https://api.setlist.fm/rest",
		Path:    filepath.Join(dir, "config.toml"),
	}
}

func loadFromPath(t *testing.T, path string) *Config {
	t.Helper()
	t.Setenv("SETLIST_FM_CONFIG", path)
	t.Setenv("SETLISTFM_API_KEY", "")
	t.Setenv("SETLIST_FM_API_KEY", "")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg
}

func TestSaveAPIKeyWritesFmApiKey(t *testing.T) {
	cfg := newTempConfig(t)
	if err := cfg.SaveAPIKey("abc123"); err != nil {
		t.Fatalf("SaveAPIKey: %v", err)
	}

	data, err := os.ReadFile(cfg.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, `fm_api_key = 'abc123'`) {
		t.Fatalf("expected fm_api_key in TOML, got:\n%s", got)
	}

	loaded := loadFromPath(t, cfg.Path)
	if loaded.SetlistFmApiKey != "abc123" {
		t.Fatalf("SetlistFmApiKey: got %q, want abc123", loaded.SetlistFmApiKey)
	}
	if loaded.AuthHeader() != "abc123" {
		t.Fatalf("AuthHeader: got %q, want abc123", loaded.AuthHeader())
	}
}

func TestSaveAPIKeyDropsLegacyOAuthFields(t *testing.T) {
	cfg := newTempConfig(t)
	legacy := `base_url = 'https://api.setlist.fm/rest'
auth_header = 'Bearer legacy'
access_token = 'legacy-access'
refresh_token = 'legacy-refresh'
token_expiry = 2024-01-01T00:00:00Z
client_id = 'legacy-client'
client_secret = 'legacy-secret'
fm_api_key = ''
`
	if err := os.WriteFile(cfg.Path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("seed legacy config: %v", err)
	}

	loaded := loadFromPath(t, cfg.Path)
	if loaded.SetlistFmApiKey != "" {
		t.Fatalf("expected empty SetlistFmApiKey when only legacy fields are set, got %q", loaded.SetlistFmApiKey)
	}
	if loaded.AuthHeader() != "" {
		t.Fatalf("AuthHeader should be empty when only legacy fields are present, got %q", loaded.AuthHeader())
	}

	if err := loaded.SaveAPIKey("newkey"); err != nil {
		t.Fatalf("SaveAPIKey: %v", err)
	}
	data, err := os.ReadFile(loaded.Path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	got := string(data)
	for _, dead := range []string{"auth_header", "access_token", "refresh_token", "token_expiry", "client_id", "client_secret"} {
		if strings.Contains(got, dead) {
			t.Errorf("expected legacy field %q to be dropped after save, got:\n%s", dead, got)
		}
	}
	if !strings.Contains(got, `fm_api_key = 'newkey'`) {
		t.Errorf("expected fm_api_key=newkey, got:\n%s", got)
	}
}

func TestClearAPIKeyEmptiesFmApiKey(t *testing.T) {
	cfg := newTempConfig(t)
	if err := cfg.SaveAPIKey("abc123"); err != nil {
		t.Fatalf("SaveAPIKey: %v", err)
	}
	if err := cfg.ClearAPIKey(); err != nil {
		t.Fatalf("ClearAPIKey: %v", err)
	}
	loaded := loadFromPath(t, cfg.Path)
	if loaded.SetlistFmApiKey != "" {
		t.Fatalf("expected empty key after Clear, got %q", loaded.SetlistFmApiKey)
	}
	if loaded.AuthHeader() != "" {
		t.Fatalf("AuthHeader should be empty after Clear, got %q", loaded.AuthHeader())
	}
}

func TestLoadAuthSourceConfig(t *testing.T) {
	cfg := newTempConfig(t)
	if err := cfg.SaveAPIKey("from-config"); err != nil {
		t.Fatalf("SaveAPIKey: %v", err)
	}
	loaded := loadFromPath(t, cfg.Path)
	if loaded.AuthSource != "config" {
		t.Fatalf("AuthSource: got %q, want config", loaded.AuthSource)
	}
}

func TestLoadEnvVarOverridesConfig(t *testing.T) {
	cfg := newTempConfig(t)
	if err := cfg.SaveAPIKey("from-config"); err != nil {
		t.Fatalf("SaveAPIKey: %v", err)
	}
	t.Setenv("SETLIST_FM_CONFIG", cfg.Path)
	t.Setenv("SETLISTFM_API_KEY", "from-env")
	loaded, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.SetlistFmApiKey != "from-env" {
		t.Fatalf("SetlistFmApiKey: got %q, want from-env", loaded.SetlistFmApiKey)
	}
	if loaded.AuthSource != "env:SETLISTFM_API_KEY" {
		t.Fatalf("AuthSource: got %q, want env:SETLISTFM_API_KEY", loaded.AuthSource)
	}
}

func TestLoadMissingFileReturnsEmptyConfig(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "no-such-file.toml")
	t.Setenv("SETLIST_FM_CONFIG", missing)
	t.Setenv("SETLISTFM_API_KEY", "")
	t.Setenv("SETLIST_FM_API_KEY", "")
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AuthHeader() != "" {
		t.Fatalf("AuthHeader should be empty for missing config, got %q", cfg.AuthHeader())
	}
}
