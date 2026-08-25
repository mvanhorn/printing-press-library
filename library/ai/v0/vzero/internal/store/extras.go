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
		// model_usage records model attribution for CLI-originated
		// generations so `spend --by model` can roll up credit cost by
		// model tier (v0-mini, v0-pro, v0-max, v0-max-fast).
		`CREATE TABLE IF NOT EXISTS model_usage (
			chat_id TEXT PRIMARY KEY,
			model TEXT NOT NULL,
			recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
