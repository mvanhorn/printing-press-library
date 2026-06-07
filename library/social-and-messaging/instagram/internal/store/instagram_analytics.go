// Copyright 2026 Mohammed Al Khamis and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// analyticsSchemaStmts are the idempotent CREATE statements backing the
// multi-brand snapshot layer. Each runs under CREATE TABLE/INDEX IF NOT
// EXISTS so EnsureAnalyticsSchema is safe to call on every command.
var analyticsSchemaStmts = []string{
	`CREATE TABLE IF NOT EXISTS ig_brands (
		slug TEXT PRIMARY KEY,
		ig_user_id TEXT NOT NULL,
		name TEXT,
		username TEXT,
		added_at TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS ig_account_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT,
		ig_user_id TEXT,
		followers_count INTEGER,
		follows_count INTEGER,
		media_count INTEGER,
		reach INTEGER,
		total_interactions INTEGER,
		accounts_engaged INTEGER,
		views INTEGER,
		captured_at TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ig_account_snapshots_slug_captured
		ON ig_account_snapshots (slug, captured_at)`,
	`CREATE TABLE IF NOT EXISTS ig_brand_media (
		slug TEXT,
		ig_user_id TEXT,
		media_id TEXT,
		caption TEXT,
		media_type TEXT,
		media_product_type TEXT,
		permalink TEXT,
		posted_at TEXT,
		like_count INTEGER,
		comments_count INTEGER,
		reach INTEGER,
		views INTEGER,
		saved INTEGER,
		shares INTEGER,
		total_interactions INTEGER,
		reels_avg_watch_time REAL,
		captured_at TEXT,
		PRIMARY KEY (slug, media_id)
	)`,
	`CREATE TABLE IF NOT EXISTS ig_tracked_competitors (
		owner_slug TEXT,
		username TEXT,
		PRIMARY KEY (owner_slug, username)
	)`,
	`CREATE TABLE IF NOT EXISTS ig_competitor_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_slug TEXT,
		username TEXT,
		followers_count INTEGER,
		media_count INTEGER,
		recent_avg_engagement REAL,
		captured_at TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ig_competitor_snapshots_owner_user_captured
		ON ig_competitor_snapshots (owner_slug, username, captured_at)`,
	`CREATE TABLE IF NOT EXISTS ig_tracked_hashtags (
		slug TEXT,
		hashtag TEXT,
		PRIMARY KEY (slug, hashtag)
	)`,
	`CREATE TABLE IF NOT EXISTS ig_hashtag_snapshots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT,
		hashtag TEXT,
		hashtag_id TEXT,
		top_media_reach INTEGER,
		top_media_engagement INTEGER,
		top_media_count INTEGER,
		captured_at TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ig_hashtag_snapshots_slug_hashtag_captured
		ON ig_hashtag_snapshots (slug, hashtag, captured_at)`,
}

// EnsureAnalyticsSchema creates the multi-brand snapshot tables and indexes
// if they do not already exist. It is idempotent and safe to call on every
// invocation of a brands/pull/analytics command.
func EnsureAnalyticsSchema(ctx context.Context, db *sql.DB) error {
	for _, stmt := range analyticsSchemaStmts {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensuring analytics schema: %w", err)
		}
	}
	return nil
}

// EnsureAnalyticsSchema is the Store-method wrapper around the package-level
// EnsureAnalyticsSchema so callers holding a *Store don't reach for DB().
func (s *Store) EnsureAnalyticsSchema(ctx context.Context) error {
	return EnsureAnalyticsSchema(ctx, s.DB())
}
