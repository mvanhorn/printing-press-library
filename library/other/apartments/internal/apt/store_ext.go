package apt

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// extSchema is the additive set of tables and indexes that hold
// apartments.com's per-listing time series, saved-search bookkeeping,
// and the local shortlist. EnsureExtSchema runs them every time so
// commands can rely on the schema being present without depending on
// the generator's migration sequence.
const extSchema = `
CREATE TABLE IF NOT EXISTS listing_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    listing_url TEXT NOT NULL,
    property_id TEXT,
    saved_search TEXT,
    observed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    max_rent INTEGER,
    beds INTEGER,
    baths REAL,
    available_at TEXT,
    fetch_status INTEGER DEFAULT 200,
    raw_data JSON
);
CREATE INDEX IF NOT EXISTS idx_snapshots_url ON listing_snapshots(listing_url);
CREATE INDEX IF NOT EXISTS idx_snapshots_search ON listing_snapshots(saved_search, observed_at);

CREATE TABLE IF NOT EXISTS saved_searches (
    slug TEXT PRIMARY KEY,
    options_json TEXT NOT NULL,
    last_synced_at DATETIME,
    listing_count INTEGER DEFAULT 0
);

CREATE TABLE IF NOT EXISTS shortlist (
    listing_url TEXT NOT NULL,
    tag TEXT NOT NULL DEFAULT '',
    note TEXT NOT NULL DEFAULT '',
    added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (listing_url, tag)
);
`

// EnsureExtSchema runs the apt-extension CREATE TABLE / CREATE INDEX
// statements. Idempotent: every command that touches these tables can
// safely call it.
func EnsureExtSchema(db *sql.DB) error {
	for _, stmt := range strings.Split(extSchema, ";") {
		s := strings.TrimSpace(stmt)
		if s == "" {
			continue
		}
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("apt schema: %s: %w", firstLine(s), err)
		}
	}
	return nil
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// SnapshotInsert is the row inserted into listing_snapshots.
type SnapshotInsert struct {
	ListingURL  string
	PropertyID  string
	SavedSearch string
	MaxRent     int
	Beds        int
	Baths       float64
	AvailableAt string
	FetchStatus int
	Raw         json.RawMessage
}

// InsertSnapshot writes one snapshot row and returns its rowid.
func InsertSnapshot(db *sql.DB, in SnapshotInsert) (int64, error) {
	if in.FetchStatus == 0 {
		in.FetchStatus = 200
	}
	res, err := db.Exec(
		`INSERT INTO listing_snapshots
		 (listing_url, property_id, saved_search, observed_at, max_rent,
		  beds, baths, available_at, fetch_status, raw_data)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		in.ListingURL, in.PropertyID, in.SavedSearch, time.Now().UTC(),
		in.MaxRent, in.Beds, in.Baths, in.AvailableAt, in.FetchStatus,
		string(in.Raw),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpsertSavedSearch stores or refreshes a saved-search row keyed on
// slug. listing_count is the count from the most recent sync.
func UpsertSavedSearch(db *sql.DB, slug string, opts SearchOptions, count int) error {
	optsJSON, err := json.Marshal(opts)
	if err != nil {
		return err
	}
	_, err = db.Exec(
		`INSERT INTO saved_searches (slug, options_json, last_synced_at, listing_count)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(slug) DO UPDATE SET options_json = excluded.options_json,
		   last_synced_at = excluded.last_synced_at,
		   listing_count = excluded.listing_count`,
		slug, string(optsJSON), time.Now().UTC(), count,
	)
	return err
}

// SyncTimestamp is one observed_at marker for a saved-search.
type SyncTimestamp struct {
	ObservedAt time.Time
}

// LatestSyncTimestamps returns up to N most recent distinct sync
// snapshots (rounded to the second) for a saved-search slug, newest
// first. Each apartments.com sync produces a burst of rows with
// timestamps within milliseconds of each other; the GROUP BY clamps
// them into one logical sync.
func LatestSyncTimestamps(db *sql.DB, slug string, limit int) ([]time.Time, error) {
	if limit <= 0 {
		limit = 2
	}
	rows, err := db.Query(
		`SELECT MAX(observed_at) AS ts
		 FROM listing_snapshots
		 WHERE saved_search = ?
		 GROUP BY strftime('%Y-%m-%d %H:%M:%S', observed_at)
		 ORDER BY ts DESC
		 LIMIT ?`,
		slug, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []time.Time
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		t := parseStoredTime(s)
		out = append(out, t)
	}
	return out, rows.Err()
}

// SnapshotRow is one materialized listing-snapshot row.
type SnapshotRow struct {
	ListingURL  string    `json:"listing_url"`
	PropertyID  string    `json:"property_id,omitempty"`
	SavedSearch string    `json:"saved_search,omitempty"`
	ObservedAt  time.Time `json:"observed_at"`
	MaxRent     int       `json:"max_rent,omitempty"`
	Beds        int       `json:"beds,omitempty"`
	Baths       float64   `json:"baths,omitempty"`
	AvailableAt string    `json:"available_at,omitempty"`
	FetchStatus int       `json:"fetch_status,omitempty"`
}

// SnapshotsForSearchAt returns the rows for one saved-search whose
// observed_at falls within ±2 seconds of `at` — the granularity at
// which LatestSyncTimestamps groups bursts.
func SnapshotsForSearchAt(db *sql.DB, slug string, at time.Time) ([]SnapshotRow, error) {
	rows, err := db.Query(
		`SELECT listing_url, property_id, saved_search, observed_at, max_rent, beds, baths, available_at, fetch_status
		 FROM listing_snapshots
		 WHERE saved_search = ?
		   AND ABS(strftime('%s', observed_at) - strftime('%s', ?)) <= 2`,
		slug, at.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SnapshotRow
	for rows.Next() {
		var (
			r           SnapshotRow
			ts          string
			propertyID  sql.NullString
			savedSearch sql.NullString
			availableAt sql.NullString
			maxRent     sql.NullInt64
			beds        sql.NullInt64
			baths       sql.NullFloat64
			fetchStatus sql.NullInt64
		)
		if err := rows.Scan(&r.ListingURL, &propertyID, &savedSearch, &ts, &maxRent, &beds, &baths, &availableAt, &fetchStatus); err != nil {
			return nil, err
		}
		r.PropertyID = propertyID.String
		r.SavedSearch = savedSearch.String
		r.ObservedAt = parseStoredTime(ts)
		r.MaxRent = int(maxRent.Int64)
		r.Beds = int(beds.Int64)
		r.Baths = baths.Float64
		r.AvailableAt = availableAt.String
		r.FetchStatus = int(fetchStatus.Int64)
		out = append(out, r)
	}
	return out, rows.Err()
}

// SnapshotsForURL returns every snapshot row for one listing URL,
// oldest first (history view).
func SnapshotsForURL(db *sql.DB, listingURL string) ([]SnapshotRow, error) {
	rows, err := db.Query(
		`SELECT listing_url, property_id, saved_search, observed_at, max_rent, beds, baths, available_at, fetch_status
		 FROM listing_snapshots
		 WHERE listing_url = ?
		 ORDER BY observed_at ASC`,
		listingURL,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SnapshotRow
	for rows.Next() {
		var (
			r           SnapshotRow
			ts          string
			propertyID  sql.NullString
			savedSearch sql.NullString
			availableAt sql.NullString
			maxRent     sql.NullInt64
			beds        sql.NullInt64
			baths       sql.NullFloat64
			fetchStatus sql.NullInt64
		)
		if err := rows.Scan(&r.ListingURL, &propertyID, &savedSearch, &ts, &maxRent, &beds, &baths, &availableAt, &fetchStatus); err != nil {
			return nil, err
		}
		r.PropertyID = propertyID.String
		r.SavedSearch = savedSearch.String
		r.ObservedAt = parseStoredTime(ts)
		r.MaxRent = int(maxRent.Int64)
		r.Beds = int(beds.Int64)
		r.Baths = baths.Float64
		r.AvailableAt = availableAt.String
		r.FetchStatus = int(fetchStatus.Int64)
		out = append(out, r)
	}
	return out, rows.Err()
}

// parseStoredTime is a local copy of cliutil.ParseStoredTime to avoid
// a hard dependency on cliutil from the apt package's pure-data path.
func parseStoredTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05.999999 -0700 MST",
		"2006-01-02 15:04:05.999 -0700 MST",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05.999999999 -0700",
		"2006-01-02 15:04:05 -0700",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}
