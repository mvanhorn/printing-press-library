package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMalformedActiveJSONConfigFailsClosed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	configDir := filepath.Join(home, "config", "copper-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{invalid`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "parsing") {
		t.Fatalf("malformed active JSON config must fail clearly: %v", err)
	}
}

func TestCoexistingTOMLAndJSONDoesNotMixAccountCredentials(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	configDir := filepath.Join(home, "config", "copper-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	tomlPath := filepath.Join(configDir, "config.toml")
	jsonPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(tomlPath, []byte("base_url = \"https://preferred.example\"\napi_key = \"synthetic-toml-key\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonPath, []byte(`{"base_url":"https://old.example","api_key":"synthetic-json-key","user_email":"synthetic-json-email"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "https://preferred.example" || cfg.CopperApiKey != "synthetic-toml-key" || cfg.CopperUserEmail != "" || cfg.LegacySourcePath() != jsonPath {
		t.Fatal("coexisting files mixed typed credentials from different accounts or lost TOML settings")
	}
	if err := cfg.SaveCredential("synthetic-new-key"); err != nil {
		t.Fatal(err)
	}
	if has, err := FileHasCredentialFields(jsonPath); err != nil || has {
		t.Fatalf("old JSON credential fields remain after save: has=%t err=%v", has, err)
	}
	if has, err := FileHasCredentialFields(tomlPath); err != nil || has {
		t.Fatalf("new TOML contains credential fields: has=%t err=%v", has, err)
	}
	reloaded, err := Load("")
	if err != nil || reloaded.BaseURL != "https://preferred.example" || reloaded.CopperApiKey != "synthetic-new-key" {
		t.Fatalf("migrated settings or credentials did not reload: err=%v", err)
	}
}

func TestCoexistingTOMLSettingsAndJSONCredentialsMigrateTogether(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	configDir := filepath.Join(home, "config", "copper-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	tomlPath := filepath.Join(configDir, "config.toml")
	jsonPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(tomlPath, []byte("base_url = \"https://preferred.example\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonPath, []byte(`{"api_key":"synthetic-json-key","user_email":"synthetic-json-email"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != "https://preferred.example" || cfg.CopperApiKey != "synthetic-json-key" || cfg.CopperUserEmail != "synthetic-json-email" || cfg.LegacySourcePath() != jsonPath {
		t.Fatal("JSON typed credentials did not migrate as one set with TOML settings preserved")
	}
	if err := cfg.SaveCredential("synthetic-new-key"); err != nil {
		t.Fatal(err)
	}
	if has, err := FileHasCredentialFields(jsonPath); err != nil || has {
		t.Fatalf("old JSON credential fields remain after save: has=%t err=%v", has, err)
	}
}

func TestMalformedCoexistingJSONDoesNotBlockActiveTOML(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	configDir := filepath.Join(home, "config", "copper-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("base_url = \"https://preferred.example\"\napi_key = \"synthetic-toml-key\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jsonPath, []byte(`{invalid`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("")
	if err != nil || cfg.BaseURL != "https://preferred.example" || cfg.CopperApiKey != "synthetic-toml-key" || cfg.LegacySourcePath() != jsonPath {
		t.Fatalf("malformed stale JSON blocked active TOML: %v", err)
	}
	if err := cfg.SaveCredential("synthetic-new-key"); err == nil || !strings.Contains(err.Error(), "migration incomplete") {
		t.Fatalf("auth save did not report unverified old JSON scrub: %v", err)
	}
}

func TestAuthSaveReportsIncompleteLegacyScrub(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	configDir := filepath.Join(home, "config", "copper-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(jsonPath, []byte(`{"api_key":"synthetic-old-key"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(jsonPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(jsonPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := cfg.SaveCredential("synthetic-new-key"); err == nil || !strings.Contains(err.Error(), "migration incomplete") {
		t.Fatalf("auth save hid failed legacy scrub: %v", err)
	}
	if _, err := os.Stat(filepath.Join(configDir, "config.toml")); err != nil {
		t.Fatalf("new config was not saved before scrub failure: %v", err)
	}
}
