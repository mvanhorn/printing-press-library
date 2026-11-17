// Copyright 2026 Josselin and contributors. Licensed under Apache-2.0. See LICENSE.

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
		// One saved results page: a search the user ran with --save (rank
		// check, batch run, engines/geo compare). Keyword identity is
		// (query, engine, country, lang).
		`CREATE TABLE IF NOT EXISTS serp_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query TEXT NOT NULL,
			engine TEXT NOT NULL,
			country TEXT NOT NULL DEFAULT '',
			lang TEXT NOT NULL DEFAULT '',
			location TEXT NOT NULL DEFAULT '',
			command TEXT NOT NULL DEFAULT '',
			taken_at TEXT NOT NULL,
			result_count INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_serp_snapshots_key ON serp_snapshots(query, engine, country, lang, taken_at)`,
		// Keyword lookups compare lower(query); this expression index serves them.
		`CREATE INDEX IF NOT EXISTS idx_serp_snapshots_lkey ON serp_snapshots(lower(query), engine, country, lang, taken_at)`,
		`CREATE TABLE IF NOT EXISTS serp_results (
			snapshot_id INTEGER NOT NULL REFERENCES serp_snapshots(id) ON DELETE CASCADE,
			position INTEGER NOT NULL,
			url TEXT NOT NULL,
			domain TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			snippet TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_serp_results_snapshot ON serp_results(snapshot_id, position)`,
		`CREATE INDEX IF NOT EXISTS idx_serp_results_domain ON serp_results(domain)`,
		// News articles already shown by news watch, one row per dedupe key.
		// The query column holds the watch scope (query|country|lang), and every
		// key is prefixed with that scope, so separate watches keep separate memories.
		`CREATE TABLE IF NOT EXISTS news_seen (
			key TEXT PRIMARY KEY,
			query TEXT NOT NULL,
			url TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT '',
			published TEXT NOT NULL DEFAULT '',
			first_seen TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_news_seen_scope ON news_seen(query)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
