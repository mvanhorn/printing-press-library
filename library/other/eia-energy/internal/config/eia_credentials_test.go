// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
package config

import "testing"

func TestExplicitCredentialUpdateWinsOverLoadedEnvOverride(t *testing.T) {
	cfg := &Config{
		EiaApiKey:    "from-env",
		envOverrides: map[string]bool{"EiaApiKey": true},
		fileConfig:   &Config{EiaApiKey: "old-on-disk"},
	}

	// This is the persistence-critical portion of SaveCredential: explicitly
	// setting a token clears the override before updating the file snapshot.
	cfg.EiaApiKey = "new-token"
	delete(cfg.envOverrides, "EiaApiKey")
	cfg.updateFileConfigField("EiaApiKey")
	got := cfg.configForSave().EiaApiKey
	if got != "new-token" {
		t.Fatalf("persisted API key = %q, want explicit new token", got)
	}
}
