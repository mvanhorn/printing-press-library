// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// QuickCommerceObservation is a durable snapshot from a QuickCommerce API response.
type QuickCommerceObservation struct {
	ID         string
	Resource   string
	ItemID     string
	Platform   string
	Location   string
	Query      string
	CapturedAt time.Time
	Data       json.RawMessage
	Price      sql.NullFloat64
	MRP        sql.NullFloat64
	Inventory  sql.NullInt64
	Available  sql.NullBool
	Quantity   string
	ETA        string
	Open       sql.NullBool
	StoreID    string
}

// EnsureQuickCommerceHistory creates the append-only observation table used by
// history, stale, coverage, and value commands. It is intentionally outside the
// generated migration list so regeneration cannot discard the local analysis layer.
func EnsureQuickCommerceHistory(ctx context.Context, s *Store) error {
	if s == nil || s.DB() == nil {
		return fmt.Errorf("quickcommerce history: nil store")
	}
	_, err := s.DB().ExecContext(ctx, `CREATE TABLE IF NOT EXISTS quickcommerce_observations (
		id TEXT PRIMARY KEY,
		resource TEXT NOT NULL,
		item_id TEXT,
		platform TEXT,
		location TEXT,
		query TEXT,
		captured_at DATETIME NOT NULL,
		data JSON NOT NULL,
		price REAL,
		mrp REAL,
		inventory INTEGER,
		available INTEGER,
		quantity TEXT,
		eta TEXT,
		open INTEGER,
		store_id TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_qc_obs_item_time ON quickcommerce_observations(item_id, captured_at DESC);
	CREATE INDEX IF NOT EXISTS idx_qc_obs_location_platform ON quickcommerce_observations(location, platform, resource);
	CREATE INDEX IF NOT EXISTS idx_qc_obs_captured_at ON quickcommerce_observations(captured_at DESC);`)
	return err
}

// InsertQuickCommerceObservation appends one normalized API observation.
func InsertQuickCommerceObservation(ctx context.Context, s *Store, o QuickCommerceObservation) error {
	if err := EnsureQuickCommerceHistory(ctx, s); err != nil {
		return err
	}
	if o.CapturedAt.IsZero() {
		o.CapturedAt = time.Now().UTC()
	}
	if len(o.Data) == 0 {
		o.Data = json.RawMessage(`{}`)
	}
	_, err := s.DB().ExecContext(ctx, `INSERT OR REPLACE INTO quickcommerce_observations
		(id, resource, item_id, platform, location, query, captured_at, data, price, mrp,
		 inventory, available, quantity, eta, open, store_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		o.ID, o.Resource, o.ItemID, o.Platform, o.Location, o.Query, o.CapturedAt.UTC(),
		o.Data, nullFloat(o.Price), nullFloat(o.MRP), nullInt(o.Inventory), nullBool(o.Available),
		o.Quantity, o.ETA, nullBool(o.Open), o.StoreID)
	return err
}

func nullFloat(v sql.NullFloat64) any {
	if v.Valid {
		return v.Float64
	}
	return nil
}
func nullInt(v sql.NullInt64) any {
	if v.Valid {
		return v.Int64
	}
	return nil
}
func nullBool(v sql.NullBool) any {
	if v.Valid {
		return v.Bool
	}
	return nil
}
