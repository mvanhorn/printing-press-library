// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAuthSourceMatchesCredentialPrecedence(t *testing.T) {
	t.Setenv("FIGMA_ACCESS_TOKEN", "access-token")
	t.Setenv("FIGMA_API_TOKEN", "api-token")
	t.Setenv("FIGMA_API_KEY", "api-key")

	configPath := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(configPath, []byte("auth_header = 'stored-token'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.AuthHeader(), "access-token"; got != want {
		t.Fatalf("AuthHeader() = %q, want %q", got, want)
	}
	if got, want := cfg.AuthSource, "env:FIGMA_ACCESS_TOKEN"; got != want {
		t.Fatalf("AuthSource = %q, want %q", got, want)
	}
}

func TestLoadAuthAliasesOverridePersistedCredential(t *testing.T) {
	for _, tc := range []struct {
		name   string
		env    string
		value  string
		source string
	}{
		{name: "api token", env: "FIGMA_API_TOKEN", value: "api-token", source: "env:FIGMA_API_TOKEN"},
		{name: "api key", env: "FIGMA_API_KEY", value: "api-key", source: "env:FIGMA_API_KEY"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("FIGMA_ACCESS_TOKEN", "")
			t.Setenv("FIGMA_API_TOKEN", "")
			t.Setenv("FIGMA_API_KEY", "")
			t.Setenv(tc.env, tc.value)

			configPath := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(configPath, []byte("access_token = 'stored-token'\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if got := cfg.AuthHeader(); got != tc.value {
				t.Fatalf("AuthHeader() = %q, want %q", got, tc.value)
			}
			if got := cfg.AuthSource; got != tc.source {
				t.Fatalf("AuthSource = %q, want %q", got, tc.source)
			}
		})
	}
}
