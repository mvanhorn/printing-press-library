// Copyright 2026 zaydiscold. Licensed under Apache-2.0. See LICENSE.

package config

import "testing"

func TestLoadDefaultBaseURLIsProduction(t *testing.T) {
	t.Setenv("GODADDY_CONFIG", t.TempDir()+"/missing.toml")
	t.Setenv("GODADDY_BASE_URL", "")
	t.Setenv("GODADDY_API_BASE_URL", "")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := cfg.BaseURL, "https://api.godaddy.com"; got != want {
		t.Fatalf("BaseURL = %q, want %q", got, want)
	}
}

func TestLoadBaseURLEnvOverride(t *testing.T) {
	t.Setenv("GODADDY_CONFIG", t.TempDir()+"/missing.toml")
	t.Setenv("GODADDY_BASE_URL", "https://api.ote-godaddy.com")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got, want := cfg.BaseURL, "https://api.ote-godaddy.com"; got != want {
		t.Fatalf("BaseURL = %q, want %q", got, want)
	}
}
