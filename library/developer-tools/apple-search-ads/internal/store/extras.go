// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.

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
		// reporting_cache: local store for Apple Search Ads campaign-level metrics.
		// Populated by `analytics sync-cache`, queried by `analytics query`.
		`CREATE TABLE IF NOT EXISTS reporting_cache (
			date          TEXT NOT NULL,
			campaign_id   TEXT NOT NULL,
			campaign_name TEXT NOT NULL DEFAULT '',
			impressions   INTEGER NOT NULL DEFAULT 0,
			taps          INTEGER NOT NULL DEFAULT 0,
			installs      INTEGER NOT NULL DEFAULT 0,
			spend         REAL NOT NULL DEFAULT 0.0,
			cpa           REAL NOT NULL DEFAULT 0.0,
			ttr           REAL NOT NULL DEFAULT 0.0,
			granularity   TEXT NOT NULL DEFAULT 'daily',
			synced_at     TEXT NOT NULL DEFAULT (datetime('now')),
			PRIMARY KEY (date, campaign_id, granularity)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_reporting_cache_date        ON reporting_cache(date)`,
		`CREATE INDEX IF NOT EXISTS idx_reporting_cache_campaign_id ON reporting_cache(campaign_id)`,
		`CREATE INDEX IF NOT EXISTS idx_reporting_cache_cpa         ON reporting_cache(cpa)`,
		// campaign_templates: saved campaign structure templates for cross-org apply.
		`CREATE TABLE IF NOT EXISTS campaign_templates (
			name       TEXT PRIMARY KEY,
			payload    TEXT NOT NULL,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now'))
		)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
