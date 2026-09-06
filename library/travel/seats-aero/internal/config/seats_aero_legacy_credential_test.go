package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSeatsAeroLegacyCredentialFallback(t *testing.T) {
	clearCredEnv(t)
	t.Setenv("SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION", "")
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("aero_partner_partner_authorization = \"legacy-key\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SeatsAeroApiKey != "legacy-key" || cfg.AuthSource != "config:aero_partner_partner_authorization (legacy)" {
		t.Fatalf("key=%q auth_source=%q", cfg.SeatsAeroApiKey, cfg.AuthSource)
	}
	if cfg.CredentialSource != "legacy config path (aero_partner_partner_authorization)" {
		t.Fatalf("credential_source=%q want stable legacy key label", cfg.CredentialSource)
	}
}

func TestSeatsAeroLegacyCredentialEnvFallback(t *testing.T) {
	clearCredEnv(t)
	t.Setenv("SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION", "legacy-env-key")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SeatsAeroApiKey != "legacy-env-key" || cfg.AuthSource != "env:SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION" {
		t.Fatalf("key=%q auth_source=%q", cfg.SeatsAeroApiKey, cfg.AuthSource)
	}
}

func TestSeatsAeroCanonicalCredentialEnvWins(t *testing.T) {
	clearCredEnv(t)
	t.Setenv("SEATS_AERO_API_KEY", "canonical-env-key")
	t.Setenv("SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION", "legacy-env-key")
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("aero_api_key = \"file-key\"\naero_partner_partner_authorization = \"legacy-file-key\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SeatsAeroApiKey != "canonical-env-key" || cfg.AuthSource != "env:SEATS_AERO_API_KEY" {
		t.Fatalf("key=%q auth_source=%q", cfg.SeatsAeroApiKey, cfg.AuthSource)
	}
}
