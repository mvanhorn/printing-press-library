// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

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
		// psx_watchlist: the saved symbol set. PSX has no user accounts, so
		// this table is the only place a per-user watchlist can exist.
		`CREATE TABLE IF NOT EXISTS psx_watchlist (
			symbol      TEXT PRIMARY KEY,
			added_at    TEXT NOT NULL,
			added_price REAL,
			note        TEXT
		)`,
		// psx_snapshots: append-only history of whole-market and screener
		// rows. The portal renders only a current view and retains nothing,
		// so every longitudinal command (diff, drift, unusual, rotation)
		// reads from here rather than from upstream.
		`CREATE TABLE IF NOT EXISTS psx_snapshots (
			taken_at TEXT NOT NULL,
			kind     TEXT NOT NULL,
			symbol   TEXT NOT NULL,
			data     TEXT NOT NULL,
			PRIMARY KEY (taken_at, kind, symbol)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_psx_snapshots_kind_symbol
			ON psx_snapshots (kind, symbol, taken_at)`,
		`CREATE INDEX IF NOT EXISTS idx_psx_snapshots_kind_taken
			ON psx_snapshots (kind, taken_at)`,
		// Pad taken_at values written before snapshots moved to the fixed-width
		// nanosecond layout. Every query above orders and filters taken_at as
		// TEXT, so lexicographic order has to equal chronological order. The
		// legacy second-resolution form ("...:05Z") breaks that against the
		// fractional form ("...:05.000000000Z") inside one second, because 'Z'
		// (0x5A) sorts above '.' (0x2E) -- the newer row would sort first.
		//
		// Idempotent: the pattern is exactly the unpadded 20-character UTC form,
		// and LIKE matches the whole value, so a padded 30-character row cannot
		// match. OR REPLACE covers the sub-nanosecond chance that padding
		// collides with an existing row for the same instant, kind, and symbol.
		`UPDATE OR REPLACE psx_snapshots
			SET taken_at = substr(taken_at, 1, 19) || '.000000000Z'
			WHERE taken_at LIKE '____-__-__T__:__:__Z'`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
