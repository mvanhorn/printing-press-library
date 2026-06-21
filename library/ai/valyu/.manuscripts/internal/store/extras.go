// Copyright 2026 Anchal Sharma and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateExtras runs after the generated store migrations and before the
// schema-version stamp. Edit this file to add tables for novel commands.
// Keep all statements idempotent (CREATE TABLE IF NOT EXISTS).
func (s *Store) migrateExtras(ctx context.Context, conn *sql.Conn) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS api_cost_ledger (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			command     TEXT    NOT NULL,
			cost        REAL    NOT NULL DEFAULT 0.0,
			created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cost_ledger_created ON api_cost_ledger(created_at)`,
		`CREATE TABLE IF NOT EXISTS search_history (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			query       TEXT    NOT NULL,
			params      TEXT    NOT NULL DEFAULT '{}',
			result_urls TEXT    NOT NULL DEFAULT '[]',
			cost        REAL    NOT NULL DEFAULT 0.0,
			created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		)`,
		`CREATE INDEX IF NOT EXISTS idx_search_history_created ON search_history(created_at)`,
		`CREATE TABLE IF NOT EXISTS search_templates (
			name        TEXT    PRIMARY KEY,
			query       TEXT    NOT NULL,
			params      TEXT    NOT NULL DEFAULT '{}',
			created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
		)`,
		`CREATE TABLE IF NOT EXISTS research_queue (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id     TEXT    NOT NULL UNIQUE,
			label       TEXT    NOT NULL,
			mode        TEXT    NOT NULL DEFAULT 'standard',
			state       TEXT    NOT NULL DEFAULT 'submitted',
			output_fmt  TEXT    NOT NULL DEFAULT 'markdown',
			submitted_at TEXT   NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now')),
			completed_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_research_queue_state ON research_queue(state)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
