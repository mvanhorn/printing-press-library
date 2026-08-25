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
		`CREATE TABLE IF NOT EXISTS research_sessions (
			id TEXT PRIMARY KEY,
			created_at TEXT NOT NULL,
			notes TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS research_session_members (
			session_id TEXT NOT NULL,
			kind TEXT NOT NULL,
			ref_id TEXT NOT NULL,
			created_at TEXT NOT NULL,
			PRIMARY KEY(session_id, kind, ref_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_research_session_members_session_id
		 ON research_session_members(session_id)`,
		`CREATE TABLE IF NOT EXISTS balance_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			captured_at TEXT NOT NULL,
			org_id TEXT,
			credit_balance_cents REAL,
			pending_debit_balance_cents REAL,
			will_invoice INTEGER,
			raw_json TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_balance_snapshots_captured_at
		 ON balance_snapshots(captured_at)`,
		`CREATE TABLE IF NOT EXISTS task_interactions (
			run_id TEXT PRIMARY KEY,
			previous_interaction_id TEXT,
			created_at TEXT NOT NULL,
			raw_json TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_task_interactions_previous
		 ON task_interactions(previous_interaction_id)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
