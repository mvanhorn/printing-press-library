// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveTokensPersistsOnlyShortLivedToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "config.toml")
	cfg := &Config{Path: path, BaseURL: "https://apis-sandbox.fedex.com", CacheAccessToken: true}
	if err := cfg.SaveTokens("sentinel-client-id", "sentinel-client-secret", "short-lived-access-token", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(data)
	for _, forbidden := range []string{"sentinel-client-id", "sentinel-client-secret"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("config persisted reusable credential %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, "short-lived-access-token") {
		t.Fatalf("config did not persist access token: %s", text)
	}
	if got := mustMode(t, filepath.Dir(path)); got != 0o700 {
		t.Fatalf("config dir mode=%#o, want 0700", got)
	}
	if got := mustMode(t, path); got != 0o600 {
		t.Fatalf("config file mode=%#o, want 0600", got)
	}
}

func TestSaveTightensExistingModesAndRemovesLegacySecrets(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "config")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.toml")
	legacy := []byte("client_id = 'legacy-id'\nclient_secret = 'legacy-secret'\nauth_header = 'Bearer legacy-bearer'\n")
	if err := os.WriteFile(path, legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.CacheAccessToken = true
	if err := cfg.SaveTokens(cfg.ClientID, cfg.ClientSecret, "new-access-token", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"legacy-id", "legacy-secret", "legacy-bearer"} {
		if strings.Contains(string(data), forbidden) {
			t.Fatalf("legacy credential %q remained in config: %s", forbidden, data)
		}
	}
	if got := mustMode(t, dir); got != 0o700 {
		t.Fatalf("config dir mode=%#o, want 0700", got)
	}
	if got := mustMode(t, path); got != 0o600 {
		t.Fatalf("config file mode=%#o, want 0600", got)
	}
}

func TestAutomaticTokenMintDoesNotPersistWithoutOptIn(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config", "config.toml")
	cfg := &Config{Path: path, BaseURL: "https://apis-sandbox.fedex.com"}
	if err := cfg.SaveTokens("sentinel-client-id", "sentinel-client-secret", "memory-only-access-token", "", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	if cfg.AccessToken != "memory-only-access-token" {
		t.Fatal("uncached token was not retained in process memory")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("token cache was created without opt-in: %v", err)
	}
}

func TestClearTokensRemovesAllCachedAndLegacyCredentialFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := &Config{
		Path:              path,
		CacheAccessToken:  true,
		AuthHeaderVal:     "Bearer sentinel-auth",
		AccessToken:       "sentinel-access",
		RefreshToken:      "sentinel-refresh",
		ClientID:          "sentinel-id",
		ClientSecret:      "sentinel-secret",
		FedexApiKey:       "sentinel-key",
		FedexSecretKey:    "sentinel-key-secret",
		TrackClientID:     "sentinel-track-id",
		TrackClientSecret: "sentinel-track-secret",
		TrackAccessToken:  "sentinel-track-access",
		TrackApiKey:       "sentinel-track-key",
		TrackSecretKey:    "sentinel-track-key-secret",
	}
	if err := cfg.ClearTokens(); err != nil {
		t.Fatalf("ClearTokens: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sentinel-") {
		t.Fatalf("cleared config still contains credentials: %s", data)
	}
	if cfg.AuthHeaderVal != "" || cfg.AccessToken != "" || cfg.ClientID != "" || cfg.ClientSecret != "" || cfg.TrackAccessToken != "" || cfg.TrackClientSecret != "" || cfg.CacheAccessToken {
		t.Fatalf("ClearTokens left credentials in memory: %#v", cfg)
	}
}

func mustMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}
