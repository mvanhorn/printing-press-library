package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadReadsDotEnvBesideDefaultConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	clearAmazonAdsEnv(t)

	configDir := filepath.Join(home, ".config", "amazon-ads-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	dotEnv := []byte("AMAZON_ADS_CLIENT_ID=client-from-file\nAMAZON_ADS_CLIENT_SECRET='secret-from-file'\nAMAZON_ADS_REFRESH_TOKEN=\"refresh-from-file\"\nAMAZON_ADS_PROFILE_ID=profile-from-file\n")
	if err := os.WriteFile(filepath.Join(configDir, ".env"), dotEnv, 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ClientID != "client-from-file" {
		t.Fatalf("ClientID = %q, want client-from-file", cfg.ClientID)
	}
	if cfg.ClientSecret != "secret-from-file" {
		t.Fatalf("ClientSecret = %q, want secret-from-file", cfg.ClientSecret)
	}
	if cfg.RefreshToken != "refresh-from-file" {
		t.Fatalf("RefreshToken = %q, want refresh-from-file", cfg.RefreshToken)
	}
	if cfg.AmazonAdsProfileId != "profile-from-file" {
		t.Fatalf("AmazonAdsProfileId = %q, want profile-from-file", cfg.AmazonAdsProfileId)
	}
	if cfg.AmazonAdsApiClientId != "client-from-file" {
		t.Fatalf("AmazonAdsApiClientId = %q, want client-from-file", cfg.AmazonAdsApiClientId)
	}
}

func TestProcessEnvOverridesDotEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	clearAmazonAdsEnv(t)
	t.Setenv("AMAZON_ADS_CLIENT_ID", "client-from-env")

	configDir := filepath.Join(home, ".config", "amazon-ads-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, ".env"), []byte("AMAZON_ADS_CLIENT_ID=client-from-file\n"), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.ClientID != "client-from-env" {
		t.Fatalf("ClientID = %q, want client-from-env", cfg.ClientID)
	}
	if cfg.AuthSource != "env:AMAZON_ADS_CLIENT_ID" {
		t.Fatalf("AuthSource = %q, want env:AMAZON_ADS_CLIENT_ID", cfg.AuthSource)
	}
}

func TestSaveTokensWritesDotEnvBesideConfig(t *testing.T) {
	home := t.TempDir()
	clearAmazonAdsEnv(t)

	configDir := filepath.Join(home, ".config", "amazon-ads-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	dotEnvPath := filepath.Join(configDir, ".env")
	initial := strings.Join([]string{
		"# existing user notes",
		"AMAZON_ADS_CLIENT_ID=old-client",
		"AMAZON_ADS_REFRESH_TOKEN=",
		"AMAZON_ADS_BASE_URL=https://example.test",
		"",
	}, "\n")
	if err := os.WriteFile(dotEnvPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	cfg := &Config{Path: filepath.Join(configDir, "config.toml"), AmazonAdsProfileId: "profile-123"}
	if err := cfg.SaveTokens("client-new", "secret with spaces", "access-token", "refresh-new", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens returned error: %v", err)
	}

	data, err := os.ReadFile(dotEnvPath)
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	got := string(data)
	for _, want := range []string{
		"# existing user notes",
		"AMAZON_ADS_CLIENT_ID=client-new",
		"AMAZON_ADS_CLIENT_SECRET=\"secret with spaces\"",
		"AMAZON_ADS_REFRESH_TOKEN=refresh-new",
		"AMAZON_ADS_API_CLIENT_ID=client-new",
		"AMAZON_ADS_PROFILE_ID=profile-123",
		"AMAZON_ADS_BASE_URL=https://example.test",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf(".env missing %q in:\n%s", want, got)
		}
	}
}

func TestSaveAmazonAdsProfileWritesDotEnv(t *testing.T) {
	home := t.TempDir()
	clearAmazonAdsEnv(t)

	configDir := filepath.Join(home, ".config", "amazon-ads-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	cfg := &Config{
		Path:                 filepath.Join(configDir, "config.toml"),
		AmazonAdsClientId:    "client-id",
		AmazonAdsApiClientId: "client-id",
	}
	if err := cfg.SaveAmazonAdsProfile("profile-new"); err != nil {
		t.Fatalf("SaveAmazonAdsProfile returned error: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(configDir, ".env"))
	if err != nil {
		t.Fatalf("read .env: %v", err)
	}
	got := string(data)
	for _, want := range []string{
		"AMAZON_ADS_API_CLIENT_ID=client-id",
		"AMAZON_ADS_PROFILE_ID=profile-new",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf(".env missing %q in:\n%s", want, got)
		}
	}
}

func clearAmazonAdsEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"AMAZON_ADS_CLIENT_ID",
		"AMAZON_ADS_CLIENT_SECRET",
		"AMAZON_ADS_REFRESH_TOKEN",
		"AMAZON_ADS_API_CLIENT_ID",
		"AMAZON_ADS_PROFILE_ID",
		"AMAZON_ADS_BASE_URL",
		"AMAZON_ADS_AUTHORIZATION_URL",
		"AMAZON_ADS_TOKEN_URL",
		"AMAZON_ADS_CONFIG",
	} {
		t.Setenv(key, "")
	}
}
