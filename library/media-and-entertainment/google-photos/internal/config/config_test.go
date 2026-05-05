// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveTokens_WithAccountStoresAccountBucket(t *testing.T) {
	cfg := &Config{Path: filepath.Join(t.TempDir(), "config.toml")}
	expiry := time.Now().Add(time.Hour).UTC()

	if err := cfg.SaveTokens("User@Example.COM", "client-id", "client-secret", "access", "refresh", expiry); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}

	loaded, err := Load(cfg.Path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	loaded.SelectAccount("user@example.com")
	token, ok := loaded.SelectedOAuthAccount()
	if !ok {
		t.Fatalf("expected stored account token")
	}
	if token.AccessToken != "access" || token.RefreshToken != "refresh" {
		t.Fatalf("stored token mismatch: %+v", token)
	}
	if loaded.DefaultAccount != "user@example.com" {
		t.Fatalf("default account = %q, want user@example.com", loaded.DefaultAccount)
	}
	if got := loaded.AuthHeader(); got != "Bearer access" {
		t.Fatalf("AuthHeader = %q, want Bearer access", got)
	}
}

func TestClearTokens_WithAccountPreservesLegacyToken(t *testing.T) {
	cfg := &Config{
		Path:         filepath.Join(t.TempDir(), "config.toml"),
		AccessToken:  "legacy",
		RefreshToken: "legacy-refresh",
	}
	if err := cfg.SaveTokens("user@example.com", "client-id", "", "access", "refresh", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}
	if err := cfg.ClearTokens("user@example.com"); err != nil {
		t.Fatalf("ClearTokens: %v", err)
	}
	loaded, err := Load(cfg.Path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.AccessToken != "legacy" {
		t.Fatalf("legacy token should remain, got %q", loaded.AccessToken)
	}
	if _, ok := loaded.Accounts["user@example.com"]; ok {
		t.Fatalf("account token should have been removed")
	}
}
