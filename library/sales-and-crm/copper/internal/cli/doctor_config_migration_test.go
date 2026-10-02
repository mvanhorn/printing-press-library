package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/copper/internal/config"
)

func TestDoctorWarnsWhenLegacyJSONStillHoldsCredentials(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	configDir := filepath.Join(home, "config", "copper-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	jsonPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(jsonPath, []byte(`{"api_key":"synthetic-key"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	report := map[string]any{}
	collectCredentialsLocationReport(report, cfg)
	warning, _ := report["credentials_location_warning"].(string)
	if !strings.Contains(warning, "legacy secrets remain") || !strings.Contains(warning, jsonPath) {
		t.Fatalf("doctor did not identify unsanitized legacy JSON: %q", warning)
	}
	if err := os.Remove(jsonPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(jsonPath, 0o700); err != nil {
		t.Fatal(err)
	}
	report = map[string]any{}
	collectCredentialsLocationReport(report, cfg)
	warning, _ = report["credentials_location_warning"].(string)
	if !strings.Contains(warning, "cannot verify credential scrub") || !strings.Contains(warning, jsonPath) {
		t.Fatalf("doctor hid an unreadable legacy JSON path: %q", warning)
	}
}

func TestDoctorDoesNotProbeDefaultJSONForExplicitProfile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	defaultDir := filepath.Join(home, "config", "copper-pp-cli")
	if err := os.MkdirAll(defaultDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(defaultDir, "config.json"), []byte(`{"api_key":"synthetic-other-profile"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	explicitPath := filepath.Join(home, "separate-profile.toml")
	if err := os.WriteFile(explicitPath, []byte("base_url = \"https://preferred.example\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(explicitPath)
	if err != nil {
		t.Fatal(err)
	}
	report := map[string]any{}
	collectCredentialsLocationReport(report, cfg)
	if warning, ok := report["credentials_location_warning"]; ok {
		t.Fatalf("doctor inspected unrelated default profile: %v", warning)
	}
}
