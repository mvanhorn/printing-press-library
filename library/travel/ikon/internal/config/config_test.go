// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveTokensPreservesEnvOverriddenBaseURL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`base_url = "https://account.ikonpass.com"`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("IKON_BASE_URL", "https://mock.example.test")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cfg.BaseURL, "https://mock.example.test"; got != want {
		t.Fatalf("BaseURL = %q, want %q", got, want)
	}
	if err := cfg.SaveTokens("", "", "session=abc", "", time.Time{}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "mock.example.test") {
		t.Fatalf("env-only BaseURL was persisted to config:\n%s", data)
	}
	if !strings.Contains(string(data), "account.ikonpass.com") {
		t.Fatalf("file BaseURL was not preserved:\n%s", data)
	}
}

func TestSaveTokensWritesConfigAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveTokens("", "", "session=abc", "", time.Time{}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("config mode = %v, want 0600", got)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".config.toml.tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary config files left behind: %v", matches)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "session=abc") {
		t.Fatalf("saved token missing from config:\n%s", data)
	}
}
