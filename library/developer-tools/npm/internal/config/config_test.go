package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnvTokenIsNotPersistedBySaveTokens(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := &Config{Path: path, BaseURL: "https://registry.npmjs.org", NpmRegistrySBearerAuth: "env-only"}

	if err := cfg.SaveTokens("", "", "saved-token", "", time.Time{}); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "env-only") || strings.Contains(string(data), "registry_s_bearer_auth") {
		t.Fatalf("env token was persisted to config:\n%s", string(data))
	}
}

func TestClearTokensRemovesRegistryBearerAuth(t *testing.T) {
	cfg := &Config{
		Path:                   filepath.Join(t.TempDir(), "config.toml"),
		BaseURL:                "https://registry.npmjs.org",
		AccessToken:            "saved-token",
		NpmRegistrySBearerAuth: "env-only",
	}

	if err := cfg.ClearTokens(); err != nil {
		t.Fatalf("ClearTokens: %v", err)
	}
	if cfg.NpmRegistrySBearerAuth != "" {
		t.Fatalf("registry bearer auth should be cleared, got %q", cfg.NpmRegistrySBearerAuth)
	}
}
