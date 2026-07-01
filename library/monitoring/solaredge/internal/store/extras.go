// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.

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
		// solaredge_call_log backs the budget tracker (RecordSolarEdgeAPICalls /
		// SolarEdgeCallsToday / SolarEdgeCallsTodayAllSites in
		// solaredge_migrations.go). See that file for why this table exists.
		`CREATE TABLE IF NOT EXISTS solaredge_call_log (
			day TEXT NOT NULL,
			site_id TEXT NOT NULL,
			calls INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (day, site_id)
		)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
