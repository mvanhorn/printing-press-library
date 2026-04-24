package config

import "testing"

func TestAuthHeaderPrefersScrapeCreatorsAPIKey(t *testing.T) {
	cfg := &Config{
		AuthHeaderVal: "from-config-header",
		APIKey:        "from-env-key",
	}

	if got := cfg.AuthHeader(); got != "from-env-key" {
		t.Fatalf("AuthHeader() = %q, want from-env-key", got)
	}
}

func TestAuthHeaderFallsBackToLegacyConfigHeader(t *testing.T) {
	cfg := &Config{AuthHeaderVal: "from-config-header"}

	if got := cfg.AuthHeader(); got != "from-config-header" {
		t.Fatalf("AuthHeader() = %q, want from-config-header", got)
	}
}
