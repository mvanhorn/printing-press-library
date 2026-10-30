package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveCookiesTightensExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("base_url = \"https://www.redfin.com\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Path: path}
	if err := cfg.SaveCookies("_abck=a1"); err != nil {
		t.Fatalf("SaveCookies: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("mode = %o, want 600", mode)
	}
}
