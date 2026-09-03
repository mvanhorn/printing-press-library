// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTokenCacheRejectsMissingExpiryAndNeverPersistsRefreshToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := &Config{Path: path, BaseURL: "https://apis-sandbox.fedex.com", CacheAccessToken: true}
	if err := cfg.SaveTokens("", "", "no-expiry-token", "sentinel-refresh", time.Time{}); err == nil {
		t.Fatal("SaveTokens accepted a cached token with no expiry")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("invalid token cache created a file: %v", err)
	}

	if err := cfg.SaveTokens("", "", "bounded-token", "sentinel-refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens bounded token: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "sentinel-refresh") || strings.Contains(string(data), "refresh_token = 'sentinel-refresh'") {
		t.Fatalf("refresh token was serialized: %s", data)
	}
}

func TestLoadDiscardsExpiredCachedToken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	data := "access_token = 'expired-token'\ntoken_expiry = 2020-01-01T00:00:00Z\ncache_access_token = true\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AccessToken != "" || !cfg.TokenExpiry.IsZero() {
		t.Fatalf("expired token survived load: %#v", cfg)
	}
}
