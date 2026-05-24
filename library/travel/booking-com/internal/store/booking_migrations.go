// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// EnsureBookingTables lazily creates the hand-coded Booking.com analysis
// tables used by novel commands. It is intentionally separate from the
// generated store migration so generated store.go can be refreshed unchanged.
func EnsureBookingTables(ctx context.Context, db *sql.DB) error {
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS price_history (
			slug TEXT,
			country TEXT,
			checkin TEXT,
			checkout TEXT,
			group_adults INT,
			currency TEXT,
			price REAL,
			observed_at TEXT,
			PRIMARY KEY(slug, country, checkin, checkout, group_adults, observed_at)
		)`,
		`CREATE TABLE IF NOT EXISTS watches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT,
			country TEXT,
			checkin TEXT,
			checkout TEXT,
			group_adults INT,
			added_at TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_price_history_slug_dates ON price_history(slug, country, checkin, checkout, group_adults, observed_at)`,
		`CREATE INDEX IF NOT EXISTS idx_watches_slug_dates ON watches(slug, country, checkin, checkout, group_adults)`,
	} {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure booking tables: %w", err)
		}
	}
	return nil
}
