// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAuthorizationFromEnvironment(t *testing.T) {
	t.Setenv("AIRBYTE_ADMIN_AUTH_HEADER", "Basic local-user-pass")
	t.Setenv("AIRBYTE_ADMIN_TOKEN", "ignored-token")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got := cfg.AuthHeader(); got != "Basic local-user-pass" {
		t.Fatalf("AuthHeader() = %q, want full header from AIRBYTE_ADMIN_AUTH_HEADER", got)
	}
	if got := cfg.AuthSource; got != "env:AIRBYTE_ADMIN_AUTH_HEADER" {
		t.Fatalf("AuthSource = %q, want env:AIRBYTE_ADMIN_AUTH_HEADER", got)
	}
}

func TestLoadBearerTokenFromEnvironment(t *testing.T) {
	t.Setenv("AIRBYTE_ADMIN_TOKEN", "airbyte-token")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got := cfg.AuthHeader(); got != "Bearer airbyte-token" {
		t.Fatalf("AuthHeader() = %q, want bearer header", got)
	}
	if got := cfg.AuthSource; got != "env:AIRBYTE_ADMIN_TOKEN" {
		t.Fatalf("AuthSource = %q, want env:AIRBYTE_ADMIN_TOKEN", got)
	}
}

func TestLoadBearerTokenDoesNotDoublePrefix(t *testing.T) {
	t.Setenv("AIRBYTE_ADMIN_TOKEN", "Bearer already-prefixed")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got := cfg.AuthHeader(); got != "Bearer already-prefixed" {
		t.Fatalf("AuthHeader() = %q, want token without duplicate bearer prefix", got)
	}
}

func TestLoadConfigAuthHeaderAndEnvOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("auth_header = \"Bearer from-config\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if got := cfg.AuthHeader(); got != "Bearer from-config" {
		t.Fatalf("AuthHeader() = %q, want config auth header", got)
	}
	if got := cfg.AuthSource; got != "config" {
		t.Fatalf("AuthSource = %q, want config", got)
	}

	t.Setenv("AIRBYTE_ADMIN_TOKEN", "from-env")
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("Load with env returned error: %v", err)
	}
	if got := cfg.AuthHeader(); got != "Bearer from-env" {
		t.Fatalf("AuthHeader() = %q, want env token override", got)
	}
	if got := cfg.AuthSource; got != "env:AIRBYTE_ADMIN_TOKEN" {
		t.Fatalf("AuthSource = %q, want env:AIRBYTE_ADMIN_TOKEN", got)
	}
}
