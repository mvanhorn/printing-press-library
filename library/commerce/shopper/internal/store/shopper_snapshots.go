// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written shopper snapshot helpers. Shared by basket-diff, price-watch,
// restock-predict, and catalog-drift. Tables are created lazily on first use.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// ensureCartSnapshotsTable creates cart_snapshots if it doesn't exist.
func ensureCartSnapshotsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS cart_snapshots (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		taken_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		items_json TEXT NOT NULL
	)`)
	return err
}

// ensurePriceSnapshotsTable creates price_snapshots if it doesn't exist.
func ensurePriceSnapshotsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS price_snapshots (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		product_id TEXT NOT NULL,
		name       TEXT NOT NULL,
		price_cents INTEGER NOT NULL DEFAULT 0,
		unit_price  REAL NOT NULL DEFAULT 0,
		unit_label  TEXT NOT NULL DEFAULT '',
		pack_grams  INTEGER NOT NULL DEFAULT 0,
		taken_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS idx_price_snaps_product ON price_snapshots(product_id, taken_at)`)
	return err
}

// CartSnapshotItem represents a single cart item stored in a snapshot.
type CartSnapshotItem struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Qty      float64 `json:"qty"`
	Price    float64 `json:"price"`
	UnitPrice float64 `json:"unit_price,omitempty"`
}

// CartSnapshot represents a stored cart snapshot.
type CartSnapshot struct {
	ID      int64
	TakenAt time.Time
	Items   []CartSnapshotItem
}

// SnapshotCart stores a new cart snapshot. Returns the new snapshot's row ID.
func SnapshotCart(db *sql.DB, items []CartSnapshotItem) (int64, error) {
	if err := ensureCartSnapshotsTable(db); err != nil {
		return 0, fmt.Errorf("ensuring cart_snapshots table: %w", err)
	}
	data, err := json.Marshal(items)
	if err != nil {
		return 0, fmt.Errorf("marshaling cart items: %w", err)
	}
	res, err := db.Exec(
		`INSERT INTO cart_snapshots (taken_at, items_json) VALUES (?, ?)`,
		time.Now().UTC().Format(time.RFC3339), string(data),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// CountCartSnapshots returns the total number of cart snapshots.
func CountCartSnapshots(db *sql.DB) (int, error) {
	if err := ensureCartSnapshotsTable(db); err != nil {
		return 0, err
	}
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM cart_snapshots`).Scan(&n)
	return n, err
}

// LatestCartSnapshots returns up to n most recent cart snapshots (newest first).
func LatestCartSnapshots(db *sql.DB, n int) ([]CartSnapshot, error) {
	if err := ensureCartSnapshotsTable(db); err != nil {
		return nil, err
	}
	rows, err := db.Query(
		`SELECT id, taken_at, items_json FROM cart_snapshots ORDER BY id DESC LIMIT ?`, n,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCartSnapshots(rows)
}

// OlderCartSnapshots returns snapshots taken before a given snapshot ID, newest first.
func OlderCartSnapshots(db *sql.DB, beforeID int64, n int) ([]CartSnapshot, error) {
	if err := ensureCartSnapshotsTable(db); err != nil {
		return nil, err
	}
	rows, err := db.Query(
		`SELECT id, taken_at, items_json FROM cart_snapshots WHERE id < ? ORDER BY id DESC LIMIT ?`,
		beforeID, n,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanCartSnapshots(rows)
}

func scanCartSnapshots(rows *sql.Rows) ([]CartSnapshot, error) {
	var out []CartSnapshot
	for rows.Next() {
		var snap CartSnapshot
		var takenAtStr sql.NullString
		var itemsJSON sql.NullString
		if err := rows.Scan(&snap.ID, &takenAtStr, &itemsJSON); err != nil {
			return nil, err
		}
		if takenAtStr.Valid {
			snap.TakenAt, _ = time.Parse(time.RFC3339, takenAtStr.String)
			if snap.TakenAt.IsZero() {
				snap.TakenAt, _ = time.Parse("2006-01-02 15:04:05", takenAtStr.String)
			}
		}
		if itemsJSON.Valid {
			_ = json.Unmarshal([]byte(itemsJSON.String), &snap.Items)
		}
		out = append(out, snap)
	}
	return out, rows.Err()
}

// PriceSnapshot represents a stored product price snapshot.
type PriceSnapshot struct {
	ID        int64
	ProductID string
	Name      string
	PriceCents int64
	UnitPrice  float64
	UnitLabel  string
	PackGrams  int64
	TakenAt   time.Time
}

// SnapshotPrices stores price snapshots for a batch of products.
func SnapshotPrices(db *sql.DB, items []PriceSnapshot) error {
	if err := ensurePriceSnapshotsTable(db); err != nil {
		return fmt.Errorf("ensuring price_snapshots table: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, item := range items {
		takenAt := now
		if !item.TakenAt.IsZero() {
			takenAt = item.TakenAt.UTC().Format(time.RFC3339)
		}
		_, err := tx.Exec(
			`INSERT INTO price_snapshots (product_id, name, price_cents, unit_price, unit_label, pack_grams, taken_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			item.ProductID, item.Name, item.PriceCents, item.UnitPrice, item.UnitLabel, item.PackGrams, takenAt,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// PriceSnapshotWindow returns all price snapshots for the given product IDs within a time window.
// If productIDs is empty, returns snapshots for all products in the window.
func PriceSnapshotWindow(db *sql.DB, since time.Time, productIDs []string) ([]PriceSnapshot, error) {
	if err := ensurePriceSnapshotsTable(db); err != nil {
		return nil, err
	}
	sinceStr := since.UTC().Format(time.RFC3339)
	var rows *sql.Rows
	var err error
	if len(productIDs) == 0 {
		rows, err = db.Query(
			`SELECT id, product_id, name, price_cents, unit_price, unit_label, pack_grams, taken_at
			 FROM price_snapshots WHERE taken_at >= ? ORDER BY product_id, taken_at`,
			sinceStr,
		)
	} else {
		// Build a parameterized IN clause
		placeholders := make([]byte, 0, len(productIDs)*3)
		args := make([]any, 0, len(productIDs)+1)
		args = append(args, sinceStr)
		for i, pid := range productIDs {
			if i > 0 {
				placeholders = append(placeholders, ',')
			}
			placeholders = append(placeholders, '?')
			args = append(args, pid)
		}
		q := fmt.Sprintf(
			`SELECT id, product_id, name, price_cents, unit_price, unit_label, pack_grams, taken_at
			 FROM price_snapshots WHERE taken_at >= ? AND product_id IN (%s) ORDER BY product_id, taken_at`,
			string(placeholders),
		)
		rows, err = db.Query(q, args...)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPriceSnapshots(rows)
}

// CountPriceSnapshots returns distinct products that have at least one snapshot.
func CountPriceSnapshots(db *sql.DB) (int, error) {
	if err := ensurePriceSnapshotsTable(db); err != nil {
		return 0, err
	}
	var n int
	err := db.QueryRow(`SELECT COUNT(DISTINCT product_id) FROM price_snapshots`).Scan(&n)
	return n, err
}

func scanPriceSnapshots(rows *sql.Rows) ([]PriceSnapshot, error) {
	var out []PriceSnapshot
	for rows.Next() {
		var s PriceSnapshot
		var takenAtStr sql.NullString
		var unitLabel sql.NullString
		if err := rows.Scan(&s.ID, &s.ProductID, &s.Name, &s.PriceCents, &s.UnitPrice, &unitLabel, &s.PackGrams, &takenAtStr); err != nil {
			return nil, err
		}
		if unitLabel.Valid {
			s.UnitLabel = unitLabel.String
		}
		if takenAtStr.Valid {
			s.TakenAt, _ = time.Parse(time.RFC3339, takenAtStr.String)
			if s.TakenAt.IsZero() {
				s.TakenAt, _ = time.Parse("2006-01-02 15:04:05", takenAtStr.String)
			}
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
