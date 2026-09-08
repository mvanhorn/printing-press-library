// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored PBS panel schema. Kept in its own file, and applied by a lazy
// init the reading commands call, so that `generate --force` preserves it: the
// migration slice in store.go is generator-owned and edits there are lost on
// the next regeneration.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

var pbsSchemaOnce sync.Once
var pbsSchemaErr error

// pbsMigrations is the typed panel schema.
//
// The generic `resources` table is unsuitable for this data: the panel is
// ~128,000 narrow numeric observations per full backfill, and every query the
// CLI exposes filters on (as_of, city, item, stat). Storing those as JSON blobs
// would force a full scan plus a JSON extract for every question.
//
// Three design points are load-bearing:
//
//  1. `value` is NULLABLE and `value_state` is NOT NULL. A missing price is
//     recorded as NULL with its state, never as 0. PBS writes a numeric zero for
//     an uncollected price, and averaging those as real prices moves a national
//     figure by -16.6% on a measured item.
//  2. `item_desc` is the join key, not the source's `sr`/item number. The
//     report's Sr restarts inside each of three sections and the sections are
//     re-ranked weekly; the annexure's item numbering shifts when the basket
//     changes.
//  3. `coverage` records not-fetched, fetched, empty and error as DISTINCT
//     states. Collapsing them is how a coverage map lies about a short series.
var pbsMigrations = []string{
	`CREATE TABLE IF NOT EXISTS pbs_release (
		as_of            TEXT NOT NULL,
		kind             TEXT NOT NULL,
		as_of_raw        TEXT,
		index_pos        INTEGER,
		filename_mismatch INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (as_of, kind)
	)`,
	`CREATE TABLE IF NOT EXISTS pbs_release_file (
		as_of     TEXT NOT NULL,
		kind      TEXT NOT NULL,
		role      TEXT NOT NULL,
		url       TEXT NOT NULL,
		filename  TEXT NOT NULL,
		ext       TEXT,
		index_key TEXT,
		PRIMARY KEY (as_of, kind, url)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_pbs_file_role ON pbs_release_file(role, ext)`,

	`CREATE TABLE IF NOT EXISTS pbs_price (
		as_of       TEXT NOT NULL,
		surface     TEXT NOT NULL,
		city        TEXT NOT NULL,
		city_code   TEXT,
		item_desc   TEXT NOT NULL,
		item_no     INTEGER,
		unit        TEXT,
		stat        TEXT NOT NULL,
		value       REAL,
		value_state TEXT NOT NULL,
		block       INTEGER,
		desc_suspect INTEGER NOT NULL DEFAULT 0,
		source      TEXT NOT NULL,
		PRIMARY KEY (as_of, surface, city, item_desc, stat)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_pbs_price_item ON pbs_price(item_desc, as_of)`,
	`CREATE INDEX IF NOT EXISTS idx_pbs_price_city ON pbs_price(city, as_of)`,
	`CREATE INDEX IF NOT EXISTS idx_pbs_price_asof ON pbs_price(as_of)`,

	`CREATE TABLE IF NOT EXISTS pbs_national (
		as_of      TEXT NOT NULL,
		item_desc  TEXT NOT NULL,
		series     TEXT NOT NULL,
		value      REAL,
		value_state TEXT NOT NULL,
		PRIMARY KEY (as_of, item_desc, series)
	)`,

	`CREATE TABLE IF NOT EXISTS pbs_weight (
		as_of            TEXT NOT NULL,
		item_desc        TEXT NOT NULL,
		section          TEXT,
		sr               INTEGER,
		unit             TEXT,
		national_price   REAL,
		price_prev_week  REAL,
		price_cor_week   REAL,
		pct_prev_week    REAL,
		pct_cor_week     REAL,
		weight_lowest    REAL,
		weight_combined  REAL,
		impact_lowest    REAL,
		impact_combined  REAL,
		PRIMARY KEY (as_of, item_desc)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_pbs_weight_item ON pbs_weight(item_desc, as_of)`,

	`CREATE TABLE IF NOT EXISTS pbs_weight_total (
		as_of            TEXT NOT NULL,
		section          TEXT NOT NULL,
		declared_count   INTEGER,
		observed_count   INTEGER,
		weight_lowest    REAL,
		weight_combined  REAL,
		impact_lowest    REAL,
		impact_combined  REAL,
		PRIMARY KEY (as_of, section)
	)`,

	`CREATE TABLE IF NOT EXISTS pbs_index (
		as_of      TEXT NOT NULL,
		quintile   TEXT NOT NULL,
		band_raw   TEXT,
		band_low   REAL,
		band_high  REAL,
		idx_value  REAL,
		prev_week  REAL,
		cor_week   REAL,
		pct_prev   REAL,
		pct_cor    REAL,
		PRIMARY KEY (as_of, quintile)
	)`,

	`CREATE TABLE IF NOT EXISTS pbs_coverage (
		as_of        TEXT NOT NULL,
		kind         TEXT NOT NULL,
		role         TEXT NOT NULL,
		url          TEXT,
		state        TEXT NOT NULL,
		http_status  INTEGER,
		sha256       TEXT,
		bytes        INTEGER,
		rows_parsed  INTEGER,
		present_cells INTEGER,
		zero_cells   INTEGER,
		blank_cells  INTEGER,
		na_cells     INTEGER,
		unparseable_cells INTEGER,
		parser       TEXT,
		note         TEXT,
		fetched_at   TEXT,
		PRIMARY KEY (as_of, kind, role)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_pbs_coverage_state ON pbs_coverage(state)`,
}

// EnsurePBSSchema applies the panel schema. Safe to call repeatedly and from
// several commands; the work happens once per process.
func EnsurePBSSchema(ctx context.Context, db *sql.DB) error {
	pbsSchemaOnce.Do(func() {
		for _, stmt := range pbsMigrations {
			if _, err := db.ExecContext(ctx, stmt); err != nil {
				pbsSchemaErr = fmt.Errorf("pbs schema: %w", err)
				return
			}
		}
	})
	return pbsSchemaErr
}
