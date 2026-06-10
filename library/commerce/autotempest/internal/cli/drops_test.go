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
