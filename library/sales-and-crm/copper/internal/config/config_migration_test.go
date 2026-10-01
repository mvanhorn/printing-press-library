package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMalformedActiveJSONConfigFailsClosed(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	configDir := filepath.Join(home, "config", "copper-pp-cli")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{invalid`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(""); err == nil || !strings.Contains(err.Error(), "parsing") {
		t.Fatalf("malformed active JSON config must fail clearly: %v", err)
	}
}
