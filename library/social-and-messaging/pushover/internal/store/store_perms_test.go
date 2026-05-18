// Copyright 2026 twidtwid. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestOpenWithContextLocksDownDBFilePermissions pins the contract that
// store.OpenWithContext leaves the on-disk SQLite file (and any sidecars
// SQLite chose to create during migration) at 0o600. sql.Open is lazy
// and would otherwise let the file land at default-umask 0o644 next to
// data that may include session secrets — the same class of issue
// already fixed for the HTTP cache. A regression that drops the chmod
// fails this test on any default-umask developer machine and on CI.
func TestOpenWithContextLocksDownDBFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes are not meaningful on Windows")
	}
	dbPath := filepath.Join(t.TempDir(), "store.db")
	s, err := OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("OpenWithContext: %v", err)
	}
	defer s.Close()

	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat db: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("db file mode = %o, want 0o600", mode)
	}
	// WAL/SHM may or may not exist depending on whether the migration
	// triggered a write; whichever do exist must also be 0o600.
	for _, suffix := range []string{"-wal", "-shm"} {
		info, err := os.Stat(dbPath + suffix)
		if err != nil {
			continue
		}
		if mode := info.Mode().Perm(); mode != 0o600 {
			t.Fatalf("%s file mode = %o, want 0o600", dbPath+suffix, mode)
		}
	}
}
