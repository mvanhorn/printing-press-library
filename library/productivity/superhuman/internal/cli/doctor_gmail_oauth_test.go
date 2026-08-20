// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/productivity/superhuman/internal/config"
)

// seedOAuthAccount writes a tokens.json with a Google account whose
// AccessToken expiry is `expiresFromNow` from time.Now(). Negative
// durations seed an already-expired token.
func seedOAuthAccount(t *testing.T, tokenStorePath, email, googleID string, expiresFromNow time.Duration) {
	t.Helper()
	store := auth.NewStoreAt(tokenStorePath)
	expiresMs := time.Now().Add(expiresFromNow).UnixMilli()
	_, err := store.Upsert(email, auth.AccountTokens{
		Type:         "google",
		AccessToken:  "ya29.test",
		RefreshToken: "rt-" + email,
		UserID:       googleID,
		Expires:      expiresMs,
		SuperhumanToken: auth.SuperhumanToken{
			Token:   "id-" + email,
			Expires: time.Now().Add(time.Hour).UnixMilli(),
		},
		LastUsedAt: time.Now().UnixMilli(),
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// configForOAuthTest writes a config.toml pointing at a temp store and
// returns the loaded *config.Config so the doctor helper can be called
// directly.
func configForOAuthTest(t *testing.T, email string) *config.Config {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	tokenStorePath := filepath.Join(dir, "tokens.json")
	writeConfigPointingAt(t, configPath, "http://unused", email)
	if email != "" {
		seedOAuthAccount(t, tokenStorePath, email, "gid-test", time.Hour)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

func TestCollectGmailOAuthReport_HealthyToken(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	tokenStorePath := filepath.Join(dir, "tokens.json")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")
	seedOAuthAccount(t, tokenStorePath, "user@example.com", "gid-test", 50*time.Minute)
	cfg, _ := config.Load(configPath)

	got := collectGmailOAuthReport(cfg)
	if !strings.HasPrefix(got, "ok") {
		t.Fatalf("expected ok, got %q", got)
	}
}

func TestCollectGmailOAuthReport_ExpiringSoon(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	tokenStorePath := filepath.Join(dir, "tokens.json")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")
	seedOAuthAccount(t, tokenStorePath, "user@example.com", "gid-test", 90*time.Second)
	cfg, _ := config.Load(configPath)

	got := collectGmailOAuthReport(cfg)
	if !strings.HasPrefix(got, "WARN") {
		t.Fatalf("expected WARN for <5min, got %q", got)
	}
	if !strings.Contains(got, "auth login --disk") {
		t.Fatalf("WARN should hint at 'auth login --disk': %q", got)
	}
}

func TestCollectGmailOAuthReport_Expired(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	tokenStorePath := filepath.Join(dir, "tokens.json")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")
	seedOAuthAccount(t, tokenStorePath, "user@example.com", "gid-test", -10*time.Minute)
	cfg, _ := config.Load(configPath)

	got := collectGmailOAuthReport(cfg)
	if !strings.HasPrefix(got, "FAILED") {
		t.Fatalf("expected FAILED for expired token, got %q", got)
	}
	if !strings.Contains(got, "refresh-on-401") {
		t.Fatalf("FAILED should mention recovery path: %q", got)
	}
}

func TestCollectGmailOAuthReport_NoActiveAccount(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	writeConfigPointingAt(t, configPath, "http://unused", "")
	cfg, _ := config.Load(configPath)

	got := collectGmailOAuthReport(cfg)
	if !strings.HasPrefix(got, "skipped") {
		t.Fatalf("expected skipped, got %q", got)
	}
}

func TestCollectGmailOAuthReport_NoAccessToken(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	tokenStorePath := filepath.Join(dir, "tokens.json")
	writeConfigPointingAt(t, configPath, "http://unused", "user@example.com")
	// Seed an account WITHOUT an AccessToken.
	store := auth.NewStoreAt(tokenStorePath)
	_, _ = store.Upsert("user@example.com", auth.AccountTokens{
		Type:    "google",
		UserID:  "gid-test",
		Expires: time.Now().Add(time.Hour).UnixMilli(),
	})
	cfg, _ := config.Load(configPath)

	got := collectGmailOAuthReport(cfg)
	if !strings.HasPrefix(got, "FAILED") {
		t.Fatalf("expected FAILED for missing access token, got %q", got)
	}
}
