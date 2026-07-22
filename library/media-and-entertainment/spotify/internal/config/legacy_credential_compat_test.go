package config

import (
	"os"
	"path/filepath"
	"testing"
)

func sandboxCredentialEnv(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
	for _, k := range []string{"SPOTIFY_OAUTH_2_0", "SPOTIFY_WEB_OAUTH_2_0", "SPOTIFY_CONFIG"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
}

func TestLegacyWebOauth20ConfigKeyPromotes(t *testing.T) {
	sandboxCredentialEnv(t)
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte("web_oauth_2_0 = \"legacy-bearer\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.AuthHeader(); got != "Bearer legacy-bearer" {
		t.Fatalf("AuthHeader() = %q, want Bearer legacy-bearer", got)
	}
}

func TestLegacyWebOauth20EnvVarHonored(t *testing.T) {
	sandboxCredentialEnv(t)
	t.Setenv("SPOTIFY_WEB_OAUTH_2_0", "legacy-env-bearer")
	cfg, err := Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.AuthHeader(); got != "Bearer legacy-env-bearer" {
		t.Fatalf("AuthHeader() = %q, want Bearer legacy-env-bearer", got)
	}
	if cfg.AuthSource != "env:SPOTIFY_WEB_OAUTH_2_0" {
		t.Fatalf("AuthSource = %q", cfg.AuthSource)
	}
}

func TestNewOauth20KeyWinsOverLegacy(t *testing.T) {
	sandboxCredentialEnv(t)
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte("oauth_2_0 = \"new-bearer\"\nweb_oauth_2_0 = \"legacy-bearer\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.AuthHeader(); got != "Bearer new-bearer" {
		t.Fatalf("AuthHeader() = %q, want new key to win", got)
	}
}
