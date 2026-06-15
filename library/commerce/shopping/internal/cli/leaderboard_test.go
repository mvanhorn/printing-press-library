// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestNovelLeaderboard(t *testing.T) {
	db, dbPath := newTestStore(t)

	t.Run("empty store returns empty array", func(t *testing.T) {
		out := runNovelCmd(t, newNovelLeaderboardCmd, dbPath)
		var rows []leaderboardRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) != 0 {
			t.Fatalf("expected empty array, got %d rows", len(rows))
		}
	})

	// walmart: avg discount (60+40)/2 = 50, both on sale.
	insertProduct(t, db, "1", "walmart", map[string]any{"discount_percentage": 60.0, "current_price": 10.0})
	insertProduct(t, db, "2", "walmart", map[string]any{"discount_percentage": 40.0, "current_price": 20.0})
	// target: avg discount 10, one on sale.
	insertProduct(t, db, "3", "target", map[string]any{"discount_percentage": 10.0, "current_price": 100.0})
	insertProduct(t, db, "4", "target", map[string]any{"discount_percentage": 0.0, "current_price": 200.0})

	t.Run("ranks by avg discount desc", func(t *testing.T) {
		out := runNovelCmd(t, newNovelLeaderboardCmd, dbPath, "--by", "avg-discount")
		var rows []leaderboardRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 retailer rows, got %d (%s)", len(rows), out)
		}
		if rows[0].RetailerID != "walmart" {
			t.Fatalf("expected walmart first (highest avg discount), got %q", rows[0].RetailerID)
		}
		if rows[0].AvgDiscount == nil || *rows[0].AvgDiscount != 50.0 {
			t.Fatalf("expected avg discount 50, got %v", rows[0].AvgDiscount)
		}
		if rows[0].OnSaleCount != 2 {
			t.Fatalf("expected 2 on-sale for walmart, got %d", rows[0].OnSaleCount)
		}
		if rows[0].ProductCount != 2 {
			t.Fatalf("expected product_count 2, got %d", rows[0].ProductCount)
		}
	})

	t.Run("ranks by avg price desc", func(t *testing.T) {
		out := runNovelCmd(t, newNovelLeaderboardCmd, dbPath, "--by", "avg-price")
		var rows []leaderboardRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) != 2 || rows[0].RetailerID != "target" {
			t.Fatalf("expected target first under avg-price (150 vs 15), got %s", out)
		}
	})
}
