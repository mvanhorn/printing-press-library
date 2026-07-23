// Copyright 2026 Kent Martin and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored history tables for the zoho-campaigns novel commands.
// Lazy-init: invoked by the commands that need them, not by the generated
// migration slice, so this file survives regeneration untouched.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

var campaignHistorySchema = []string{
	`CREATE TABLE IF NOT EXISTS campaign_report_snapshots (
		campaign_key  TEXT NOT NULL,
		campaign_name TEXT NOT NULL DEFAULT '',
		taken_at      TEXT NOT NULL,
		emails_sent   INTEGER NOT NULL DEFAULT 0,
		delivered     INTEGER NOT NULL DEFAULT 0,
		opens         INTEGER NOT NULL DEFAULT 0,
		unique_clicks INTEGER NOT NULL DEFAULT 0,
		bounces       INTEGER NOT NULL DEFAULT 0,
		hard_bounces  INTEGER NOT NULL DEFAULT 0,
		soft_bounces  INTEGER NOT NULL DEFAULT 0,
		unsubscribes  INTEGER NOT NULL DEFAULT 0,
		spams         INTEGER NOT NULL DEFAULT 0,
		open_percent  REAL NOT NULL DEFAULT 0,
		click_percent REAL NOT NULL DEFAULT 0,
		sent_time     TEXT NOT NULL DEFAULT '',
		PRIMARY KEY (campaign_key, taken_at)
	)`,
	`CREATE TABLE IF NOT EXISTS list_count_snapshots (
		listkey  TEXT NOT NULL,
		listname TEXT NOT NULL DEFAULT '',
		taken_at TEXT NOT NULL,
		contacts INTEGER NOT NULL DEFAULT 0,
		unsubs   INTEGER NOT NULL DEFAULT 0,
		bounces  INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (listkey, taken_at)
	)`,
	`CREATE TABLE IF NOT EXISTS recipient_actions (
		campaign_key TEXT NOT NULL,
		email        TEXT NOT NULL,
		action       TEXT NOT NULL,
		first_name   TEXT NOT NULL DEFAULT '',
		last_name    TEXT NOT NULL DEFAULT '',
		company      TEXT NOT NULL DEFAULT '',
		fetched_at   TEXT NOT NULL,
		PRIMARY KEY (campaign_key, email, action)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_ra_email ON recipient_actions(email)`,
	`CREATE TABLE IF NOT EXISTS recipient_action_syncs (
		campaign_key TEXT NOT NULL,
		action       TEXT NOT NULL,
		fetched_at   TEXT NOT NULL,
		PRIMARY KEY (campaign_key, action)
	)`,
}

// EnsureCampaignHistoryTables creates the snapshot/history tables used by the
// delta, digest, growth, engagement, bounce-audit, and journey commands.
func EnsureCampaignHistoryTables(ctx context.Context, db *sql.DB) error {
	for _, stmt := range campaignHistorySchema {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("creating campaign history tables: %w", err)
		}
	}
	return nil
}
