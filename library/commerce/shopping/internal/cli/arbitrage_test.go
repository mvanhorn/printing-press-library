// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestNovelArbitrage(t *testing.T) {
	db, dbPath := newTestStore(t)

	t.Run("empty store returns empty array", func(t *testing.T) {
		out := runNovelCmd(t, newNovelArbitrageCmd, dbPath)
		var rows []arbitrageRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) != 0 {
			t.Fatalf("expected empty array, got %d rows", len(rows))
		}
	})

	insertProduct(t, db, "1", "walmart", map[string]any{
		"product_name": "High ROI", "current_price": 10.0, "in_stock": true,
		"profitability": map[string]any{
			"amazon_price": 30.0, "profit_per_unit": 12.0, "margin": 40.0, "roi": 120.0, "status": "profitable",
		},
	})
	insertProduct(t, db, "2", "target", map[string]any{
		"product_name": "Low ROI", "current_price": 20.0, "in_stock": true,
		"profitability": map[string]any{
			"amazon_price": 22.0, "profit_per_unit": 1.0, "margin": 5.0, "roi": 10.0, "status": "marginal",
		},
	})
	// No profitability block -> excluded.
	insertProduct(t, db, "3", "homedepot", map[string]any{
		"product_name": "No Profit Data", "current_price": 5.0,
	})

	t.Run("ranks by roi desc and excludes rows without profitability", func(t *testing.T) {
		out := runNovelCmd(t, newNovelArbitrageCmd, dbPath)
		var rows []arbitrageRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 rows with profitability, got %d (%s)", len(rows), out)
		}
		if rows[0].ProductID != "1" {
			t.Fatalf("expected highest ROI (id 1) first, got %q", rows[0].ProductID)
		}
		if rows[0].ROI == nil || *rows[0].ROI != 120.0 {
			t.Fatalf("expected ROI 120, got %v", rows[0].ROI)
		}
		if rows[0].BuyPrice == nil || *rows[0].BuyPrice != 10.0 {
			t.Fatalf("expected buy_price 10, got %v", rows[0].BuyPrice)
		}
	})

	t.Run("min-roi filter", func(t *testing.T) {
		out := runNovelCmd(t, newNovelArbitrageCmd, dbPath, "--min-roi", "50")
		var rows []arbitrageRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) != 1 || rows[0].ProductID != "1" {
			t.Fatalf("expected only high-ROI row, got %s", out)
		}
	})
}
