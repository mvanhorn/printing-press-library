// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.
//
// schema_extra.go is hand-written and adds the novel-feature tables on top of
// the generator-owned schema. Called lazily from each novel command instead of
// from migrate() so a regenerated store.go (which would clobber a migration
// list) does not strand novel commands without their tables.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// extraMigrations are the additional tables and indexes used by the novel
// commands (download, search, watchlist, whatsnew, diff, highlights diff,
// doctor full). All statements are idempotent — safe to call repeatedly.
var extraMigrations = []string{
	`CREATE TABLE IF NOT EXISTS media_files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		shortcode TEXT NOT NULL,
		carousel_index INTEGER NOT NULL DEFAULT 0,
		kind TEXT NOT NULL CHECK(kind IN ('image','video')),
		cdn_url TEXT NOT NULL,
		local_path TEXT,
		width INTEGER,
		height INTEGER,
		bytes INTEGER,
		sha256 TEXT,
		alt_text TEXT,
		taken_at INTEGER,
		owner_username TEXT,
		caption TEXT,
		downloaded_at INTEGER,
		UNIQUE(shortcode, carousel_index, cdn_url)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_media_files_shortcode ON media_files(shortcode)`,
	`CREATE INDEX IF NOT EXISTS idx_media_files_owner ON media_files(owner_username)`,
	`CREATE INDEX IF NOT EXISTS idx_media_files_downloaded ON media_files(downloaded_at)`,

	`CREATE VIRTUAL TABLE IF NOT EXISTS media_files_fts USING fts5(
		shortcode, owner_username, caption, alt_text,
		content='media_files', content_rowid='id', tokenize='porter unicode61'
	)`,

	`CREATE TABLE IF NOT EXISTS watchlist (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		entry_kind TEXT NOT NULL CHECK(entry_kind IN ('profile','hashtag','location')),
		entry_value TEXT NOT NULL,
		added_at INTEGER NOT NULL,
		last_synced_at INTEGER,
		UNIQUE(entry_kind, entry_value)
	)`,

	`CREATE TABLE IF NOT EXISTS download_history (
		shortcode TEXT PRIMARY KEY,
		owner_username TEXT,
		status TEXT NOT NULL,
		attempts INTEGER NOT NULL DEFAULT 1,
		last_error TEXT,
		last_attempt_at INTEGER NOT NULL,
		first_seen_at INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_download_history_owner ON download_history(owner_username)`,

	`CREATE TABLE IF NOT EXISTS rate_limit_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		endpoint TEXT NOT NULL,
		status_code INTEGER NOT NULL,
		ts INTEGER NOT NULL,
		retry_after INTEGER
	)`,
	`CREATE INDEX IF NOT EXISTS idx_rate_limit_ts ON rate_limit_events(ts)`,

	`CREATE TABLE IF NOT EXISTS highlight_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_username TEXT NOT NULL,
		taken_at INTEGER NOT NULL,
		tray_json TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_highlight_snapshots_owner ON highlight_snapshots(owner_username, taken_at)`,

	`CREATE TABLE IF NOT EXISTS sync_runs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		started_at INTEGER NOT NULL,
		finished_at INTEGER,
		scope TEXT NOT NULL,
		added_count INTEGER NOT NULL DEFAULT 0,
		skipped_count INTEGER NOT NULL DEFAULT 0,
		error_count INTEGER NOT NULL DEFAULT 0,
		notes TEXT
	)`,
}

// ApplyExtraMigrations creates the novel-feature tables if they don't exist.
// Idempotent and cheap on warm DBs (CREATE TABLE IF NOT EXISTS short-circuits
// at SQLite's metadata layer); novel commands call this on entry so a fresh
// install gets the tables even if no sync has run yet.
func ApplyExtraMigrations(ctx context.Context, db *sql.DB) error {
	for _, stmt := range extraMigrations {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("extra migration: %w", err)
		}
	}
	return nil
}
