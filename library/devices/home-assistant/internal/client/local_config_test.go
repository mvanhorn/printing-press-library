package client

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalConfigRejectsTraversalAndSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HASS_CONFIG_DIR", root)
	if _, err := LocalConfigPath("../secret", false); err == nil {
		t.Fatal("traversal must be rejected")
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "www")); err != nil {
		t.Fatal(err)
	}
	if _, err := LocalConfigPath("www/file.txt", true); err == nil {
		t.Fatal("symlink escape must be rejected")
	}
}

func TestLocalConfigWriteIsReadableAndSecretsMasked(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HASS_CONFIG_DIR", root)
	if err := os.Mkdir(filepath.Join(root, "themes"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := LocalConfigWrite("themes/night.yaml", []byte("primary: blue\n")); err != nil {
		t.Fatal(err)
	}
	raw, err := LocalConfigRead("themes/night.yaml")
	if err != nil || string(raw) != "primary: blue\n" {
		t.Fatalf("read after atomic write = %q, %v", raw, err)
	}
	if _, err := LocalConfigRead("secrets.yaml"); err == nil {
		t.Fatal("secrets must never be readable")
	}
}
