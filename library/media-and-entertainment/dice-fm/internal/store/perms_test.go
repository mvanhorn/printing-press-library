// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Test for store-at-rest perms (Task 16): the data dir is 0700 and the db file
// is 0600 (the store holds fan PII at rest).
package store

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestStorePermsLeastPrivilege(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX perms not applicable on Windows")
	}
	dir := filepath.Join(t.TempDir(), "share", "dice-fm-pp-cli")
	dbPath := filepath.Join(dir, "data.db")

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	// Force file creation via a write.
	if err := s.Upsert("events", "e1", []byte(`{"id":"e1","name":"Show"}`)); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	dinfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dinfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("store dir perm = %o, want 0700", perm)
	}
	finfo, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("stat db: %v", err)
	}
	if perm := finfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("db file perm = %o, want 0600", perm)
	}
}
