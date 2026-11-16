package config

import (
	"os"
	"path/filepath"
	"testing"
)

func loadWith(t *testing.T, server, tomlBody string) *Config {
	t.Helper()
	clearCredEnv(t)
	t.Setenv("HOME_ASSISTANT_SERVER", server)
	t.Setenv("HOME_ASSISTANT_BASE_URL", "")
	p := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(p, []byte(tomlBody), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestServerKeepsSchemeAndIgnoresBaseURLDoubling(t *testing.T) {
	cfg := loadWith(t, "http://192.168.1.142:8123/", "base_url = \"http://192.168.1.142:8123\"\n")
	if got := cfg.TemplateVars["server"]; got != "http://192.168.1.142:8123" {
		t.Fatalf("server = %q", got)
	}
}

func TestServerFromConfigBaseURLWhenEnvUnset(t *testing.T) {
	cfg := loadWith(t, "", "base_url = \"https://ha.example:8123\"\n")
	if got := cfg.TemplateVars["server"]; got != "https://ha.example:8123" {
		t.Fatalf("server = %q", got)
	}
}

func TestServerWithoutSchemeGetsHTTP(t *testing.T) {
	cfg := loadWith(t, "192.168.1.142:8123", "")
	if got := cfg.TemplateVars["server"]; got != "http://192.168.1.142:8123" {
		t.Fatalf("server = %q", got)
	}
}
