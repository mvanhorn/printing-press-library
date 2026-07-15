// Copyright 2026 danielkhunter and contributors. Licensed under Apache-2.0. See LICENSE.

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
		// topic_snapshots accumulates one row per heatmap keyword each time a
		// snapshot is captured (by `momentum`/`narrative`). The time dimension
		// is what lets those commands report movement the live snapshot cannot.
		`CREATE TABLE IF NOT EXISTS "topic_snapshots" (
			"captured_at" TEXT NOT NULL,
			"keyword" TEXT NOT NULL,
			"slug" TEXT,
			"count" INTEGER,
			"engagement" INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS "idx_topic_snapshots_captured" ON "topic_snapshots"("captured_at")`,
		// cli_state holds small key/value cursors for novel commands.
		`CREATE TABLE IF NOT EXISTS "cli_state" (
			"key" TEXT PRIMARY KEY,
			"value" TEXT
		)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
