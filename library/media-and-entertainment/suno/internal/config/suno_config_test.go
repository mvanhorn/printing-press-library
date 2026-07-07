// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func loadTempConfig(t *testing.T, body string) (*Config, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return cfg, path
}

func TestSaveStudioSessionRoundTrips(t *testing.T) {
	cfg, path := loadTempConfig(t, "base_url='https://studio-api-prod.suno.com'\njwt='x'\n")
	if err := cfg.SaveStudioSession("a=1; b=2", 1893456000); err != nil {
		t.Fatalf("SaveStudioSession: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.StudioCookieHeader() != "a=1; b=2" {
		t.Errorf("StudioCookieHeader = %q, want %q", reloaded.StudioCookieHeader(), "a=1; b=2")
	}
	if reloaded.SunoJwtExpiry != 1893456000 {
		t.Errorf("SunoJwtExpiry = %d, want 1893456000", reloaded.SunoJwtExpiry)
	}
}

func TestSaveSunoSessionPreservesStudioHeader(t *testing.T) {
	cfg, path := loadTempConfig(t, "base_url='https://studio-api-prod.suno.com'\njwt='old'\nstudio_cookie_header='keep=1'\n")
	if err := cfg.SaveSunoSession("newjwt", "", "", ""); err != nil {
		t.Fatalf("SaveSunoSession: %v", err)
	}
	reloaded, _ := Load(path)
	if reloaded.SunoJwt != "newjwt" {
		t.Errorf("SunoJwt = %q, want newjwt", reloaded.SunoJwt)
	}
	if reloaded.StudioCookieHeader() != "keep=1" {
		t.Errorf("StudioCookieHeader = %q, want keep=1 (cookies are decoupled from the JWT)", reloaded.StudioCookieHeader())
	}
}

func TestStudioCookieHeaderNilSafe(t *testing.T) {
	var c *Config
	if c.StudioCookieHeader() != "" {
		t.Errorf("nil StudioCookieHeader = %q, want empty", c.StudioCookieHeader())
	}
	if c.IsEnvAuth() {
		t.Errorf("nil IsEnvAuth = true, want false")
	}
}

func TestSaveSunoJWTOnlyClearsStudioPair(t *testing.T) {
	cfg, path := loadTempConfig(t, "base_url='https://studio-api-prod.suno.com'\njwt='old'\nstudio_cookie_header='stale=1'\njwt_expiry=111\n")
	if err := cfg.SaveSunoJWTOnly("newjwt"); err != nil {
		t.Fatalf("SaveSunoJWTOnly: %v", err)
	}
	reloaded, _ := Load(path)
	if reloaded.SunoJwt != "newjwt" {
		t.Errorf("SunoJwt = %q, want newjwt", reloaded.SunoJwt)
	}
	if reloaded.StudioCookieHeader() != "" {
		t.Errorf("StudioCookieHeader = %q, want empty (cleared by new JWT)", reloaded.StudioCookieHeader())
	}
	if reloaded.SunoJwtExpiry != 0 {
		t.Errorf("SunoJwtExpiry = %d, want 0 (cleared by new JWT)", reloaded.SunoJwtExpiry)
	}
}
