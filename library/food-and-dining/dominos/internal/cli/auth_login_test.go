// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/dominos/internal/config"
)

func TestSignInURLForMarket(t *testing.T) {
	if got := signInURLForMarket(config.MarketUS); got != usSignInURL {
		t.Fatalf("US sign-in URL = %q", got)
	}
	if got := signInURLForMarket(config.MarketCanada); got != canadaSignInURL {
		t.Fatalf("Canada sign-in URL = %q", got)
	}
}

func TestAuthCommandRegistersLoginOnce(t *testing.T) {
	cmd := newAuthCmd(&rootFlags{})
	loginCount := 0
	for _, child := range cmd.Commands() {
		if child.Name() == "login" {
			loginCount++
		}
	}
	if loginCount != 1 {
		t.Fatalf("registered %d login commands, want 1", loginCount)
	}
}

func TestSaveHarvestedAuthPersistsSelectedMarket(t *testing.T) {
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

	configPath := filepath.Join(t.TempDir(), "config.toml")
	flags := &rootFlags{configPath: configPath, market: config.MarketCanada, asJSON: true}
	env := harvestEnvelope{
		Token:      "test-token-with-more-than-twenty-characters",
		CustomerID: "test-customer-id",
		Email:      "customer@example.test",
	}

	var output bytes.Buffer
	if err := saveHarvestedAuth(&output, flags, env, "test"); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Market != config.MarketCanada || cfg.BaseURL != config.CanadaBaseURL {
		t.Fatalf("got market=%q base_url=%q", cfg.Market, cfg.BaseURL)
	}
	if cfg.DominosCustomerID != env.CustomerID || cfg.AccessToken != env.Token {
		t.Fatal("saved auth fields do not match harvested fields")
	}
	if !bytes.Contains(output.Bytes(), []byte(`"market": "ca"`)) {
		t.Fatalf("JSON output did not include market: %s", output.String())
	}
}

func TestMarketFlagOverridesEnvironment(t *testing.T) {
	t.Setenv("DOMINOS_MARKET", "us")
	t.Setenv("DOMINOS_BASE_URL", "")
	flags := &rootFlags{
		configPath: filepath.Join(t.TempDir(), "missing.toml"),
		market:     config.MarketCanada,
	}

	cfg, err := flags.loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Market != config.MarketCanada || cfg.BaseURL != config.CanadaBaseURL {
		t.Fatalf("got market=%q base_url=%q", cfg.Market, cfg.BaseURL)
	}
}
