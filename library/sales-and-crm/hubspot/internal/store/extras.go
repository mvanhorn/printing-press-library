// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

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
		// contact_score_snapshots: populated lazily by the score-drift and
		// daily-digest analytics commands so they can compute deltas across
		// invocations without depending on HubSpot history APIs.
		`CREATE TABLE IF NOT EXISTS contact_score_snapshots (
			contact_id TEXT NOT NULL,
			snapshot_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			hubspotscore REAL,
			lifecyclestage TEXT,
			hs_lead_status TEXT,
			hubspot_owner_id TEXT,
			PRIMARY KEY (contact_id, snapshot_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_contact_score_snapshots_at ON contact_score_snapshots(snapshot_at)`,
		// contact_engagement_snapshots: populated lazily by daily-digest so
		// engagement deltas (opens/clicks/last-activity) are observable across
		// runs even when HubSpot's properties endpoint only returns the
		// current value.
		`CREATE TABLE IF NOT EXISTS contact_engagement_snapshots (
			contact_id TEXT NOT NULL,
			snapshot_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			hs_email_open INTEGER,
			hs_email_click INTEGER,
			hs_last_activity_date TEXT,
			PRIMARY KEY (contact_id, snapshot_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_contact_engagement_snapshots_at ON contact_engagement_snapshots(snapshot_at)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
