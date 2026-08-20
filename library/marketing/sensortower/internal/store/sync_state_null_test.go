// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored regression test. Markerless on purpose: it guards a patch to
// generated code (see .printing-press-patches/), so `generate --force` must not
// clobber it.

package store

import (
	"path/filepath"
	"testing"
	"time"
)

// GetSyncState reads three nullable columns. backfillColumns adds last_cursor
// and last_synced_at to a pre-existing sync_state via ALTER TABLE, which leaves
// every legacy row NULL there, so a bare string/time.Time scan fails with
// "converting NULL to string is unsupported" — and because every caller in
// internal/cli discards the error, the failure surfaces as a silent re-sync from
// page 1 rather than as a diagnostic.
func TestGetSyncStateNullableColumns(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if _, err := s.DB().Exec(
		`INSERT INTO sync_state (resource_type, last_cursor, last_synced_at, total_count)
		 VALUES (?, NULL, NULL, 5)`, "widgets"); err != nil {
		t.Fatal(err)
	}

	cursor, lastSynced, count, err := s.GetSyncState("widgets")
	if err != nil {
		t.Fatalf("GetSyncState on NULL columns: unexpected error: %v", err)
	}
	if cursor != "" {
		t.Errorf("cursor = %q, want %q", cursor, "")
	}
	if !lastSynced.IsZero() {
		t.Errorf("lastSynced = %v, want zero", lastSynced)
	}
	// The regression that mattered: total_count is NOT NULL and must survive a
	// NULL in a sibling column rather than being dropped to 0 with the row.
	if count != 5 {
		t.Errorf("count = %d, want 5 (a NULL sibling column must not drop total_count)", count)
	}
}

// The non-NULL path must keep round-tripping intact.
func TestGetSyncStateRoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	if err := s.SaveSyncState("widgets", "cursor-42", 7); err != nil {
		t.Fatal(err)
	}
	cursor, lastSynced, count, err := s.GetSyncState("widgets")
	if err != nil {
		t.Fatal(err)
	}
	if cursor != "cursor-42" {
		t.Errorf("cursor = %q, want %q", cursor, "cursor-42")
	}
	if count != 7 {
		t.Errorf("count = %d, want 7", count)
	}
	if time.Since(lastSynced) > time.Hour || lastSynced.IsZero() {
		t.Errorf("lastSynced = %v, want a recent timestamp", lastSynced)
	}
}

// A resource type that was never synced is a legitimate "nothing yet", not an
// error: sql.ErrNoRows must degrade to zero values.
func TestGetSyncStateMissingRow(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	cursor, lastSynced, count, err := s.GetSyncState("never-synced")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cursor != "" || !lastSynced.IsZero() || count != 0 {
		t.Errorf("got (%q, %v, %d), want zero values", cursor, lastSynced, count)
	}
}
