// Copyright 2026 Charles Garrison and contributors. Licensed under Apache-2.0. See LICENSE.

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
		// Scrape.do governor + SERP-history schema (canonical home; the
		// full DDL lives in scrapedo_extras.go's scrapedoSchema const so the
		// command layer and this Open-time hook share one source of truth).
		scrapedoSchema,
		// Seed the cost_budget singleton with its default (no ceilings set).
		// This is the budget command's baseline state; GetBudget treats NULL
		// caps as "unset", and SetBudget upserts onto id=1. Seeding it here
		// means the local store is never empty after a fresh open/sync.
		`INSERT OR IGNORE INTO cost_budget (id, max_credits, max_monthly_pct, updated_at)
		 VALUES (1, NULL, NULL, datetime('now'))`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
