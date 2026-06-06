// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"fmt"
)

// shoppingExtraDDL is the idempotent CREATE TABLE / CREATE INDEX set backing
// the hand-written novel commands (index price-history capture, watch list).
// These tables live alongside the generated products/shopping tables but are
// owned by the novel-feature code, not the generator.
var shoppingExtraDDL = []string{
	`CREATE TABLE IF NOT EXISTS price_points (
		retailers_id TEXT NOT NULL,
		product_id   TEXT NOT NULL,
		ts           TEXT NOT NULL,
		price        REAL,
		amz_buy_box  REAL,
		walmart_price REAL,
		source       TEXT,
		PRIMARY KEY (retailers_id, product_id, ts)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_price_points_retailer_product
		ON price_points (retailers_id, product_id)`,
	`CREATE TABLE IF NOT EXISTS watches (
		retailers_id TEXT NOT NULL,
		product_id   TEXT NOT NULL,
		target_price REAL,
		added_at     TEXT NOT NULL,
		PRIMARY KEY (retailers_id, product_id)
	)`,
}

// EnsureShoppingExtras creates the novel-feature auxiliary tables if they do
// not already exist. It is safe to call on every store open: each statement
// is guarded by IF NOT EXISTS, so a repeat call is a no-op.
func (s *Store) EnsureShoppingExtras(ctx context.Context) error {
	for _, stmt := range shoppingExtraDDL {
		if _, err := s.DB().ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("ensure shopping extras: %w", err)
		}
	}
	return nil
}
