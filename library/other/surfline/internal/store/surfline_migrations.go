// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored extra schema for Surfline novel commands. Kept out of the
// generated migration slice so it survives regeneration: novel commands call
// EnsureSurflineTables lazily before touching these tables.

package store

import "context"

// EnsureSurflineTables creates the journal-snapshot and alert-rule tables if
// they do not exist. Safe to call repeatedly; novel commands invoke it before
// any read or write against these tables.
func (s *Store) EnsureSurflineTables(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS surfline_journal (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			spot_id TEXT NOT NULL,
			spot_name TEXT,
			captured_at INTEGER NOT NULL,
			rating_key TEXT,
			rating_value REAL,
			surf_min REAL,
			surf_max REAL,
			swell_height REAL,
			swell_period REAL,
			wind_speed REAL,
			wind_direction_type TEXT,
			snapshot TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_surfline_journal_spot ON surfline_journal(spot_id, captured_at)`,
		`CREATE TABLE IF NOT EXISTS surfline_alert (
			name TEXT PRIMARY KEY,
			spot_id TEXT NOT NULL,
			min_surf REAL,
			min_period REAL,
			max_wind REAL,
			offshore_only INTEGER NOT NULL DEFAULT 0,
			min_rating REAL,
			created_at INTEGER NOT NULL
		)`,
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	for _, stmt := range stmts {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}
