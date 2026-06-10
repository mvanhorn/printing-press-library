// Copyright 2026 richardadonnell. Licensed under Apache-2.0. See LICENSE.
// Hand-written: watch (saved search) + snapshot tables for the `watch` command.
// Lazy CREATE TABLE IF NOT EXISTS so these survive store regen without touching
// the generated migrate() slice.

package store

import (
	"database/sql"
	"fmt"
	"time"
)

// ensureWatchTables creates the watches and watch_snapshots tables if absent.
// Called lazily by every watch operation so a fresh DB (or one created before
// these tables existed) is upgraded on first use. Idempotent.
func (s *Store) ensureWatchTables() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS watches (
			name TEXT PRIMARY KEY,
			site TEXT NOT NULL DEFAULT 'moto',
			q TEXT,
			location TEXT,
			make TEXT,
			model TEXT,
			style TEXT,
			state TEXT,
			sort TEXT,
			limit_n INTEGER NOT NULL DEFAULT 24,
			max_pages INTEGER NOT NULL DEFAULT 5,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS watch_snapshots (
			watch_name TEXT NOT NULL,
			listing_id TEXT NOT NULL,
			price TEXT,
			title TEXT,
			url TEXT,
			snapshot_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (watch_name, listing_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_watch_snapshots_name ON watch_snapshots(watch_name)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("creating watch tables: %w", err)
		}
	}
	return nil
}

// Watch is a saved search query.
type Watch struct {
	Name      string    `json:"name"`
	Site      string    `json:"site"`
	Q         string    `json:"q,omitempty"`
	Location  string    `json:"location,omitempty"`
	Make      string    `json:"make,omitempty"`
	Model     string    `json:"model,omitempty"`
	Style     string    `json:"style,omitempty"`
	State     string    `json:"state,omitempty"`
	Sort      string    `json:"sort,omitempty"`
	Limit     int       `json:"limit"`
	MaxPages  int       `json:"max_pages"`
	CreatedAt time.Time `json:"created_at"`
}

// SaveWatch inserts or replaces a saved watch by name.
func (s *Store) SaveWatch(w Watch) error {
	if err := s.ensureWatchTables(); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO watches (name, site, q, location, make, model, style, state, sort, limit_n, max_pages, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET site=excluded.site, q=excluded.q, location=excluded.location,
		   make=excluded.make, model=excluded.model, style=excluded.style, state=excluded.state,
		   sort=excluded.sort, limit_n=excluded.limit_n, max_pages=excluded.max_pages`,
		w.Name, nonEmptyOr(w.Site, "moto"), w.Q, w.Location, w.Make, w.Model, w.Style, w.State, w.Sort, w.Limit, w.MaxPages,
	)
	return err
}

// GetWatch returns a single watch by name, or (Watch{}, false, nil) if absent.
func (s *Store) GetWatch(name string) (Watch, bool, error) {
	if err := s.ensureWatchTables(); err != nil {
		return Watch{}, false, err
	}
	row := s.db.QueryRow(
		`SELECT name, site, q, location, make, model, style, state, sort, limit_n, max_pages, created_at
		 FROM watches WHERE name = ?`, name)
	w, err := scanWatch(row)
	if err == sql.ErrNoRows {
		return Watch{}, false, nil
	}
	if err != nil {
		return Watch{}, false, err
	}
	return w, true, nil
}

// ListWatches returns all saved watches ordered by name.
func (s *Store) ListWatches() ([]Watch, error) {
	if err := s.ensureWatchTables(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT name, site, q, location, make, model, style, state, sort, limit_n, max_pages, created_at
		 FROM watches ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Watch, 0)
	for rows.Next() {
		w, err := scanWatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// DeleteWatch removes a watch and its snapshots. Returns whether a row existed.
func (s *Store) DeleteWatch(name string) (bool, error) {
	if err := s.ensureWatchTables(); err != nil {
		return false, err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	res, err := tx.Exec(`DELETE FROM watches WHERE name = ?`, name)
	if err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if _, err := tx.Exec(`DELETE FROM watch_snapshots WHERE watch_name = ?`, name); err != nil {
		_ = tx.Rollback()
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

// SnapshotRow is one listing's last-seen state under a watch.
type SnapshotRow struct {
	ListingID string
	Price     string
	Title     string
	URL       string
}

// GetSnapshot returns the previous snapshot for a watch keyed by listing id.
func (s *Store) GetSnapshot(watchName string) (map[string]SnapshotRow, error) {
	if err := s.ensureWatchTables(); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(
		`SELECT listing_id, price, title, url FROM watch_snapshots WHERE watch_name = ?`, watchName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]SnapshotRow{}
	for rows.Next() {
		var id string
		var price, title, url sql.NullString
		if err := rows.Scan(&id, &price, &title, &url); err != nil {
			return nil, err
		}
		out[id] = SnapshotRow{ListingID: id, Price: price.String, Title: title.String, URL: url.String}
	}
	return out, rows.Err()
}

// ReplaceSnapshot atomically replaces the stored snapshot for a watch.
func (s *Store) ReplaceSnapshot(watchName string, rows []SnapshotRow) error {
	if err := s.ensureWatchTables(); err != nil {
		return err
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM watch_snapshots WHERE watch_name = ?`, watchName); err != nil {
		return err
	}
	stmt, err := tx.Prepare(
		`INSERT INTO watch_snapshots (watch_name, listing_id, price, title, url, snapshot_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(watchName, r.ListingID, r.Price, r.Title, r.URL); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// rowScanner abstracts *sql.Row and *sql.Rows for scanWatch.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanWatch(sc rowScanner) (Watch, error) {
	var w Watch
	var site, q, loc, mk, model, style, state, srt sql.NullString
	var created sql.NullString
	if err := sc.Scan(&w.Name, &site, &q, &loc, &mk, &model, &style, &state, &srt, &w.Limit, &w.MaxPages, &created); err != nil {
		return Watch{}, err
	}
	w.Site = nonEmptyOr(site.String, "moto")
	w.Q, w.Location, w.Make, w.Model, w.Style, w.State, w.Sort = q.String, loc.String, mk.String, model.String, style.String, state.String, srt.String
	if created.Valid {
		// Reuse the store's tolerant timestamp parse via a couple of layouts.
		for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999 -0700 MST", "2006-01-02 15:04:05 -0700 MST", "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, created.String); err == nil {
				w.CreatedAt = t
				break
			}
		}
	}
	return w, nil
}

func nonEmptyOr(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
