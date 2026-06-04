package config

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

func TestAuthHeader_BasicAuthFromEmailAndKey(t *testing.T) {
	cfg := &Config{
		GorgiasUsername: "account-email-placeholder",
		GorgiasApiKey:   "secret-123",
	}
	got := cfg.AuthHeader()
	wantCreds := base64.StdEncoding.EncodeToString([]byte("account-email-placeholder:secret-123"))
	want := "Basic " + wantCreds
	if got != want {
		t.Errorf("AuthHeader: want %q, got %q", want, got)
	}
}

func TestAuthHeader_EmptyWhenEitherFieldMissing(t *testing.T) {
	for _, c := range []struct {
		name string
		cfg  Config
	}{
		{"both empty", Config{}},
		{"username only", Config{GorgiasUsername: "account-email-placeholder"}},
		{"key only", Config{GorgiasApiKey: "secret-123"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.cfg.AuthHeader(); got != "" {
				t.Errorf("partial credential: want empty, got %q", got)
			}
		})
	}
}

func TestAuthHeader_PreferStoredAuthHeader(t *testing.T) {
	cfg := &Config{
		AuthHeaderVal:   "Bearer pre-baked",
		GorgiasUsername: "ignored-username-placeholder",
		GorgiasApiKey:   "ignored",
	}
	if got := cfg.AuthHeader(); got != "Bearer pre-baked" {
		t.Errorf("stored auth header must shadow username/key, got %q", got)
	}
}

func TestSaveCredentials_RoundTripsToFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := &Config{Path: path}
	if err := cfg.SaveCredentials("account-email-placeholder", "k-1"); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.GorgiasUsername != "account-email-placeholder" {
		t.Errorf("username round-trip: want account-email-placeholder, got %q", loaded.GorgiasUsername)
	}
	if loaded.GorgiasApiKey != "k-1" {
		t.Errorf("api_key round-trip: want k-1, got %q", loaded.GorgiasApiKey)
	}
}

func TestSaveCredentials_ClearsLegacyAuthHeader(t *testing.T) {
	cfg := &Config{
		Path:          filepath.Join(t.TempDir(), "config.toml"),
		AuthHeaderVal: "Bearer leftover",
	}
	if err := cfg.SaveCredentials("account-email-placeholder", "k-1"); err != nil {
		t.Fatal(err)
	}
	if cfg.AuthHeaderVal != "" {
		t.Errorf("AuthHeaderVal must be cleared to avoid shadowing fresh credentials; got %q", cfg.AuthHeaderVal)
	}
}

func TestLoad_EnvVarsOverrideFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := &Config{Path: path, GorgiasUsername: "file-username-placeholder", GorgiasApiKey: "file-key"}
	if err := cfg.save(); err != nil {
		t.Fatal(err)
	}

	// Set env vars; they should win over file values.
	t.Setenv("GORGIAS_USERNAME", "env-username-placeholder")
	t.Setenv("GORGIAS_API_KEY", "env-key")

	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.GorgiasUsername != "env-username-placeholder" {
		t.Errorf("env-var username should win: got %q", loaded.GorgiasUsername)
	}
	if loaded.GorgiasApiKey != "env-key" {
		t.Errorf("env-var api_key should win: got %q", loaded.GorgiasApiKey)
	}
	if loaded.AuthSource != "env:GORGIAS_API_KEY" {
		t.Errorf("AuthSource should reflect env origin, got %q", loaded.AuthSource)
	}
}

func TestLoad_AuthSourceConfigWhenFileBacked(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	cfg := &Config{Path: path, GorgiasUsername: "file-username-placeholder", GorgiasApiKey: "file-key"}
	if err := cfg.save(); err != nil {
		t.Fatal(err)
	}
	// Ensure no env vars set
	os.Unsetenv("GORGIAS_USERNAME")
	os.Unsetenv("GORGIAS_API_KEY")
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AuthSource != "config" {
		t.Errorf("AuthSource for file-backed creds: want %q, got %q", "config", loaded.AuthSource)
	}
}

func TestClearTokens(t *testing.T) {
	cfg := &Config{
		Path:            filepath.Join(t.TempDir(), "config.toml"),
		GorgiasUsername: "account-email-placeholder",
		GorgiasApiKey:   "k-1",
		AuthHeaderVal:   "Bearer x",
	}
	if err := cfg.ClearTokens(); err != nil {
		t.Fatal(err)
	}
	if cfg.GorgiasUsername != "" || cfg.GorgiasApiKey != "" || cfg.AuthHeaderVal != "" {
		t.Errorf("ClearTokens must zero all credential fields, got %+v", cfg)
	}
}
