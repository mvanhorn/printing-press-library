// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored schema for public-catalog snapshots.
//
// Kept in its own file (not the generated migration slice in store.go) so
// `generate --force` preserves it.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// CatalogSnapshotRow is one product as observed on a public store at a point in
// time. Prices are MAJOR units (dollars, not cents) — the Store API delivers
// minor units and the caller normalizes before inserting.
type CatalogSnapshotRow struct {
	StoreURL     string  `json:"store_url"`
	ProductID    string  `json:"product_id"`
	SnapshotAt   string  `json:"snapshot_at"`
	Name         string  `json:"name"`
	SKU          string  `json:"sku"`
	Permalink    string  `json:"permalink"`
	Currency     string  `json:"currency"`
	Price        float64 `json:"price"`
	RegularPrice float64 `json:"regular_price"`
	SalePrice    float64 `json:"sale_price"`
	OnSale       bool    `json:"on_sale"`
	InStock      bool    `json:"is_in_stock"`
	// Truncated marks a snapshot that hit its page cap. Membership deltas
	// (added/removed) are meaningless against a partial snapshot.
	Truncated bool     `json:"truncated"`
	_         struct{} // discourage positional literals
}

// EnsureCatalogSnapshots creates the snapshot table on first use.
func (s *Store) EnsureCatalogSnapshots(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS catalog_snapshots (
			store_url     TEXT NOT NULL,
			product_id    TEXT NOT NULL,
			snapshot_at   TEXT NOT NULL,
			name          TEXT,
			sku           TEXT,
			permalink     TEXT,
			currency      TEXT,
			price         REAL,
			regular_price REAL,
			sale_price    REAL,
			on_sale       INTEGER,
			is_in_stock   INTEGER,
			truncated     INTEGER DEFAULT 0,
			PRIMARY KEY (store_url, product_id, snapshot_at)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_catalog_snapshots_store_time
			ON catalog_snapshots(store_url, snapshot_at)`,
	}
	for _, stmt := range stmts {
		if _, err := s.DB().ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("creating catalog_snapshots: %w", err)
		}
	}
	// Additive migration for tables created before the truncation flag existed.
	// SQLite has no IF NOT EXISTS for columns, so a duplicate-column error here
	// is the expected no-op.
	if _, err := s.DB().ExecContext(ctx, `ALTER TABLE catalog_snapshots ADD COLUMN truncated INTEGER DEFAULT 0`); err != nil &&
		!strings.Contains(err.Error(), "duplicate column") {
		return fmt.Errorf("adding catalog_snapshots.truncated: %w", err)
	}
	return nil
}

// InsertCatalogSnapshot writes one snapshot in a single transaction.
//
// It deliberately does NOT call Upsert/UpsertBatch: those open their own write
// transaction and SQLite WAL permits only one writer at a time.
func (s *Store) InsertCatalogSnapshot(ctx context.Context, rows []CatalogSnapshotRow) error {
	if len(rows) == 0 {
		return nil
	}
	if err := s.EnsureCatalogSnapshots(ctx); err != nil {
		return err
	}
	tx, err := s.DB().BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning snapshot transaction: %w", err)
	}
	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO catalog_snapshots
			(store_url, product_id, snapshot_at, name, sku, permalink, currency,
			 price, regular_price, sale_price, on_sale, is_in_stock, truncated)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("preparing snapshot insert: %w", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.ExecContext(ctx, r.StoreURL, r.ProductID, r.SnapshotAt, r.Name, r.SKU,
			r.Permalink, r.Currency, r.Price, r.RegularPrice, r.SalePrice,
			boolToInt(r.OnSale), boolToInt(r.InStock), boolToInt(r.Truncated)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("inserting snapshot row for product %s: %w", r.ProductID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing snapshot: %w", err)
	}
	return nil
}

// CatalogSnapshotTimes lists a store's snapshot timestamps, newest first.
func (s *Store) CatalogSnapshotTimes(ctx context.Context, storeURL string) ([]string, error) {
	if err := s.EnsureCatalogSnapshots(ctx); err != nil {
		return nil, err
	}
	rows, err := s.DB().QueryContext(ctx,
		`SELECT DISTINCT snapshot_at FROM catalog_snapshots WHERE store_url = ? ORDER BY snapshot_at DESC`, storeURL)
	if err != nil {
		return nil, fmt.Errorf("listing snapshots: %w", err)
	}
	out := make([]string, 0)
	for rows.Next() {
		var at string
		if err := rows.Scan(&at); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning snapshot time: %w", err)
		}
		out = append(out, at)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating snapshot times: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing snapshot time rows: %w", err)
	}
	return out, nil
}

// CatalogSnapshotAt returns every row of one snapshot, fully drained.
func (s *Store) CatalogSnapshotAt(ctx context.Context, storeURL, snapshotAt string) ([]CatalogSnapshotRow, error) {
	if err := s.EnsureCatalogSnapshots(ctx); err != nil {
		return nil, err
	}
	rows, err := s.DB().QueryContext(ctx, `
		SELECT product_id, COALESCE(name,''), COALESCE(sku,''), COALESCE(permalink,''),
		       COALESCE(currency,''), COALESCE(price,0), COALESCE(regular_price,0),
		       COALESCE(sale_price,0), COALESCE(on_sale,0), COALESCE(is_in_stock,0),
		       COALESCE(truncated,0)
		FROM catalog_snapshots WHERE store_url = ? AND snapshot_at = ?`, storeURL, snapshotAt)
	if err != nil {
		return nil, fmt.Errorf("reading snapshot: %w", err)
	}
	out := make([]CatalogSnapshotRow, 0)
	for rows.Next() {
		var r CatalogSnapshotRow
		var onSale, inStock, truncated sql.NullInt64
		if err := rows.Scan(&r.ProductID, &r.Name, &r.SKU, &r.Permalink, &r.Currency,
			&r.Price, &r.RegularPrice, &r.SalePrice, &onSale, &inStock, &truncated); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("scanning snapshot row: %w", err)
		}
		r.StoreURL, r.SnapshotAt = storeURL, snapshotAt
		r.OnSale, r.InStock = onSale.Int64 == 1, inStock.Int64 == 1
		r.Truncated = truncated.Int64 == 1
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("iterating snapshot rows: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("closing snapshot rows: %w", err)
	}
	return out, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
