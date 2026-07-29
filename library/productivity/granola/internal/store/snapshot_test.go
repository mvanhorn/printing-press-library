// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"os"
	"path/filepath"
	"testing"
)

// TestWALPending pins the disclosure signal: informational only, never
// load-bearing (no open branches on it), true exactly when a non-empty
// -wal sits next to the file.
func TestWALPending(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")

	if WALPending(dbPath) {
		t.Error("no -wal on disk: want false")
	}
	if err := os.WriteFile(dbPath+"-wal", []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	if WALPending(dbPath) {
		t.Error("empty -wal carries no state: want false")
	}
	if err := os.WriteFile(dbPath+"-wal", []byte("frame"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !WALPending(dbPath) {
		t.Error("non-empty -wal: want true")
	}
}
