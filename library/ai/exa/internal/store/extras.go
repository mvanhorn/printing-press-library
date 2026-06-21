// Copyright 2026 Anchal Sharma and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateExtras runs after the generated store migrations and before the
// schema-version stamp. Adds tables for novel transcendence features.
// All migrations are idempotent via CREATE TABLE IF NOT EXISTS.
func (s *Store) migrateExtras(ctx context.Context, conn *sql.Conn) error {
	migrations := []string{
		// budget_config: one-row table for daily spend cap.
		`CREATE TABLE IF NOT EXISTS budget_config (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			daily_limit_cents INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// spend_log: per-API-call credit usage for budget tracking.
		`CREATE TABLE IF NOT EXISTS spend_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			command TEXT NOT NULL,
			endpoint TEXT NOT NULL,
			estimated_cents INTEGER NOT NULL DEFAULT 0,
			logged_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_spend_log_date ON spend_log(date(logged_at))`,
		// query_templates: stored search templates with variable interpolation.
		`CREATE TABLE IF NOT EXISTS query_templates (
			name TEXT PRIMARY KEY,
			query_template TEXT NOT NULL,
			category TEXT,
			type TEXT,
			include_domains TEXT,
			exclude_domains TEXT,
			num_results INTEGER DEFAULT 10,
			variables TEXT,
			last_run_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		// monitor_delivery_log: log each monitor fire for digest roll-ups.
		`CREATE TABLE IF NOT EXISTS monitor_delivery_log (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			monitor_id TEXT NOT NULL,
			monitor_name TEXT,
			results_count INTEGER DEFAULT 0,
			result_urls TEXT,
			fired_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_monitor_delivery_monitor_id ON monitor_delivery_log(monitor_id)`,
		`CREATE INDEX IF NOT EXISTS idx_monitor_delivery_fired_at ON monitor_delivery_log(date(fired_at))`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
