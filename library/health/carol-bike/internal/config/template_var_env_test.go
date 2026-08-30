// Copyright 2026 bricenice17 and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"path/filepath"
	"testing"
)

func TestLoadResolvesRiderIDTemplateVariable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CAROL_BIKE_RIDER_ID", "rider-scope-test")

	cfg, err := Load(filepath.Join(home, "missing-config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.TemplateVars["riderId"]; got != "rider-scope-test" {
		t.Fatalf("riderId = %q, want rider-scope-test", got)
	}
}
