// Package crestronstore owns the Crestron-specific tables in the CLI's local
// SQLite database.
//
// The generator emits sync and full-text search only for JSON endpoints, and
// every Crestron surface is server-rendered HTML, so this package supplies the
// local mirror by hand. It manages its own tables and its own migrations, and
// never writes through the generated store helpers, so a regeneration cannot
// clobber it.
package crestronstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	// This package calls sql.Open("sqlite") directly, so it owns the driver
	// registration rather than relying on another package importing it first.
	_ "modernc.org/sqlite"
)

// Product is a catalog product row.
type Product struct {
	Model        string `json:"model"`
	Description  string `json:"description,omitempty"`
	URL          string `json:"url,omitempty"`
	DocumentID   string `json:"document_id,omitempty"`
	CategoryPath string `json:"category_path,omitempty"`
	SKU          string `json:"sku,omitempty"`
	ImageURL     string `json:"image_url,omitempty"`
	Discontinued bool   `json:"discontinued"`
	SyncedAt     string `json:"synced_at,omitempty"`
}

// Release is a firmware or software release row.
type Release struct {
	URL       string   `json:"url"`
	Title     string   `json:"title"`
	Version   string   `json:"version,omitempty"`
	Date      string   `json:"date,omitempty"`
	Type      string   `json:"type,omitempty"`
	Models    []string `json:"models,omitempty"`
	Notes     string   `json:"release_notes,omitempty"`
	ChangeLog string   `json:"change_log,omitempty"`
	SyncedAt  string   `json:"synced_at,omitempty"`
}

// Category is a catalog category row.
type Category struct {
	Path         string `json:"path"`
	DocumentID   string `json:"document_id,omitempty"`
	NodeID       string `json:"node_id,omitempty"`
	ProductCount int    `json:"product_count"`
	SyncedAt     string `json:"synced_at,omitempty"`
}

// Store wraps the CLI's SQLite database with the Crestron-owned tables.
type Store struct {
	db *sql.DB
}

// Open opens the database at path and applies the Crestron migrations.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the database handle.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the handle for read-only queries in novel commands.
func (s *Store) DB() *sql.DB { return s.db }

// migrations are additive and idempotent so re-running sync is always safe.
var migrations = []string{
	`CREATE TABLE IF NOT EXISTS crestron_products (
		model         TEXT PRIMARY KEY,
		description   TEXT NOT NULL DEFAULT '',
		url           TEXT NOT NULL DEFAULT '',
		document_id   TEXT NOT NULL DEFAULT '',
		category_path TEXT NOT NULL DEFAULT '',
		sku           TEXT NOT NULL DEFAULT '',
		image_url     TEXT NOT NULL DEFAULT '',
		discontinued  INTEGER NOT NULL DEFAULT 0,
		synced_at     TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS crestron_categories (
		path          TEXT PRIMARY KEY,
		document_id   TEXT NOT NULL DEFAULT '',
		node_id       TEXT NOT NULL DEFAULT '',
		product_count INTEGER NOT NULL DEFAULT 0,
		synced_at     TEXT NOT NULL DEFAULT ''
	)`,
	`CREATE TABLE IF NOT EXISTS crestron_releases (
		url        TEXT PRIMARY KEY,
		title      TEXT NOT NULL DEFAULT '',
		version    TEXT NOT NULL DEFAULT '',
		date       TEXT NOT NULL DEFAULT '',
		type       TEXT NOT NULL DEFAULT '',
		models     TEXT NOT NULL DEFAULT '[]',
		notes      TEXT NOT NULL DEFAULT '',
		change_log TEXT NOT NULL DEFAULT '',
		synced_at  TEXT NOT NULL DEFAULT ''
	)`,
	// The many-to-many join that makes "which release covers model X" answerable.
	// One release row can name seven models; the site never exposes this mapping.
	`CREATE TABLE IF NOT EXISTS crestron_release_models (
		model       TEXT NOT NULL,
		release_url TEXT NOT NULL,
		PRIMARY KEY (model, release_url)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_release_models_model ON crestron_release_models(model)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS crestron_releases_fts USING fts5(
		url UNINDEXED, title, version, notes, change_log,
		content='crestron_releases', content_rowid='rowid'
	)`,
	`CREATE VIRTUAL TABLE IF NOT EXISTS crestron_products_fts USING fts5(
		model UNINDEXED, description, sku,
		content='crestron_products', content_rowid='rowid'
	)`,
}

func (s *Store) migrate(ctx context.Context) error {
	for _, stmt := range migrations {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migration failed (%.60s): %w", stmt, err)
		}
	}
	return nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339) }

// UpsertProducts writes products in a single transaction.
func (s *Store) UpsertProducts(ctx context.Context, products []Product) error {
	if len(products) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO crestron_products
			(model, description, url, document_id, category_path, sku, image_url, discontinued, synced_at)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(model) DO UPDATE SET
			description   = CASE WHEN excluded.description   != '' THEN excluded.description   ELSE crestron_products.description   END,
			url           = CASE WHEN excluded.url           != '' THEN excluded.url           ELSE crestron_products.url           END,
			document_id   = CASE WHEN excluded.document_id   != '' THEN excluded.document_id   ELSE crestron_products.document_id   END,
			category_path = CASE WHEN excluded.category_path != '' THEN excluded.category_path ELSE crestron_products.category_path END,
			sku           = CASE WHEN excluded.sku           != '' THEN excluded.sku           ELSE crestron_products.sku           END,
			image_url     = CASE WHEN excluded.image_url     != '' THEN excluded.image_url     ELSE crestron_products.image_url     END,
			discontinued  = excluded.discontinued,
			synced_at     = excluded.synced_at`)
	if err != nil {
		return err
	}
	defer func() { _ = stmt.Close() }()

	ts := now()
	for _, p := range products {
		disc := 0
		if p.Discontinued {
			disc = 1
		}
		if _, err := stmt.ExecContext(ctx, p.Model, p.Description, p.URL, p.DocumentID,
			p.CategoryPath, p.SKU, p.ImageURL, disc, ts); err != nil {
			return fmt.Errorf("upserting product %q: %w", p.Model, err)
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO crestron_products_fts(crestron_products_fts) VALUES('rebuild')`); err != nil {
		return fmt.Errorf("rebuilding product index: %w", err)
	}
	return tx.Commit()
}

// UpsertCategories writes catalog categories.
func (s *Store) UpsertCategories(ctx context.Context, cats []Category) error {
	if len(cats) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	ts := now()
	for _, c := range cats {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO crestron_categories (path, document_id, node_id, product_count, synced_at)
			VALUES (?,?,?,?,?)
			ON CONFLICT(path) DO UPDATE SET
				document_id   = excluded.document_id,
				node_id       = excluded.node_id,
				product_count = excluded.product_count,
				synced_at     = excluded.synced_at`,
			c.Path, c.DocumentID, c.NodeID, c.ProductCount, ts); err != nil {
			return fmt.Errorf("upserting category %q: %w", c.Path, err)
		}
	}
	return tx.Commit()
}

// UpsertReleases writes releases and rebuilds the model join for each.
func (s *Store) UpsertReleases(ctx context.Context, releases []Release) error {
	if len(releases) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	ts := now()
	for _, r := range releases {
		modelsJSON, err := json.Marshal(r.Models)
		if err != nil {
			return fmt.Errorf("encoding models for %q: %w", r.Title, err)
		}
		// Notes and change log arrive only on an authenticated fetch, so an
		// empty value must never overwrite text a previous sync captured.
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO crestron_releases (url, title, version, date, type, models, notes, change_log, synced_at)
			VALUES (?,?,?,?,?,?,?,?,?)
			ON CONFLICT(url) DO UPDATE SET
				title      = excluded.title,
				version    = excluded.version,
				date       = excluded.date,
				type       = excluded.type,
				models     = excluded.models,
				notes      = CASE WHEN excluded.notes      != '' THEN excluded.notes      ELSE crestron_releases.notes      END,
				change_log = CASE WHEN excluded.change_log != '' THEN excluded.change_log ELSE crestron_releases.change_log END,
				synced_at  = excluded.synced_at`,
			r.URL, r.Title, r.Version, r.Date, r.Type, string(modelsJSON), r.Notes, r.ChangeLog, ts); err != nil {
			return fmt.Errorf("upserting release %q: %w", r.Title, err)
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM crestron_release_models WHERE release_url = ?`, r.URL); err != nil {
			return err
		}
		for _, m := range r.Models {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO crestron_release_models (model, release_url) VALUES (?,?)`,
				m, r.URL); err != nil {
				return err
			}
		}
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO crestron_releases_fts(crestron_releases_fts) VALUES('rebuild')`); err != nil {
		return fmt.Errorf("rebuilding release index: %w", err)
	}
	return tx.Commit()
}

// Counts reports how many rows each Crestron table holds.
func (s *Store) Counts(ctx context.Context) (map[string]int, error) {
	out := map[string]int{}
	for _, t := range []string{"crestron_products", "crestron_categories", "crestron_releases", "crestron_release_models"} {
		var n int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+t).Scan(&n); err != nil {
			return out, err
		}
		out[strings.TrimPrefix(t, "crestron_")] = n
	}
	return out, nil
}

// LastSync returns the newest synced_at across the Crestron tables.
func (s *Store) LastSync(ctx context.Context) (string, error) {
	var v sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT MAX(t) FROM (
			SELECT MAX(synced_at) AS t FROM crestron_products
			UNION ALL SELECT MAX(synced_at) FROM crestron_releases
			UNION ALL SELECT MAX(synced_at) FROM crestron_categories
		)`).Scan(&v)
	if err != nil {
		return "", err
	}
	return v.String, nil
}
