// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.

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
	// If resources_history was created with the legacy schema (column "id TEXT"
	// instead of "resource_id TEXT"), drop it so the CREATE TABLE below can
	// recreate it with the correct schema. The table contains ephemeral snapshot
	// data used by ddl_drift; dropping it is safe — drift history is rebuilt on
	// the next sync pair.
	var oldColCount int
	_ = conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM pragma_table_info('resources_history')
		WHERE name = 'id' AND type = 'TEXT'
	`).Scan(&oldColCount)
	if oldColCount > 0 {
		if _, err := conn.ExecContext(ctx, `DROP TABLE IF EXISTS resources_history`); err != nil {
			return fmt.Errorf("extra migration failed dropping legacy resources_history: %w", err)
		}
	}

	migrations := []string{
		// Drop legacy index names that referenced the old column before recreating.
		`DROP INDEX IF EXISTS idx_rh_type_id`,
		`DROP INDEX IF EXISTS idx_rh_captured`,
		`CREATE TABLE IF NOT EXISTS resources_history (
			id            INTEGER PRIMARY KEY AUTOINCREMENT,
			resource_type TEXT    NOT NULL,
			resource_id   TEXT    NOT NULL,
			data          JSON    NOT NULL,
			captured_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_resources_history_type_id
			ON resources_history (resource_type, resource_id)`,
		`CREATE INDEX IF NOT EXISTS idx_resources_history_captured_at
			ON resources_history (captured_at)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
