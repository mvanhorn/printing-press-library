// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/forkable/internal/config"
)

func TestDoctorFindsCoexistingLegacyJSONCredentials(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FORKABLE_HOME", "")
	t.Setenv("FORKABLE_DATA_DIR", "")
	t.Setenv("XDG_DATA_HOME", "")
	configDir := filepath.Join(home, ".config", "forkable-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	activePath := filepath.Join(configDir, "config.toml")
	oldJSONPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(activePath, []byte("base_url = \"https://current.example\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldJSONPath, []byte(`{"access_token":"synthetic-secret"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	report := map[string]any{}
	collectCredentialsLocationReport(report, cfg)
	locations, ok := report["credentials_locations"].([]string)
	if !ok || len(locations) != 1 || locations[0] != oldJSONPath {
		t.Fatalf("legacy JSON was not reported: %v", report["credentials_locations"])
	}
	warning, _ := report["credentials_location_warning"].(string)
	if !strings.Contains(warning, oldJSONPath) {
		t.Fatalf("legacy JSON secret warning missing: %q", warning)
	}
}

func TestDoctorDoesNotClaimUnrelatedSiblingJSONForExplicitConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FORKABLE_HOME", "")
	t.Setenv("FORKABLE_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	sharedDir := filepath.Join(home, "shared")
	if err := os.MkdirAll(sharedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	activePath := filepath.Join(sharedDir, "config.toml")
	if err := os.WriteFile(activePath, []byte("base_url = \"https://current.example\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	unrelatedPath := filepath.Join(sharedDir, "config.json")
	if err := os.WriteFile(unrelatedPath, []byte(`{"access_token":"synthetic-other-app-value"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(activePath)
	if err != nil {
		t.Fatal(err)
	}
	report := map[string]any{}
	collectCredentialsLocationReport(report, cfg)
	if locations, ok := report["credentials_locations"].([]string); ok {
		for _, location := range locations {
			if location == unrelatedPath {
				t.Fatalf("doctor claimed unrelated config: %v", locations)
			}
		}
	}
	if warning, ok := report["credentials_location_warning"].(string); ok && strings.Contains(warning, unrelatedPath) {
		t.Fatalf("doctor suggested scrubbing unrelated config: %q", warning)
	}
}

func TestDoctorWarnsWhenLegacyJSONCannotBeInspected(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("FORKABLE_HOME", "")
	t.Setenv("FORKABLE_CONFIG", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	configDir := filepath.Join(home, ".config", "forkable-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("base_url = \"https://current.example\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	malformedPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(malformedPath, []byte(`{"access_token":`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	report := map[string]any{}
	collectCredentialsLocationReport(report, cfg)
	paths, ok := report["credentials_uninspected_locations"].([]string)
	if !ok || len(paths) != 1 || paths[0] != malformedPath {
		t.Fatalf("doctor did not identify unreadable legacy file: %v", report)
	}
	warning, _ := report["credentials_location_warning"].(string)
	if !strings.Contains(warning, malformedPath) || !strings.Contains(warning, "WARN") {
		t.Fatalf("doctor did not warn about unreadable legacy file: %q", warning)
	}
}
