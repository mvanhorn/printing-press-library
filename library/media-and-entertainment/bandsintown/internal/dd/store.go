// Package dd implements the Double-Deer-style routing/lineup/trend tables and
// queries that the novel CLI commands operate over. The generated `internal/store`
// package gives us a generic key-value `resources` table; this package adds
// typed tables for joins (events × venues × tracked artists, lineup junction,
// historical tracker snapshots) that the framework store can't express.
//
// The schema piggybacks on the same SQLite file the framework uses, so users
// run `bandsintown-pp-cli sync` once and both surfaces are populated by the
// `pull` subcommand below.
package dd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Schema is the DDL applied at Open time. Every statement is idempotent
// (CREATE ... IF NOT EXISTS) so reopens are safe.
var Schema = []string{
	`CREATE TABLE IF NOT EXISTS dd_tracked_artists (
		name        TEXT PRIMARY KEY,
		mbid        TEXT,
		tier        TEXT DEFAULT '',
		added_at    TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS dd_artists (
		name                  TEXT PRIMARY KEY,
		bit_id                TEXT,
		mbid                  TEXT,
		url                   TEXT,
		image_url             TEXT,
		thumb_url             TEXT,
		facebook_page_url     TEXT,
		tracker_count         INTEGER DEFAULT 0,
		upcoming_event_count  INTEGER DEFAULT 0,
		fetched_at            TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE TABLE IF NOT EXISTS dd_events (
		id            TEXT PRIMARY KEY,
		artist_name   TEXT NOT NULL,
		datetime      TEXT NOT NULL,
		description   TEXT,
		url           TEXT,
		on_sale_at    TEXT,
		venue_name    TEXT,
		venue_city    TEXT,
		venue_region  TEXT,
		venue_country TEXT,
		venue_lat     REAL,
		venue_lng     REAL,
		fetched_at    TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_dd_events_artist ON dd_events(artist_name)`,
	`CREATE INDEX IF NOT EXISTS idx_dd_events_datetime ON dd_events(datetime)`,
	`CREATE INDEX IF NOT EXISTS idx_dd_events_country ON dd_events(venue_country)`,
	`CREATE TABLE IF NOT EXISTS dd_lineup_members (
		event_id    TEXT NOT NULL,
		artist_name TEXT NOT NULL,
		PRIMARY KEY (event_id, artist_name)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_dd_lineup_artist ON dd_lineup_members(artist_name)`,
	`CREATE TABLE IF NOT EXISTS dd_offers (
		event_id TEXT NOT NULL,
		type     TEXT,
		status   TEXT,
		url      TEXT,
		PRIMARY KEY (event_id, type, url)
	)`,
	`CREATE TABLE IF NOT EXISTS dd_artist_snapshots (
		artist_name          TEXT NOT NULL,
		snapped_at           TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
		tracker_count        INTEGER DEFAULT 0,
		upcoming_event_count INTEGER DEFAULT 0,
		PRIMARY KEY (artist_name, snapped_at)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_dd_snapshots_at ON dd_artist_snapshots(snapped_at)`,
}

// Open opens (and lazily creates) the SQLite database at dbPath and applies
// the schema. The same file is shared with the framework store.
func Open(ctx context.Context, dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		return nil, fmt.Errorf("dbPath required")
	}
	// SQLite returns "unable to open database file: out of memory (14)" when
	// the parent directory doesn't exist — create it eagerly before opening.
	if dir := filepath.Dir(dbPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	// modernc.org/sqlite is the driver the framework uses; piggyback on it.
	// Match the framework store's pragmas so dd writes share the same WAL mode
	// and 5-second busy-retry budget — without these, dd writes hit SQLITE_BUSY
	// immediately on lock contention while the framework store retries.
	db, err := sql.Open("sqlite", dbPath+"?_journal_mode=WAL&_synchronous=NORMAL&_busy_timeout=5000&_foreign_keys=ON&_temp_store=MEMORY&_mmap_size=268435456")
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dbPath, err)
	}
	for _, stmt := range Schema {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("schema: %s: %w", filepath.Base(dbPath), err)
		}
	}
	return db, nil
}
