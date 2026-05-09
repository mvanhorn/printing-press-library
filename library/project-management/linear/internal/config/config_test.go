package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLinearAPIKey_persistsApiKeyAndClearsOAuthFields(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := &Config{
		Path:         path,
		BaseURL:      "https://api.linear.app/graphql",
		AccessToken:  "old-oauth",
		RefreshToken: "old-refresh",
		ClientID:     "cid",
	}
	if err := cfg.SaveTokens(cfg.ClientID, "", cfg.AccessToken, cfg.RefreshToken, cfg.TokenExpiry); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}

	cfg2, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg2.SaveLinearAPIKey("lin_api_test_example"); err != nil {
		t.Fatalf("SaveLinearAPIKey: %v", err)
	}

	cfg3, err := Load(path)
	if err != nil {
		t.Fatalf("Load after SaveLinearAPIKey: %v", err)
	}
	if cfg3.LinearApiKey != "lin_api_test_example" {
		t.Fatalf("api_key: got %q", cfg3.LinearApiKey)
	}
	if cfg3.AccessToken != "" || cfg3.RefreshToken != "" || cfg3.ClientID != "" {
		t.Fatalf("expected oauth fields cleared, got access=%q refresh=%q clientID=%q", cfg3.AccessToken, cfg3.RefreshToken, cfg3.ClientID)
	}
}

func TestSaveTokens_persistsAccessToken(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := &Config{
		Path:        path,
		BaseURL:     "https://api.linear.app/graphql",
		LinearApiKey: "lin_api_existing",
	}
	if err := cfg.SaveLinearAPIKey(cfg.LinearApiKey); err != nil {
		t.Fatalf("SaveLinearAPIKey: %v", err)
	}

	cfg2, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg2.SaveTokens("", "", "oauth_access_example", "", cfg2.TokenExpiry); err != nil {
		t.Fatalf("SaveTokens: %v", err)
	}

	cfg3, err := Load(path)
	if err != nil {
		t.Fatalf("Load after SaveTokens: %v", err)
	}
	if cfg3.AccessToken != "oauth_access_example" {
		t.Fatalf("access_token: got %q", cfg3.AccessToken)
	}
	if cfg3.LinearApiKey != "lin_api_existing" {
		// set-token does not clear api_key; api_key still takes precedence in AuthHeader().
		t.Fatalf("api_key: got %q", cfg3.LinearApiKey)
	}
	if got := cfg3.AuthHeader(); got != "lin_api_existing" {
		t.Fatalf("AuthHeader: want api_key value, got %q", got)
	}
}

func TestAuthHeader_oauthAccessTokenOnly_usesBearerScheme(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Path:         "/dev/null",
		BaseURL:      "https://api.linear.app/graphql",
		AccessToken:  "oauth_access_example",
		LinearApiKey: "",
	}
	if got := cfg.AuthHeader(); got != "Bearer oauth_access_example" {
		t.Fatalf("AuthHeader: got %q", got)
	}
}

func TestLoad_authSourceConfigAccessToken(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("access_token = 'oauth_from_file'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AuthSource != "config:access_token" {
		t.Fatalf("AuthSource: got %q", cfg.AuthSource)
	}
	if got := cfg.AuthHeader(); got != "Bearer oauth_from_file" {
		t.Fatalf("AuthHeader: got %q", got)
	}
}

func TestLoad_authSourceConfigApiKey(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("api_key = 'lin_api_from_file'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AuthSource != "config:api_key" {
		t.Fatalf("AuthSource: got %q", cfg.AuthSource)
	}
}
