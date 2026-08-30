// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored Rental Car Spain store extension: saved searches and price snapshots.
// Kept in its own file (not the generated store.go) so generate --force
// preserves it.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// SavedSearch is a named, re-runnable Málaga search.
type SavedSearch struct {
	Name         string `json:"name"`
	LocationCode string `json:"location_code"`
	DropoffCode  string `json:"dropoff_code,omitempty"`
	Pickup       string `json:"pickup"`
	Dropoff      string `json:"dropoff"`
	DriverAge    int    `json:"driver_age"`
	CreatedAt    string `json:"created_at"`
}

// PriceSnapshot is one recorded observation of a search's cheapest prices.
type PriceSnapshot struct {
	ID              int64           `json:"id"`
	SearchKey       string          `json:"search_key"`
	TakenAt         string          `json:"taken_at"`
	CheapestTotal   float64         `json:"cheapest_total"`
	CheapestFITotal float64         `json:"cheapest_full_insurance_total"`
	Currency        string          `json:"currency"`
	OfferCount      int             `json:"offer_count"`
	OffersJSON      json.RawMessage `json:"offers,omitempty"`
}

// EnsureCarSchema lazily creates the Rental Car Spain tables. Safe to call repeatedly.
func (s *Store) EnsureCarSchema(ctx context.Context) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS saved_searches (
    name          TEXT PRIMARY KEY,
    location_code TEXT NOT NULL,
    dropoff_code  TEXT,
    pickup        TEXT NOT NULL,
    dropoff       TEXT NOT NULL,
    driver_age    INTEGER NOT NULL DEFAULT 35,
    created_at    TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS price_snapshots (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    search_key          TEXT NOT NULL,
    taken_at            TEXT NOT NULL,
    cheapest_total      REAL,
    cheapest_fi_total   REAL,
    currency            TEXT,
    offer_count         INTEGER,
    offers_json         TEXT
);
CREATE INDEX IF NOT EXISTS idx_snapshots_key ON price_snapshots(search_key, taken_at);
CREATE TABLE IF NOT EXISTS supplier_ratings (
    airport    TEXT NOT NULL,
    supplier   TEXT NOT NULL,
    score      REAL NOT NULL,
    reviews    INTEGER NOT NULL DEFAULT 0,
    source     TEXT,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (airport, supplier)
);
CREATE TABLE IF NOT EXISTS fx_rates (
    currency   TEXT PRIMARY KEY,
    rate       REAL NOT NULL,
    ecb_date   TEXT,
    updated_at TEXT NOT NULL
);
`
	if _, err := s.DB().ExecContext(ctx, ddl); err != nil {
		return fmt.Errorf("creating rentalcarspain schema: %w", err)
	}
	return nil
}

// SupplierRating is a cached customer rating for a supplier at an airport.
type SupplierRating struct {
	Supplier  string    `json:"supplier"`
	Score     float64   `json:"score"`
	Reviews   int       `json:"reviews"`
	Source    string    `json:"source"`
	UpdatedAt time.Time `json:"updated_at,omitempty"` // when this rating was last cached
}

// UpsertSupplierRatings records supplier ratings for an airport, keeping the
// entry with the most reviews so a low-count score never overwrites a
// higher-confidence one.
func (s *Store) UpsertSupplierRatings(ctx context.Context, airport string, ratings []SupplierRating) error {
	if airport == "" || len(ratings) == 0 {
		return nil
	}
	if err := s.EnsureCarSchema(ctx); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, r := range ratings {
		if r.Score <= 0 || r.Supplier == "" {
			continue
		}
		_, err := s.DB().ExecContext(ctx, `
INSERT INTO supplier_ratings (airport, supplier, score, reviews, source, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(airport, supplier) DO UPDATE SET
    score=excluded.score, reviews=excluded.reviews, source=excluded.source, updated_at=excluded.updated_at
    WHERE excluded.reviews >= supplier_ratings.reviews`,
			airport, r.Supplier, r.Score, r.Reviews, r.Source, now)
		if err != nil {
			return err
		}
	}
	return nil
}

// GetSupplierRatings returns the cached ratings for an airport, each stamped
// with when it was last refreshed (UpdatedAt). Callers apply their own
// freshness policy; malformed timestamps yield a zero UpdatedAt (treated as
// stale).
func (s *Store) GetSupplierRatings(ctx context.Context, airport string) ([]SupplierRating, error) {
	if err := s.EnsureCarSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.DB().QueryContext(ctx, `
SELECT supplier, score, reviews, COALESCE(source,''), updated_at FROM supplier_ratings WHERE airport = ?`, airport)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SupplierRating
	for rows.Next() {
		var r SupplierRating
		var updated string
		if err := rows.Scan(&r.Supplier, &r.Score, &r.Reviews, &r.Source, &updated); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339, updated); err == nil {
			r.UpdatedAt = t
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FXCache is a cached set of ECB reference rates.
type FXCache struct {
	Date      string             // ECB publication date
	Rates     map[string]float64 // currency -> units per 1 EUR
	UpdatedAt time.Time          // when cached
}

// UpsertFXRates replaces the cached FX rate set with a freshly fetched one.
func (s *Store) UpsertFXRates(ctx context.Context, ecbDate string, rates map[string]float64) error {
	if len(rates) == 0 {
		return nil
	}
	if err := s.EnsureCarSchema(ctx); err != nil {
		return err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(ctx, `DELETE FROM fx_rates`); err != nil {
		return err
	}
	for cur, rate := range rates {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO fx_rates (currency, rate, ecb_date, updated_at) VALUES (?, ?, ?, ?)`,
			cur, rate, ecbDate, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetFXRates returns the cached rate set, or an empty FXCache (nil Rates) when
// nothing is cached.
func (s *Store) GetFXRates(ctx context.Context) (FXCache, error) {
	if err := s.EnsureCarSchema(ctx); err != nil {
		return FXCache{}, err
	}
	rows, err := s.DB().QueryContext(ctx, `SELECT currency, rate, COALESCE(ecb_date,''), updated_at FROM fx_rates`)
	if err != nil {
		return FXCache{}, err
	}
	defer rows.Close()
	out := FXCache{Rates: map[string]float64{}}
	for rows.Next() {
		var cur, date, updated string
		var rate float64
		if err := rows.Scan(&cur, &rate, &date, &updated); err != nil {
			return FXCache{}, err
		}
		out.Rates[cur] = rate
		out.Date = date
		if t, err := time.Parse(time.RFC3339, updated); err == nil {
			out.UpdatedAt = t
		}
	}
	if len(out.Rates) == 0 {
		out.Rates = nil
	}
	return out, rows.Err()
}

// RatingCacheStat summarizes the cached supplier ratings for one airport.
type RatingCacheStat struct {
	Airport   string    `json:"airport"`
	Suppliers int       `json:"suppliers"`
	Oldest    time.Time `json:"oldest"`
	Newest    time.Time `json:"newest"`
}

// SupplierRatingCacheStats returns a per-airport summary of the rating cache
// (supplier count and the oldest/newest refresh timestamps), airports ordered
// alphabetically.
func (s *Store) SupplierRatingCacheStats(ctx context.Context) ([]RatingCacheStat, error) {
	if err := s.EnsureCarSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.DB().QueryContext(ctx, `
SELECT airport, COUNT(*), MIN(updated_at), MAX(updated_at)
FROM supplier_ratings GROUP BY airport ORDER BY airport`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RatingCacheStat
	for rows.Next() {
		var st RatingCacheStat
		var oldest, newest string
		if err := rows.Scan(&st.Airport, &st.Suppliers, &oldest, &newest); err != nil {
			return nil, err
		}
		if t, err := time.Parse(time.RFC3339, oldest); err == nil {
			st.Oldest = t
		}
		if t, err := time.Parse(time.RFC3339, newest); err == nil {
			st.Newest = t
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// ClearSupplierRatings deletes cached ratings for one airport, or all airports
// when airport is empty, and returns how many rows were removed.
func (s *Store) ClearSupplierRatings(ctx context.Context, airport string) (int, error) {
	if err := s.EnsureCarSchema(ctx); err != nil {
		return 0, err
	}
	var res sql.Result
	var err error
	if airport == "" {
		res, err = s.DB().ExecContext(ctx, `DELETE FROM supplier_ratings`)
	} else {
		res, err = s.DB().ExecContext(ctx, `DELETE FROM supplier_ratings WHERE airport = ?`, airport)
	}
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// PurgeStaleSupplierRatings deletes cached ratings older than maxAge and
// returns how many rows were removed. A non-positive maxAge is a no-op.
func (s *Store) PurgeStaleSupplierRatings(ctx context.Context, maxAge time.Duration) (int, error) {
	if maxAge <= 0 {
		return 0, nil
	}
	if err := s.EnsureCarSchema(ctx); err != nil {
		return 0, err
	}
	cutoff := time.Now().UTC().Add(-maxAge).Format(time.RFC3339)
	res, err := s.DB().ExecContext(ctx, `DELETE FROM supplier_ratings WHERE updated_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

// AddSavedSearch inserts or replaces a named search.
func (s *Store) AddSavedSearch(ctx context.Context, ss SavedSearch) error {
	if err := s.EnsureCarSchema(ctx); err != nil {
		return err
	}
	if ss.CreatedAt == "" {
		ss.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if ss.DriverAge == 0 {
		ss.DriverAge = 35
	}
	_, err := s.DB().ExecContext(ctx, `
INSERT INTO saved_searches (name, location_code, dropoff_code, pickup, dropoff, driver_age, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(name) DO UPDATE SET
    location_code=excluded.location_code,
    dropoff_code=excluded.dropoff_code,
    pickup=excluded.pickup,
    dropoff=excluded.dropoff,
    driver_age=excluded.driver_age`,
		ss.Name, ss.LocationCode, ss.DropoffCode, ss.Pickup, ss.Dropoff, ss.DriverAge, ss.CreatedAt)
	return err
}

// GetSavedSearch returns a named search, or (nil, nil) when it does not exist.
func (s *Store) GetSavedSearch(ctx context.Context, name string) (*SavedSearch, error) {
	if err := s.EnsureCarSchema(ctx); err != nil {
		return nil, err
	}
	row := s.DB().QueryRowContext(ctx, `
SELECT name, location_code, COALESCE(dropoff_code,''), pickup, dropoff, driver_age, created_at
FROM saved_searches WHERE name = ?`, name)
	var ss SavedSearch
	var dropoff sql.NullString
	err := row.Scan(&ss.Name, &ss.LocationCode, &dropoff, &ss.Pickup, &ss.Dropoff, &ss.DriverAge, &ss.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ss.DropoffCode = dropoff.String
	return &ss, nil
}

// ListSavedSearches returns all saved searches, newest first.
func (s *Store) ListSavedSearches(ctx context.Context) ([]SavedSearch, error) {
	if err := s.EnsureCarSchema(ctx); err != nil {
		return nil, err
	}
	rows, err := s.DB().QueryContext(ctx, `
SELECT name, location_code, COALESCE(dropoff_code,''), pickup, dropoff, driver_age, created_at
FROM saved_searches ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]SavedSearch, 0)
	for rows.Next() {
		var ss SavedSearch
		var dropoff sql.NullString
		if err := rows.Scan(&ss.Name, &ss.LocationCode, &dropoff, &ss.Pickup, &ss.Dropoff, &ss.DriverAge, &ss.CreatedAt); err != nil {
			return nil, err
		}
		ss.DropoffCode = dropoff.String
		out = append(out, ss)
	}
	return out, rows.Err()
}

// RemoveSavedSearch deletes a named search and reports whether a row was removed.
func (s *Store) RemoveSavedSearch(ctx context.Context, name string) (bool, error) {
	if err := s.EnsureCarSchema(ctx); err != nil {
		return false, err
	}
	res, err := s.DB().ExecContext(ctx, `DELETE FROM saved_searches WHERE name = ?`, name)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// RecordSnapshot appends a price observation for a search key.
func (s *Store) RecordSnapshot(ctx context.Context, snap PriceSnapshot) error {
	if err := s.EnsureCarSchema(ctx); err != nil {
		return err
	}
	if snap.TakenAt == "" {
		snap.TakenAt = time.Now().UTC().Format(time.RFC3339)
	}
	if snap.Currency == "" {
		snap.Currency = "EUR"
	}
	var offersJSON any
	if len(snap.OffersJSON) > 0 {
		offersJSON = string(snap.OffersJSON)
	}
	_, err := s.DB().ExecContext(ctx, `
INSERT INTO price_snapshots (search_key, taken_at, cheapest_total, cheapest_fi_total, currency, offer_count, offers_json)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		snap.SearchKey, snap.TakenAt, snap.CheapestTotal, snap.CheapestFITotal, snap.Currency, snap.OfferCount, offersJSON)
	return err
}

// ListSnapshots returns recorded snapshots for a search key, oldest first.
// limit <= 0 returns all.
func (s *Store) ListSnapshots(ctx context.Context, searchKey string, limit int) ([]PriceSnapshot, error) {
	if err := s.EnsureCarSchema(ctx); err != nil {
		return nil, err
	}
	q := `
SELECT id, search_key, taken_at, COALESCE(cheapest_total,0), COALESCE(cheapest_fi_total,0),
       COALESCE(currency,'EUR'), COALESCE(offer_count,0)
FROM price_snapshots WHERE search_key = ? ORDER BY taken_at ASC`
	args := []any{searchKey}
	if limit > 0 {
		q += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]PriceSnapshot, 0)
	for rows.Next() {
		var p PriceSnapshot
		if err := rows.Scan(&p.ID, &p.SearchKey, &p.TakenAt, &p.CheapestTotal, &p.CheapestFITotal, &p.Currency, &p.OfferCount); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
