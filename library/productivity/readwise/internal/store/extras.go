// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

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
		`CREATE TABLE IF NOT EXISTS watch_saved_search (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			name        TEXT NOT NULL UNIQUE,
			kind        TEXT NOT NULL,
			filters_json TEXT NOT NULL DEFAULT '{}',
			created_at  TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS watch_seen (
			watch_id  INTEGER NOT NULL,
			item_kind TEXT NOT NULL,
			item_id   TEXT NOT NULL,
			seen_at   TEXT NOT NULL,
			PRIMARY KEY(watch_id, item_kind, item_id)
		)`,
		`CREATE TABLE IF NOT EXISTS webhook (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			url        TEXT NOT NULL,
			secret     TEXT NOT NULL DEFAULT '',
			events     TEXT NOT NULL DEFAULT '*',
			include_text INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
