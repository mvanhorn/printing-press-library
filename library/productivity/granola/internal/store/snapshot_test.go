// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSnapshotDSNQuery pins the WAL-vs-immutable choice: a non-empty -wal
// means committed state can live outside the main file, so the snapshot
// must read through the WAL; otherwise immutable inspects with zero side
// files. Both branches stay mode=ro either way.
func TestSnapshotDSNQuery(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data.db")

	if q := snapshotDSNQuery(dbPath); !strings.Contains(q, "immutable=1") {
		t.Errorf("no -wal on disk: want immutable snapshot, got %q", q)
	}

	if err := os.WriteFile(dbPath+"-wal", []byte{}, 0o600); err != nil {
		t.Fatal(err)
	}
	if q := snapshotDSNQuery(dbPath); !strings.Contains(q, "immutable=1") {
		t.Errorf("empty -wal carries no state: want immutable snapshot, got %q", q)
	}

	if err := os.WriteFile(dbPath+"-wal", []byte("frame"), 0o600); err != nil {
		t.Fatal(err)
	}
	q := snapshotDSNQuery(dbPath)
	if strings.Contains(q, "immutable=1") {
		t.Errorf("non-empty -wal: immutable would miss WAL state, got %q", q)
	}
	if !strings.Contains(q, "mode=ro") {
		t.Errorf("WAL-aware branch must stay read-only, got %q", q)
	}
}
