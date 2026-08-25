// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestMigrateExtrasPadsLegacySnapshotTimestamps covers a store written before
// snapshots moved to the fixed-width nanosecond layout. Snapshot queries order
// and filter taken_at as TEXT, so an unpadded legacy row must be rewritten or
// it sorts after a newer row from the same second.
func TestMigrateExtrasPadsLegacySnapshotTimestamps(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "psx.db")

	s, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	// Legacy second-resolution row, and a newer row 500ms later already in the
	// padded form. Chronologically legacy < newer; lexicographically, without
	// the migration, "…:00Z" > "…:00.500000000Z".
	const (
		legacy = "2026-08-19T20:00:00Z"
		padded = "2026-08-19T20:00:00.000000000Z"
		newer  = "2026-08-19T20:00:00.500000000Z"
	)
	for _, at := range []string{legacy, newer} {
		if _, err := s.DB().Exec(
			`INSERT INTO psx_snapshots (taken_at, kind, symbol, data) VALUES (?, ?, ?, ?)`,
			at, "market", "OGDC", `{"symbol":"OGDC"}`); err != nil {
			t.Fatalf("seed %s: %v", at, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	// Reopening re-runs migrateExtras, which is where the padding happens.
	s2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer s2.Close()

	rows, err := s2.DB().Query(
		`SELECT taken_at FROM psx_snapshots WHERE kind = ? ORDER BY taken_at ASC`, "market")
	if err != nil {
		t.Fatalf("query snapshots: %v", err)
	}
	defer rows.Close()

	var got []string
	for rows.Next() {
		var at string
		if err := rows.Scan(&at); err != nil {
			t.Fatalf("scan: %v", err)
		}
		got = append(got, at)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate: %v", err)
	}

	want := []string{padded, newer}
	if len(got) != len(want) {
		t.Fatalf("got %d rows %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %q, want %q (lexical order must equal chronological order)", i, got[i], want[i])
		}
	}

	// Re-running must not double-pad: the migration already ran twice by now.
	var padCount int
	if err := s2.DB().QueryRow(
		`SELECT COUNT(*) FROM psx_snapshots WHERE taken_at = ?`, padded).Scan(&padCount); err != nil {
		t.Fatalf("count padded: %v", err)
	}
	if padCount != 1 {
		t.Errorf("padded row count = %d, want 1 (migration is not idempotent)", padCount)
	}
}
