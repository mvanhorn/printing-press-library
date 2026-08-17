// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func clearConfigEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"DOMINOS_CONFIG",
		"DOMINOS_USERNAME",
		"DOMINOS_PASSWORD",
		"DOMINOS_TOKEN",
		"DOMINOS_MARKET",
		"DOMINOS_BASE_URL",
	} {
		t.Setenv(key, "")
	}
}

func TestLoadMarketDefaults(t *testing.T) {
	clearConfigEnv(t)

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Market != MarketUS || cfg.BaseURL != USBaseURL {
		t.Fatalf("got market=%q base_url=%q", cfg.Market, cfg.BaseURL)
	}
}

func TestLoadCanadianMarketFromConfig(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("market = 'canada'\nbase_url = 'https://www.dominos.com/api'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Market != MarketCanada || cfg.BaseURL != CanadaBaseURL {
		t.Fatalf("got market=%q base_url=%q", cfg.Market, cfg.BaseURL)
	}
	wantHeaders := map[string]string{
		"DPZ-Market":   "CANADA",
		"Market":       "CANADA",
		"DPZ-Language": "en",
	}
	if !reflect.DeepEqual(cfg.DefaultHeaders(), wantHeaders) {
		t.Fatalf("got headers %#v, want %#v", cfg.DefaultHeaders(), wantHeaders)
	}
}

func TestLoadInfersCanadaFromLegacyBaseURL(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("base_url = 'https://order.dominos.ca'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Market != MarketCanada || cfg.BaseURL != CanadaBaseURL {
		t.Fatalf("got market=%q base_url=%q", cfg.Market, cfg.BaseURL)
	}
}

func TestLoadMarketEnvironmentOverridesConfig(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("market = 'us'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DOMINOS_MARKET", "ca")

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Market != MarketCanada || cfg.BaseURL != CanadaBaseURL {
		t.Fatalf("got market=%q base_url=%q", cfg.Market, cfg.BaseURL)
	}
}

func TestLoadExplicitBaseURLOverridesMarketDefault(t *testing.T) {
	clearConfigEnv(t)
	t.Setenv("DOMINOS_MARKET", "ca")
	t.Setenv("DOMINOS_BASE_URL", "https://example.test")

	cfg, err := Load(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Market != MarketCanada || cfg.BaseURL != "https://example.test" {
		t.Fatalf("got market=%q base_url=%q", cfg.Market, cfg.BaseURL)
	}
}

func TestNormalizeMarketRejectsUnknownValue(t *testing.T) {
	_, err := NormalizeMarket("uk")
	if err == nil || err.Error() != `unsupported market "uk" (use us or ca)` {
		t.Fatalf("got error %v", err)
	}
}

func TestStoredCredentialsStayBoundToTheirMarket(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("market = 'us'\naccess_token = 'stored-token'\ncustomer_id = 'us-customer'\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.CredentialMarket != MarketUS {
		t.Fatalf("credential market = %q, want us", cfg.CredentialMarket)
	}
	if err := cfg.SetMarket(MarketCanada); err != nil {
		t.Fatal(err)
	}
	if !cfg.CredentialMarketMismatch() {
		t.Fatal("market override did not detect stored credential mismatch")
	}
	if got := cfg.AuthHeader(); got != "" {
		t.Fatalf("mismatched credential produced auth header %q", got)
	}
}

func TestLegacyStoredTokenStaysBoundToItsMarket(t *testing.T) {
	clearConfigEnv(t)
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("market = 'us'\ntoken = 'legacy-stored-token'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.SetMarket(MarketCanada); err != nil {
		t.Fatal(err)
	}
	if !cfg.CredentialMarketMismatch() || cfg.AuthHeader() != "" {
		t.Fatalf("legacy token crossed markets: mismatch=%v header=%q", cfg.CredentialMarketMismatch(), cfg.AuthHeader())
	}
}

func TestSavingTokenForNewMarketClearsStaleMarketFields(t *testing.T) {
	clearConfigEnv(t)
	cfg := &Config{
		Path:              filepath.Join(t.TempDir(), "config.toml"),
		Market:            MarketCanada,
		CredentialMarket:  MarketUS,
		AuthHeaderVal:     "Bearer stale-header",
		DominosToken:      "stale-token",
		DominosCustomerID: "stale-us-customer",
	}
	if err := cfg.SaveTokens("", "", "new-canadian-token", "", time.Time{}); err != nil {
		t.Fatal(err)
	}
	if cfg.CredentialMarket != MarketCanada || cfg.DominosCustomerID != "" || cfg.AuthHeaderVal != "" || cfg.DominosToken != "" {
		t.Fatalf("stale fields survived: %#v", cfg)
	}
	if cfg.AuthHeader() != "Bearer new-canadian-token" {
		t.Fatalf("new token is not active: %q", cfg.AuthHeader())
	}
}

func TestHarvestedCredentialsReplaceCustomerAcrossMarkets(t *testing.T) {
	clearConfigEnv(t)
	cfg := &Config{
		Path:              filepath.Join(t.TempDir(), "config.toml"),
		Market:            MarketCanada,
		CredentialMarket:  MarketUS,
		DominosCustomerID: "stale-us-customer",
	}
	if err := cfg.SaveHarvestedCredentials("new-canadian-token", "canadian-customer", "canadian@example.test"); err != nil {
		t.Fatal(err)
	}
	if cfg.CredentialMarket != MarketCanada || cfg.DominosCustomerID != "canadian-customer" || cfg.DominosUsername != "canadian@example.test" {
		t.Fatalf("harvested credentials were not saved atomically: %#v", cfg)
	}
}

func TestManualTokenClearsUnverifiedCustomerProfile(t *testing.T) {
	clearConfigEnv(t)
	cfg := &Config{
		Path:              filepath.Join(t.TempDir(), "config.toml"),
		Market:            MarketCanada,
		CredentialMarket:  MarketCanada,
		DominosCustomerID: "old-customer",
	}
	if err := cfg.SaveManualToken("manual-token"); err != nil {
		t.Fatal(err)
	}
	if cfg.DominosCustomerID != "" || cfg.CredentialMarket != MarketCanada || cfg.AuthHeader() != "Bearer manual-token" {
		t.Fatalf("manual token retained stale profile: %#v", cfg)
	}
}
