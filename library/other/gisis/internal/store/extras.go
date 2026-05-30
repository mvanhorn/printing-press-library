// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

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
		// Watchlist for the `ship pin` / `ship refresh --pinned` novel features.
		// Standalone (no FK to "ship") so an IMO can be pinned before it is first
		// fetched, and so cache pruning never cascades into the watchlist.
		`CREATE TABLE IF NOT EXISTS ship_pins (
			imo_number TEXT PRIMARY KEY,
			label TEXT,
			pinned_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// Indexes backing the cache-query novel features (ship list / stale /
		// owner fleet). The "ship" table is created by the generated migration
		// loop that runs immediately before migrateExtras, so it exists here.
		`CREATE INDEX IF NOT EXISTS idx_ship_flag ON "ship"(flag)`,
		`CREATE INDEX IF NOT EXISTS idx_ship_registered_owner ON "ship"(registered_owner)`,
		`CREATE INDEX IF NOT EXISTS idx_ship_type ON "ship"(ship_type)`,
		`CREATE INDEX IF NOT EXISTS idx_ship_synced_at ON "ship"(synced_at)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
