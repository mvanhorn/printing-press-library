// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package secureio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicTightensFileButLeavesCallerDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(path, []byte("new")); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	// The parent may be a caller-chosen output directory; only the file is
	// tightened. CLI-owned directories go through EnsurePrivateDir.
	if got := mode(t, dir); got != 0o755 {
		t.Fatalf("caller directory mode changed to %#o, want 0755", got)
	}
	if got := mode(t, path); got != PrivateFileMode {
		t.Fatalf("file mode=%#o, want %#o", got, PrivateFileMode)
	}

	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("target"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(link, []byte("replacement")); err == nil {
		t.Fatal("WriteFileAtomic accepted a symlink target")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "target" {
		t.Fatalf("symlink target was modified: %q", data)
	}
}

func TestEnsurePrivateDirTightensExistingDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := EnsurePrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	if got := mode(t, dir); got != PrivateDirMode {
		t.Fatalf("dir mode=%#o, want %#o", got, PrivateDirMode)
	}
}

func TestWriteFileAtomicCreatesMissingDirectoriesPrivate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "new", "labels")
	if err := WriteFileAtomic(filepath.Join(dir, "a.pdf"), []byte("x")); err != nil {
		t.Fatal(err)
	}
	if got := mode(t, dir); got != PrivateDirMode {
		t.Fatalf("new dir mode=%#o, want %#o", got, PrivateDirMode)
	}
}

func TestEnsurePrivateDirRejectsSymlink(t *testing.T) {
	realDir := filepath.Join(t.TempDir(), "real")
	if err := os.Mkdir(realDir, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(realDir), "link")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatal(err)
	}
	if err := EnsurePrivateDir(link); err == nil {
		t.Fatal("EnsurePrivateDir accepted a symlink")
	}
}

func mode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}
