// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSetPublication(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	if err := cfg.SetPublication("trevinsays"); err != nil {
		t.Fatalf("SetPublication returned error: %v", err)
	}
	if got := cfg.TemplateVars["publication"]; got != "trevinsays" {
		t.Fatalf("TemplateVars[publication] = %q, want %q", got, "trevinsays")
	}
}

func TestSetPublicationRejectsHostInjection(t *testing.T) {
	t.Parallel()

	cfg := &Config{}
	if err := cfg.SetPublication("trevinsays.evil.test"); err == nil {
		t.Fatal("SetPublication accepted a multi-label host")
	}
	if _, ok := cfg.TemplateVars["publication"]; ok {
		t.Fatal("invalid publication should not be stored")
	}
}

func TestLoadRejectsInvalidPublicationEnv(t *testing.T) {
	t.Setenv("SUBSTACK_PUBLICATION", "trevinsays.evil.test")
	t.Setenv("SUBSTACK_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))

	_, err := Load("")
	if err == nil {
		t.Fatal("Load accepted invalid SUBSTACK_PUBLICATION")
	}
	if !strings.Contains(err.Error(), "SUBSTACK_PUBLICATION") {
		t.Fatalf("error = %q, want SUBSTACK_PUBLICATION context", err)
	}
	if !strings.Contains(err.Error(), "single DNS label") {
		t.Fatalf("error = %q, want actionable single-label guidance", err)
	}
}
