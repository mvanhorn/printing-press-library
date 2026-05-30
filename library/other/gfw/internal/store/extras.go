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
		// Watchlist for the `watch pin` / `watch refresh` / `watch since` novel
		// features. Keyed by GFW vessel id; standalone (no FK) so a vessel can be
		// pinned before it is first fetched into the resources cache.
		`CREATE TABLE IF NOT EXISTS gfw_watchlist (
			vessel_id TEXT PRIMARY KEY,
			label TEXT,
			pinned_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
