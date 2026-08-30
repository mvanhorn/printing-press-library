// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

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
		`CREATE TABLE IF NOT EXISTS resource_snapshots (
			resource_type TEXT NOT NULL,
			resource_id TEXT NOT NULL,
			data JSON NOT NULL,
			snapshot_date TEXT NOT NULL,
			captured_at DATETIME NOT NULL,
			PRIMARY KEY (resource_type, resource_id, snapshot_date)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_resource_snapshots_date ON resource_snapshots(snapshot_date)`,
		`CREATE INDEX IF NOT EXISTS idx_resource_snapshots_type_date ON resource_snapshots(resource_type, snapshot_date)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
