// Copyright 2026 Victor Wibisono and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// agodaSchema holds the hand-authored tables that back this CLI's price-history
// features. They live outside the generated migration list so a regeneration
// cannot drop them.
//
// price_observations is an append-only time series: every search writes what it
// saw, and the watch command compares the latest observation against the
// trailing median. Storing both price bases keeps historical rows meaningful
// even though only the all-in figure is used for drop detection today.
const agodaSchema = `
CREATE TABLE IF NOT EXISTS price_observations (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    property_id       INTEGER NOT NULL,
    property_name     TEXT    NOT NULL DEFAULT '',
    city_id           INTEGER NOT NULL DEFAULT 0,
    checkin           TEXT    NOT NULL,
    nights            INTEGER NOT NULL DEFAULT 1,
    adults            INTEGER NOT NULL DEFAULT 2,
    rooms             INTEGER NOT NULL DEFAULT 1,
    currency          TEXT    NOT NULL DEFAULT '',
    price_advertised  REAL    NOT NULL DEFAULT 0,
    price_all_in      REAL    NOT NULL DEFAULT 0,
    observed_at       TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_price_obs_lookup
    ON price_observations (property_id, checkin, nights, adults, rooms, currency);

CREATE INDEX IF NOT EXISTS idx_price_obs_observed_at
    ON price_observations (observed_at);

-- properties is the local corpus that the search command queries offline. It is
-- populated as a side effect of every live search, so the corpus grows with use
-- rather than requiring a separate bulk sync the API does not support.
CREATE TABLE IF NOT EXISTS properties (
    property_id   INTEGER PRIMARY KEY,
    name          TEXT    NOT NULL DEFAULT '',
    city_id       INTEGER NOT NULL DEFAULT 0,
    address       TEXT    NOT NULL DEFAULT '',
    star_rating   REAL    NOT NULL DEFAULT 0,
    review_score  REAL    NOT NULL DEFAULT 0,
    review_count  INTEGER NOT NULL DEFAULT 0,
    latitude      REAL    NOT NULL DEFAULT 0,
    longitude     REAL    NOT NULL DEFAULT 0,
    last_seen_at  TEXT    NOT NULL DEFAULT ''
);

CREATE VIRTUAL TABLE IF NOT EXISTS properties_fts USING fts5(
    name, address, content='properties', content_rowid='property_id'
);

CREATE TRIGGER IF NOT EXISTS properties_ai AFTER INSERT ON properties BEGIN
    INSERT INTO properties_fts(rowid, name, address) VALUES (new.property_id, new.name, new.address);
END;

CREATE TRIGGER IF NOT EXISTS properties_au AFTER UPDATE ON properties BEGIN
    INSERT INTO properties_fts(properties_fts, rowid, name, address)
        VALUES('delete', old.property_id, old.name, old.address);
    INSERT INTO properties_fts(rowid, name, address) VALUES (new.property_id, new.name, new.address);
END;

CREATE TABLE IF NOT EXISTS price_watches (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    city_id       INTEGER NOT NULL,
    destination   TEXT    NOT NULL DEFAULT '',
    checkin       TEXT    NOT NULL,
    nights        INTEGER NOT NULL DEFAULT 1,
    adults        INTEGER NOT NULL DEFAULT 2,
    rooms         INTEGER NOT NULL DEFAULT 1,
    currency      TEXT    NOT NULL DEFAULT '',
    created_at    TEXT    NOT NULL,
    UNIQUE (city_id, checkin, nights, adults, rooms, currency)
);
`

// EnsureAgodaSchema creates the hand-authored tables if they do not exist.
//
// It is safe to call on every command invocation: each statement is guarded by
// IF NOT EXISTS.
func EnsureAgodaSchema(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("nil database handle")
	}
	if _, err := db.ExecContext(ctx, agodaSchema); err != nil {
		return fmt.Errorf("creating agoda price-history schema: %w", err)
	}
	return nil
}
