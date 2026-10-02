package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSaveTokensPreservesConfigSymlinkAndPrivateTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target", "config.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}
	initial, err := Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if err := initial.SaveTokens("client", "", "old", "refresh", time.Now()); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias.toml")
	if err := os.Symlink(target, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	cfg, err := Load(alias)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Path != alias {
		t.Fatalf("config path = %q, want selected alias %q", cfg.Path, alias)
	}
	if err := cfg.SaveTokens("client", "", "rotated", "refresh-2", time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(alias)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("config alias was replaced: %v", err)
	}
	info, err = os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("target permissions = %v", info.Mode().Perm())
	}
	reloaded, err := Load(target)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.AccessToken != "rotated" || reloaded.RefreshToken != "refresh-2" {
		t.Fatal("target retained stale OAuth credentials")
	}
}

func TestWritePrivateFileRejectsDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	alias := filepath.Join(dir, "alias.toml")
	if err := os.Symlink(filepath.Join(dir, "missing.toml"), alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := WritePrivateFile(alias, []byte("secret")); err == nil {
		t.Fatal("dangling symlink was replaced")
	}
	info, err := os.Lstat(alias)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("dangling symlink was replaced: %v", err)
	}
}
