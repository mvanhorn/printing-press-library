package store

import (
	"context"
	"fmt"
	"time"
)

var youtubeExtrasMigrations = []string{
	`CREATE TABLE IF NOT EXISTS yt_transcripts (
		video_id TEXT NOT NULL,
		language TEXT NOT NULL,
		format TEXT NOT NULL DEFAULT 'vtt',
		auto_generated INTEGER NOT NULL DEFAULT 0,
		content TEXT NOT NULL,
		fetched_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (video_id, language)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_yt_transcripts_fetched ON yt_transcripts(fetched_at)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS yt_transcripts_fts USING fts5(
		video_id UNINDEXED, language UNINDEXED, content,
		tokenize='porter unicode61'
	)`,
	`CREATE TABLE IF NOT EXISTS yt_trending_snapshots (
		region TEXT NOT NULL,
		category_id TEXT NOT NULL DEFAULT '0',
		captured_at DATETIME NOT NULL,
		position INTEGER NOT NULL,
		video_id TEXT NOT NULL,
		title TEXT,
		channel_id TEXT,
		channel_title TEXT,
		view_count INTEGER,
		PRIMARY KEY (region, category_id, captured_at, position)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_yt_trending_video ON yt_trending_snapshots(video_id)`,
	`CREATE INDEX IF NOT EXISTS idx_yt_trending_captured ON yt_trending_snapshots(captured_at)`,
	`CREATE TABLE IF NOT EXISTS yt_quota_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		ts DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		api_key_hash TEXT NOT NULL,
		command TEXT NOT NULL,
		endpoint TEXT NOT NULL,
		units INTEGER NOT NULL,
		response_status INTEGER,
		notes TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_yt_quota_ts ON yt_quota_log(ts)`,
	`CREATE INDEX IF NOT EXISTS idx_yt_quota_key ON yt_quota_log(api_key_hash, ts)`,
	`CREATE TABLE IF NOT EXISTS yt_video_snapshots (
		video_id TEXT NOT NULL,
		captured_at DATETIME NOT NULL,
		channel_id TEXT,
		title TEXT,
		published_at DATETIME,
		view_count INTEGER,
		like_count INTEGER,
		comment_count INTEGER,
		duration_seconds INTEGER,
		PRIMARY KEY (video_id, captured_at)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_yt_video_snapshots_channel ON yt_video_snapshots(channel_id, captured_at)`,
	`CREATE TABLE IF NOT EXISTS yt_channels (
		channel_id TEXT PRIMARY KEY,
		title TEXT,
		description TEXT,
		uploads_playlist_id TEXT,
		subscriber_count INTEGER,
		view_count INTEGER,
		video_count INTEGER,
		published_at DATETIME,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS yt_videos (
		video_id TEXT PRIMARY KEY,
		channel_id TEXT,
		title TEXT,
		description TEXT,
		published_at DATETIME,
		duration_seconds INTEGER,
		view_count INTEGER,
		like_count INTEGER,
		comment_count INTEGER,
		tags TEXT,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_yt_videos_channel ON yt_videos(channel_id, published_at)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS yt_videos_fts USING fts5(
		video_id UNINDEXED, channel_id UNINDEXED, title, description, tags,
		tokenize='porter unicode61'
	)`,
	`CREATE TABLE IF NOT EXISTS yt_comments (
		comment_id TEXT PRIMARY KEY,
		video_id TEXT,
		author_channel_id TEXT,
		author_display_name TEXT,
		text TEXT,
		like_count INTEGER,
		published_at DATETIME,
		parent_id TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_yt_comments_video ON yt_comments(video_id)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS yt_comments_fts USING fts5(
		comment_id UNINDEXED, video_id UNINDEXED, author_display_name, text,
		tokenize='porter unicode61'
	)`,
}

// EnsureYouTubeExtras creates all YouTube-specific tables if they don't already
// exist. Idempotent — safe to call from every transcendence command.
func (s *Store) EnsureYouTubeExtras(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	for _, m := range youtubeExtrasMigrations {
		if _, err := s.db.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("ensure youtube extras: %w", err)
		}
	}
	return nil
}

// LogQuota inserts a row into yt_quota_log. Pass an empty apiKeyHash for
// anonymous unauthenticated calls (rare on YouTube — most endpoints require
// a key). units is the documented quota cost for the endpoint.
func (s *Store) LogQuota(ctx context.Context, apiKeyHash, command, endpoint string, units int, status int, notes string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO yt_quota_log(ts, api_key_hash, command, endpoint, units, response_status, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		time.Now().UTC().Format(time.RFC3339), apiKeyHash, command, endpoint, units, status, notes)
	return err
}
