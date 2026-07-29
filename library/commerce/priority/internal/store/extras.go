// Copyright 2026 alon-auto and contributors. Licensed under Apache-2.0. See LICENSE.

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
		// Parsed $metadata cache: one row per form / field / subform, keyed by
		// tenant (normalized base URL) so multiple Priority instances can share
		// one local store without colliding.
		`CREATE TABLE IF NOT EXISTS pp_meta_forms (
			tenant TEXT NOT NULL,
			form TEXT NOT NULL,
			PRIMARY KEY (tenant, form)
		)`,
		`CREATE TABLE IF NOT EXISTS pp_meta_fields (
			tenant TEXT NOT NULL,
			form TEXT NOT NULL,
			field TEXT NOT NULL,
			type TEXT,
			mandatory INTEGER NOT NULL DEFAULT 0,
			description TEXT,
			PRIMARY KEY (tenant, form, field)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_pp_meta_fields_field ON pp_meta_fields(tenant, field)`,
		`CREATE TABLE IF NOT EXISTS pp_meta_subforms (
			tenant TEXT NOT NULL,
			form TEXT NOT NULL,
			subform TEXT NOT NULL,
			target TEXT,
			collection INTEGER NOT NULL DEFAULT 1,
			PRIMARY KEY (tenant, form, subform)
		)`,
		`CREATE TABLE IF NOT EXISTS pp_meta_state (
			tenant TEXT PRIMARY KEY,
			refreshed_at DATETIME,
			form_count INTEGER,
			field_count INTEGER
		)`,
		// Named schema snapshots for forms diff.
		`CREATE TABLE IF NOT EXISTS pp_schema_snapshots (
			tenant TEXT NOT NULL,
			name TEXT NOT NULL,
			taken_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			data JSON NOT NULL,
			PRIMARY KEY (tenant, name)
		)`,
		// Per-tenant API-licensing probe verdicts for forms licensed.
		`CREATE TABLE IF NOT EXISTS pp_license_verdicts (
			tenant TEXT NOT NULL,
			form TEXT NOT NULL,
			verdict TEXT NOT NULL,
			http_status INTEGER,
			message TEXT,
			checked_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (tenant, form)
		)`,
		// $batch journals for batch load / batch resume.
		`CREATE TABLE IF NOT EXISTS pp_batch_journal (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			source TEXT,
			total INTEGER NOT NULL DEFAULT 0,
			succeeded INTEGER NOT NULL DEFAULT 0,
			failed INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS pp_batch_ops (
			journal_id INTEGER NOT NULL,
			op_index INTEGER NOT NULL,
			op_id TEXT,
			method TEXT,
			url TEXT,
			body JSON,
			headers JSON,
			depends_on TEXT,
			atomicity_group TEXT,
			status INTEGER,
			response JSON,
			error TEXT,
			PRIMARY KEY (journal_id, op_index)
		)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("extra migration failed: %w", err)
		}
	}
	return nil
}
