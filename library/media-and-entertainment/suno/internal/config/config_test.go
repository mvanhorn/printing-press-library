// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func TestLoadPrefersStoredJWTOverEnvToken(t *testing.T) {
	t.Setenv("SUNO_TOKEN", "env-token")
	t.Setenv("SUNO_JWT", "")

	path := writeTestConfig(t, `
jwt = "stored-token"
clerk_client_cookie = "stored-cookie"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.AuthHeader(); got != "Bearer stored-token" {
		t.Fatalf("AuthHeader() = %q, want stored token", got)
	}
	if cfg.AuthSource != "config" {
		t.Fatalf("AuthSource = %q, want config", cfg.AuthSource)
	}
	if cfg.EnvSunoJwt != "env-token" {
		t.Fatalf("EnvSunoJwt = %q, want retained fallback", cfg.EnvSunoJwt)
	}
}

func TestLoadUsesEnvTokenWhenNoStoredCredentials(t *testing.T) {
	t.Setenv("SUNO_TOKEN", "env-token")
	t.Setenv("SUNO_JWT", "")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.AuthHeader(); got != "Bearer env-token" {
		t.Fatalf("AuthHeader() = %q, want env token", got)
	}
	if cfg.AuthSource != "env:SUNO_TOKEN" {
		t.Fatalf("AuthSource = %q, want env:SUNO_TOKEN", cfg.AuthSource)
	}
}

func TestLoadKeepsEnvFallbackWhenOnlyStoredSessionExists(t *testing.T) {
	t.Setenv("SUNO_TOKEN", "env-token")
	t.Setenv("SUNO_JWT", "")

	path := writeTestConfig(t, `clerk_client_cookie = "stored-cookie"`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AuthSource != "config" {
		t.Fatalf("AuthSource before header = %q, want config", cfg.AuthSource)
	}
	if got := cfg.AuthHeader(); got != "Bearer env-token" {
		t.Fatalf("AuthHeader() = %q, want env fallback", got)
	}
	if cfg.AuthSource != "env:SUNO_TOKEN" {
		t.Fatalf("AuthSource after fallback = %q, want env:SUNO_TOKEN", cfg.AuthSource)
	}
}

// TestLoadEnvOnlyAuthDetectedBeforeAuthHeader is the regression guard for the
// env-only auth-routing bug: Load() must label AuthSource from the env token so
// IsEnvAuth() is correct before the first AuthHeader() call. newClient() consults
// IsEnvAuth() to route env-only users to the per-run browser cookie pull; when
// AuthSource was only set lazily in AuthHeader(), env users read false here and
// silently fell into the managed Clerk-session path with no studio cookies.
func TestLoadEnvOnlyAuthDetectedBeforeAuthHeader(t *testing.T) {
	cases := []struct{ name, tokenEnv, jwtEnv, wantSource string }{
		{"SUNO_TOKEN", "env-token", "", "env:SUNO_TOKEN"},
		{"SUNO_JWT", "", "env-jwt", "env:SUNO_JWT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SUNO_TOKEN", tc.tokenEnv)
			t.Setenv("SUNO_JWT", tc.jwtEnv)

			cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			// The assertion that matters: IsEnvAuth() before any AuthHeader() call.
			if !cfg.IsEnvAuth() {
				t.Fatalf("IsEnvAuth() = false right after Load(); env-only users must be detected before the first AuthHeader() call (AuthSource = %q)", cfg.AuthSource)
			}
			if cfg.AuthSource != tc.wantSource {
				t.Fatalf("AuthSource = %q, want %q", cfg.AuthSource, tc.wantSource)
			}
		})
	}
}

// TestClearTokensRevokesStudioSession guards that logout fully revokes the
// cached studio session — the durable suno.com cookie header and its paired
// expiry — not just the JWT/OAuth credentials.
func TestClearTokensRevokesStudioSession(t *testing.T) {
	path := writeTestConfig(t, "base_url='https://studio-api-prod.suno.com'\njwt='x'\nstudio_cookie_header='a=1; b=2'\njwt_expiry=1893456000\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.StudioCookieHeader() == "" {
		t.Fatalf("precondition: studio cookie header not loaded")
	}
	if err := cfg.ClearTokens(); err != nil {
		t.Fatalf("ClearTokens: %v", err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := reloaded.StudioCookieHeader(); got != "" {
		t.Errorf("StudioCookieHeader after logout = %q, want empty", got)
	}
	if reloaded.SunoJwtExpiry != 0 {
		t.Errorf("SunoJwtExpiry after logout = %d, want 0", reloaded.SunoJwtExpiry)
	}
	if reloaded.SunoJwt != "" {
		t.Errorf("SunoJwt after logout = %q, want empty", reloaded.SunoJwt)
	}
}
