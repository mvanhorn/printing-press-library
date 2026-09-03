// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCachedTokenIsBoundToFedExBaseURL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := &Config{
		Path:             path,
		BaseURL:          "https://apis.fedex.com",
		CacheAccessToken: true,
	}
	if err := cfg.SaveTokens("", "", "production-token", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.BaseURL != "https://apis.fedex.com" || loaded.TokenBaseURL != loaded.BaseURL || loaded.AccessToken != "production-token" {
		t.Fatalf("production token binding not preserved: %#v", loaded)
	}
}

func TestLoadDiscardsTokenBoundToDifferentEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := "base_url = 'https://apis.fedex.com'\naccess_token = 'sandbox-token'\ntoken_base_url = 'https://apis-sandbox.fedex.com'\ntoken_expiry = " + time.Now().Add(time.Hour).UTC().Format(time.RFC3339) + "\ncache_access_token = true\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AccessToken != "" || cfg.TokenBaseURL != "" {
		t.Fatalf("mismatched cached token survived load: %#v", cfg)
	}
}

func TestLoadDiscardsLegacyTokenWithoutOrigin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := "base_url = 'https://apis-sandbox.fedex.com'\naccess_token = 'legacy-token'\ntoken_expiry = " + time.Now().Add(time.Hour).UTC().Format(time.RFC3339) + "\ncache_access_token = true\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AccessToken != "" || cfg.TokenBaseURL != "" {
		t.Fatalf("unbound cached token survived load: %#v", cfg)
	}
}

func TestBaseURLOverrideDiscardsCachedTokenBeforeUse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := &Config{Path: path, BaseURL: "https://apis-sandbox.fedex.com", CacheAccessToken: true}
	if err := cfg.SaveTokens("", "", "sandbox-token", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	t.Setenv("FEDEX_BASE_URL", "https://apis.fedex.com")
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.AccessToken != "" || loaded.TokenBaseURL != "" {
		t.Fatalf("sandbox token survived production override: %#v", loaded)
	}
}

func TestClientIDsAreNeverBearerTokens(t *testing.T) {
	cfg := &Config{FedexApiKey: "client-id", TrackApiKey: "track-client-id"}
	if got := cfg.AuthHeader(); got != "" {
		t.Fatalf("client ID became default bearer header: %q", got)
	}
	if got := cfg.AuthHeaderForPath("/track/v1/trackingnumbers"); got != "" {
		t.Fatalf("Track client ID became bearer header: %q", got)
	}
}
