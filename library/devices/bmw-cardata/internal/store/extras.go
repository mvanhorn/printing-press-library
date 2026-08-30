// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// migrateExtras runs after the generated store migrations and before the
// schema-version stamp. BMW CarData transcendence features need append-only
// time-series tables (the generic `resources` table only keeps the latest
// row per id, which cannot power SoC trends, trip reconstruction, or
// snapshot diffs).
//
// All statements are idempotent so every store open can safely re-run them.
func (s *Store) migrateExtras(ctx context.Context, conn *sql.Conn) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS cardata_vehicles (
			vin                       TEXT PRIMARY KEY,
			brand                     TEXT,
			model_range               TEXT,
			series                    TEXT,
			model_name                TEXT,
			propulsion_type           TEXT,
			drive_train               TEXT,
			engine                    TEXT,
			hvs_max_energy_absolute   TEXT,
			construction_date         TEXT,
			sim_status                TEXT,
			is_telematics_capable     INTEGER,
			raw                       TEXT
		)`,
		// Append-only telematic time-series: one row per (vin, descriptor, ts).
		`CREATE TABLE IF NOT EXISTS cardata_telematic_snapshots (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			vin        TEXT NOT NULL,
			descriptor TEXT NOT NULL,
			value      TEXT,
			unit       TEXT,
			ts         TEXT,
			fetched_at TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cardata_telematic_vin_desc_ts
			ON cardata_telematic_snapshots(vin, descriptor, ts)`,
		// Dedupe timestamped replays by their BMW timestamp. Missing timestamps
		// use fetched_at so each fresh fetch remains a distinct observation.
		// NULLIF also normalizes empty timestamps written by older CLI builds.
		`DROP INDEX IF EXISTS ux_cardata_telematic_vin_desc_ts`,
		`DROP INDEX IF EXISTS ux_cardata_telematic_vin_desc_ts_or_fetched`,
		`CREATE UNIQUE INDEX IF NOT EXISTS ux_cardata_telematic_vin_desc_ts_or_fetched_v2
			ON cardata_telematic_snapshots(vin, descriptor, COALESCE(NULLIF(ts, ''), fetched_at))`,
		// Charging sessions keyed by (vin, start_time).
		`CREATE TABLE IF NOT EXISTS cardata_charging_sessions (
			vin         TEXT NOT NULL,
			start_time  INTEGER NOT NULL,
			data        TEXT NOT NULL,
			fetched_at  TEXT NOT NULL,
			PRIMARY KEY (vin, start_time)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cardata_charging_vin_start
			ON cardata_charging_sessions(vin, start_time)`,
		// VSS descriptor catalogue (static reference, seeded on first use).
		`CREATE TABLE IF NOT EXISTS cardata_descriptor_catalogue (
			descriptor TEXT PRIMARY KEY,
			unit       TEXT,
			domain     TEXT,
			description TEXT
		)`,
		// Daily API-call counter for the quota tracker (~50 calls/day cap).
		`CREATE TABLE IF NOT EXISTS cardata_api_calls (
			day   TEXT PRIMARY KEY,
			count INTEGER NOT NULL DEFAULT 0
		)`,
	}
	for _, m := range migrations {
		if _, err := conn.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("cardata extra migration failed (%s): %w", firstLine(m), err)
		}
	}
	return nil
}

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
