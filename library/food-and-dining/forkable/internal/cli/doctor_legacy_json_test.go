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
	report := map[string]any{}
	collectCredentialsLocationReport(report, &config.Config{Path: activePath, CredentialSource: "config-kind path"})
	locations, ok := report["credentials_locations"].([]string)
	if !ok || len(locations) != 1 || locations[0] != oldJSONPath {
		t.Fatalf("legacy JSON was not reported: %v", report["credentials_locations"])
	}
	warning, _ := report["credentials_location_warning"].(string)
	if !strings.Contains(warning, oldJSONPath) {
		t.Fatalf("legacy JSON secret warning missing: %q", warning)
	}
}
