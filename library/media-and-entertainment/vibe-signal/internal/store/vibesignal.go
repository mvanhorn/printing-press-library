// Copyright 2026 not0xjarvis and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored store extension for the vibe-signal aggregator (not
// generator-emitted). Lazy CREATE TABLE IF NOT EXISTS keeps this decoupled from
// the generated migration slice in store.go.

package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// SignalRow is the unified cross-source signal record persisted by vibe-signal.
// It mirrors source.Signal but lives in the store package to avoid an import
// cycle (the cli layer maps between the two).
type SignalRow struct {
	Source      string
	SourceID    string
	Query       string
	Title       string
	URL         string
	Author      string
	Points      int
	Comments    int
	PublishedAt time.Time
	Excerpt     string
	RawJSON     string
}

const vibeSignalSchema = `
CREATE TABLE IF NOT EXISTS signals (
    source       TEXT NOT NULL,
    source_id    TEXT NOT NULL,
    query        TEXT NOT NULL,
    title        TEXT NOT NULL,
    url          TEXT,
    author       TEXT,
    points       INTEGER DEFAULT 0,
    comments     INTEGER DEFAULT 0,
    published_at TEXT,
    excerpt      TEXT,
    raw_json     TEXT,
    run_id       TEXT NOT NULL,
    PRIMARY KEY (source, source_id, query, run_id)
);
CREATE TABLE IF NOT EXISTS runs (
    run_id        TEXT PRIMARY KEY,
    query         TEXT NOT NULL,
    window_days   INTEGER NOT NULL,
    created_at    TEXT NOT NULL,
    coverage_json TEXT
);
CREATE INDEX IF NOT EXISTS idx_signals_query ON signals(query, source);
`

// EnsureVibeSignalSchema creates the signals and runs tables if absent.
func (s *Store) EnsureVibeSignalSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, vibeSignalSchema); err != nil {
		return fmt.Errorf("ensuring vibe-signal schema: %w", err)
	}
	return nil
}

// RecordRun records a report/sync snapshot run.
func (s *Store) RecordRun(ctx context.Context, runID, query string, windowDays int, coverageJSON string) error {
	if err := s.EnsureVibeSignalSchema(ctx); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO runs (run_id, query, window_days, created_at, coverage_json)
		 VALUES (?, ?, ?, ?, ?)`,
		runID, query, windowDays, time.Now().UTC().Format(time.RFC3339), coverageJSON)
	if err != nil {
		return fmt.Errorf("recording run: %w", err)
	}
	return nil
}

// UpsertSignals writes signal rows for a run inside a single transaction.
func (s *Store) UpsertSignals(ctx context.Context, runID string, rows []SignalRow) error {
	if err := s.EnsureVibeSignalSchema(ctx); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin signals tx: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR REPLACE INTO signals
		 (source, source_id, query, title, url, author, points, comments, published_at, excerpt, raw_json, run_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("prepare signals insert: %w", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		var published string
		if !r.PublishedAt.IsZero() {
			published = r.PublishedAt.UTC().Format(time.RFC3339)
		}
		if _, err := stmt.ExecContext(ctx,
			r.Source, r.SourceID, r.Query, r.Title, r.URL, r.Author,
			r.Points, r.Comments, published, r.Excerpt, r.RawJSON, runID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("insert signal %s/%s: %w", r.Source, r.SourceID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit signals tx: %w", err)
	}
	return nil
}

// QuerySignals returns stored signals for a topic, optionally filtered to one
// source, deduplicated by (source, source_id) keeping the most recent, ordered
// by published_at descending. Returns an empty slice when nothing matches.
func (s *Store) QuerySignals(ctx context.Context, query, sourceFilter string, limit int) ([]SignalRow, error) {
	if err := s.EnsureVibeSignalSchema(ctx); err != nil {
		return nil, err
	}
	sqlStmt := `SELECT source, source_id, query, title, url, author, points, comments, published_at, excerpt, raw_json
	            FROM signals WHERE query = ?`
	args := []any{query}
	if sourceFilter != "" {
		sqlStmt += " AND source = ?"
		args = append(args, sourceFilter)
	}
	sqlStmt += " ORDER BY published_at DESC"
	rows, err := s.db.QueryContext(ctx, sqlStmt, args...)
	if err != nil {
		return nil, fmt.Errorf("querying signals: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]bool)
	out := make([]SignalRow, 0)
	for rows.Next() {
		var (
			r         SignalRow
			url       sql.NullString
			author    sql.NullString
			published sql.NullString
			excerpt   sql.NullString
			raw       sql.NullString
		)
		if err := rows.Scan(&r.Source, &r.SourceID, &r.Query, &r.Title,
			&url, &author, &r.Points, &r.Comments, &published, &excerpt, &raw); err != nil {
			return nil, fmt.Errorf("scanning signal: %w", err)
		}
		key := r.Source + "\x00" + r.SourceID
		if seen[key] {
			continue
		}
		seen[key] = true
		r.URL = url.String
		r.Author = author.String
		r.Excerpt = excerpt.String
		r.RawJSON = raw.String
		if published.String != "" {
			r.PublishedAt = ParseStoredTimeValue(published.String)
		}
		out = append(out, r)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating signals: %w", err)
	}
	return out, nil
}

// ParseStoredTimeValue parses an RFC3339 timestamp stored by UpsertSignals,
// returning the zero time on failure.
func ParseStoredTimeValue(s string) time.Time {
	if ts, err := time.Parse(time.RFC3339, s); err == nil {
		return ts
	}
	return time.Time{}
}
