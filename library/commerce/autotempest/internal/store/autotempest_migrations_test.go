// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

// TestBackfillSearchMembers proves a local DB created before the membership
// table keeps its `drops <name>` scoping: existing non-empty
// at_listings.search_name values seed at_search_members on the next ensure.
func TestBackfillSearchMembers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "data.db")
	s, err := OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer s.Close()
	db := s.DB()

	// First ensure creates the tables (incl. the new membership table).
	if err := EnsureAutoTempestTables(db); err != nil {
		t.Fatalf("ensure 1: %v", err)
	}

	// Simulate a legacy listing tagged via the old single search_name column,
	// with NO membership row yet (as if written by an older binary).
	if _, err := db.Exec(`INSERT INTO at_listings
		(listing_id, search_name, first_seen, last_seen, price_cents)
		VALUES ('legacy-1','old-search',1000,2000,1500000)`); err != nil {
		t.Fatalf("seed legacy listing: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM at_search_members`); err != nil {
		t.Fatalf("clear members: %v", err)
	}

	// Re-running ensure must backfill the membership row from search_name.
	if err := EnsureAutoTempestTables(db); err != nil {
		t.Fatalf("ensure 2 (backfill): %v", err)
	}

	var name string
	var firstAdded int64
	if err := db.QueryRow(
		`SELECT search_name, first_added FROM at_search_members WHERE listing_id = 'legacy-1'`,
	).Scan(&name, &firstAdded); err != nil {
		t.Fatalf("backfilled membership not found: %v", err)
	}
	if name != "old-search" {
		t.Errorf("backfilled search_name = %q, want old-search", name)
	}
	if firstAdded != 1000 { // COALESCE(first_seen, last_seen) -> first_seen
		t.Errorf("first_added = %d, want 1000 (first_seen)", firstAdded)
	}

	// Idempotent: a third ensure adds no duplicate rows.
	if err := EnsureAutoTempestTables(db); err != nil {
		t.Fatalf("ensure 3: %v", err)
	}
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM at_search_members WHERE listing_id = 'legacy-1'`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 membership row after repeated ensures, got %d", count)
	}
}
