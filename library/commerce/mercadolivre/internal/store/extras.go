// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateExtras runs after the generated store migrations and before the
// schema-version stamp. It is the canonical place for novel-feature auxiliary
// tables that need to live in the local store.
//
// Edit this file when adding tables for novel commands. Keep migrations
// idempotent with CREATE TABLE IF NOT EXISTS / CREATE INDEX IF NOT EXISTS so
// every store open can safely re-run them.
func (s *Store) migrateExtras(ctx context.Context, conn *sql.Conn) error {
	migrations := []string{
		// Technical-spec attributes, one row per (catalog_id, attribute name).
		`CREATE TABLE IF NOT EXISTS attribute (
			catalog_id TEXT,
			name TEXT,
			value TEXT,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (catalog_id, name)
		)`,
		// Append-only price observations for dispersion / price-history.
		`CREATE TABLE IF NOT EXISTS price_snapshot (
			catalog_id TEXT,
			price REAL,
			currency TEXT,
			captured_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_price_snapshot_catalog_id ON price_snapshot(catalog_id)`,
		// Seller name scraped from a search-page poly-card, keyed by catalog_id.
		`CREATE TABLE IF NOT EXISTS listing_seller (
			catalog_id TEXT PRIMARY KEY,
			seller TEXT,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Total shipping window (handling + transit) in days, per catalog_id.
		`CREATE TABLE IF NOT EXISTS product_delivery (
			catalog_id TEXT PRIMARY KEY,
			min_days INTEGER,
			max_days INTEGER,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
