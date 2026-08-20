// Copyright 2026 Martin Kessler and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigEnvironment(t *testing.T) {
	// Clean environment first
	defer os.Setenv("QBO_ENVIRONMENT", os.Getenv("QBO_ENVIRONMENT"))
	defer os.Setenv("QBO_BASE_URL", os.Getenv("QBO_BASE_URL"))
	defer os.Setenv("QBO_CONFIG", os.Getenv("QBO_CONFIG"))

	tempConfig := filepath.Join(t.TempDir(), "nonexistent.toml")
	os.Setenv("QBO_CONFIG", tempConfig)

	t.Run("default (no env)", func(t *testing.T) {
		os.Unsetenv("QBO_ENVIRONMENT")
		os.Unsetenv("QBO_BASE_URL")
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}
		expected := "https://sandbox-quickbooks.api.intuit.com"
		if cfg.BaseURL != expected {
			t.Errorf("expected BaseURL %q, got %q", expected, cfg.BaseURL)
		}
	})

	t.Run("production env", func(t *testing.T) {
		os.Setenv("QBO_ENVIRONMENT", "production")
		os.Unsetenv("QBO_BASE_URL")
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}
		expected := "https://quickbooks.api.intuit.com"
		if cfg.BaseURL != expected {
			t.Errorf("expected BaseURL %q, got %q", expected, cfg.BaseURL)
		}
	})

	t.Run("explicit QBO_BASE_URL override", func(t *testing.T) {
		os.Setenv("QBO_ENVIRONMENT", "production")
		os.Setenv("QBO_BASE_URL", "https://custom.api.intuit.com")
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}
		expected := "https://custom.api.intuit.com"
		if cfg.BaseURL != expected {
			t.Errorf("expected BaseURL %q, got %q", expected, cfg.BaseURL)
		}
	})

	t.Run("with realm ID", func(t *testing.T) {
		os.Setenv("QBO_ENVIRONMENT", "production")
		os.Setenv("QBO_REALM_ID", "123456789")
		os.Unsetenv("QBO_BASE_URL")
		defer os.Unsetenv("QBO_REALM_ID")
		cfg, err := Load("")
		if err != nil {
			t.Fatalf("failed to load config: %v", err)
		}
		expected := "https://quickbooks.api.intuit.com/v3/company/123456789"
		if cfg.BaseURL != expected {
			t.Errorf("expected BaseURL %q, got %q", expected, cfg.BaseURL)
		}
	})
}
