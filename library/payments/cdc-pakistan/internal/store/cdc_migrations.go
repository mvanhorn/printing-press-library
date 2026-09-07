// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"fmt"
)

// CDC-specific schema.
//
// This lives in its own file, and runs from a lazy initialiser invoked by the
// commands that need it, rather than being appended to the migrations slice in
// store.go. store.go carries the DO-NOT-EDIT generator header, so an edit there
// is lost on the next `generate --force`; a separate file is preserved.
//
// DESIGN NOTES THAT ARE LOAD-BEARING, not commentary:
//
//   - `documents` stores THREE dates and derives none of them. CDC's listing
//     `meta` text carries no year ("30 December"); the /assets/uploads/YYYY/MM/
//     path is the UPLOAD date, which can differ from the document's own as-of
//     date; and `year_param` keys on the upload date too. Collapsing them would
//     misfile every document uploaded after its own as-of date.
//
//   - `coverage_buckets` is TRI-state, and the third state is the point. A
//     coverage map without persisted negatives cannot distinguish "CDC never
//     published this" from "we never asked", which is the difference between a
//     real gap and an unrun query.
//
//   - `penetration_rows` keeps paid-up and percentage columns NULLABLE because
//     part B (funds) genuinely has no share capital. NULL means "not published
//     for this instrument class"; 0 means "published as zero". CDC writes `-`
//     for the latter.
//
//   - `data_quality_findings` is the shared substrate every verify command
//     writes to. Per-command checks nobody reads are decoration.
var cdcMigrations = []string{
	`CREATE TABLE IF NOT EXISTS cdc_documents (
		url             TEXT PRIMARY KEY,
		title           TEXT NOT NULL,
		category        TEXT NOT NULL,
		meta_day_month  TEXT,
		upload_year     TEXT,
		upload_month    TEXT,
		year_param      INTEGER,
		legacy_path     INTEGER NOT NULL DEFAULT 0,
		as_of_date      TEXT,
		as_of_source    TEXT,
		event_kind      TEXT,
		event_state     TEXT,
		event_action    TEXT,
		event_confidence TEXT,
		first_seen      TEXT NOT NULL,
		last_seen       TEXT NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_cdc_documents_category ON cdc_documents(category)`,
	`CREATE INDEX IF NOT EXISTS idx_cdc_documents_state ON cdc_documents(event_state)`,
	`CREATE INDEX IF NOT EXISTS idx_cdc_documents_asof ON cdc_documents(as_of_date)`,

	`CREATE TABLE IF NOT EXISTS cdc_coverage_buckets (
		category     TEXT NOT NULL,
		year_param   INTEGER NOT NULL,
		paged        INTEGER NOT NULL,
		state        TEXT NOT NULL CHECK (state IN ('found','not-found-at-source','not-probed','transport-error')),
		http_status  INTEGER,
		item_count   INTEGER NOT NULL DEFAULT 0,
		block_hash   TEXT,
		probed_at    TEXT,
		PRIMARY KEY (category, year_param, paged)
	)`,

	`CREATE TABLE IF NOT EXISTS cdc_vintages (
		vintage_date TEXT NOT NULL,
		part         TEXT NOT NULL DEFAULT '',
		url          TEXT NOT NULL,
		layout       TEXT,
		title        TEXT,
		producer     TEXT,
		backend      TEXT,
		pages        INTEGER,
		rows_parsed  INTEGER,
		isins_in_text INTEGER,
		coverage_pct REAL,
		schema_fingerprint TEXT,
		fetched_at   TEXT NOT NULL,
		PRIMARY KEY (vintage_date, part)
	)`,

	`CREATE TABLE IF NOT EXISTS cdc_penetration_rows (
		vintage_date     TEXT NOT NULL,
		part             TEXT NOT NULL DEFAULT '',
		isin             TEXT NOT NULL,
		seq              INTEGER,
		name             TEXT,
		name_markers     TEXT,
		symbol           TEXT,
		live_date        TEXT,
		maturity_date    TEXT,
		status           TEXT,
		row_layout       TEXT,
		shares_in_cds    REAL,
		market_value     REAL,
		paid_up_incl_gop REAL,
		pct_incl_gop     REAL,
		paid_up_excl_gop REAL,
		pct_excl_gop     REAL,
		extract_backend  TEXT,
		PRIMARY KEY (vintage_date, part, isin)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_cdc_pen_isin ON cdc_penetration_rows(isin)`,
	`CREATE INDEX IF NOT EXISTS idx_cdc_pen_symbol ON cdc_penetration_rows(symbol)`,

	`CREATE TABLE IF NOT EXISTS cdc_stats_snapshots (
		as_of        TEXT NOT NULL,
		metric_key   TEXT NOT NULL,
		label        TEXT NOT NULL,
		raw          TEXT NOT NULL,
		known        INTEGER NOT NULL DEFAULT 1,
		provenance   TEXT NOT NULL CHECK (provenance IN ('live','archive')),
		first_seen   TEXT NOT NULL,
		last_seen    TEXT NOT NULL,
		PRIMARY KEY (as_of, metric_key, label)
	)`,

	`CREATE TABLE IF NOT EXISTS cdc_identity_events (
		isin         TEXT,
		symbol_from  TEXT,
		symbol_to    TEXT,
		name_from    TEXT,
		name_to      TEXT,
		effective_date TEXT,
		source_url   TEXT NOT NULL,
		source_title TEXT,
		confidence   TEXT,
		PRIMARY KEY (source_url, isin, symbol_from, symbol_to)
	)`,

	`CREATE TABLE IF NOT EXISTS cdc_data_quality_findings (
		surface     TEXT NOT NULL,
		vintage     TEXT NOT NULL DEFAULT '',
		subject     TEXT NOT NULL DEFAULT '',
		check_name  TEXT NOT NULL,
		severity    TEXT NOT NULL CHECK (severity IN ('error','warning','info')),
		detail      TEXT NOT NULL,
		acknowledged INTEGER NOT NULL DEFAULT 0,
		found_at    TEXT NOT NULL,
		PRIMARY KEY (surface, vintage, subject, check_name)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_cdc_findings_sev ON cdc_data_quality_findings(severity, acknowledged)`,
}

// EnsureCDCSchema creates the CDC tables if absent. It is idempotent and safe to
// call from every command's RunE; CREATE TABLE IF NOT EXISTS is a no-op once the
// table exists.
func EnsureCDCSchema(ctx context.Context, s *Store) error {
	if s == nil {
		return fmt.Errorf("store is nil")
	}
	for _, stmt := range cdcMigrations {
		if _, err := s.DB().ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("cdc schema: %w", err)
		}
	}
	return nil
}

// UnacknowledgedFindings counts error-severity findings that no operator has
// acknowledged. `--strict` emit paths refuse a vintage carrying any, which is
// what stops a known-bad extraction reaching a regression by accident.
func UnacknowledgedFindings(ctx context.Context, s *Store, vintage string) (int, error) {
	const q = `SELECT COUNT(*) FROM cdc_data_quality_findings
	           WHERE severity = 'error' AND acknowledged = 0
	             AND ($1 = '' OR vintage = $1)`
	var n int
	if err := s.DB().QueryRowContext(ctx, q, vintage).Scan(&n); err != nil {
		return 0, fmt.Errorf("counting findings: %w", err)
	}
	return n, nil
}
