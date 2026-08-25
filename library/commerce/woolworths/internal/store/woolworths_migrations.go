// Copyright 2026 Richard Gill and contributors. Licensed under Apache-2.0. See LICENSE.

// Woolworths-specific local tables.
//
// The Woolworths API exposes only "now" — it has no historical endpoint of any
// kind. Every differentiating command in this CLI (real-special, cycle,
// specials-diff, and the historical-low annotations on swap) is powered by the
// two tables created here, which accumulate as the user runs sync.
//
// This file is hand-authored and lives beside the generated store so that
// `generate --force` preserves it. Do not add these migrations to the generated
// migration slice in store.go.

package store

import (
	"context"
	"database/sql"
	"fmt"
)

const woolworthsSchema = `
CREATE TABLE IF NOT EXISTS price_observation (
	stockcode     TEXT    NOT NULL,
	observed_at   INTEGER NOT NULL,
	name          TEXT,
	brand         TEXT,
	price         REAL,
	was_price     REAL,
	savings       REAL,
	is_on_special INTEGER NOT NULL DEFAULT 0,
	is_half_price INTEGER NOT NULL DEFAULT 0,
	cup_price     REAL,
	cup_measure   TEXT,
	cup_string    TEXT,
	package_size  TEXT,
	is_available  INTEGER NOT NULL DEFAULT 1,
	PRIMARY KEY (stockcode, observed_at)
);

CREATE INDEX IF NOT EXISTS idx_price_obs_stockcode
	ON price_observation (stockcode, observed_at DESC);

CREATE INDEX IF NOT EXISTS idx_price_obs_halfprice
	ON price_observation (stockcode, is_half_price, observed_at DESC);

CREATE TABLE IF NOT EXISTS specials_membership (
	group_node  TEXT    NOT NULL,
	stockcode   TEXT    NOT NULL,
	observed_at INTEGER NOT NULL,
	name        TEXT,
	price       REAL,
	was_price   REAL,
	PRIMARY KEY (group_node, stockcode, observed_at)
);

CREATE INDEX IF NOT EXISTS idx_specials_group
	ON specials_membership (group_node, observed_at DESC);

CREATE INDEX IF NOT EXISTS idx_specials_stockcode
	ON specials_membership (stockcode, group_node, observed_at DESC);
`

// EnsureWoolworthsTables creates the Woolworths-specific local tables if they do
// not already exist. It is safe to call on every command invocation; the
// statements are all IF NOT EXISTS.
//
// Call this after opening the store and before any query against
// price_observation or specials_membership.
func EnsureWoolworthsTables(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("ensure woolworths tables: nil database handle")
	}
	if _, err := db.ExecContext(ctx, woolworthsSchema); err != nil {
		return fmt.Errorf("ensure woolworths tables: %w", err)
	}
	return nil
}

// PriceObservation is one recorded sighting of a product's price at a point in
// time. Nullable columns are modelled as sql.Null* so a NULL never silently
// drops the whole row during scanning.
type PriceObservation struct {
	Stockcode   string  `json:"stockcode"`
	ObservedAt  int64   `json:"observed_at"`
	Name        string  `json:"name,omitempty"`
	Brand       string  `json:"brand,omitempty"`
	Price       float64 `json:"price"`
	WasPrice    float64 `json:"was_price,omitempty"`
	Savings     float64 `json:"savings,omitempty"`
	IsOnSpecial bool    `json:"is_on_special"`
	IsHalfPrice bool    `json:"is_half_price"`
	CupPrice    float64 `json:"cup_price,omitempty"`
	CupMeasure  string  `json:"cup_measure,omitempty"`
	CupString   string  `json:"cup_string,omitempty"`
	PackageSize string  `json:"package_size,omitempty"`
	IsAvailable bool    `json:"is_available"`
}

// RecordPriceObservations inserts a batch of observations in one transaction.
//
// This opens its own write transaction, so it must NOT be called from inside an
// already-open write transaction: SQLite WAL permits a single writer and the
// nested write would busy-wait or fail.
func RecordPriceObservations(ctx context.Context, db *sql.DB, obs []PriceObservation) error {
	if db == nil {
		return fmt.Errorf("record price observations: nil database handle")
	}
	if len(obs) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("record price observations: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO price_observation
			(stockcode, observed_at, name, brand, price, was_price, savings,
			 is_on_special, is_half_price, cup_price, cup_measure, cup_string,
			 package_size, is_available)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("record price observations: prepare: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, o := range obs {
		if _, err := stmt.ExecContext(ctx,
			o.Stockcode, o.ObservedAt, o.Name, o.Brand, o.Price, o.WasPrice, o.Savings,
			boolToInt(o.IsOnSpecial), boolToInt(o.IsHalfPrice), o.CupPrice, o.CupMeasure,
			o.CupString, o.PackageSize, boolToInt(o.IsAvailable),
		); err != nil {
			return fmt.Errorf("record price observations: exec %s: %w", o.Stockcode, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("record price observations: commit: %w", err)
	}
	return nil
}

// SpecialsMember is one product's membership of a specials group at a point in time.
type SpecialsMember struct {
	GroupNode  string  `json:"group_node"`
	Stockcode  string  `json:"stockcode"`
	ObservedAt int64   `json:"observed_at"`
	Name       string  `json:"name,omitempty"`
	Price      float64 `json:"price,omitempty"`
	WasPrice   float64 `json:"was_price,omitempty"`
}

// RecordSpecialsMembership inserts a batch of specials-group memberships in one
// transaction. Same single-writer caveat as RecordPriceObservations.
func RecordSpecialsMembership(ctx context.Context, db *sql.DB, members []SpecialsMember) error {
	if db == nil {
		return fmt.Errorf("record specials membership: nil database handle")
	}
	if len(members) == 0 {
		return nil
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("record specials membership: begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO specials_membership
			(group_node, stockcode, observed_at, name, price, was_price)
		VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("record specials membership: prepare: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for _, m := range members {
		if _, err := stmt.ExecContext(ctx,
			m.GroupNode, m.Stockcode, m.ObservedAt, m.Name, m.Price, m.WasPrice,
		); err != nil {
			return fmt.Errorf("record specials membership: exec %s/%s: %w", m.GroupNode, m.Stockcode, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("record specials membership: commit: %w", err)
	}
	return nil
}

// PriceHistory returns every observation for one stockcode, oldest first.
//
// Uses the drain-first pattern: the result set is fully scanned and closed
// before the caller can issue any follow-up query. Nullable columns scan through
// sql.Null* so a NULL yields a zero value rather than dropping the row.
func PriceHistory(ctx context.Context, db *sql.DB, stockcode string) ([]PriceObservation, error) {
	if db == nil {
		return nil, fmt.Errorf("price history: nil database handle")
	}
	rows, err := db.QueryContext(ctx, `
		SELECT stockcode, observed_at, name, brand, price, was_price, savings,
		       is_on_special, is_half_price, cup_price, cup_measure, cup_string,
		       package_size, is_available
		FROM price_observation
		WHERE stockcode = ?
		ORDER BY observed_at ASC`, stockcode)
	if err != nil {
		return nil, fmt.Errorf("price history: query: %w", err)
	}
	out := make([]PriceObservation, 0)
	for rows.Next() {
		var (
			o                                           PriceObservation
			name, brand, cupMeasure, cupString, pkgSize sql.NullString
			price, wasPrice, savings, cupPrice          sql.NullFloat64
			isOnSpecial, isHalfPrice, isAvailable       sql.NullInt64
		)
		if err := rows.Scan(&o.Stockcode, &o.ObservedAt, &name, &brand, &price, &wasPrice,
			&savings, &isOnSpecial, &isHalfPrice, &cupPrice, &cupMeasure, &cupString,
			&pkgSize, &isAvailable); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("price history: scan: %w", err)
		}
		o.Name, o.Brand = name.String, brand.String
		o.Price, o.WasPrice, o.Savings, o.CupPrice = price.Float64, wasPrice.Float64, savings.Float64, cupPrice.Float64
		o.CupMeasure, o.CupString, o.PackageSize = cupMeasure.String, cupString.String, pkgSize.String
		o.IsOnSpecial, o.IsHalfPrice = isOnSpecial.Int64 == 1, isHalfPrice.Int64 == 1
		o.IsAvailable = isAvailable.Int64 == 1
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("price history: iterate: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("price history: close: %w", err)
	}
	return out, nil
}

// SpecialsGroupAt returns the membership of one specials group at a given
// observation timestamp. Drain-first, NULL-safe.
func SpecialsGroupAt(ctx context.Context, db *sql.DB, groupNode string, observedAt int64) ([]SpecialsMember, error) {
	if db == nil {
		return nil, fmt.Errorf("specials group: nil database handle")
	}
	rows, err := db.QueryContext(ctx, `
		SELECT group_node, stockcode, observed_at, name, price, was_price
		FROM specials_membership
		WHERE group_node = ? AND observed_at = ?
		ORDER BY stockcode ASC`, groupNode, observedAt)
	if err != nil {
		return nil, fmt.Errorf("specials group: query: %w", err)
	}
	out := make([]SpecialsMember, 0)
	for rows.Next() {
		var (
			m               SpecialsMember
			name            sql.NullString
			price, wasPrice sql.NullFloat64
		)
		if err := rows.Scan(&m.GroupNode, &m.Stockcode, &m.ObservedAt, &name, &price, &wasPrice); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("specials group: scan: %w", err)
		}
		m.Name, m.Price, m.WasPrice = name.String, price.Float64, wasPrice.Float64
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("specials group: iterate: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("specials group: close: %w", err)
	}
	return out, nil
}

// SpecialsSnapshotTimes returns the distinct observation timestamps recorded for
// a specials group, newest first.
func SpecialsSnapshotTimes(ctx context.Context, db *sql.DB, groupNode string, limit int) ([]int64, error) {
	if db == nil {
		return nil, fmt.Errorf("specials snapshots: nil database handle")
	}
	if limit <= 0 {
		limit = 2
	}
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT observed_at FROM specials_membership
		WHERE group_node = ?
		ORDER BY observed_at DESC
		LIMIT ?`, groupNode, limit)
	if err != nil {
		return nil, fmt.Errorf("specials snapshots: query: %w", err)
	}
	out := make([]int64, 0, limit)
	for rows.Next() {
		var ts int64
		if err := rows.Scan(&ts); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("specials snapshots: scan: %w", err)
		}
		out = append(out, ts)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, fmt.Errorf("specials snapshots: iterate: %w", err)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("specials snapshots: close: %w", err)
	}
	return out, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
