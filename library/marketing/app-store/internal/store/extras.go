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
//
// The App Store snapshot tables (captures, app_snapshots, chart_snapshots,
// reviews + reviews_fts, ratings_snapshots, version_entries,
// privacy_snapshots, iap_prices, app_events, featured_snapshots,
// keyword_ranks) are defined once in appStoreMigrations (app_store_store.go)
// and applied here, inside the migration lock: the tables first, then the
// columns added since a table first shipped (appStoreColumns), then the
// statements that need those columns (appStorePostColumnMigrations).
func (s *Store) migrateExtras(ctx context.Context, conn *sql.Conn) error {
	for _, m := range appStoreMigrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	for _, c := range appStoreColumns {
		if err := s.ensureColumn(ctx, conn, c.table, c.column, c.decl); err != nil {
			return fmt.Errorf("extra column migration failed: %w", err)
		}
	}
	for _, m := range appStorePostColumnMigrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
