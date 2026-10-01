package config

import (
	"errors"
	"testing"
)

func TestLoadEnvironmentTokenWithoutResolvableHome(t *testing.T) {
	t.Setenv("DREAMING_CONFIG", "")
	t.Setenv("DREAMING_TOKEN", "synthetic-test-token")
	noHome := func() (string, error) { return "", errors.New("synthetic home lookup failure") }
	cfg, err := load("", noHome, noHome)
	if err != nil {
		t.Fatalf("environment token should work without a home directory: %v", err)
	}
	if cfg.Path != "" {
		t.Fatalf("config path = %q, want no path when home lookup fails", cfg.Path)
	}
	if cfg.AuthHeader() != "Bearer synthetic-test-token" {
		t.Fatal("environment token was not applied")
	}
}
