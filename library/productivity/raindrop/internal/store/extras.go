// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.

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
		`CREATE TABLE IF NOT EXISTS raindrop_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			bookmark_id TEXT NOT NULL,
			old_data JSON NOT NULL,
			new_data JSON NOT NULL,
			changed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS raindrop_history_bookmark_time
			ON raindrop_history(bookmark_id, changed_at DESC)`,
		`CREATE TRIGGER IF NOT EXISTS raindrops_history_before_update
			BEFORE UPDATE OF data ON raindrops
			WHEN old.data <> new.data
			BEGIN
				INSERT INTO raindrop_history(bookmark_id, old_data, new_data, changed_at)
				VALUES(old.id, old.data, new.data, CURRENT_TIMESTAMP);
			END`,
		`CREATE TABLE IF NOT EXISTS cleanup_plans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			kind TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'planned',
			payload JSON NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			applied_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS inbox_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			status TEXT NOT NULL DEFAULT 'open',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS inbox_items (
			session_id INTEGER NOT NULL,
			bookmark_id TEXT NOT NULL,
			state TEXT NOT NULL DEFAULT 'pending',
			decision JSON,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(session_id, bookmark_id),
			FOREIGN KEY(session_id) REFERENCES inbox_sessions(id)
		)`,
		`CREATE TABLE IF NOT EXISTS reading_state (
			bookmark_id TEXT PRIMARY KEY,
			status TEXT NOT NULL DEFAULT 'queued',
			last_shown_at DATETIME,
			completed_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS triage_workflows (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			query TEXT NOT NULL,
			batch_size INTEGER NOT NULL DEFAULT 10,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS triage_items (
			workflow_id INTEGER NOT NULL,
			bookmark_id TEXT NOT NULL,
			state TEXT NOT NULL DEFAULT 'queued',
			attempts INTEGER NOT NULL DEFAULT 0,
			last_error TEXT,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(workflow_id, bookmark_id),
			FOREIGN KEY(workflow_id) REFERENCES triage_workflows(id)
		)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
