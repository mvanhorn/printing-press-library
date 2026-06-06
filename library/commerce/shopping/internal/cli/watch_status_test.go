// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestNovelWatchAddAndStatus(t *testing.T) {
	db, dbPath := newTestStore(t)

	t.Run("empty store status returns empty array", func(t *testing.T) {
		out := runNovelCmd(t, newNovelWatchStatusCmd, dbPath)
		var rows []watchStatusRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) != 0 {
			t.Fatalf("expected empty array, got %d rows", len(rows))
		}
	})

	// Add a watch with a target price.
	t.Run("add writes a watch", func(t *testing.T) {
		out := runNovelCmd(t, newNovelWatchAddCmd, dbPath, "walmart", "100", "--target-price", "12.00")
		var res watchAddResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if res.RetailerID != "walmart" || res.ProductID != "100" {
			t.Fatalf("unexpected confirmation: %s", out)
		}
		if res.TargetPrice == nil || *res.TargetPrice != 12.00 {
			t.Fatalf("expected target_price 12.00, got %v", res.TargetPrice)
		}
	})

	// Seed product current price (11.00 -> at/under target) and price history.
	insertProduct(t, db, "100", "walmart", map[string]any{"product_name": "Widget", "current_price": 11.00})
	insertPricePoint(t, db, "walmart", "100", "2026-05-22T00:00:00Z", 15.00)
	insertPricePoint(t, db, "walmart", "100", "2026-05-29T00:00:00Z", 11.00)

	t.Run("status reports current, previous, delta and hit_target", func(t *testing.T) {
		out := runNovelCmd(t, newNovelWatchStatusCmd, dbPath)
		var rows []watchStatusRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) != 1 {
			t.Fatalf("expected 1 watch row, got %d (%s)", len(rows), out)
		}
		r := rows[0]
		if r.CurrentPrice == nil || *r.CurrentPrice != 11.00 {
			t.Fatalf("expected current_price 11.00 from products, got %v", r.CurrentPrice)
		}
		if r.PreviousPrice == nil || *r.PreviousPrice != 15.00 {
			t.Fatalf("expected previous_price 15.00 from prior price point, got %v", r.PreviousPrice)
		}
		if r.Delta == nil || *r.Delta != -4.00 {
			t.Fatalf("expected delta -4.00, got %v", r.Delta)
		}
		if r.HitTarget == nil || !*r.HitTarget {
			t.Fatalf("expected hit_target true (11.00 <= 12.00), got %v", r.HitTarget)
		}
	})
}

// Regression: when `index` runs without --price-history after a prior
// --price-history sync, products.current_price is fresher than the most recent
// recorded price point. The delta must span exactly one change (current vs the
// latest price point), not two (current vs the second-most-recent point).
func TestNovelWatchStatusProductsDivergeFromHistory(t *testing.T) {
	db, dbPath := newTestStore(t)
	runNovelCmd(t, newNovelWatchAddCmd, dbPath, "walmart", "500")
	insertProduct(t, db, "500", "walmart", map[string]any{"product_name": "Gizmo", "current_price": 9.00})
	insertPricePoint(t, db, "walmart", "500", "2026-05-15T00:00:00Z", 15.00)
	insertPricePoint(t, db, "walmart", "500", "2026-05-22T00:00:00Z", 11.00)

	out := runNovelCmd(t, newNovelWatchStatusCmd, dbPath)
	var rows []watchStatusRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal: %v\nout: %s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d (%s)", len(rows), out)
	}
	r := rows[0]
	if r.CurrentPrice == nil || *r.CurrentPrice != 9.00 {
		t.Fatalf("expected current 9.00 from products, got %v", r.CurrentPrice)
	}
	if r.PreviousPrice == nil || *r.PreviousPrice != 11.00 {
		t.Fatalf("expected previous 11.00 (latest price point, not 15.00), got %v", r.PreviousPrice)
	}
	if r.Delta == nil || *r.Delta != -2.00 {
		t.Fatalf("expected delta -2.00 (9.00 - 11.00), got %v", r.Delta)
	}
}
