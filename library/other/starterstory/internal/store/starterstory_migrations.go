// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored StarterStory local-index backbone (see .printing-press-patches/).

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// EnsureStarterStoryIndex lazily creates the local sitemap-index tables.
//
// ss_index holds one row per classified StarterStory URL (section + slug +
// humanized title + revenue parsed from the slug) and tracks first_seen /
// last_seen so the whats-new command can diff a fresh crawl against the prior
// one. ss_meta is a single-row (id=1) table recording the last two index runs
// so whats-new can scope "new since last run" without a second timestamp
// column per index row.
//
// Idempotent: CREATE TABLE IF NOT EXISTS makes repeated calls a no-op, so
// read-path commands can call this defensively before querying.
func (s *Store) EnsureStarterStoryIndex(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS ss_index(
			url TEXT PRIMARY KEY,
			section TEXT NOT NULL,
			slug TEXT NOT NULL,
			title TEXT,
			revenue INTEGER DEFAULT 0,
			first_seen TEXT,
			last_seen TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_ss_index_section ON ss_index(section)`,
		`CREATE INDEX IF NOT EXISTS idx_ss_index_revenue ON ss_index(revenue)`,
		`CREATE INDEX IF NOT EXISTS idx_ss_index_first_seen ON ss_index(first_seen)`,
		`CREATE TABLE IF NOT EXISTS ss_meta(
			id INTEGER PRIMARY KEY CHECK(id=1),
			last_run TEXT,
			prev_run TEXT
		)`,
	}
	for _, stmt := range stmts {
		if _, err := s.DB().ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure ss_index schema: %w", err)
		}
	}
	return nil
}

// IndexRow is a single classified sitemap entry to upsert into ss_index.
type IndexRow struct {
	URL     string
	Section string
	Slug    string
	Title   string
	Revenue int64
}

// StarterStoryLastRun returns the last_run timestamp recorded in ss_meta, or
// the empty string when the index has never been built.
func (s *Store) StarterStoryLastRun(ctx context.Context) (string, error) {
	var lastRun sql.NullString
	err := s.DB().QueryRowContext(ctx, `SELECT last_run FROM ss_meta WHERE id=1`).Scan(&lastRun)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read ss_meta.last_run: %w", err)
	}
	return lastRun.String, nil
}

// RebuildStarterStoryIndex upserts every row and advances ss_meta in a single
// write transaction. now must be an RFC3339 UTC timestamp; it is used as both
// last_run and, for newly-seen rows, first_seen so that whats-new can key on
// first_seen == last_run. Existing rows keep their original first_seen and only
// refresh last_seen, title, revenue, and section.
func (s *Store) RebuildStarterStoryIndex(ctx context.Context, rows []IndexRow, now string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin index rebuild: %w", err)
	}
	defer tx.Rollback()

	// Advance ss_meta first: prev_run inherits the current last_run, last_run
	// becomes now. Done before the row upserts so brand-new rows landing with
	// first_seen == now line up with last_run for the whats-new diff.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ss_meta(id, last_run, prev_run)
		VALUES (1, ?, NULL)
		ON CONFLICT(id) DO UPDATE SET prev_run = ss_meta.last_run, last_run = excluded.last_run
	`, now); err != nil {
		return fmt.Errorf("update ss_meta: %w", err)
	}

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO ss_index(url, section, slug, title, revenue, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(url) DO UPDATE SET
			section = excluded.section,
			slug = excluded.slug,
			title = excluded.title,
			revenue = excluded.revenue,
			last_seen = excluded.last_seen
	`)
	if err != nil {
		return fmt.Errorf("prepare index upsert: %w", err)
	}
	defer stmt.Close()

	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, r.URL, r.Section, r.Slug, r.Title, r.Revenue, now, now); err != nil {
			return fmt.Errorf("upsert index row %s: %w", r.URL, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit index rebuild: %w", err)
	}
	return nil
}
