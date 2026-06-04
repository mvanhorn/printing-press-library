// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

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
		`CREATE TABLE IF NOT EXISTS normalized_reports (
			id TEXT PRIMARY KEY,
			kind TEXT NOT NULL,
			source_path TEXT,
			row_count INTEGER NOT NULL,
			imported_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS normalized_report_rows (
			report_id TEXT NOT NULL,
			row_index INTEGER NOT NULL,
			data JSON NOT NULL,
			PRIMARY KEY (report_id, row_index),
			FOREIGN KEY (report_id) REFERENCES normalized_reports(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_normalized_report_rows_report_id ON normalized_report_rows(report_id)`,
		`CREATE TABLE IF NOT EXISTS keyword_snapshots (
			id TEXT PRIMARY KEY,
			name TEXT,
			source_path TEXT,
			snapshot_at DATETIME NOT NULL,
			row_count INTEGER NOT NULL,
			imported_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS keyword_snapshot_rows (
			snapshot_id TEXT NOT NULL,
			row_index INTEGER NOT NULL,
			keyword TEXT NOT NULL,
			campaign TEXT,
			ad_group TEXT,
			bid REAL,
			cpc REAL,
			spend REAL,
			sales REAL,
			orders INTEGER,
			clicks INTEGER,
			data JSON NOT NULL,
			PRIMARY KEY (snapshot_id, row_index),
			FOREIGN KEY (snapshot_id) REFERENCES keyword_snapshots(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_keyword_snapshot_rows_keyword ON keyword_snapshot_rows(keyword)`,
		`CREATE INDEX IF NOT EXISTS idx_keyword_snapshots_snapshot_at ON keyword_snapshots(snapshot_at)`,
		`CREATE TABLE IF NOT EXISTS automation_audit (
			id TEXT PRIMARY KEY,
			command TEXT NOT NULL,
			mode TEXT NOT NULL,
			report_path TEXT,
			plan_count INTEGER NOT NULL,
			payload JSON NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_automation_audit_created_at ON automation_audit(created_at)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
