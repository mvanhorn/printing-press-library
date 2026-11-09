package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadKeepsReadingLegacyDotConfigPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("KLAVIYO_CONFIG", "")
	t.Setenv("KLAVIYO_API_KEY", "")
	path := filepath.Join(home, ".config", "klaviyo-pp-cli", "config.toml")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("api_key = 'legacy-key'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Path != path || cfg.KlaviyoApiKey != "legacy-key" {
		t.Fatalf("loaded path=%q api_key=%q, want legacy dot-config file", cfg.Path, cfg.KlaviyoApiKey)
	}
}

func TestRootDirHonorsXDGConfigHome(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", root)
	got, err := RootDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != root {
		t.Fatalf("RootDir() = %q, want %q", got, root)
	}
}
