// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestNovelDeals(t *testing.T) {
	db, dbPath := newTestStore(t)

	t.Run("empty store returns empty array", func(t *testing.T) {
		out := runNovelCmd(t, newNovelDealsCmd, dbPath)
		var rows []dealRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) != 0 {
			t.Fatalf("expected empty array, got %d rows", len(rows))
		}
	})

	insertProduct(t, db, "1", "walmart", map[string]any{
		"product_name": "Big Deal", "brand": "Nike", "current_price": 10.0, "original_price": 40.0,
		"discount_percentage": 75.0, "rating": 4.8, "review_count": 200, "in_stock": true,
		"category": "Electronics > Gadgets",
	})
	insertProduct(t, db, "2", "target", map[string]any{
		"product_name": "Small Deal", "brand": "Acme", "current_price": 30.0, "original_price": 40.0,
		"discount_percentage": 25.0, "rating": 3.0, "review_count": 5, "in_stock": true,
		"category": "Home",
	})
	insertProduct(t, db, "3", "homedepot", map[string]any{
		"product_name": "No Discount", "current_price": 50.0, "discount_percentage": 0.0, "in_stock": false,
	})

	t.Run("min-discount filter and discount ordering", func(t *testing.T) {
		out := runNovelCmd(t, newNovelDealsCmd, dbPath, "--min-discount", "20")
		var rows []dealRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 rows above 20%% discount, got %d (%s)", len(rows), out)
		}
		if rows[0].ProductID != "1" {
			t.Fatalf("expected deepest discount first (id 1), got %q", rows[0].ProductID)
		}
		if rows[0].DiscountPercentage == nil || *rows[0].DiscountPercentage != 75.0 {
			t.Fatalf("expected 75%% discount, got %v", rows[0].DiscountPercentage)
		}
	})

	t.Run("sort by price ascending", func(t *testing.T) {
		out := runNovelCmd(t, newNovelDealsCmd, dbPath, "--sort", "price", "--min-discount", "0")
		var rows []dealRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) < 2 || rows[0].ProductID != "1" {
			t.Fatalf("expected cheapest (id 1) first under price sort, got %s", out)
		}
	})

	t.Run("category filter", func(t *testing.T) {
		out := runNovelCmd(t, newNovelDealsCmd, dbPath, "--category", "Electronics")
		var rows []dealRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) != 1 || rows[0].ProductID != "1" {
			t.Fatalf("expected only Electronics row, got %s", out)
		}
	})
}
