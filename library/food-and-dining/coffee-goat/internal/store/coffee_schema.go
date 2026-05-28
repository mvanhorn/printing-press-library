// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

// coffee_schema.go is the hand-authored schema layer for the
// coffee-goat domain. It augments the generator-emitted resources
// and products tables with the second-corpus + personal-history
// shape declared in research/prior-brief.md (Data Layer).
//
// All tables are idempotent (CREATE ... IF NOT EXISTS); the FTS5
// triggers are likewise IF NOT EXISTS so the migration is safe to
// re-run on existing databases. Tables index on natural keys so
// repeated sync runs upsert correctly.
//
// Wired into the generator migrate path via EnsureCoffeeSchema,
// called once after every Open/OpenWithContext. Idempotent.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

// coffeeSchemaStatements is the full set of DDL the coffee-goat
// domain depends on. Order matters: tables before the FTS5 virtual
// tables that reference them, and FTS5 virtual tables before the
// triggers that hang off them. Repeat-safe via IF NOT EXISTS on
// every statement.
var coffeeSchemaStatements = []string{
	// --- roasters: 24-roaster registry projected into the store
	`CREATE TABLE IF NOT EXISTS roasters (
		slug TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		country TEXT,
		transport TEXT,
		sync_url TEXT,
		last_synced_at DATETIME,
		last_status TEXT
	)`,

	// --- roaster_products: unified Product shape across all 24 sources
	`CREATE TABLE IF NOT EXISTS roaster_products (
		roaster_slug TEXT NOT NULL,
		handle TEXT NOT NULL,
		title TEXT,
		vendor TEXT,
		body_text TEXT,
		origin TEXT,
		producer TEXT,
		process TEXT,
		varietal TEXT,
		altitude TEXT,
		roast_level TEXT,
		tags_json TEXT,
		price_cents INTEGER,
		currency TEXT,
		weight_g INTEGER,
		url TEXT,
		image_url TEXT,
		in_stock INTEGER,
		published_at TEXT,
		updated_at TEXT,
		first_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		PRIMARY KEY (roaster_slug, handle)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_roaster_products_origin ON roaster_products(origin)`,
	`CREATE INDEX IF NOT EXISTS idx_roaster_products_process ON roaster_products(process)`,
	`CREATE INDEX IF NOT EXISTS idx_roaster_products_in_stock ON roaster_products(in_stock)`,
	`CREATE INDEX IF NOT EXISTS idx_roaster_products_first_seen ON roaster_products(first_seen_at)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS roaster_products_fts USING fts5(
		title, body_text, origin, producer, varietal, tags_json,
		tokenize='porter unicode61'
	)`,

	// --- reviews: Coffee Review WP REST + RSS-derived rows
	`CREATE TABLE IF NOT EXISTS reviews (
		id TEXT PRIMARY KEY,
		source TEXT NOT NULL,
		source_url TEXT,
		roaster_name TEXT,
		bean_name TEXT,
		score INTEGER,
		descriptors_json TEXT,
		published_at TEXT,
		reviewer TEXT,
		raw_json TEXT,
		last_seen_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_reviews_roaster ON reviews(roaster_name)`,
	`CREATE INDEX IF NOT EXISTS idx_reviews_score ON reviews(score)`,

	// --- youtube_reviews: Hoffmann/Hedrick transcripts via youtube-pp-cli
	`CREATE TABLE IF NOT EXISTS youtube_reviews (
		video_id TEXT PRIMARY KEY,
		creator TEXT NOT NULL,
		channel_id TEXT,
		video_title TEXT,
		video_published_at TEXT,
		transcript_text TEXT,
		mentioned_roaster_slugs_json TEXT,
		mentioned_bean_handles_json TEXT,
		last_synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_youtube_reviews_creator ON youtube_reviews(creator)`,
	`CREATE INDEX IF NOT EXISTS idx_youtube_reviews_published ON youtube_reviews(video_published_at)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS youtube_reviews_fts USING fts5(
		video_title, transcript_text,
		tokenize='porter unicode61'
	)`,

	// --- beans: personal cellar
	`CREATE TABLE IF NOT EXISTS beans (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		product_slug TEXT,
		roaster_slug TEXT,
		roast_date TEXT,
		purchase_date TEXT,
		price_paid_cents INTEGER,
		current_mass_g INTEGER,
		notes TEXT,
		added_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,
	`CREATE INDEX IF NOT EXISTS idx_beans_roaster ON beans(roaster_slug)`,
	`CREATE INDEX IF NOT EXISTS idx_beans_product ON beans(product_slug)`,

	// --- brews: brew log with FK to beans
	`CREATE TABLE IF NOT EXISTS brews (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		bean_id INTEGER,
		method TEXT,
		grind TEXT,
		dose_g REAL,
		yield_g REAL,
		time_s INTEGER,
		temperature_c REAL,
		water_tds_ppm INTEGER,
		rating INTEGER,
		notes TEXT,
		descriptors_json TEXT,
		brewed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (bean_id) REFERENCES beans(id) ON DELETE CASCADE
	)`,
	`CREATE INDEX IF NOT EXISTS idx_brews_bean ON brews(bean_id)`,
	`CREATE INDEX IF NOT EXISTS idx_brews_method ON brews(method)`,
	`CREATE INDEX IF NOT EXISTS idx_brews_rating ON brews(rating)`,

	// --- watchlist: persistent saved queries
	`CREATE TABLE IF NOT EXISTS watchlist (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT UNIQUE,
		query TEXT NOT NULL,
		filters_json TEXT,
		last_sync_anchor DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`,

	// --- palate_profiles: friend-pick imported signatures
	`CREATE TABLE IF NOT EXISTS palate_profiles (
		name TEXT PRIMARY KEY,
		signature_json TEXT NOT NULL,
		imported_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		source TEXT
	)`,

	// --- coffee_sync_state: per-source sync orchestration (distinct from
	// the generator's sync_state which keys on resource_type, not source).
	`CREATE TABLE IF NOT EXISTS coffee_sync_state (
		source TEXT PRIMARY KEY,
		last_synced_at DATETIME,
		last_status TEXT,
		item_count INTEGER DEFAULT 0
	)`,
}

// EnsureCoffeeSchema runs the coffee-goat-specific DDL exactly once
// against the connection. Idempotent — every statement is IF NOT
// EXISTS — so it is safe to call from Open() at every process start.
//
// The function takes a *Store (not *sql.Conn) because the schema
// touches multiple tables, FTS5 virtual tables, and indexes; running
// it on the connection pool lets SQLite serialize internally rather
// than holding a single pinned connection across every CREATE
// statement.
func (s *Store) EnsureCoffeeSchema(ctx context.Context) error {
	for _, stmt := range coffeeSchemaStatements {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("coffee schema migration failed on %q: %w", firstLine(stmt), err)
		}
	}
	return nil
}

// firstLine returns the first non-empty trimmed line of stmt for use
// in error messages. SQLite errors swallow the full DDL otherwise.
func firstLine(stmt string) string {
	for i := 0; i < len(stmt); i++ {
		if stmt[i] == '\n' {
			return stmt[:i]
		}
	}
	return stmt
}

// UpsertRoasterProduct writes (or updates) one product row. Called by
// every source adapter. first_seen_at is preserved on conflict so
// the "new since watch" anchor stays stable across syncs; only
// last_seen_at and the live attribute set advance.
func (s *Store) UpsertRoasterProduct(roasterSlug, handle string, fields map[string]any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	cols := []string{"roaster_slug", "handle"}
	vals := []any{roasterSlug, handle}
	placeholders := []string{"?", "?"}
	updates := []string{}

	allowed := map[string]bool{
		"title": true, "vendor": true, "body_text": true,
		"origin": true, "producer": true, "process": true, "varietal": true,
		"altitude": true, "roast_level": true, "tags_json": true,
		"price_cents": true, "currency": true, "weight_g": true,
		"url": true, "image_url": true, "in_stock": true,
		"published_at": true, "updated_at": true,
	}
	for k, v := range fields {
		if !allowed[k] {
			continue
		}
		cols = append(cols, k)
		vals = append(vals, v)
		placeholders = append(placeholders, "?")
		updates = append(updates, fmt.Sprintf("%s=excluded.%s", k, k))
	}
	updates = append(updates, "last_seen_at=CURRENT_TIMESTAMP")

	q := fmt.Sprintf(
		`INSERT INTO roaster_products (%s) VALUES (%s)
		 ON CONFLICT(roaster_slug, handle) DO UPDATE SET %s`,
		joinSimple(cols), joinSimple(placeholders), joinSimple(updates),
	)
	// Wrap the upsert + FTS DELETE + FTS INSERT in a single transaction.
	// Without this, a failure between FTS DELETE and FTS INSERT (e.g.
	// SQLITE_FULL on disk pressure) would leave the product row present
	// in roaster_products but absent from roaster_products_fts, and
	// subsequent search/watch queries would silently miss it until the
	// next sync of that roaster overwrites the gap. The whole flow has
	// to commit atomically or roll back.
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("upsert roaster_product %s/%s begin: %w", roasterSlug, handle, err)
	}
	defer func() {
		_ = tx.Rollback() // No-op after a successful Commit.
	}()
	if _, err := tx.Exec(q, vals...); err != nil {
		return fmt.Errorf("upsert roaster_product %s/%s: %w", roasterSlug, handle, err)
	}
	// Mirror into FTS5. roaster_products_fts is content-less (no
	// content=table binding), so we DELETE+INSERT to keep it in sync
	// after upserts.
	if _, err := tx.Exec(
		`DELETE FROM roaster_products_fts WHERE rowid IN (
			SELECT rowid FROM roaster_products WHERE roaster_slug=? AND handle=?
		)`,
		roasterSlug, handle,
	); err != nil {
		return fmt.Errorf("roaster_products_fts cleanup: %w", err)
	}
	if _, err := tx.Exec(
		`INSERT INTO roaster_products_fts (rowid, title, body_text, origin, producer, varietal, tags_json)
		 SELECT rowid, title, body_text, origin, producer, varietal, tags_json
		 FROM roaster_products WHERE roaster_slug=? AND handle=?`,
		roasterSlug, handle,
	); err != nil {
		return fmt.Errorf("roaster_products_fts insert: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("upsert roaster_product %s/%s commit: %w", roasterSlug, handle, err)
	}
	return nil
}

// RoasterProductHit is a domain-specific row returned by
// SearchRoasterProducts. Keeping it in the store package so CLI callers
// don't have to re-scan the FTS join themselves.
type RoasterProductHit struct {
	Roaster    string
	Handle     string
	Title      string
	Origin     string
	Process    string
	PriceCents int
	Currency   string
	WeightG    int
	InStock    bool
	URL        string
}

// SearchRoasterProducts runs an FTS5 match across the synced cross-roaster
// catalog. This is the domain-specific FTS path; the generic Search()
// queries the legacy "products" table which coffee-goat's sync flow does
// not populate.
func (s *Store) SearchRoasterProducts(query string, limit int) ([]RoasterProductHit, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(
		`SELECT rp.roaster_slug, rp.handle, COALESCE(rp.title,''), COALESCE(rp.origin,''),
		        COALESCE(rp.process,''), COALESCE(rp.price_cents,0), COALESCE(rp.currency,''),
		        COALESCE(rp.weight_g,0), COALESCE(rp.in_stock,0), COALESCE(rp.url,'')
		 FROM roaster_products rp
		 JOIN roaster_products_fts ON roaster_products_fts.rowid = rp.rowid
		 WHERE roaster_products_fts MATCH ?
		 ORDER BY rank LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RoasterProductHit
	for rows.Next() {
		var h RoasterProductHit
		var inStockInt int
		if err := rows.Scan(&h.Roaster, &h.Handle, &h.Title, &h.Origin, &h.Process,
			&h.PriceCents, &h.Currency, &h.WeightG, &inStockInt, &h.URL); err != nil {
			return nil, err
		}
		h.InStock = inStockInt == 1
		out = append(out, h)
	}
	return out, rows.Err()
}

// joinSimple joins string slices with ", " — avoids dragging strings
// into this file just for one helper.
func joinSimple(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for i := 1; i < len(parts); i++ {
		out += ", " + parts[i]
	}
	return out
}

// SaveCoffeeSyncState records the last sync verdict for a named
// source ("shopify", "coffee-review", "youtube", or one of the
// individual roaster slugs). Distinct from the generator's
// SaveSyncState which is keyed on resource_type.
func (s *Store) SaveCoffeeSyncState(source, status string, itemCount int) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO coffee_sync_state (source, last_synced_at, last_status, item_count)
		 VALUES (?, CURRENT_TIMESTAMP, ?, ?)
		 ON CONFLICT(source) DO UPDATE SET
		   last_synced_at=CURRENT_TIMESTAMP,
		   last_status=excluded.last_status,
		   item_count=excluded.item_count`,
		source, status, itemCount,
	)
	return err
}

// LastCoffeeSyncAt returns the timestamp of the most recent successful
// sync for a source, or zero-value when never synced. Used by the
// YouTube adapter to skip videos older than the last cursor.
func (s *Store) LastCoffeeSyncAt(source string) (sql.NullString, error) {
	var ts sql.NullString
	err := s.db.QueryRow(
		`SELECT last_synced_at FROM coffee_sync_state WHERE source=?`,
		source,
	).Scan(&ts)
	if err == sql.ErrNoRows {
		return sql.NullString{}, nil
	}
	return ts, err
}
