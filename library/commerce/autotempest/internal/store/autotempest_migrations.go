// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"fmt"
	"strings"
)

// atTableStatements are the idempotent DDL statements for the AutoTempest
// novel-feature tables. Kept as a package var so EnsureAutoTempestTables and
// any future migration hook share one source of truth.
var atTableStatements = []string{
	`CREATE TABLE IF NOT EXISTS at_listings (
		listing_id TEXT PRIMARY KEY,
		vin TEXT,
		title TEXT,
		make TEXT,
		model TEXT,
		year INTEGER,
		trim TEXT,
		price_cents INTEGER,
		mileage INTEGER,
		location TEXT,
		zip TEXT,
		country TEXT,
		distance REAL,
		dealer_name TEXT,
		seller_type TEXT,
		source TEXT,
		sitecode TEXT,
		vehicle_title TEXT,
		listing_type TEXT,
		current_bid_cents INTEGER,
		bids INTEGER,
		url TEXT,
		img TEXT,
		search_name TEXT,
		first_seen INTEGER,
		last_seen INTEGER,
		raw TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_at_listings_make_model_year ON at_listings(make, model, year)`,
	`CREATE INDEX IF NOT EXISTS idx_at_listings_vin ON at_listings(vin)`,
	`CREATE INDEX IF NOT EXISTS idx_at_listings_source ON at_listings(source)`,
	`CREATE INDEX IF NOT EXISTS idx_at_listings_price ON at_listings(price_cents)`,
	// Append-only price history keyed by (listing_id, ts). UNIQUE is on ts
	// (unix seconds) rather than price so the FULL timeline — including a price
	// that drops then recovers to a prior value — is preserved. The
	// write-path only inserts when the price differs from the listing's most
	// recent snapshot, so unchanged re-syncs do not bloat the table; the ts
	// uniqueness only collapses two writes that land in the same second.
	`CREATE TABLE IF NOT EXISTS at_price_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		listing_id TEXT,
		ts INTEGER,
		price_cents INTEGER,
		mileage INTEGER,
		UNIQUE(listing_id, ts)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_at_snapshots_listing ON at_price_snapshots(listing_id)`,
	`CREATE TABLE IF NOT EXISTS at_saved_searches (
		name TEXT PRIMARY KEY,
		query TEXT,
		params TEXT,
		created INTEGER,
		last_run INTEGER
	)`,
	// Many-to-many membership: a listing can belong to multiple overlapping
	// saved searches. This is the scoping source of truth for `drops <name>`,
	// replacing the single at_listings.search_name column (which a later
	// `watch run <other>` would overwrite, silently dropping the listing from
	// the first search).
	`CREATE TABLE IF NOT EXISTS at_search_members (
		search_name TEXT NOT NULL,
		listing_id TEXT NOT NULL,
		first_added INTEGER,
		PRIMARY KEY(search_name, listing_id)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_at_search_members_listing ON at_search_members(listing_id)`,
}

// EnsureAutoTempestTables creates the AutoTempest novel-feature tables if they
// do not already exist. Idempotent — every statement uses IF NOT EXISTS — so
// commands can call it lazily after opening the store, before any read/write.
func EnsureAutoTempestTables(db *sql.DB) error {
	if err := migrateSnapshotUnique(db); err != nil {
		return err
	}
	for _, stmt := range atTableStatements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("ensuring autotempest tables: %w", err)
		}
	}
	if err := backfillSearchMembers(db); err != nil {
		return err
	}
	return nil
}

// backfillSearchMembers seeds at_search_members from any existing non-empty
// at_listings.search_name values, so a local DB created before the membership
// table keeps its `drops <name>` scoping. INSERT OR IGNORE makes it idempotent:
// re-running adds nothing once the membership rows exist. Runs after the CREATE
// statements so both tables are present.
//
// first_added is set to the listing's first_seen when available (it best
// approximates when the listing entered the search) and falls back to its
// last_seen. This one-time backfill only covers the legacy single-column
// association; going forward the write path inserts membership rows directly.
func backfillSearchMembers(db *sql.DB) error {
	_, err := db.Exec(`INSERT OR IGNORE INTO at_search_members (search_name, listing_id, first_added)
		SELECT search_name, listing_id, COALESCE(first_seen, last_seen)
		FROM at_listings
		WHERE search_name IS NOT NULL AND search_name != ''`)
	if err != nil {
		return fmt.Errorf("backfilling at_search_members: %w", err)
	}
	return nil
}

// migrateSnapshotUnique rebuilds at_price_snapshots when an existing database
// still carries the old UNIQUE(listing_id, price_cents, mileage) constraint,
// which silently dropped oscillation recoveries (a price returning to a prior
// value was never re-recorded). The new schema is UNIQUE(listing_id, ts).
//
// Idempotent: a fresh DB (table absent) and an already-migrated DB (constraint
// not present in the stored DDL) both short-circuit. The rebuild copies every
// existing row into the new table, deduplicating any (listing_id, ts)
// collisions via INSERT OR IGNORE, then swaps it in.
func migrateSnapshotUnique(db *sql.DB) error {
	var ddl sql.NullString
	err := db.QueryRow(
		`SELECT sql FROM sqlite_master WHERE type='table' AND name='at_price_snapshots'`,
	).Scan(&ddl)
	if err == sql.ErrNoRows || !ddl.Valid {
		return nil // fresh DB; CREATE below makes the new schema
	}
	if err != nil {
		return fmt.Errorf("inspecting at_price_snapshots schema: %w", err)
	}
	if !strings.Contains(ddl.String, "price_cents, mileage") &&
		!strings.Contains(ddl.String, "price_cents,mileage") {
		return nil // already on the new (or some newer) schema
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin snapshot migration: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`CREATE TABLE IF NOT EXISTS at_price_snapshots_v2 (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		listing_id TEXT,
		ts INTEGER,
		price_cents INTEGER,
		mileage INTEGER,
		UNIQUE(listing_id, ts)
	)`); err != nil {
		return fmt.Errorf("create at_price_snapshots_v2: %w", err)
	}
	// Preserve chronological order; INSERT OR IGNORE collapses same-second
	// duplicates onto the (listing_id, ts) unique.
	if _, err := tx.Exec(`INSERT OR IGNORE INTO at_price_snapshots_v2
		(listing_id, ts, price_cents, mileage)
		SELECT listing_id, ts, price_cents, mileage
		FROM at_price_snapshots ORDER BY ts ASC, id ASC`); err != nil {
		return fmt.Errorf("copy snapshots into v2: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE at_price_snapshots`); err != nil {
		return fmt.Errorf("drop old at_price_snapshots: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE at_price_snapshots_v2 RENAME TO at_price_snapshots`); err != nil {
		return fmt.Errorf("rename at_price_snapshots_v2: %w", err)
	}
	return tx.Commit()
}
