// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// revenueCatNovelMigrations holds hand-authored auxiliary tables for the novel
// RevenueCat commands. These are NOT generated from the OpenAPI spec; they back
// command-specific local history (e.g. revenue-snapshot's run-over-run diff).
//
// Keep every statement idempotent (CREATE TABLE/INDEX IF NOT EXISTS) — it is
// re-run on every store open via migrateExtras.
var revenueCatNovelMigrations = []string{
	// rc_snapshots persists one row per `revenue-snapshot` run so the command
	// can diff the current metrics against the prior run. Metrics themselves
	// are live-only (RevenueCat does not expose historical overview metrics),
	// so this table is the command's own history ledger.
	`CREATE TABLE IF NOT EXISTS rc_snapshots (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id   TEXT NOT NULL,
		captured_at  TEXT NOT NULL,
		mrr          REAL NOT NULL DEFAULT 0,
		arr          REAL NOT NULL DEFAULT 0,
		active_subs  REAL NOT NULL DEFAULT 0,
		active_trials REAL NOT NULL DEFAULT 0,
		revenue      REAL NOT NULL DEFAULT 0,
		metrics_json TEXT NOT NULL DEFAULT '{}'
	)`,
	`CREATE INDEX IF NOT EXISTS idx_rc_snapshots_project_captured
		ON rc_snapshots (project_id, captured_at)`,
}

// migrateRevenueCatNovel applies the hand-authored novel-feature tables. It is
// invoked from migrateExtras so it runs after the generated migrations and
// before the schema-version stamp.
func (s *Store) migrateRevenueCatNovel(ctx context.Context, conn *sql.Conn) error {
	for _, m := range revenueCatNovelMigrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("revenuecat novel migration failed: %w", err)
		}
	}
	return nil
}
