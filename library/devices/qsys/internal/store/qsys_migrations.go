// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// EnsureQSYSSchema creates the Q-SYS corpus tables.
//
// These live outside the generic `resources` table because the corpus is not a
// REST resource set: it is two scraped websites plus a PDF layer, joined on a
// normalized model name. Keeping them in a separate file (rather than appending
// to the emitted migration slice in store.go) is what lets `generate --force`
// preserve them.
//
// Safe to call repeatedly; every statement is IF NOT EXISTS.
func EnsureQSYSSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS qsys_products (
			model           TEXT PRIMARY KEY,
			title           TEXT NOT NULL DEFAULT '',
			is_product      INTEGER NOT NULL DEFAULT 1,
			family          TEXT NOT NULL DEFAULT '',
			slug            TEXT NOT NULL DEFAULT '',
			url             TEXT NOT NULL DEFAULT '',
			overview        TEXT NOT NULL DEFAULT '',
			spec_pdf_url    TEXT NOT NULL DEFAULT '',
			manual_pdf_url  TEXT NOT NULL DEFAULT '',
			spec_text       TEXT NOT NULL DEFAULT '',
			discontinued    INTEGER NOT NULL DEFAULT 0,
			synced_at       TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_qsys_products_family ON qsys_products(family)`,

		`CREATE TABLE IF NOT EXISTS qsys_pages (
			url        TEXT PRIMARY KEY,
			section    TEXT NOT NULL DEFAULT '',
			title      TEXT NOT NULL DEFAULT '',
			body       TEXT NOT NULL DEFAULT '',
			synced_at  TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_qsys_pages_section ON qsys_pages(section)`,

		`CREATE TABLE IF NOT EXISTS qsys_compat (
			qds_version      TEXT PRIMARY KEY,
			release_date     TEXT NOT NULL DEFAULT '',
			added_hardware   TEXT NOT NULL DEFAULT '',
			removed_hardware TEXT NOT NULL DEFAULT '',
			synced_at        TEXT NOT NULL DEFAULT ''
		)`,

		// support.qsys.com knowledge base. Category is the sitemap path segment
		// and is stored verbatim: `fault`, `bom risks`, and `qds` all filter on
		// it, and it is the only classification the vendor publishes.
		//
		// No normalized match key is stored on purpose. Fault-string matching
		// folds punctuation at query time, and a stored key would silently go
		// stale the first time that folding is improved.
		`CREATE TABLE IF NOT EXISTS qsys_support (
			url        TEXT PRIMARY KEY,
			category   TEXT NOT NULL DEFAULT '',
			slug       TEXT NOT NULL DEFAULT '',
			title      TEXT NOT NULL DEFAULT '',
			body       TEXT NOT NULL DEFAULT '',
			synced_at  TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_qsys_support_category ON qsys_support(category)`,

		// Harvest bookkeeping. `coverage` reads this to report how much of each
		// site actually parsed, so a silent extraction regression is visible as
		// a number instead of an empty result set.
		`CREATE TABLE IF NOT EXISTS qsys_harvest (
			source        TEXT PRIMARY KEY,
			attempted     INTEGER NOT NULL DEFAULT 0,
			succeeded     INTEGER NOT NULL DEFAULT 0,
			with_specs    INTEGER NOT NULL DEFAULT 0,
			last_error    TEXT NOT NULL DEFAULT '',
			finished_at   TEXT NOT NULL DEFAULT ''
		)`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS qsys_pages_fts USING fts5(
			url UNINDEXED, section UNINDEXED, title, body
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS qsys_products_fts USING fts5(
			model UNINDEXED, family UNINDEXED, overview, spec_text
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS qsys_support_fts USING fts5(
			url UNINDEXED, category UNINDEXED, title, body
		)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("qsys schema: %w", err)
		}
	}
	return nil
}

// SearchCorpus runs a full-text query across the harvest-built Q-SYS corpus
// (help pages + product pages) and returns one JSON object per match. The
// corpus lives in qsys_pages/qsys_products with its own FTS indexes, separate
// from the generic resources table, so the generated db.Search cannot see it;
// the search command calls this alongside db.Search and dedups.
//
// Tables missing (corpus not harvested yet) are skipped, not errors, so a
// search on a fresh database returns only whatever is actually indexed.
func (s *Store) SearchCorpus(query string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	matchQuery := FTSMatchQuery(query)
	if matchQuery == "" {
		return nil, nil
	}
	out := make([]json.RawMessage, 0, limit)

	prows, err := s.db.Query(
		`SELECT url, section, title, snippet(qsys_pages_fts, 3, '[', ']', ' ... ', 12)
		 FROM qsys_pages_fts WHERE qsys_pages_fts MATCH ? ORDER BY rank LIMIT ?`,
		matchQuery, limit)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	for prows.Next() {
		var url, section, title, snip string
		if err := prows.Scan(&url, &section, &title, &snip); err != nil {
			_ = prows.Close()
			return nil, err
		}
		b, err := json.Marshal(map[string]any{
			"resource_type": "page",
			"url":           url,
			"section":       section,
			"title":         title,
			"snippet":       snip,
		})
		if err != nil {
			_ = prows.Close()
			return nil, err
		}
		out = append(out, b)
	}
	if err := prows.Err(); err != nil {
		_ = prows.Close()
		return nil, err
	}
	_ = prows.Close()

	qrows, err := s.db.Query(
		`SELECT p.model, p.title, p.family, snippet(qsys_products_fts, 3, '[', ']', ' ... ', 12)
		 FROM qsys_products_fts
		 JOIN qsys_products p ON p.model = qsys_products_fts.model
		 WHERE qsys_products_fts MATCH ? ORDER BY rank LIMIT ?`,
		matchQuery, limit)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return out, nil
		}
		return nil, err
	}
	for qrows.Next() {
		var model, title, family, snip string
		if err := qrows.Scan(&model, &title, &family, &snip); err != nil {
			_ = qrows.Close()
			return nil, err
		}
		b, err := json.Marshal(map[string]any{
			"resource_type": "product",
			"model":         model,
			"title":         title,
			"family":        family,
			"snippet":       snip,
		})
		if err != nil {
			_ = qrows.Close()
			return nil, err
		}
		out = append(out, b)
	}
	if err := qrows.Err(); err != nil {
		_ = qrows.Close()
		return nil, err
	}
	_ = qrows.Close()

	srows, err := s.db.Query(
		`SELECT url, category, title, snippet(qsys_support_fts, 3, '[', ']', ' ... ', 12)
		 FROM qsys_support_fts WHERE qsys_support_fts MATCH ? ORDER BY rank LIMIT ?`,
		matchQuery, limit)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return out, nil
		}
		return nil, err
	}
	for srows.Next() {
		var url, category, title, snip string
		if err := srows.Scan(&url, &category, &title, &snip); err != nil {
			_ = srows.Close()
			return nil, err
		}
		b, err := json.Marshal(map[string]any{
			"resource_type": "support_article",
			"url":           url,
			"category":      category,
			"title":         title,
			"snippet":       snip,
		})
		if err != nil {
			_ = srows.Close()
			return nil, err
		}
		out = append(out, b)
	}
	if err := srows.Err(); err != nil {
		_ = srows.Close()
		return nil, err
	}
	_ = srows.Close()
	return out, nil
}
