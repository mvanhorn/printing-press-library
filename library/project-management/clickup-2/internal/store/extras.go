// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.

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
		// pm_snapshot backs the `changed-since` activity-delta command. Each
		// row is the last-observed fingerprint of a task; `changed-since`
		// diffs the live task table against it, reports the changes, then
		// rewrites the snapshot. fingerprint columns are deliberately narrow
		// (the fields that "change" in a PM workflow) so the diff is cheap.
		`CREATE TABLE IF NOT EXISTS pm_snapshot (
			task_id TEXT PRIMARY KEY,
			status TEXT,
			assignee_ids TEXT,
			due_date INTEGER,
			date_updated INTEGER,
			snapshot_at INTEGER
		)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
