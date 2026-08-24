// Copyright 2026 smoochy and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

const sampleDotenv = `# shared printing-press credentials
export SYNOLOGY_ACCOUNT = nas-user
SYNOLOGY_PASSWORD="secret-value"

SYNOLOGY_BASE_URL='https://nas.example.lan:5001'
SYNOLOGY_INSECURE_TLS=1
NOT_A_PAIR
`

func writeSampleDotenv(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(sampleDotenv), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(dotenvPathEnv, path)
	return path
}

func TestApplyDotenv_FillsPlaceholderFields(t *testing.T) {
	writeSampleDotenv(t)

	cfg := &Config{BaseURL: defaultBaseURL}
	applyDotenv(cfg)

	if cfg.BaseURL != "https://nas.example.lan:5001" {
		t.Errorf("base url = %q", cfg.BaseURL)
	}
	if !cfg.InsecureTLS {
		t.Error("insecure tls not set")
	}
}

// A base URL that came from the config file must survive.
func TestApplyDotenv_DoesNotOverwriteConfiguredBaseURL(t *testing.T) {
	writeSampleDotenv(t)

	cfg := &Config{BaseURL: "https://configured.example.lan:5001"}
	applyDotenv(cfg)

	if cfg.BaseURL != "https://configured.example.lan:5001" {
		t.Errorf("base url overwritten: %q", cfg.BaseURL)
	}
}

// Credentials must never land in Config, where save() could persist them.
func TestApplyDotenv_LeavesCredentialsOutOfConfig(t *testing.T) {
	writeSampleDotenv(t)

	cfg := &Config{BaseURL: defaultBaseURL}
	applyDotenv(cfg)

	if cfg.AuthHeaderVal != "" || cfg.AccessToken != "" || cfg.ClientSecret != "" {
		t.Errorf("credential fields populated: %+v", cfg)
	}
}

func TestDotenvLookup(t *testing.T) {
	writeSampleDotenv(t)

	if got := DotenvLookup("SYNOLOGY_PASSWORD"); got != "secret-value" {
		t.Errorf("password = %q", got)
	}
	if got := DotenvLookup("SYNOLOGY_ACCOUNT"); got != "nas-user" {
		t.Errorf("account = %q", got)
	}
	if got := DotenvLookup("SYNOLOGY_OTP_CODE"); got != "" {
		t.Errorf("absent key = %q", got)
	}
}

func TestDotenv_MissingFileIsNoOp(t *testing.T) {
	t.Setenv(dotenvPathEnv, filepath.Join(t.TempDir(), "absent.env"))

	cfg := &Config{BaseURL: defaultBaseURL}
	applyDotenv(cfg)

	if cfg.BaseURL != defaultBaseURL || cfg.InsecureTLS {
		t.Errorf("unexpected mutation: %+v", cfg)
	}
	if got := DotenvLookup("SYNOLOGY_PASSWORD"); got != "" {
		t.Errorf("lookup on missing file = %q", got)
	}
}
