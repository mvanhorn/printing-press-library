// Copyright 2026 twidtwid. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestOpenPushoverLocalDBLocksDownFilePermissions covers the second
// SQLite open path in this CLI — the local notification-history /
// inbox-sync ledger created by openPushoverLocalDB. Same contract as
// the store's main DB: 0o600 on the file and any sidecars SQLite
// created during migration. A regression that drops the chmod fails
// this test on any default-umask machine.
func TestOpenPushoverLocalDBLocksDownFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes are not meaningful on Windows")
	}
	dbPath := filepath.Join(t.TempDir(), "history.db")
	db, err := openPushoverLocalDB(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("openPushoverLocalDB: %v", err)
	}
	defer db.Close()

	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat db: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Fatalf("db file mode = %o, want 0o600", mode)
	}
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
