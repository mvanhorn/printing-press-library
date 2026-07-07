// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestEnsureShoppingExtras(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "extras.db")
	s, err := OpenWithContext(ctx, dbPath)
	if err != nil {
		t.Fatalf("OpenWithContext: %v", err)
	}
	defer s.Close()

	if err := s.EnsureShoppingExtras(ctx); err != nil {
		t.Fatalf("EnsureShoppingExtras: %v", err)
	}
	// Idempotent: a second call must not error.
	if err := s.EnsureShoppingExtras(ctx); err != nil {
		t.Fatalf("EnsureShoppingExtras (second call): %v", err)
	}

	db := s.DB()

	if _, err := db.ExecContext(ctx,
		`INSERT INTO price_points (retailers_id, product_id, ts, price, amz_buy_box, walmart_price, source)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"walmart", "123", "2026-05-22T00:00:00Z", 41.47, nil, 41.47, "price-history",
	); err != nil {
		t.Fatalf("insert price_points: %v", err)
	}

	var (
		rid       string
		pid       string
		price     float64
		walPrice  float64
		ptSource  string
		readBackN int
	)
	row := db.QueryRowContext(ctx,
		`SELECT retailers_id, product_id, price, walmart_price, source FROM price_points WHERE retailers_id = ? AND product_id = ?`,
		"walmart", "123")
	if err := row.Scan(&rid, &pid, &price, &walPrice, &ptSource); err != nil {
		t.Fatalf("scan price_points: %v", err)
	}
	if rid != "walmart" || pid != "123" || price != 41.47 || walPrice != 41.47 || ptSource != "price-history" {
		t.Fatalf("price_points roundtrip mismatch: rid=%q pid=%q price=%v wal=%v src=%q", rid, pid, price, walPrice, ptSource)
	}

	if _, err := db.ExecContext(ctx,
		`INSERT INTO watches (retailers_id, product_id, target_price, added_at) VALUES (?, ?, ?, ?)`,
		"walmart", "123", 19.99, "2026-06-05T00:00:00Z",
	); err != nil {
		t.Fatalf("insert watches: %v", err)
	}
	var target float64
	var addedAt string
	if err := db.QueryRowContext(ctx,
		`SELECT target_price, added_at FROM watches WHERE retailers_id = ? AND product_id = ?`,
		"walmart", "123").Scan(&target, &addedAt); err != nil {
		t.Fatalf("scan watches: %v", err)
	}
	if target != 19.99 || addedAt != "2026-06-05T00:00:00Z" {
		t.Fatalf("watches roundtrip mismatch: target=%v added_at=%q", target, addedAt)
	}

	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM price_points`).Scan(&readBackN); err != nil {
		t.Fatalf("count price_points: %v", err)
	}
	if readBackN != 1 {
		t.Fatalf("expected 1 price_point row, got %d", readBackN)
	}
}
