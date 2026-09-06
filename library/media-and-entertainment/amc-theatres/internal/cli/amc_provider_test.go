// Copyright 2026 Avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/amc-theatres/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/amc-theatres/internal/config"
)

func TestConfigureAMCClientSandboxAndAuthToken(t *testing.T) {
	t.Setenv("AMC_THEATRES_ENV", "sandbox")
	t.Setenv("AMC_THEATRES_AUTH_TOKEN", "viewer-token")
	c := client.New(&config.Config{BaseURL: amcProductionURL}, 0, 0)
	if err := configureAMCClient(c); err != nil {
		t.Fatalf("configureAMCClient() error = %v", err)
	}
	if c.BaseURL != amcSandboxURL {
		t.Fatalf("BaseURL = %q, want %q", c.BaseURL, amcSandboxURL)
	}
	if got := c.Config.Headers["X-AMC-Auth-Token"]; got != "viewer-token" {
		t.Fatalf("X-AMC-Auth-Token = %q", got)
	}
}

func TestConfigureAMCClientPreservesExplicitBaseURL(t *testing.T) {
	t.Setenv("AMC_THEATRES_ENV", "sandbox")
	c := client.New(&config.Config{BaseURL: "http://127.0.0.1:1234"}, 0, 0)
	if err := configureAMCClient(c); err != nil {
		t.Fatalf("configureAMCClient() error = %v", err)
	}
	if c.BaseURL != "http://127.0.0.1:1234" {
		t.Fatalf("explicit BaseURL changed to %q", c.BaseURL)
	}
}

func TestConfigureAMCClientRejectsUnknownEnvironment(t *testing.T) {
	t.Setenv("AMC_THEATRES_ENV", "staging")
	c := client.New(&config.Config{BaseURL: amcProductionURL}, 0, 0)
	if err := configureAMCClient(c); err == nil {
		t.Fatal("configureAMCClient() error = nil, want invalid environment")
	}
}
