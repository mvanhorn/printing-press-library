// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import "testing"

func TestDemoKeyIsNotAConfiguredCredential(t *testing.T) {
	cfg := &Config{}
	if got := cfg.AuthHeader(); got != "DEMO_KEY" {
		t.Fatalf("AuthHeader() = %q, want DEMO_KEY fallback", got)
	}
	if cfg.HasConfiguredAPIKey() {
		t.Fatal("public DEMO_KEY fallback must not report a configured credential")
	}

	cfg.FdcApiKey = "personal-key"
	if !cfg.HasConfiguredAPIKey() {
		t.Fatal("personal FDC API key should report configured")
	}
}
