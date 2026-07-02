// Copyright 2026 wandreis and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/mercadolivre/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/mercadolivre/internal/mlextract"
)

// PriceSnapshot is one row of the price_snapshot table.
type PriceSnapshot struct {
	CatalogID  string    `json:"catalog_id"`
	Price      float64   `json:"price"`
	Currency   string    `json:"currency"`
	CapturedAt time.Time `json:"captured_at"`
}

// UpsertAttributes replaces the stored technical-spec attributes for one
// catalog_id (DELETE then INSERT), so a re-scrape never leaves stale rows.
func (s *Store) UpsertAttributes(catalogID string, attrs []mlextract.Attribute) error {
	if catalogID == "" {
		return fmt.Errorf("UpsertAttributes: empty catalog_id")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM attribute WHERE catalog_id = ?`, catalogID); err != nil {
		return fmt.Errorf("clearing attributes for %s: %w", catalogID, err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, a := range attrs {
		if a.Name == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO attribute (catalog_id, name, value, synced_at) VALUES (?, ?, ?, ?)
			 ON CONFLICT(catalog_id, name) DO UPDATE SET value = excluded.value, synced_at = excluded.synced_at`,
			catalogID, a.Name, a.Value, now,
		); err != nil {
			return fmt.Errorf("inserting attribute %s/%s: %w", catalogID, a.Name, err)
		}
	}
	return tx.Commit()
}

// InsertPriceSnapshot appends one price observation for a catalog_id, stamped
// now (UTC, RFC3339 so it round-trips through cliutil.ParseStoredTime and
// sorts lexicographically by time).
func (s *Store) InsertPriceSnapshot(catalogID string, price float64, currency string) error {
	return s.InsertPriceSnapshotAt(catalogID, price, currency, time.Now().UTC())
}

// InsertPriceSnapshotAt is InsertPriceSnapshot with an explicit capture time.
// Exposed for tests that seed historical snapshots.
func (s *Store) InsertPriceSnapshotAt(catalogID string, price float64, currency string, at time.Time) error {
	if catalogID == "" {
		return fmt.Errorf("InsertPriceSnapshot: empty catalog_id")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO price_snapshot (catalog_id, price, currency, captured_at) VALUES (?, ?, ?, ?)`,
		catalogID, price, currency, at.UTC().Format(time.RFC3339),
	)
	return err
}

// ProductAttributes returns the stored spec attributes for a catalog_id as a
// name->value map.
func (s *Store) ProductAttributes(catalogID string) (map[string]string, error) {
	rows, err := s.db.Query(`SELECT name, value FROM attribute WHERE catalog_id = ? ORDER BY name`, catalogID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var name, value string
		if err := rows.Scan(&name, &value); err != nil {
			return nil, err
		}
		out[name] = value
	}
	return out, rows.Err()
}

// PriceSnapshots returns all snapshots for a catalog_id, oldest first. When
// since is non-zero, only snapshots at or after it are returned.
func (s *Store) PriceSnapshots(catalogID string, since time.Time) ([]PriceSnapshot, error) {
	q := `SELECT catalog_id, price, currency, captured_at FROM price_snapshot WHERE catalog_id = ?`
	args := []any{catalogID}
	if !since.IsZero() {
		q += ` AND captured_at >= ?`
		args = append(args, since.UTC().Format(time.RFC3339))
	}
	q += ` ORDER BY captured_at ASC`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []PriceSnapshot
	for rows.Next() {
		var ps PriceSnapshot
		var captured string
		if err := rows.Scan(&ps.CatalogID, &ps.Price, &ps.Currency, &captured); err != nil {
			return nil, err
		}
		ps.CapturedAt = cliutil.ParseStoredTime(captured)
		out = append(out, ps)
	}
	return out, rows.Err()
}

// LatestPriceSnapshot returns the newest snapshot for a catalog_id. ok=false
// when the catalog_id has no snapshots.
func (s *Store) LatestPriceSnapshot(catalogID string) (PriceSnapshot, bool, error) {
	var ps PriceSnapshot
	var captured string
	err := s.db.QueryRow(
		`SELECT catalog_id, price, currency, captured_at FROM price_snapshot
		 WHERE catalog_id = ? ORDER BY captured_at DESC LIMIT 1`, catalogID,
	).Scan(&ps.CatalogID, &ps.Price, &ps.Currency, &captured)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PriceSnapshot{}, false, nil
		}
		return PriceSnapshot{}, false, err
	}
	ps.CapturedAt = cliutil.ParseStoredTime(captured)
	return ps, true, nil
}

// CatalogPrices returns every observed price for a catalog_id, drawn from both
// price_snapshot rows and any listings rows sharing the catalog_id. Used by
// the dispersion command.
func (s *Store) CatalogPrices(catalogID string) ([]float64, error) {
	rows, err := s.db.Query(
		`SELECT price FROM price_snapshot WHERE catalog_id = ? AND price IS NOT NULL
		 UNION ALL
		 SELECT price FROM listings WHERE catalog_id = ? AND price IS NOT NULL`,
		catalogID, catalogID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []float64
	for rows.Next() {
		var p float64
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ProductBasic is the summary row the compare/cotacao commands need.
type ProductBasic struct {
	CatalogID string  `json:"catalog_id"`
	Name      string  `json:"name"`
	Brand     string  `json:"brand"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency"`
}

// GetProductBasic loads the typed products-table row for a catalog_id.
// ok=false when no product with that catalog_id is stored.
func (s *Store) GetProductBasic(catalogID string) (ProductBasic, bool, error) {
	var pb ProductBasic
	var name, brand, currency sql.NullString
	var price sql.NullFloat64
	err := s.db.QueryRow(
		`SELECT catalog_id, name, brand, price, currency FROM products WHERE catalog_id = ? LIMIT 1`,
		catalogID,
	).Scan(&pb.CatalogID, &name, &brand, &price, &currency)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProductBasic{}, false, nil
		}
		return ProductBasic{}, false, err
	}
	pb.Name = name.String
	pb.Brand = brand.String
	pb.Currency = currency.String
	pb.Price = price.Float64
	return pb, true, nil
}

// ListingRow is a listings-table row surfaced to the cheapest command.
type ListingRow struct {
	CatalogID string  `json:"catalog_id"`
	Name      string  `json:"name"`
	Brand     string  `json:"brand"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency"`
	URL       string  `json:"url"`
}

// ListingsMatching returns listings whose name matches the LIKE query (empty
// query = all listings), price ascending.
func (s *Store) ListingsMatching(query string) ([]ListingRow, error) {
	var rows *sql.Rows
	var err error
	if query == "" {
		rows, err = s.db.Query(
			`SELECT catalog_id, name, brand, price, currency, url FROM listings
			 WHERE catalog_id IS NOT NULL AND catalog_id != '' ORDER BY price ASC`)
	} else {
		rows, err = s.db.Query(
			`SELECT catalog_id, name, brand, price, currency, url FROM listings
			 WHERE catalog_id IS NOT NULL AND catalog_id != '' AND LOWER(name) LIKE LOWER(?)
			 ORDER BY price ASC`, "%"+query+"%")
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ListingRow
	for rows.Next() {
		var lr ListingRow
		var name, brand, currency, url sql.NullString
		var price sql.NullFloat64
		if err := rows.Scan(&lr.CatalogID, &name, &brand, &price, &currency, &url); err != nil {
			return nil, err
		}
		lr.Name = name.String
		lr.Brand = brand.String
		lr.Currency = currency.String
		lr.URL = url.String
		lr.Price = price.Float64
		out = append(out, lr)
	}
	return out, rows.Err()
}

// StaleProduct pairs a product with the age of its newest price snapshot.
type StaleProduct struct {
	CatalogID    string    `json:"catalog_id"`
	Name         string    `json:"name"`
	LastCaptured time.Time `json:"last_captured"`
	HasSnapshot  bool      `json:"has_snapshot"`
}

// ProductsWithLatestSnapshot returns every stored product joined to the
// captured_at of its newest price_snapshot (zero time when none exists).
func (s *Store) ProductsWithLatestSnapshot() ([]StaleProduct, error) {
	rows, err := s.db.Query(
		`SELECT p.catalog_id, p.name, MAX(ps.captured_at) AS last_captured
		   FROM products p
		   LEFT JOIN price_snapshot ps ON ps.catalog_id = p.catalog_id
		  WHERE p.catalog_id IS NOT NULL AND p.catalog_id != ''
		  GROUP BY p.catalog_id, p.name
		  ORDER BY p.catalog_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []StaleProduct
	for rows.Next() {
		var sp StaleProduct
		var name, captured sql.NullString
		if err := rows.Scan(&sp.CatalogID, &name, &captured); err != nil {
			return nil, err
		}
		sp.Name = name.String
		if captured.Valid && captured.String != "" {
			sp.LastCaptured = cliutil.ParseStoredTime(captured.String)
			sp.HasSnapshot = !sp.LastCaptured.IsZero()
		}
		out = append(out, sp)
	}
	return out, rows.Err()
}
