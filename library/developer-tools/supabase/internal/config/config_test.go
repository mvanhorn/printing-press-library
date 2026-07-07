// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigTokenSourceSurvivesAuthHeader(t *testing.T) {
	t.Setenv("SUPABASE_ACCESS_TOKEN", "")
	t.Setenv("SUPABASE_BASE_URL", "")
	t.Setenv("SUPABASE_CONFIG", "")

	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte("access_token = \"file-token\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AuthSource != "config" {
		t.Fatalf("AuthSource after Load = %q, want config", cfg.AuthSource)
	}
	if got := cfg.AuthHeader(); got != "Bearer file-token" {
		t.Fatalf("AuthHeader() = %q, want Bearer file-token", got)
	}
	if cfg.AuthSource != "config" {
		t.Fatalf("AuthSource after AuthHeader = %q, want config", cfg.AuthSource)
	}
}

func TestLoadEnvTokenOverridesConfigTokenSource(t *testing.T) {
	t.Setenv("SUPABASE_ACCESS_TOKEN", "env-token")
	t.Setenv("SUPABASE_BASE_URL", "")
	t.Setenv("SUPABASE_CONFIG", "")

	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte("access_token = \"file-token\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AccessToken != "env-token" {
		t.Fatalf("AccessToken = %q, want env-token", cfg.AccessToken)
	}
	if cfg.AuthSource != "env:SUPABASE_ACCESS_TOKEN" {
		t.Fatalf("AuthSource after Load = %q, want env:SUPABASE_ACCESS_TOKEN", cfg.AuthSource)
	}
	if got := cfg.AuthHeader(); got != "Bearer env-token" {
		t.Fatalf("AuthHeader() = %q, want Bearer env-token", got)
	}
	if cfg.AuthSource != "env:SUPABASE_ACCESS_TOKEN" {
		t.Fatalf("AuthSource after AuthHeader = %q, want env:SUPABASE_ACCESS_TOKEN", cfg.AuthSource)
	}
}

func TestAuthHeaderLabelsUnloadedAccessTokenAsOAuth2(t *testing.T) {
	cfg := &Config{AccessToken: "persisted-token"}
	if got := cfg.AuthHeader(); got != "Bearer persisted-token" {
		t.Fatalf("AuthHeader() = %q, want Bearer persisted-token", got)
	}
	if cfg.AuthSource != "oauth2" {
		t.Fatalf("AuthSource = %q, want oauth2", cfg.AuthSource)
	}
}
