// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build linux

package secureio

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

// walkDirNoFollow is the fallback used when openat2 is unavailable; it must
// open real directories and refuse any symlinked component.
func TestWalkDirNoFollowFallback(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real", "nested")
	if err := os.MkdirAll(real, 0o700); err != nil {
		t.Fatal(err)
	}
	fd, err := walkDirNoFollow(real, unix.O_RDONLY)
	if err != nil {
		t.Fatalf("walk real dir: %v", err)
	}
	unix.Close(fd)

	if err := os.Symlink(filepath.Join(base, "real"), filepath.Join(base, "link")); err != nil {
		t.Fatal(err)
	}
	if fd, err := walkDirNoFollow(filepath.Join(base, "link", "nested"), unix.O_PATH); err == nil {
		unix.Close(fd)
		t.Fatal("walk followed a symlinked component")
	}
	if fd, err := walkDirNoFollow(filepath.Join(base, "link"), unix.O_RDONLY); err == nil {
		unix.Close(fd)
		t.Fatal("walk opened a symlinked final component")
	}
}
