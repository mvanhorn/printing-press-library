// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenTightensExistingDatabaseAndDirectoryModes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "fedex")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "fedex.db")
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := mode(t, dir); got != 0o700 {
		t.Fatalf("database dir mode=%#o, want 0700", got)
	}
	if got := mode(t, path); got != 0o600 {
		t.Fatalf("database mode=%#o, want 0600", got)
	}
}

func TestOpenRejectsSymlinkDatabase(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.db")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "fedex.db")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if st, err := Open(link); err == nil {
		_ = st.Close()
		t.Fatal("Open accepted a symlink database path")
	}
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}
