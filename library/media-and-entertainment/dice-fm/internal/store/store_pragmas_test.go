// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
package store

import (
	"context"
	"path/filepath"
	"testing"
)

// TestStorePragmasApplied guards against the modernc.org/sqlite DSN-syntax bug
// (cli-printing-press#2394): the mattn-style "_journal_mode=WAL" params are
// silently ignored by this driver, which would leave the DB in the default
// rollback journal with busy_timeout=0. The DSN must use the "_pragma=name(value)"
// form so the intended pragmas actually take effect.
func TestStorePragmasApplied(t *testing.T) {
	s, err := OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()

	var journalMode string
	if err := s.DB().QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journalMode != "wal" {
		t.Errorf("journal_mode = %q, want \"wal\" (DSN pragmas not applied — see #2394)", journalMode)
	}

	var busyTimeout int
	if err := s.DB().QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("busy_timeout = %d, want 5000", busyTimeout)
	}

	var foreignKeys int
	if err := s.DB().QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys = %d, want 1 (ON)", foreignKeys)
	}
}
