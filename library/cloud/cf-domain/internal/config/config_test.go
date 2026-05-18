package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSaveTokensDoesNotPersistBearerAuthFromEnv(t *testing.T) {
	t.Setenv("CLOUDFLARE_API_TOKEN", "env-token")
	path := filepath.Join(t.TempDir(), "config.toml")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got := cfg.CloudflareRegistrarDomainsBearerAuth; got != "env-token" {
		t.Fatalf("expected env bearer token to be loaded, got %q", got)
	}

	if err := cfg.SaveTokens("client-id", "client-secret", "access-token", "refresh-token", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("SaveTokens() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	if strings.Contains(text, "env-token") {
		t.Fatalf("SaveTokens persisted bearer credential value; config contents:\n%s", text)
	}
	if !strings.Contains(text, "access-token") {
		t.Fatalf("SaveTokens did not persist explicit access token; config contents:\n%s", text)
	}
}

func TestClearTokensRemovesAllPersistedCredentialFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg := &Config{
		Path:                                 path,
		BaseURL:                              "https://api.cloudflare.com/client/v4",
		AuthHeaderVal:                        "Bearer header-token",
		AccessToken:                          "access-token",
		RefreshToken:                         "refresh-token",
		TokenExpiry:                          time.Now().Add(time.Hour),
		ClientID:                             "client-id",
		ClientSecret:                         "client-secret",
		CloudflareRegistrarDomainsBearerAuth: "bearer-token",
	}

	if err := cfg.ClearTokens(); err != nil {
		t.Fatalf("ClearTokens() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	text := string(data)
	for _, secret := range []string{"header-token", "access-token", "refresh-token", "client-id", "client-secret", "bearer-token"} {
		if strings.Contains(text, secret) {
			t.Fatalf("ClearTokens left credential %q in config contents:\n%s", secret, text)
		}
	}
}
