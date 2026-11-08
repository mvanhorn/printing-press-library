// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.

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
		// Mirror history for drift, digest and newcomer. console_id is part of every
		// key: site IDs repeat across consoles, and "seen before" means on this console.
		`CREATE TABLE IF NOT EXISTS unifi_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			console_id TEXT NOT NULL,
			taken_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_unifi_snapshots_console ON unifi_snapshots(console_id, id)`,
		`CREATE TABLE IF NOT EXISTS unifi_snapshot_rows (
			snapshot_id INTEGER NOT NULL REFERENCES unifi_snapshots(id) ON DELETE CASCADE,
			resource_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			data TEXT NOT NULL,
			PRIMARY KEY (snapshot_id, resource_type, entity_id)
		)`,
		`CREATE TABLE IF NOT EXISTS unifi_first_seen (
			console_id TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			entity_id TEXT NOT NULL,
			first_seen_at TEXT NOT NULL,
			last_seen_at TEXT NOT NULL,
			baseline INTEGER NOT NULL DEFAULT 0,
			label TEXT NOT NULL DEFAULT '',
			mac TEXT NOT NULL DEFAULT '',
			ip TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (console_id, resource_type, entity_id)
		)`,
		// The change journal. Before and after images are scrubbed of secret-named
		// fields before they get here; secrets_scrubbed records that undo cannot
		// restore those fields exactly.
		`CREATE TABLE IF NOT EXISTS unifi_change_journal (
			id TEXT PRIMARY KEY,
			console_id TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			state TEXT NOT NULL,
			kind TEXT NOT NULL,
			method TEXT NOT NULL,
			path TEXT NOT NULL,
			read_path TEXT NOT NULL DEFAULT '',
			resource_type TEXT NOT NULL DEFAULT '',
			entity_id TEXT NOT NULL DEFAULT '',
			request_body TEXT NOT NULL DEFAULT '',
			before_image TEXT NOT NULL DEFAULT '',
			after_image TEXT NOT NULL DEFAULT '',
			secrets_scrubbed INTEGER NOT NULL DEFAULT 0,
			undoes TEXT NOT NULL DEFAULT '',
			undone_by TEXT NOT NULL DEFAULT '',
			transport TEXT NOT NULL DEFAULT '',
			detail TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_unifi_change_journal_console ON unifi_change_journal(console_id, created_at)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
