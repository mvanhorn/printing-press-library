// Copyright 2026 Eldar and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBaseURLNeverPersistsToken regression-tests a bug where EffectiveBaseURL's
// predecessor folded the resolved API token directly into cfg.BaseURL inside
// Load(). Since BaseURL is a persisted field, saving after that mutation wrote
// the token-embedded URL to config.toml; the next Load() read the poisoned
// value back and appended a second token on top, and logout never actually
// cleared the token from the persisted URL. BaseURL must stay the clean
// template on disk across rotation and logout.
func TestBaseURLNeverPersistsToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("initial Load: %v", err)
	}
	if err := cfg.SaveCredential("TOKEN1AAAAAAAAAAAAAAA"); err != nil {
		t.Fatalf("SaveCredential(TOKEN1): %v", err)
	}

	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("reload after TOKEN1: %v", err)
	}
	if cfg.BaseURL != "https://api.workiz.com/api/v1" {
		t.Fatalf("BaseURL after first token save = %q, want clean template", cfg.BaseURL)
	}
	if got := cfg.EffectiveBaseURL(); got != "https://api.workiz.com/api/v1/TOKEN1AAAAAAAAAAAAAAA" {
		t.Fatalf("EffectiveBaseURL after TOKEN1 = %q", got)
	}

	if err := cfg.SaveCredential("TOKEN2BBBBBBBBBBBBBBB"); err != nil {
		t.Fatalf("SaveCredential(TOKEN2) rotation: %v", err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("reload after rotation: %v", err)
	}
	if cfg.BaseURL != "https://api.workiz.com/api/v1" {
		t.Fatalf("BaseURL after token rotation = %q, want clean template (no doubled token)", cfg.BaseURL)
	}
	if got := cfg.EffectiveBaseURL(); got != "https://api.workiz.com/api/v1/TOKEN2BBBBBBBBBBBBBBB" {
		t.Fatalf("EffectiveBaseURL after rotation = %q, want only the new token", got)
	}

	if err := cfg.ClearTokens(); err != nil {
		t.Fatalf("ClearTokens (logout): %v", err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("reload after logout: %v", err)
	}
	if cfg.BaseURL != "https://api.workiz.com/api/v1" {
		t.Fatalf("BaseURL after logout = %q, want clean template", cfg.BaseURL)
	}
	if cfg.WorkizApiToken != "" {
		t.Fatalf("WorkizApiToken after logout = %q, want empty", cfg.WorkizApiToken)
	}
	if got := cfg.EffectiveBaseURL(); got != "https://api.workiz.com/api/v1" {
		t.Fatalf("EffectiveBaseURL after logout = %q, want the plain template with no token", got)
	}
}

func TestEffectiveBaseURLSkipsInjectionUnderVerifyMockOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	t.Setenv("WORKIZ_BASE_URL", "http://127.0.0.1:9999")
	t.Setenv("WORKIZ_API_TOKEN", "sometoken")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.EffectiveBaseURL(); got != "http://127.0.0.1:9999" {
		t.Fatalf("EffectiveBaseURL under WORKIZ_BASE_URL override = %q, want the mock server URL with no token appended", got)
	}
}

func TestDisplayBaseURLMasksToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	os.Unsetenv("WORKIZ_BASE_URL")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.SaveCredential("supersecrettoken1234"); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got := cfg.DisplayBaseURL()
	if got == cfg.EffectiveBaseURL() {
		t.Fatalf("DisplayBaseURL() = %q, expected token to be masked, not equal to raw EffectiveBaseURL()", got)
	}
	want := "https://api.workiz.com/api/v1/****1234"
	if got != want {
		t.Fatalf("DisplayBaseURL() = %q, want %q", got, want)
	}
}

// TestAuthHeaderRequiresToken regression-tests a Greptile finding: AuthHeader()
// used to fall back to returning WorkizApiSecret alone when the token was
// absent, which made `auth status` report "credentials present" for a
// secret-only config even though every real request would fail (the secret is
// only ever sent as a POST-body field, never a header, and EffectiveBaseURL()
// omits the required token path segment without a token).
// TestSaveSecretPersists regression-tests a Greptile finding: there was no way
// to persist WorkizApiSecret to config.toml, so a user who ran `auth
// set-token` but not `export WORKIZ_API_SECRET` in every shell had every
// write request silently sent without the required auth_secret body field.
// SaveSecret must persist across reload and must not disturb the token.
func TestSaveSecretPersists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	os.Unsetenv("WORKIZ_API_SECRET")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("initial Load: %v", err)
	}
	if err := cfg.SaveCredential("TOKEN1AAAAAAAAAAAAAAA"); err != nil {
		t.Fatalf("SaveCredential: %v", err)
	}
	if err := cfg.SaveSecret("SECRET1AAAAAAAAAAAAAA"); err != nil {
		t.Fatalf("SaveSecret: %v", err)
	}

	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if cfg.WorkizApiSecret != "SECRET1AAAAAAAAAAAAAA" {
		t.Fatalf("WorkizApiSecret after reload = %q, want persisted secret", cfg.WorkizApiSecret)
	}
	if cfg.WorkizApiToken != "TOKEN1AAAAAAAAAAAAAAA" {
		t.Fatalf("WorkizApiToken after secret save = %q, want unchanged token", cfg.WorkizApiToken)
	}

	if err := cfg.SaveSecret("SECRET2BBBBBBBBBBBBBB"); err != nil {
		t.Fatalf("SaveSecret rotation: %v", err)
	}
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("reload after rotation: %v", err)
	}
	if cfg.WorkizApiSecret != "SECRET2BBBBBBBBBBBBBB" {
		t.Fatalf("WorkizApiSecret after rotation = %q, want new secret only", cfg.WorkizApiSecret)
	}
}

func TestAuthHeaderRequiresToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	os.Unsetenv("WORKIZ_BASE_URL")
	os.Unsetenv("WORKIZ_API_TOKEN")
	t.Setenv("WORKIZ_API_SECRET", "onlysecretnoToken")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.AuthHeader(); got != "" {
		t.Fatalf("AuthHeader() with secret but no token = %q, want empty (secret alone is not a usable credential)", got)
	}
}
