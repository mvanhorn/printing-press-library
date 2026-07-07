// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/autotempest/internal/autotempest"
	"github.com/mvanhorn/printing-press-library/library/commerce/autotempest/internal/store"

	_ "modernc.org/sqlite"
)

// openTestStore opens a fresh store with the AutoTempest tables ensured.
func openTestStore(t *testing.T) (*sql.DB, func()) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "data.db")
	db, err := store.OpenWithContext(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := store.EnsureAutoTempestTables(db.DB()); err != nil {
		t.Fatalf("ensure tables: %v", err)
	}
	return db.DB(), func() { db.Close() }
}

func insertListing(t *testing.T, sqlDB *sql.DB, id string, priceCents int64) {
	t.Helper()
	insertListingFull(t, sqlDB, id, "2018 Honda Civic", "Honda", "Civic", priceCents)
}

// insertListingMM inserts a listing with explicit make/model so make-vs-model
// column-binding tests can distinguish them. The title is derived from make and
// model. vin is unique per id so dedupe treats each as its own VIN.
func insertListingMM(t *testing.T, sqlDB *sql.DB, id, mk, model string, priceCents int64) {
	t.Helper()
	insertListingFull(t, sqlDB, id, mk+" "+model, mk, model, priceCents)
}

// insertListingFull is the shared insert with an explicit title; source "te".
func insertListingFull(t *testing.T, sqlDB *sql.DB, id, title, mk, model string, priceCents int64) {
	t.Helper()
	_, err := sqlDB.Exec(`INSERT INTO at_listings
		(listing_id, vin, title, make, model, year, price_cents, source, url)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(listing_id) DO UPDATE SET
			title=excluded.title, make=excluded.make, model=excluded.model,
			price_cents=excluded.price_cents`,
		id, "VIN"+id, title, mk, model, 2018, priceCents, "te", "https://example.com/"+id)
	if err != nil {
		t.Fatalf("insert listing: %v", err)
	}
}

func insertSnapshot(t *testing.T, sqlDB *sql.DB, id string, ts, priceCents int64) {
	t.Helper()
	_, err := sqlDB.Exec(`INSERT OR IGNORE INTO at_price_snapshots
		(listing_id, ts, price_cents, mileage) VALUES (?,?,?,?)`, id, ts, priceCents, 0)
	if err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
}

func snapshotCount(t *testing.T, sqlDB *sql.DB, id string) int {
	t.Helper()
	var n int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM at_price_snapshots WHERE listing_id = ?`, id).Scan(&n); err != nil {
		t.Fatalf("count snapshots: %v", err)
	}
	return n
}

// TestDropsOscillationRecovery proves the snapshot model captures the full
// timeline (including a price recovery to a prior value) and that drops does
// NOT report a stale 30000->28000 drop once the price has recovered.
//
// Three syncs: $30,000 -> $28,000 -> $30,000. Under the OLD
// UNIQUE(listing_id, price_cents, mileage) schema the recovery row (price back
// to 30000) was silently dropped by INSERT OR IGNORE, leaving only two
// snapshots and a permanent false 30000->28000 drop. Under the new
// UNIQUE(listing_id, ts) schema all three land, earliest==latest==30000, and
// the drop is correctly absent.
func TestDropsOscillationRecovery(t *testing.T) {
	ctx := context.Background()
	sqlDB, cleanup := openTestStore(t)
	defer cleanup()

	const id = "te-osc-1"
	insertListing(t, sqlDB, id, 3000000) // latest known price = $30,000

	// Three syncs at distinct seconds.
	insertSnapshot(t, sqlDB, id, 1000, 3000000) // $30,000
	insertSnapshot(t, sqlDB, id, 2000, 2800000) // $28,000
	insertSnapshot(t, sqlDB, id, 3000, 3000000) // $30,000 (recovery)

	if got := snapshotCount(t, sqlDB, id); got != 3 {
		t.Fatalf("expected 3 snapshots (full timeline incl. recovery), got %d", got)
	}

	rows, err := dropRows(ctx, sqlDB, 0, 0, "", 0)
	if err != nil {
		t.Fatalf("dropRows: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected NO current drop after recovery, got %d rows: %+v", len(rows), rows)
	}
}

// TestDropsRealDrop confirms a genuine down move (no recovery) is reported with
// the right old/new/drop figures, and that the batched metadata join populates
// display fields.
func TestDropsRealDrop(t *testing.T) {
	ctx := context.Background()
	sqlDB, cleanup := openTestStore(t)
	defer cleanup()

	const id = "te-drop-1"
	insertListing(t, sqlDB, id, 2800000)
	insertSnapshot(t, sqlDB, id, 1000, 3000000) // $30,000
	insertSnapshot(t, sqlDB, id, 2000, 2800000) // $28,000 (current)

	rows, err := dropRows(ctx, sqlDB, 0, 0, "", 0)
	if err != nil {
		t.Fatalf("dropRows: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 drop, got %d: %+v", len(rows), rows)
	}
	r := rows[0]
	if r["old_price"] != "$30,000" || r["new_price"] != "$28,000" || r["drop"] != "$2,000" {
		t.Errorf("bad drop figures: old=%v new=%v drop=%v", r["old_price"], r["new_price"], r["drop"])
	}
	if r["title"] != "2018 Honda Civic" || r["make"] != "Honda" || r["source"] != "te" {
		t.Errorf("metadata join missing fields: %+v", r)
	}
}

// TestSnapshotWritePathDedupesUnchanged proves the write path (persistListings)
// records a new snapshot only when the price differs from the MOST RECENT one:
// re-syncing the same price is a no-op.
func TestSnapshotWritePathDedupesUnchanged(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "data.db")

	l := autotempest.Listing{
		ID: "te-wp-1", VIN: "VIN1", Title: "2018 Honda Civic",
		Make: "Honda", Model: "Civic", Year: 2018,
		PriceCents: 3000000, Mileage: 50000, Source: "te", URL: "https://example.com/x",
	}
	// Two persist calls at the same price -> exactly 1 snapshot. The
	// "differs from most recent" check dedupes same-price re-syncs even within
	// the same second (independent of the ts-unique constraint).
	if err := persistListings(ctx, dbPath, "", []autotempest.Listing{l}); err != nil {
		t.Fatalf("persist 1: %v", err)
	}
	if err := persistListings(ctx, dbPath, "", []autotempest.Listing{l}); err != nil {
		t.Fatalf("persist 2 (same price): %v", err)
	}

	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()
	var n int
	if err := db.DB().QueryRow(`SELECT COUNT(*) FROM at_price_snapshots WHERE listing_id = ?`, "te-wp-1").Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 snapshot after two same-price syncs, got %d", n)
	}
}

// TestDropsMembershipSurvivesSecondSearchRun proves the many-to-many membership
// fix: a listing found under BOTH "cheap-civics" and "any-civics" must remain
// scoped to BOTH after a later run of the OTHER search. Under the old single
// at_listings.search_name column, the second run's COALESCE overwrote
// search_name, silently dropping the listing from the first search's `drops`.
func TestDropsMembershipSurvivesSecondSearchRun(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "data.db")

	l := autotempest.Listing{
		ID: "te-civic-1", VIN: "VINCIVIC", Title: "2018 Honda Civic",
		Make: "Honda", Model: "Civic", Year: 2018,
		PriceCents: 3000000, Mileage: 50000, Source: "te", URL: "https://example.com/civic",
	}

	// Run 1: the listing is found under "cheap-civics".
	if err := persistListings(ctx, dbPath, "cheap-civics", []autotempest.Listing{l}); err != nil {
		t.Fatalf("persist run 1 (cheap-civics): %v", err)
	}
	// Run 2: the SAME listing is found under a DIFFERENT search, "any-civics".
	// (Old bug: this overwrote search_name, removing it from cheap-civics.)
	if err := persistListings(ctx, dbPath, "any-civics", []autotempest.Listing{l}); err != nil {
		t.Fatalf("persist run 2 (any-civics): %v", err)
	}

	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db.Close()
	sqlDB := db.DB()

	// Membership must exist in BOTH searches.
	for _, name := range []string{"cheap-civics", "any-civics"} {
		var c int
		if err := sqlDB.QueryRow(
			`SELECT COUNT(*) FROM at_search_members WHERE search_name = ? AND listing_id = ?`,
			name, l.ID).Scan(&c); err != nil {
			t.Fatalf("membership count for %s: %v", name, err)
		}
		if c != 1 {
			t.Fatalf("listing missing from search %q after second run (membership lost)", name)
		}
	}

	// Simulate a price drop with two snapshots at distinct timestamps so drops
	// reports a current 30000 -> 28000 move (persistListings collapses to the
	// current second, so seed the timeline directly).
	if _, err := sqlDB.Exec(`DELETE FROM at_price_snapshots WHERE listing_id = ?`, l.ID); err != nil {
		t.Fatalf("clear snapshots: %v", err)
	}
	insertSnapshot(t, sqlDB, l.ID, 1000, 3000000) // $30,000
	insertSnapshot(t, sqlDB, l.ID, 2000, 2800000) // $28,000 (current)

	// The drop must appear under BOTH searches — the listing was NOT lost from
	// the first search by the second run.
	for _, name := range []string{"cheap-civics", "any-civics"} {
		rows, err := dropRows(ctx, sqlDB, 0, 0, name, 0)
		if err != nil {
			t.Fatalf("dropRows %q: %v", name, err)
		}
		if len(rows) != 1 || rows[0]["listing_id"] != l.ID {
			t.Errorf("drops %q: expected 1 row for %s, got %d: %+v", name, l.ID, len(rows), rows)
		}
	}

	// A search that never saw this listing must NOT include it.
	rows, err := dropRows(ctx, sqlDB, 0, 0, "trucks", 0)
	if err != nil {
		t.Fatalf("dropRows trucks: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("drops trucks: expected 0 rows (no membership), got %d: %+v", len(rows), rows)
	}
}
