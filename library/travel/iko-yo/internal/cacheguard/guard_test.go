// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cacheguard

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func testFile(t *testing.T, path, data string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestReadFindsCanonicalSidecarsThroughAliases(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real.db")
	testFile(t, real, "main")
	alias := filepath.Join(root, "alias.db")
	if err := os.Symlink(real, alias); err != nil {
		t.Skip(err)
	}
	for _, sidecar := range []string{"-wal", "-shm", "-journal"} {
		testFile(t, real+sidecar, "active")
		if _, err := Read(alias); err == nil {
			t.Fatalf("alias hid canonical %s", sidecar)
		}
		if err := os.Remove(real + sidecar); err != nil {
			t.Fatal(err)
		}
	}
	directory := filepath.Join(root, "directory-alias")
	if err := os.Symlink(root, directory); err != nil {
		t.Fatal(err)
	}
	testFile(t, real+"-wal", "active")
	if _, err := Read(filepath.Join(directory, "real.db")); err == nil {
		t.Fatal("directory alias hid active canonical WAL")
	}
}

func TestSnapshotRejectsChangesAcrossOperation(t *testing.T) {
	for _, change := range []string{"wal", "modify", "replace", "hard-link"} {
		t.Run(change, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "cache.db")
			testFile(t, file, "main")
			guard, err := Read(file)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "wal":
				testFile(t, file+"-wal", "new committed data")
			case "modify":
				testFile(t, file, "changed")
				future := time.Now().Add(time.Second)
				if err := os.Chtimes(file, future, future); err != nil {
					t.Fatal(err)
				}
			case "replace":
				if err := os.Rename(file, file+".old"); err != nil {
					t.Fatal(err)
				}
				testFile(t, file, "main")
			case "hard-link":
				if err := os.Link(file, file+".alias"); err != nil {
					t.Skip(err)
				}
			}
			if err := guard.Snapshot(); err == nil {
				t.Fatal("changed cache was accepted after the operation")
			}
		})
	}
}

func TestWriteRejectsAmbiguousLinksAndReservedURIBytes(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "main.db")
	testFile(t, file, "sentinel")
	alias := filepath.Join(root, "alias.db")
	if err := os.Link(file, alias); err != nil {
		t.Skip(err)
	}
	for _, path := range []string{file, alias} {
		if _, err := Write(path); err == nil {
			t.Fatal("multiply-linked database accepted for writing")
		}
	}
	for _, token := range []string{"?", "#", "%"} {
		if _, err := Write(filepath.Join(root, "new"+token+"cache", "data.db")); err == nil {
			t.Fatalf("unsafe URI byte %s accepted", token)
		}
	}
	data, err := os.ReadFile(file)
	if err != nil || string(data) != "sentinel" {
		t.Fatal("guard changed the existing database")
	}
}

func TestClonePrivateAndUnsafeTempCleanup(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "cache.db")
	testFile(t, file, "verified inode data")
	guard, err := Read(file)
	if err != nil {
		t.Fatal(err)
	}
	path, err := guard.Clone(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("snapshot not private: %v", err)
	}
	dir, err := os.Stat(filepath.Dir(path))
	if err != nil || runtime.GOOS != "windows" && dir.Mode().Perm() != 0o700 {
		t.Fatalf("snapshot directory not private: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "verified inode data" {
		t.Fatal("copy did not retain pinned data")
	}
	if err := guard.Cleanup(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("private snapshot was retained")
	}
	for _, token := range []string{"?", "#", "%"} {
		t.Run("TMPDIR_"+token, func(t *testing.T) {
			bad := filepath.Join(root, "temp"+token+"dir")
			if err := os.Mkdir(bad, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("TMPDIR", bad)
			fresh, err := Read(file)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = fresh.Clone(context.Background()); err == nil || !strings.Contains(err.Error(), "URI") {
				t.Fatalf("unsafe snapshot temp path accepted: %v", err)
			}
			entries, err := os.ReadDir(bad)
			if err != nil || len(entries) != 0 {
				t.Fatalf("failed copy retained private files: %v", err)
			}
		})
	}
}
