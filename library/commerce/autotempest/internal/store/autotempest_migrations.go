// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"fmt"
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
	`CREATE TABLE IF NOT EXISTS at_price_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		listing_id TEXT,
		ts INTEGER,
		price_cents INTEGER,
		mileage INTEGER,
		UNIQUE(listing_id, price_cents, mileage)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_at_snapshots_listing ON at_price_snapshots(listing_id)`,
	`CREATE TABLE IF NOT EXISTS at_saved_searches (
		name TEXT PRIMARY KEY,
		query TEXT,
		params TEXT,
		created INTEGER,
		last_run INTEGER
	)`,
}

// EnsureAutoTempestTables creates the AutoTempest novel-feature tables if they
// do not already exist. Idempotent — every statement uses IF NOT EXISTS — so
// commands can call it lazily after opening the store, before any read/write.
func EnsureAutoTempestTables(db *sql.DB) error {
	for _, stmt := range atTableStatements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("ensuring autotempest tables: %w", err)
		}
	}
	return nil
}
