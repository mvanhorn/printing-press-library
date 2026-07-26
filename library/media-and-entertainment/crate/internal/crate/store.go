// Copyright 2026 justinwfu and contributors. Licensed under Apache-2.0. See LICENSE.

// Package crate holds a user's record collection locally.
//
// Discogs returns one page of a collection at a time and computes nothing
// across pages, so every question worth asking about a shelf — what to play,
// what is missing, what it is worth, what you actually collect — needs the
// whole thing in one place first. That place is this package.
package crate

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// Record is one release on a shelf or wantlist.
type Record struct {
	ReleaseID int64     `json:"release_id"`
	Title     string    `json:"title"`
	Artists   []string  `json:"artists"`
	Year      int       `json:"year"`
	Labels    []string  `json:"labels"`
	Genres    []string  `json:"genres"`
	Styles    []string  `json:"styles"`
	Formats   []string  `json:"formats"`
	Rating    int       `json:"rating"`
	DateAdded time.Time `json:"date_added"`
	// Wanted is true for wantlist rows, false for owned records.
	Wanted bool `json:"wanted"`
}

// ArtistLine renders the artist credit as one string.
func (r Record) ArtistLine() string {
	if len(r.Artists) == 0 {
		return "Unknown Artist"
	}
	return strings.Join(r.Artists, ", ")
}

// Decade returns the decade label, or empty when the year is unknown.
// Discogs uses 0 for "year not recorded", which must not become the 0s.
func (r Record) Decade() string {
	if r.Year < 1000 {
		return ""
	}
	return fmt.Sprintf("%ds", (r.Year/10)*10)
}

// Price is a cached marketplace observation for one release.
type Price struct {
	ReleaseID int64
	// ReqCurrency is the currency the caller asked for ("" for the API
	// default). It is part of the cache key.
	ReqCurrency string
	Lowest      float64
	// Currency is what the API actually returned.
	Currency   string
	NumForSale int
	// HasPrice is false when the release exists in the marketplace response
	// but nothing is currently for sale. A missing price is not a price of
	// zero, and totalling it as zero would understate every floor.
	HasPrice  bool
	FetchedAt time.Time
}

// Store is the local collection database.
type Store struct{ db *sql.DB }

// Open ensures the schema exists on an already-open database handle.
func Open(ctx context.Context, db *sql.DB) (*Store, error) {
	s := &Store{db: db}
	const schema = `
CREATE TABLE IF NOT EXISTS crate_records (
	username    TEXT NOT NULL,
	release_id  INTEGER NOT NULL,
	wanted      INTEGER NOT NULL DEFAULT 0,
	title       TEXT NOT NULL DEFAULT '',
	artists     TEXT NOT NULL DEFAULT '',
	year        INTEGER NOT NULL DEFAULT 0,
	labels      TEXT NOT NULL DEFAULT '',
	genres      TEXT NOT NULL DEFAULT '',
	styles      TEXT NOT NULL DEFAULT '',
	formats     TEXT NOT NULL DEFAULT '',
	rating      INTEGER NOT NULL DEFAULT 0,
	date_added  TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (username, release_id, wanted)
);
CREATE INDEX IF NOT EXISTS idx_crate_records_user ON crate_records(username, wanted);

-- Keyed by (release_id, req_currency), not release_id alone. Discogs returns
-- prices in whatever currency was asked for, so a cache keyed only on the
-- release serves a GBP price to a USD request: --currency then appears to be
-- ignored, and totals silently mix currencies.
CREATE TABLE IF NOT EXISTS crate_prices (
	release_id   INTEGER NOT NULL,
	req_currency TEXT NOT NULL DEFAULT '',
	lowest       REAL NOT NULL DEFAULT 0,
	currency     TEXT NOT NULL DEFAULT '',
	num_for_sale INTEGER NOT NULL DEFAULT 0,
	has_price    INTEGER NOT NULL DEFAULT 0,
	fetched_at   TEXT NOT NULL,
	PRIMARY KEY (release_id, req_currency)
);

CREATE TABLE IF NOT EXISTS crate_syncs (
	username   TEXT NOT NULL,
	kind       TEXT NOT NULL,
	synced_at  TEXT NOT NULL,
	item_count INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (username, kind)
);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return nil, fmt.Errorf("creating crate schema: %w", err)
	}
	if err := migratePrices(ctx, s.db); err != nil {
		return nil, err
	}
	return s, nil
}

// pricesTableDDL is the current shape of crate_prices, kept separate so the
// migration can re-create the table without duplicating the definition.
const pricesTableDDL = `
CREATE TABLE crate_prices (
	release_id   INTEGER NOT NULL,
	req_currency TEXT NOT NULL DEFAULT '',
	lowest       REAL NOT NULL DEFAULT 0,
	currency     TEXT NOT NULL DEFAULT '',
	num_for_sale INTEGER NOT NULL DEFAULT 0,
	has_price    INTEGER NOT NULL DEFAULT 0,
	fetched_at   TEXT NOT NULL,
	PRIMARY KEY (release_id, req_currency)
);`

// migratePrices repairs a crate_prices table created before req_currency
// existed. CREATE TABLE IF NOT EXISTS is a no-op against the old shape, so
// without this every price read fails with "no such column: req_currency" —
// which takes out `floor` and `deals` entirely on any database that predates
// the column.
//
// The repair drops and re-creates rather than ALTER TABLE ADD COLUMN because
// req_currency is part of the primary key, and SQLite cannot alter a key.
// Dropping is safe: crate_prices is a cache of marketplace prices, and the
// next run re-fetches what it needs.
func migratePrices(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(crate_prices)`)
	if err != nil {
		return fmt.Errorf("inspecting crate_prices: %w", err)
	}
	defer func() { _ = rows.Close() }()
	found := false
	for rows.Next() {
		var (
			cid        int
			name, typ  string
			notNull    int
			defaultVal sql.NullString
			pk         int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &defaultVal, &pk); err != nil {
			return fmt.Errorf("inspecting crate_prices: %w", err)
		}
		if name == "req_currency" {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("inspecting crate_prices: %w", err)
	}
	if err := rows.Close(); err != nil {
		return fmt.Errorf("inspecting crate_prices: %w", err)
	}
	if found {
		return nil
	}
	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS crate_prices;`+pricesTableDDL); err != nil {
		return fmt.Errorf("migrating crate_prices to the currency-keyed shape: %w", err)
	}
	return nil
}

func joinList(v []string) string { return strings.Join(v, "\x1f") }
func splitList(v string) []string {
	if v == "" {
		return nil
	}
	return strings.Split(v, "\x1f")
}

// ReplaceRecords swaps in a freshly synced set for one user and kind.
//
// Replace rather than upsert: a record sold or removed from the wantlist must
// disappear locally, and an upsert-only sync would keep it forever.
func (s *Store) ReplaceRecords(ctx context.Context, username string, wanted bool, recs []Record) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("starting sync transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	w := 0
	kind := "collection"
	if wanted {
		w, kind = 1, "wantlist"
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM crate_records WHERE username = ? AND wanted = ?`, username, w); err != nil {
		return fmt.Errorf("clearing previous %s: %w", kind, err)
	}
	stmt, err := tx.PrepareContext(ctx, `
INSERT INTO crate_records (username, release_id, wanted, title, artists, year, labels, genres, styles, formats, rating, date_added)
VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
ON CONFLICT(username, release_id, wanted) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("preparing insert: %w", err)
	}
	defer stmt.Close()

	for _, r := range recs {
		if _, err := stmt.ExecContext(ctx, username, r.ReleaseID, w, r.Title,
			joinList(r.Artists), r.Year, joinList(r.Labels), joinList(r.Genres),
			joinList(r.Styles), joinList(r.Formats), r.Rating,
			r.DateAdded.Format(time.RFC3339)); err != nil {
			return fmt.Errorf("inserting release %d: %w", r.ReleaseID, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO crate_syncs (username, kind, synced_at, item_count) VALUES (?,?,?,?)
		 ON CONFLICT(username, kind) DO UPDATE SET synced_at=excluded.synced_at, item_count=excluded.item_count`,
		username, kind, time.Now().UTC().Format(time.RFC3339), len(recs)); err != nil {
		return fmt.Errorf("recording sync: %w", err)
	}
	return tx.Commit()
}

// Records returns a user's owned records, or their wantlist.
func (s *Store) Records(ctx context.Context, username string, wanted bool) ([]Record, error) {
	w := 0
	if wanted {
		w = 1
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT release_id, title, artists, year, labels, genres, styles, formats, rating, date_added
FROM crate_records WHERE username = ? AND wanted = ? ORDER BY release_id`, username, w)
	if err != nil {
		return nil, fmt.Errorf("reading records: %w", err)
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var r Record
		var artists, labels, genres, styles, formats, added string
		if err := rows.Scan(&r.ReleaseID, &r.Title, &artists, &r.Year, &labels,
			&genres, &styles, &formats, &r.Rating, &added); err != nil {
			return nil, fmt.Errorf("scanning record: %w", err)
		}
		r.Artists, r.Labels = splitList(artists), splitList(labels)
		r.Genres, r.Styles, r.Formats = splitList(genres), splitList(styles), splitList(formats)
		r.DateAdded, _ = time.Parse(time.RFC3339, added)
		r.Wanted = wanted
		out = append(out, r)
	}
	return out, rows.Err()
}

// OwnedIDs returns the set of release ids a user owns, for set subtraction.
func (s *Store) OwnedIDs(ctx context.Context, username string) (map[int64]bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT release_id FROM crate_records WHERE username = ? AND wanted = 0`, username)
	if err != nil {
		return nil, fmt.Errorf("reading owned ids: %w", err)
	}
	defer rows.Close()
	out := map[int64]bool{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}

// PutPrice caches one marketplace observation.
func (s *Store) PutPrice(ctx context.Context, p Price) error {
	has := 0
	if p.HasPrice {
		has = 1
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO crate_prices (release_id, req_currency, lowest, currency, num_for_sale, has_price, fetched_at)
VALUES (?,?,?,?,?,?,?)
ON CONFLICT(release_id, req_currency) DO UPDATE SET lowest=excluded.lowest, currency=excluded.currency,
  num_for_sale=excluded.num_for_sale, has_price=excluded.has_price, fetched_at=excluded.fetched_at`,
		p.ReleaseID, p.ReqCurrency, p.Lowest, p.Currency, p.NumForSale, has, p.FetchedAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("caching price for %d: %w", p.ReleaseID, err)
	}
	return nil
}

// Prices returns cached prices for one requested currency, no older than
// maxAge, keyed by release id. A zero maxAge accepts any cached price.
//
// Filtering by reqCurrency is what makes --currency work on a warm cache;
// without it a USD run silently reuses GBP figures.
func (s *Store) Prices(ctx context.Context, maxAge time.Duration, reqCurrency string) (map[int64]Price, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT release_id, req_currency, lowest, currency, num_for_sale, has_price, fetched_at
		 FROM crate_prices WHERE req_currency = ?`, reqCurrency)
	if err != nil {
		return nil, fmt.Errorf("reading prices: %w", err)
	}
	defer rows.Close()

	out := map[int64]Price{}
	for rows.Next() {
		var p Price
		var has int
		var fetched string
		if err := rows.Scan(&p.ReleaseID, &p.ReqCurrency, &p.Lowest, &p.Currency, &p.NumForSale, &has, &fetched); err != nil {
			return nil, err
		}
		p.HasPrice = has == 1
		p.FetchedAt, _ = time.Parse(time.RFC3339, fetched)
		if maxAge > 0 && time.Since(p.FetchedAt) > maxAge {
			continue
		}
		out[p.ReleaseID] = p
	}
	return out, rows.Err()
}

// SyncInfo reports when a user's collection or wantlist was last synced.
func (s *Store) SyncInfo(ctx context.Context, username, kind string) (time.Time, int, bool) {
	var at string
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT synced_at, item_count FROM crate_syncs WHERE username = ? AND kind = ?`,
		username, kind).Scan(&at, &n)
	if err != nil {
		return time.Time{}, 0, false
	}
	t, _ := time.Parse(time.RFC3339, at)
	return t, n, true
}
