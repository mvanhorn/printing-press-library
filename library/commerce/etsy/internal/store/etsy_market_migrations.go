// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written (not generated): lazy market-research tables for `market snapshot`.

package store

import (
	"database/sql"
	"fmt"
)

// EnsureMarketSchema lazily creates the market-research tables. It is called
// only by writers (`market snapshot`); readers must tolerate missing tables.
//
// Privacy: market_reviews deliberately holds a timestamp and a rating only.
// Review text and buyer identifiers are never stored.
func EnsureMarketSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS market_term_pages (
            snapshot_id INTEGER NOT NULL, term TEXT NOT NULL, page INTEGER NOT NULL,
            result_count INTEGER, listings INTEGER NOT NULL,
            PRIMARY KEY (snapshot_id, term, page))`,
		`CREATE TABLE IF NOT EXISTS market_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			taken_at TEXT NOT NULL,
			terms_json TEXT NOT NULL,
			pages INTEGER NOT NULL,
			sort_on TEXT NOT NULL,
			max_calls INTEGER NOT NULL,
			calls_used INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'running',
			with_reviews INTEGER NOT NULL DEFAULT 0,
			review_window_days INTEGER NOT NULL DEFAULT 0,
			progress_json TEXT NOT NULL DEFAULT '{}'
		)`,
		`CREATE TABLE IF NOT EXISTS market_results (
			snapshot_id INTEGER NOT NULL,
			term TEXT NOT NULL,
			page INTEGER NOT NULL,
			position INTEGER NOT NULL,
			listing_id INTEGER NOT NULL,
			shop_id INTEGER,
			title TEXT,
			price_amount INTEGER,
			price_divisor INTEGER,
			currency TEXT,
			num_favorers INTEGER,
			views INTEGER,
			tags_json TEXT,
			taxonomy_id INTEGER,
			original_creation_ts INTEGER,
			is_digital INTEGER,
			result_count INTEGER,
			PRIMARY KEY (snapshot_id, term, listing_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_market_results_snapshot ON market_results(snapshot_id, term, position)`,
		`CREATE INDEX IF NOT EXISTS idx_market_results_listing ON market_results(snapshot_id, listing_id)`,
		`CREATE INDEX IF NOT EXISTS idx_market_results_shop ON market_results(snapshot_id, shop_id)`,
		`CREATE TABLE IF NOT EXISTS market_shops (
			snapshot_id INTEGER NOT NULL,
			shop_id INTEGER NOT NULL,
			shop_name TEXT,
			transaction_sold_count INTEGER,
			listing_active_count INTEGER,
			create_date INTEGER,
			review_average REAL,
			review_count INTEGER,
			is_vacation INTEGER,
			PRIMARY KEY (snapshot_id, shop_id)
		)`,
		`CREATE TABLE IF NOT EXISTS market_reviews (
			snapshot_id INTEGER NOT NULL,
			listing_id INTEGER NOT NULL,
			created_ts INTEGER NOT NULL,
			rating INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_market_reviews_listing ON market_reviews(snapshot_id, listing_id, created_ts)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("ensure market schema: %w", err)
		}
	}
	hasName, err := etsyMarketShopsHasName(db)
	if err != nil {
		return err
	}
	if !hasName {
		if _, err := db.Exec(`ALTER TABLE market_shops ADD COLUMN shop_name TEXT`); err != nil {
			// Another connection may have completed the same upgrade.
			if added, checkErr := etsyMarketShopsHasName(db); checkErr != nil || !added {
				return fmt.Errorf("ensure market shop name: %w", err)
			}
		}
	}
	return nil
}

func etsyMarketShopsHasName(db *sql.DB) (bool, error) {
	var count int
	err := db.QueryRow(`SELECT count(*) FROM pragma_table_info('market_shops') WHERE name='shop_name'`).Scan(&count)
	return count > 0, err
}
