// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestNovelPriceDrops(t *testing.T) {
	db, dbPath := newTestStore(t)

	t.Run("empty store returns empty array", func(t *testing.T) {
		out := runNovelCmd(t, newNovelPriceDropsCmd, dbPath)
		var rows []priceDropRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) != 0 {
			t.Fatalf("expected empty array, got %d rows", len(rows))
		}
	})

	// Product 100 dropped 50 -> 30 over a week (40% drop).
	insertPricePoint(t, db, "walmart", "100", "2026-05-22T00:00:00Z", 50.0)
	insertPricePoint(t, db, "walmart", "100", "2026-05-29T00:00:00Z", 30.0)
	insertProduct(t, db, "100", "walmart", map[string]any{"product_name": "Dropper"})

	// Product 200 went UP (no drop) -> excluded.
	insertPricePoint(t, db, "target", "200", "2026-05-22T00:00:00Z", 20.0)
	insertPricePoint(t, db, "target", "200", "2026-05-29T00:00:00Z", 25.0)

	// Product 300 dropped a little (5%).
	insertPricePoint(t, db, "target", "300", "2026-05-22T00:00:00Z", 100.0)
	insertPricePoint(t, db, "target", "300", "2026-05-29T00:00:00Z", 95.0)

	t.Run("ranks drops by percent and excludes increases", func(t *testing.T) {
		out := runNovelCmd(t, newNovelPriceDropsCmd, dbPath, "--weeks", "1")
		var rows []priceDropRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) != 2 {
			t.Fatalf("expected 2 drops, got %d (%s)", len(rows), out)
		}
		if rows[0].ProductID != "100" {
			t.Fatalf("expected biggest %%-drop (id 100) first, got %q", rows[0].ProductID)
		}
		if rows[0].DropPct < 39.9 || rows[0].DropPct > 40.1 {
			t.Fatalf("expected ~40%% drop, got %v", rows[0].DropPct)
		}
		if rows[0].Drop != 20.0 {
			t.Fatalf("expected $20 drop, got %v", rows[0].Drop)
		}
		if rows[0].ProductName == nil || *rows[0].ProductName != "Dropper" {
			t.Fatalf("expected product_name joined from products, got %v", rows[0].ProductName)
		}
	})

	t.Run("min-drop-pct filter", func(t *testing.T) {
		out := runNovelCmd(t, newNovelPriceDropsCmd, dbPath, "--weeks", "1", "--min-drop-pct", "10")
		var rows []priceDropRow
		if err := json.Unmarshal([]byte(out), &rows); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if len(rows) != 1 || rows[0].ProductID != "100" {
			t.Fatalf("expected only the >=10%% drop, got %s", out)
		}
	})
}

// Regression: a malformed timestamp must be skipped. If it parsed to the zero
// time it would be "before" any target and win baseline selection, corrupting
// the drop ranking. The valid points must still produce the correct drop.
func TestNovelPriceDropsSkipsMalformedTimestamp(t *testing.T) {
	db, dbPath := newTestStore(t)
	insertPricePoint(t, db, "target", "400", "not-a-timestamp", 1.00)
	insertPricePoint(t, db, "target", "400", "2026-05-22T00:00:00Z", 50.00)
	insertPricePoint(t, db, "target", "400", "2026-05-29T00:00:00Z", 40.00)

	out := runNovelCmd(t, newNovelPriceDropsCmd, dbPath, "--weeks", "1")
	var rows []priceDropRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("unmarshal: %v\nout: %s", err, out)
	}
	if len(rows) != 1 || rows[0].ProductID != "400" {
		t.Fatalf("expected one clean drop for product 400, got %s", out)
	}
	if rows[0].OldPrice != 50.00 || rows[0].NewPrice != 40.00 {
		t.Fatalf("expected old 50.00 -> new 40.00 from valid points, got %v -> %v", rows[0].OldPrice, rows[0].NewPrice)
	}
	if rows[0].DropPct < 19.9 || rows[0].DropPct > 20.1 {
		t.Fatalf("expected ~20%% drop, got %v", rows[0].DropPct)
	}
}
