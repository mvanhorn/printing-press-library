// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// MUFAP observation storage.
//
// The generator emits no resource mirror for this API: every MUFAP panel is
// reached by a date-carrying GET (the HTML tabs) or a POST with a required
// fund code (the JSON allocation endpoint), so the syncable-resource profile
// -- which wants GET list endpoints with no required params -- matches
// nothing. This file supplies the mirror by hand.
//
// The shape is one generic row store rather than a typed table per tab. All
// five daily tabs, the monthly panel and the allocation payload are flat
// records belonging to exactly one (resource, date) pair, so a single
// (resource, date, row_key) table serves panel queries, rate derivation,
// universe reconstruction and export without six near-identical schemas
// drifting apart.
//
// Kept in a separate hand-authored file so `generate --force` preserves it.

// MUFAPRow is one upstream record: a stable within-date key and its raw JSON.
type MUFAPRow struct {
	Key     string
	Payload string
}

// MUFAPObs is a stored observation.
type MUFAPObs struct {
	Resource   string `json:"resource"`
	Date       string `json:"date"`
	Key        string `json:"row_key"`
	Payload    string `json:"payload"`
	ObservedAt string `json:"observed_at"`
}

// MUFAPCoverage is one fetch attempt's outcome for a (resource, date).
//
// RowCount 0 is a real, meaningful value: it records that the date WAS fetched
// and legitimately held no rows -- a weekend, a holiday, or a tab that does
// not publish on that date. That is exactly what separates "fetched and empty"
// from "never fetched", which a date-set diff over the observation table alone
// cannot see.
type MUFAPCoverage struct {
	Resource  string `json:"resource"`
	Date      string `json:"date"`
	RowCount  int    `json:"row_count"`
	FetchedAt string `json:"fetched_at"`
}

const mufapSchemaDDL = `
CREATE TABLE IF NOT EXISTS mufap_obs (
  resource    TEXT NOT NULL,
  date        TEXT NOT NULL,
  row_key     TEXT NOT NULL,
  payload     TEXT NOT NULL,
  observed_at TEXT NOT NULL,
  PRIMARY KEY (resource, date, row_key)
);
CREATE INDEX IF NOT EXISTS idx_mufap_obs_date     ON mufap_obs(date);
CREATE INDEX IF NOT EXISTS idx_mufap_obs_res_date ON mufap_obs(resource, date);
CREATE INDEX IF NOT EXISTS idx_mufap_obs_key      ON mufap_obs(row_key, date);

CREATE TABLE IF NOT EXISTS mufap_coverage (
  resource   TEXT NOT NULL,
  date       TEXT NOT NULL,
  row_count  INTEGER NOT NULL,
  fetched_at TEXT NOT NULL,
  PRIMARY KEY (resource, date)
);
CREATE INDEX IF NOT EXISTS idx_mufap_cov_res ON mufap_coverage(resource, date);
`

// EnsureMUFAPSchema creates the observation and coverage tables if absent.
func EnsureMUFAPSchema(ctx context.Context, s *Store) error {
	if s == nil || s.DB() == nil {
		return fmt.Errorf("mufap schema: nil store")
	}
	if _, err := s.DB().ExecContext(ctx, mufapSchemaDDL); err != nil {
		return fmt.Errorf("mufap schema: %w", err)
	}
	return nil
}

// SaveMUFAPDate mirrors one (resource, date) snapshot and records coverage.
//
// The observation rows and the coverage row are written in ONE transaction so
// the ledger can never claim a row count the data does not have. An empty rows
// slice is not an error: it stores a zero-count coverage row, which is the
// whole point of the ledger.
func SaveMUFAPDate(ctx context.Context, s *Store, resource, date string, rows []MUFAPRow, observedAt time.Time) error {
	if s == nil || s.DB() == nil {
		return fmt.Errorf("mufap save: nil store")
	}
	if resource == "" || date == "" {
		return fmt.Errorf("mufap save: resource and date are required")
	}
	stamp := observedAt.UTC().Format(time.RFC3339)
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("mufap save: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Replace the snapshot wholesale: a re-fetch of the same date is the
	// authoritative version, and leaving stale rows behind would inflate the
	// universe width for that date.
	if _, err := tx.ExecContext(ctx, `DELETE FROM mufap_obs WHERE resource = ? AND date = ?`, resource, date); err != nil {
		return fmt.Errorf("mufap save: clear %s/%s: %w", resource, date, err)
	}
	if len(rows) > 0 {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO mufap_obs (resource, date, row_key, payload, observed_at) VALUES (?, ?, ?, ?, ?)`)
		if err != nil {
			return fmt.Errorf("mufap save: prepare: %w", err)
		}
		defer func() { _ = stmt.Close() }()
		for _, r := range rows {
			if _, err := stmt.ExecContext(ctx, resource, date, r.Key, r.Payload, stamp); err != nil {
				return fmt.Errorf("mufap save: insert %s/%s/%s: %w", resource, date, r.Key, err)
			}
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO mufap_coverage (resource, date, row_count, fetched_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(resource, date) DO UPDATE SET row_count = excluded.row_count, fetched_at = excluded.fetched_at`,
		resource, date, len(rows), stamp); err != nil {
		return fmt.Errorf("mufap save: coverage %s/%s: %w", resource, date, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("mufap save: commit: %w", err)
	}
	return nil
}

// LoadMUFAPObs returns stored observations for a resource over [from, to].
// Empty from/to are open-ended. Rows are drained fully before return so no
// follow-up query runs against an open cursor on SQLite's single connection.
func LoadMUFAPObs(ctx context.Context, s *Store, resource, from, to string) ([]MUFAPObs, error) {
	if s == nil || s.DB() == nil {
		return nil, fmt.Errorf("mufap load: nil store")
	}
	q := `SELECT resource, date, row_key, payload, observed_at FROM mufap_obs WHERE resource = ?`
	args := []any{resource}
	if from != "" {
		q += ` AND date >= ?`
		args = append(args, from)
	}
	if to != "" {
		q += ` AND date <= ?`
		args = append(args, to)
	}
	q += ` ORDER BY date, row_key`
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("mufap load: query: %w", err)
	}
	out := make([]MUFAPObs, 0)
	for rows.Next() {
		var o MUFAPObs
		var payload, observed sql.NullString
		if err := rows.Scan(&o.Resource, &o.Date, &o.Key, &payload, &observed); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("mufap load: scan: %w", err)
		}
		o.Payload = payload.String
		o.ObservedAt = observed.String
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("mufap load: iterate: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("mufap load: close: %w", err)
	}
	return out, nil
}

// LoadMUFAPCoverage returns the coverage ledger for a resource over [from, to].
// An empty resource returns every resource's ledger.
func LoadMUFAPCoverage(ctx context.Context, s *Store, resource, from, to string) ([]MUFAPCoverage, error) {
	if s == nil || s.DB() == nil {
		return nil, fmt.Errorf("mufap coverage: nil store")
	}
	q := `SELECT resource, date, row_count, fetched_at FROM mufap_coverage WHERE 1=1`
	args := []any{}
	if resource != "" {
		q += ` AND resource = ?`
		args = append(args, resource)
	}
	if from != "" {
		q += ` AND date >= ?`
		args = append(args, from)
	}
	if to != "" {
		q += ` AND date <= ?`
		args = append(args, to)
	}
	q += ` ORDER BY resource, date`
	rows, err := s.DB().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("mufap coverage: query: %w", err)
	}
	out := make([]MUFAPCoverage, 0)
	for rows.Next() {
		var c MUFAPCoverage
		var n sql.NullInt64
		var fetched sql.NullString
		if err := rows.Scan(&c.Resource, &c.Date, &n, &fetched); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("mufap coverage: scan: %w", err)
		}
		c.RowCount = int(n.Int64)
		c.FetchedAt = fetched.String
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("mufap coverage: iterate: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("mufap coverage: close: %w", err)
	}
	return out, nil
}

// MUFAPFetchedDates returns the set of dates already fetched for a resource,
// including dates that legitimately held zero rows.
func MUFAPFetchedDates(ctx context.Context, s *Store, resource string) (map[string]bool, error) {
	cov, err := LoadMUFAPCoverage(ctx, s, resource, "", "")
	if err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(cov))
	for _, c := range cov {
		set[c.Date] = true
	}
	return set, nil
}
