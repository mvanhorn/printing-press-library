// Custom Craigslist tables that extend the generated schema. We don't edit
// store.go (DO NOT EDIT) so the migrations live in this sibling file. Callers
// invoke EnsureCLTables(ctx) after Open() before querying.
//
// Schema:
//   listings           — one row per posting we've ever seen (source of truth)
//   listing_images     — image refs per posting
//   listing_snapshots  — price/title history; powers drift, repost, dup
//   areas, categories  — reference taxonomies (refreshed by `catalog refresh`)
//   saved_searches     — local-only watch definitions
//   seen_listings      — per-watch "alerted on this pid already" markers
//   favorites          — local pinned listings with notes

package store

import (
	"context"
	"fmt"
)

// EnsureCLTables creates the Craigslist-specific tables if they don't exist.
// Idempotent — every CREATE uses IF NOT EXISTS.
func (s *Store) EnsureCLTables(ctx context.Context) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS listings (
			pid          INTEGER PRIMARY KEY,
			uuid         TEXT NOT NULL,
			site         TEXT NOT NULL,
			subarea      TEXT,
			neighborhood TEXT,
			category_abbr TEXT,
			category_id  INTEGER,
			title        TEXT NOT NULL,
			body         TEXT,
			body_text    TEXT,
			price        INTEGER,
			price_display TEXT,
			lat          REAL,
			lng          REAL,
			image_count  INTEGER DEFAULT 0,
			images_json  TEXT,
			attributes_json TEXT,
			canonical_url TEXT,
			slug         TEXT,
			posted_at    INTEGER,
			updated_at   INTEGER,
			first_seen_at INTEGER NOT NULL,
			last_seen_at INTEGER NOT NULL,
			status       TEXT DEFAULT 'active'
		)`,
		`CREATE INDEX IF NOT EXISTS idx_listings_site_cat ON listings(site, category_abbr)`,
		`CREATE INDEX IF NOT EXISTS idx_listings_uuid ON listings(uuid)`,
		`CREATE INDEX IF NOT EXISTS idx_listings_posted_at ON listings(posted_at)`,
		`CREATE INDEX IF NOT EXISTS idx_listings_price ON listings(price)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS listings_fts USING fts5(
			title,
			body_text,
			attributes_text,
			content='',
			tokenize='porter'
		)`,
		`CREATE TABLE IF NOT EXISTS listing_images (
			pid INTEGER NOT NULL,
			idx INTEGER NOT NULL,
			image_id TEXT NOT NULL,
			PRIMARY KEY (pid, idx)
		)`,
		`CREATE TABLE IF NOT EXISTS listing_snapshots (
			pid          INTEGER NOT NULL,
			observed_at  INTEGER NOT NULL,
			price        INTEGER,
			title        TEXT,
			body_hash    TEXT,
			status       TEXT DEFAULT 'active',
			PRIMARY KEY (pid, observed_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_pid ON listing_snapshots(pid)`,
		`CREATE TABLE IF NOT EXISTS cl_areas (
			area_id      INTEGER PRIMARY KEY,
			abbreviation TEXT,
			hostname     TEXT NOT NULL,
			country      TEXT,
			region       TEXT,
			description  TEXT,
			short_description TEXT,
			lat          REAL,
			lng          REAL,
			timezone     TEXT,
			parent_area_id INTEGER,
			refreshed_at INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_areas_hostname ON cl_areas(hostname)`,
		`CREATE INDEX IF NOT EXISTS idx_areas_country_region ON cl_areas(country, region)`,
		`CREATE TABLE IF NOT EXISTS cl_categories (
			category_id  INTEGER PRIMARY KEY,
			abbreviation TEXT NOT NULL,
			description  TEXT,
			type         TEXT,
			refreshed_at INTEGER
		)`,
		`CREATE INDEX IF NOT EXISTS idx_categories_abbr ON cl_categories(abbreviation)`,
		`CREATE TABLE IF NOT EXISTS saved_searches (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			name         TEXT UNIQUE NOT NULL,
			query        TEXT,
			sites_csv    TEXT NOT NULL,
			category     TEXT NOT NULL DEFAULT 'sss',
			min_price    INTEGER,
			max_price    INTEGER,
			has_pic      INTEGER DEFAULT 0,
			postal       TEXT,
			search_distance INTEGER,
			title_only   INTEGER DEFAULT 0,
			negate_csv   TEXT,
			created_at   INTEGER NOT NULL,
			last_run_at  INTEGER
		)`,
		`CREATE TABLE IF NOT EXISTS seen_listings (
			watch_id    INTEGER NOT NULL,
			pid         INTEGER NOT NULL,
			first_alert_at INTEGER NOT NULL,
			last_price  INTEGER,
			last_title  TEXT,
			PRIMARY KEY (watch_id, pid)
		)`,
		`CREATE TABLE IF NOT EXISTS favorites (
			pid       INTEGER PRIMARY KEY,
			note      TEXT,
			added_at  INTEGER NOT NULL
		)`,
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ensure cl tables: begin: %w", err)
	}
	defer tx.Rollback()
	for _, m := range migrations {
		if _, err := tx.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("ensure cl tables: %w (migration: %s)", err, firstLine(m))
		}
	}
	return tx.Commit()
}

func firstLine(s string) string {
	for i, r := range s {
		if r == '\n' {
			return s[:i]
		}
	}
	return s
}
