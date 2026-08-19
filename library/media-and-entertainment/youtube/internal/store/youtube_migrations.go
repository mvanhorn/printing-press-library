// Copyright 2026 Justin and contributors. Licensed under Apache-2.0. See LICENSE.

// Analyst databank schema for the competitor-monitoring machine. Lazy-init:
// every novel command that touches these tables calls EnsureAnalystSchema
// first. Kept in its own file (never edit the generated migration slice in
// store.go) so regeneration preserves it.

package store

import (
	"context"
	"fmt"
)

// analystSchemaStatements are idempotent (IF NOT EXISTS) so repeated calls
// are cheap and safe across binaries.
var analystSchemaStatements = []string{
	`CREATE TABLE IF NOT EXISTS yt_watchlist (
		channel_id TEXT PRIMARY KEY,
		handle TEXT DEFAULT '',
		title TEXT DEFAULT '',
		note TEXT DEFAULT '',
		added_at TEXT NOT NULL,
		last_monitored_at TEXT DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS yt_channel_snapshots (
		channel_id TEXT NOT NULL,
		captured_at TEXT NOT NULL,
		subscriber_count INTEGER DEFAULT 0,
		view_count INTEGER DEFAULT 0,
		video_count INTEGER DEFAULT 0,
		PRIMARY KEY (channel_id, captured_at)
	)`,
	`CREATE TABLE IF NOT EXISTS yt_videos (
		video_id TEXT PRIMARY KEY,
		channel_id TEXT NOT NULL,
		title TEXT DEFAULT '',
		published_at TEXT DEFAULT '',
		duration_seconds INTEGER DEFAULT 0,
		is_short INTEGER DEFAULT 0,
		description TEXT DEFAULT ''
	)`,
	`CREATE INDEX IF NOT EXISTS idx_yt_videos_channel ON yt_videos(channel_id, published_at)`,
	`CREATE TABLE IF NOT EXISTS yt_video_snapshots (
		video_id TEXT NOT NULL,
		captured_at TEXT NOT NULL,
		view_count INTEGER DEFAULT 0,
		like_count INTEGER DEFAULT 0,
		comment_count INTEGER DEFAULT 0,
		PRIMARY KEY (video_id, captured_at)
	)`,
	`CREATE TABLE IF NOT EXISTS yt_comments (
		comment_id TEXT PRIMARY KEY,
		video_id TEXT DEFAULT '',
		channel_id TEXT DEFAULT '',
		author TEXT DEFAULT '',
		text TEXT DEFAULT '',
		like_count INTEGER DEFAULT 0,
		published_at TEXT DEFAULT '',
		is_reply INTEGER DEFAULT 0
	)`,
	`CREATE INDEX IF NOT EXISTS idx_yt_comments_video ON yt_comments(video_id)`,
	`CREATE INDEX IF NOT EXISTS idx_yt_comments_channel ON yt_comments(channel_id)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS yt_comments_fts USING fts5(text, content='yt_comments', content_rowid='rowid')`,
	`CREATE TRIGGER IF NOT EXISTS yt_comments_fts_ai AFTER INSERT ON yt_comments BEGIN
		INSERT INTO yt_comments_fts(rowid, text) VALUES (new.rowid, new.text);
	END`,
	`CREATE TRIGGER IF NOT EXISTS yt_comments_fts_ad AFTER DELETE ON yt_comments BEGIN
		INSERT INTO yt_comments_fts(yt_comments_fts, rowid, text) VALUES('delete', old.rowid, old.text);
	END`,
	`CREATE TRIGGER IF NOT EXISTS yt_comments_fts_au AFTER UPDATE ON yt_comments BEGIN
		INSERT INTO yt_comments_fts(yt_comments_fts, rowid, text) VALUES('delete', old.rowid, old.text);
		INSERT INTO yt_comments_fts(rowid, text) VALUES (new.rowid, new.text);
	END`,
	`CREATE TABLE IF NOT EXISTS yt_packaging (
		video_id TEXT PRIMARY KEY,
		channel_id TEXT DEFAULT '',
		title TEXT DEFAULT '',
		thumb_url TEXT DEFAULT '',
		thumb_path TEXT DEFAULT '',
		hook_text TEXT DEFAULT '',
		duration_seconds INTEGER DEFAULT 0,
		view_count INTEGER DEFAULT 0,
		captured_at TEXT DEFAULT '',
		hook_error TEXT DEFAULT '',
		thumb_error TEXT DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS yt_monitor_runs (
		run_id INTEGER PRIMARY KEY AUTOINCREMENT,
		started_at TEXT NOT NULL,
		finished_at TEXT DEFAULT '',
		channels INTEGER DEFAULT 0,
		new_videos INTEGER DEFAULT 0,
		video_snapshots INTEGER DEFAULT 0,
		comments_synced INTEGER DEFAULT 0,
		quota_units_est INTEGER DEFAULT 0,
		notes TEXT DEFAULT ''
	)`,
}

// EnsureAnalystSchema creates the competitor-monitoring tables when absent.
// The DDL runs inside BEGIN IMMEDIATE so concurrent processes (e.g. parallel
// sample probes sharing one databank) serialize on SQLite's write lock instead
// of racing table/FTS creation. A fast existence probe skips DDL entirely on
// the common already-initialized path.
func (s *Store) EnsureAnalystSchema(ctx context.Context) error {
	s.lockForWrite()
	defer s.unlockAfterWrite()
	var probe string
	err := s.db.QueryRowContext(ctx,
		`SELECT name FROM sqlite_master WHERE type='table' AND name='yt_monitor_runs'`).Scan(&probe)
	if err == nil && probe == "yt_monitor_runs" {
		return nil
	}
	// Pin one pooled connection so BEGIN IMMEDIATE, the DDL, and COMMIT are
	// guaranteed to share a transaction (the pool would otherwise scatter
	// them across connections, leaking a dangling write lock).
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("analyst schema conn: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("analyst schema lock: %w", err)
	}
	for _, stmt := range analystSchemaStatements {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			_, _ = conn.ExecContext(ctx, `ROLLBACK`)
			return fmt.Errorf("analyst schema: %w", err)
		}
	}
	if _, err := conn.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("analyst schema commit: %w", err)
	}
	return nil
}
