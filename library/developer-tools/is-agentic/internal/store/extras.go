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
	const createSnapshots = `CREATE TABLE IF NOT EXISTS agentic_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		target TEXT NOT NULL,
		fetched_at TEXT NOT NULL,
		scanned_at TEXT,
		report_url TEXT,
		score REAL,
		report_json BLOB NOT NULL
	)`

	// Early development builds declared score NOT NULL. Upgrade that table in
	// place so existing local stores can accept valid reports whose score is
	// unavailable, rather than relying on CREATE TABLE IF NOT EXISTS.
	rows, err := conn.QueryContext(ctx, `PRAGMA table_info(agentic_snapshots)`)
	if err != nil {
		return fmt.Errorf("inspect report snapshot schema: %w", err)
	}
	legacyScoreRequired := false
	hasScore := false
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull, pk int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			rows.Close()
			return fmt.Errorf("read report snapshot schema: %w", err)
		}
		if name == "score" {
			hasScore = true
			legacyScoreRequired = notNull != 0
		}
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("close report snapshot schema: %w", err)
	}
	if legacyScoreRequired || (hasScore == false && agenticTableExists(ctx, conn, "agentic_snapshots")) {
		if _, err := conn.ExecContext(ctx, `DROP INDEX IF EXISTS idx_agentic_snapshots_target_fetched`); err != nil {
			return fmt.Errorf("drop legacy report snapshot index: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `ALTER TABLE agentic_snapshots RENAME TO agentic_snapshots_legacy`); err != nil {
			return fmt.Errorf("rename legacy report snapshots: %w", err)
		}
		if _, err := conn.ExecContext(ctx, createSnapshots); err != nil {
			return fmt.Errorf("create nullable report snapshots: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `INSERT INTO agentic_snapshots(id, target, fetched_at, scanned_at, report_url, score, report_json) SELECT id, target, fetched_at, scanned_at, report_url, score, report_json FROM agentic_snapshots_legacy`); err != nil {
			return fmt.Errorf("copy legacy report snapshots: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `DROP TABLE agentic_snapshots_legacy`); err != nil {
			return fmt.Errorf("drop legacy report snapshots: %w", err)
		}
	} else if _, err := conn.ExecContext(ctx, createSnapshots); err != nil {
		return fmt.Errorf("extra migration failed: %w", err)
	}
	if _, err := conn.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS idx_agentic_snapshots_target_fetched ON agentic_snapshots(target, fetched_at DESC)`); err != nil {
		return fmt.Errorf("create report snapshot index: %w", err)
	}
	return nil
}

func agenticTableExists(ctx context.Context, conn *sql.Conn, table string) bool {
	var count int
	if err := conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&count); err != nil {
		return false
	}
	return count > 0
}
