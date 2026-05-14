package ytstore

import (
	"context"
	"database/sql"
	"fmt"
)

// EnsureSchema creates the YouTube-specific typed tables on top of the
// generic Printing Press resources table. Idempotent.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS yt_channels (
			channel_id TEXT PRIMARY KEY,
			handle     TEXT,
			title      TEXT,
			kind       TEXT NOT NULL DEFAULT 'own',
			added_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_synced_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_yt_channels_kind ON yt_channels(kind)`,

		`CREATE TABLE IF NOT EXISTS yt_videos (
			video_id     TEXT PRIMARY KEY,
			channel_id   TEXT NOT NULL,
			title        TEXT,
			description  TEXT,
			published_at TEXT,
			duration_s   INTEGER,
			category_id  TEXT,
			tags         TEXT,
			synced_at    DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_yt_videos_channel ON yt_videos(channel_id)`,
		`CREATE INDEX IF NOT EXISTS idx_yt_videos_published ON yt_videos(published_at)`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS yt_videos_fts USING fts5(
			title, description, content='yt_videos', content_rowid='rowid'
		)`,

		`CREATE TABLE IF NOT EXISTS yt_video_metrics_daily (
			video_id   TEXT NOT NULL,
			day        TEXT NOT NULL,
			views      INTEGER,
			likes      INTEGER,
			comments   INTEGER,
			watch_time_s INTEGER,
			ctr        REAL,
			avg_view_pct REAL,
			impressions INTEGER,
			PRIMARY KEY (video_id, day)
		)`,

		`CREATE TABLE IF NOT EXISTS yt_retention_curves (
			video_id    TEXT NOT NULL,
			recorded_at DATETIME NOT NULL,
			points      TEXT NOT NULL,
			PRIMARY KEY (video_id, recorded_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_yt_retention_video ON yt_retention_curves(video_id, recorded_at DESC)`,

		`CREATE TABLE IF NOT EXISTS yt_demographics (
			video_id      TEXT NOT NULL,
			recorded_at   DATETIME NOT NULL,
			segment_key   TEXT NOT NULL,
			segment_value TEXT NOT NULL,
			watch_pct     REAL,
			PRIMARY KEY (video_id, recorded_at, segment_key, segment_value)
		)`,

		`CREATE TABLE IF NOT EXISTS yt_thumbnail_impressions (
			video_id     TEXT NOT NULL,
			thumb_variant TEXT NOT NULL DEFAULT 'default',
			day          TEXT NOT NULL,
			impressions  INTEGER,
			ctr          REAL,
			PRIMARY KEY (video_id, thumb_variant, day)
		)`,

		`CREATE TABLE IF NOT EXISTS yt_search_idea_gap (
			competitor_channel_id TEXT NOT NULL,
			video_id              TEXT NOT NULL,
			topic_signal          TEXT,
			title                 TEXT,
			seen_at               DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (competitor_channel_id, video_id)
		)`,

		`CREATE TABLE IF NOT EXISTS yt_script_videos (
			video_id           TEXT PRIMARY KEY,
			script_path        TEXT NOT NULL,
			signal_line        TEXT,
			belief_shift_line  TEXT,
			cta_line           TEXT,
			linked_at          DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS yt_watchlist (
			channel_id TEXT PRIMARY KEY,
			handle     TEXT,
			title      TEXT,
			niche      TEXT,
			added_at   DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		`CREATE TABLE IF NOT EXISTS yt_quota_log (
			day          TEXT NOT NULL,
			endpoint     TEXT NOT NULL,
			units_used   INTEGER NOT NULL,
			recorded_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (day, endpoint, recorded_at)
		)`,
	}

	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("yt schema migration: %w (stmt: %s)", err, s[:min(60, len(s))])
		}
	}
	// FTS triggers to keep yt_videos_fts in sync with yt_videos
	triggers := []string{
		`CREATE TRIGGER IF NOT EXISTS yt_videos_ai AFTER INSERT ON yt_videos BEGIN
			INSERT INTO yt_videos_fts(rowid, title, description) VALUES (new.rowid, new.title, new.description);
		END`,
		`CREATE TRIGGER IF NOT EXISTS yt_videos_ad AFTER DELETE ON yt_videos BEGIN
			INSERT INTO yt_videos_fts(yt_videos_fts, rowid, title, description) VALUES ('delete', old.rowid, old.title, old.description);
		END`,
		`CREATE TRIGGER IF NOT EXISTS yt_videos_au AFTER UPDATE ON yt_videos BEGIN
			INSERT INTO yt_videos_fts(yt_videos_fts, rowid, title, description) VALUES ('delete', old.rowid, old.title, old.description);
			INSERT INTO yt_videos_fts(rowid, title, description) VALUES (new.rowid, new.title, new.description);
		END`,
	}
	for _, t := range triggers {
		if _, err := db.ExecContext(ctx, t); err != nil {
			return fmt.Errorf("yt fts trigger: %w", err)
		}
	}
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
