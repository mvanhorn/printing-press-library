// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package redfin

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// EnsureExtSchema runs the redfin-specific schema migrations idempotently.
// Safe to call on every Open; CREATE TABLE IF NOT EXISTS / CREATE INDEX IF
// NOT EXISTS make this a no-op on already-migrated databases.
func EnsureExtSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS listing_snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			listing_url TEXT NOT NULL,
			property_id INTEGER,
			saved_search TEXT,
			observed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			status TEXT,
			price INTEGER,
			beds REAL,
			baths REAL,
			sqft INTEGER,
			dom INTEGER,
			fetch_status INTEGER DEFAULT 200,
			raw_data JSON
		)`,
		`CREATE INDEX IF NOT EXISTS idx_redfin_snapshots_url ON listing_snapshots(listing_url)`,
		`CREATE INDEX IF NOT EXISTS idx_redfin_snapshots_search ON listing_snapshots(saved_search, observed_at)`,
		`CREATE TABLE IF NOT EXISTS saved_searches (
			slug TEXT PRIMARY KEY,
			options_json TEXT NOT NULL,
			last_synced_at DATETIME,
			listing_count INTEGER DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS regions (
			region_id INTEGER NOT NULL,
			region_type INTEGER NOT NULL,
			name TEXT,
			state TEXT,
			parent_metro_id INTEGER,
			cached_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (region_id, region_type)
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("redfin migrate: %w", err)
		}
	}
	return nil
}

// Snapshot is one observation of a Listing tied to a saved-search sync run.
// Snapshots accumulate over time so watch can diff them.
type Snapshot struct {
	ID          int64
	ListingURL  string
	PropertyID  int64
	SavedSearch string
	ObservedAt  time.Time
	Status      string
	Price       int
	Beds        float64
	Baths       float64
	Sqft        int
	DOM         int
	FetchStatus int
	RawData     json.RawMessage
}

// InsertSnapshot writes one observation to listing_snapshots.
func InsertSnapshot(db *sql.DB, s Snapshot) error {
	_, err := db.Exec(
		`INSERT INTO listing_snapshots (listing_url, property_id, saved_search, observed_at, status, price, beds, baths, sqft, dom, fetch_status, raw_data)
		 VALUES (?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ListingURL, s.PropertyID, s.SavedSearch, nullTime(s.ObservedAt),
		s.Status, s.Price, s.Beds, s.Baths, s.Sqft, s.DOM, fetchStatusOr200(s.FetchStatus), string(s.RawData),
	)
	return err
}

func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t
}

func fetchStatusOr200(s int) int {
	if s == 0 {
		return 200
	}
	return s
}

// LatestSyncTimestamps returns the N most recent distinct observed_at values
// for a saved search, sorted newest-first.
func LatestSyncTimestamps(db *sql.DB, slug string, limit int) ([]time.Time, error) {
	if limit <= 0 {
		limit = 2
	}
	rows, err := db.Query(
		`SELECT DISTINCT observed_at FROM listing_snapshots WHERE saved_search = ? ORDER BY observed_at DESC LIMIT ?`,
		slug, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []time.Time
	for rows.Next() {
		var t time.Time
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// SnapshotsForSearchAt returns every snapshot for a saved search at a given
// observed_at timestamp. Used by watch to assemble two side-by-side sync runs.
func SnapshotsForSearchAt(db *sql.DB, slug string, t time.Time) ([]Snapshot, error) {
	rows, err := db.Query(
		`SELECT id, listing_url, property_id, COALESCE(saved_search,''), observed_at,
		        COALESCE(status,''), COALESCE(price,0), COALESCE(beds,0), COALESCE(baths,0),
		        COALESCE(sqft,0), COALESCE(dom,0), COALESCE(fetch_status,200), COALESCE(raw_data,'')
		 FROM listing_snapshots WHERE saved_search = ? AND observed_at = ?`,
		slug, t,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Snapshot
	for rows.Next() {
		var s Snapshot
		var raw string
		if err := rows.Scan(&s.ID, &s.ListingURL, &s.PropertyID, &s.SavedSearch, &s.ObservedAt,
			&s.Status, &s.Price, &s.Beds, &s.Baths, &s.Sqft, &s.DOM, &s.FetchStatus, &raw); err != nil {
			return nil, err
		}
		if raw != "" {
			s.RawData = json.RawMessage(raw)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SnapshotsForURL returns every snapshot ever recorded for a listing URL,
// oldest-first.
func SnapshotsForURL(db *sql.DB, url string) ([]Snapshot, error) {
	rows, err := db.Query(
		`SELECT id, listing_url, COALESCE(property_id,0), COALESCE(saved_search,''), observed_at,
		        COALESCE(status,''), COALESCE(price,0), COALESCE(beds,0), COALESCE(baths,0),
		        COALESCE(sqft,0), COALESCE(dom,0), COALESCE(fetch_status,200), COALESCE(raw_data,'')
		 FROM listing_snapshots WHERE listing_url = ? ORDER BY observed_at ASC`,
		url,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Snapshot
	for rows.Next() {
		var s Snapshot
		var raw string
		if err := rows.Scan(&s.ID, &s.ListingURL, &s.PropertyID, &s.SavedSearch, &s.ObservedAt,
			&s.Status, &s.Price, &s.Beds, &s.Baths, &s.Sqft, &s.DOM, &s.FetchStatus, &raw); err != nil {
			return nil, err
		}
		if raw != "" {
			s.RawData = json.RawMessage(raw)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// UpsertSavedSearch records a saved-search slug with the options used to
// produce it and the count of listings returned.
func UpsertSavedSearch(db *sql.DB, slug string, opts SearchOptions, count int) error {
	b, err := json.Marshal(opts)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`INSERT INTO saved_searches (slug, options_json, last_synced_at, listing_count)
		 VALUES (?, ?, CURRENT_TIMESTAMP, ?)
		 ON CONFLICT(slug) DO UPDATE SET options_json = excluded.options_json,
		        last_synced_at = excluded.last_synced_at, listing_count = excluded.listing_count`,
		slug, string(b), count,
	)
	return err
}

// GetSavedSearch returns the stored SearchOptions for a slug, or zero/false
// when no row exists.
func GetSavedSearch(db *sql.DB, slug string) (SearchOptions, bool, error) {
	var raw string
	err := db.QueryRow(`SELECT options_json FROM saved_searches WHERE slug = ?`, slug).Scan(&raw)
	if err == sql.ErrNoRows {
		return SearchOptions{}, false, nil
	}
	if err != nil {
		return SearchOptions{}, false, err
	}
	var opts SearchOptions
	if err := json.Unmarshal([]byte(raw), &opts); err != nil {
		return SearchOptions{}, false, err
	}
	return opts, true, nil
}

// UpsertRegion caches a region's metadata for resolving region_id ↔ name.
func UpsertRegion(db *sql.DB, regionID int64, regionType int, name, state string, parentMetroID int64) error {
	_, err := db.Exec(
		`INSERT INTO regions (region_id, region_type, name, state, parent_metro_id, cached_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(region_id, region_type) DO UPDATE SET name = excluded.name,
		        state = excluded.state, parent_metro_id = excluded.parent_metro_id,
		        cached_at = excluded.cached_at`,
		regionID, regionType, name, state, parentMetroID,
	)
	return err
}

// LookupRegion returns name/state for a cached region, or "" when not cached.
func LookupRegion(db *sql.DB, regionID int64, regionType int) (name, state string, ok bool, err error) {
	err = db.QueryRow(
		`SELECT COALESCE(name,''), COALESCE(state,'') FROM regions WHERE region_id = ? AND region_type = ?`,
		regionID, regionType,
	).Scan(&name, &state)
	if err == sql.ErrNoRows {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return name, state, true, nil
}

// ContextDB is a small alias to the parts of *sql.DB the helpers need; using
// the concrete type keeps callers simple without forcing import of redfin in
// store package callers.
type ContextDB interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}
