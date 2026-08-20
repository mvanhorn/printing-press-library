// Copyright 2026 NicholasSpisak and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestNovelCompare(t *testing.T) {
	db, dbPath := newTestStore(t)

	t.Run("empty store returns empty results", func(t *testing.T) {
		out := runNovelCmd(t, newNovelCompareCmd, dbPath, "012345678905")
		var res compareResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if res.Count != 0 || len(res.Results) != 0 {
			t.Fatalf("expected empty results, got count=%d results=%v", res.Count, res.Results)
		}
		if res.Cheapest != nil {
			t.Fatalf("expected nil cheapest, got %v", *res.Cheapest)
		}
		if res.LookupType != "upc" {
			t.Fatalf("expected default lookup_type upc, got %q", res.LookupType)
		}
	})

	// Two retailers carry the same UPC at different prices.
	insertProduct(t, db, "111", "walmart", map[string]any{
		"upc": "012345678905", "current_price": 19.99, "product_name": "Widget", "in_stock": true,
		"product_url": "https://walmart.example/widget",
	})
	insertProduct(t, db, "222", "target", map[string]any{
		"upc": "012345678905", "current_price": 14.50, "product_name": "Widget", "in_stock": true,
		"product_url": "https://target.example/widget",
	})
	// A different UPC must not appear.
	insertProduct(t, db, "333", "homedepot", map[string]any{
		"upc": "999999999999", "current_price": 5.00, "product_name": "Other",
	})

	t.Run("populated store ranks cheapest first with delta", func(t *testing.T) {
		out := runNovelCmd(t, newNovelCompareCmd, dbPath, "012345678905", "--lookup-type", "upc")
		var res compareResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if res.Count != 2 {
			t.Fatalf("expected 2 results, got %d (%s)", res.Count, out)
		}
		if res.Cheapest == nil || *res.Cheapest != 14.50 {
			t.Fatalf("expected cheapest 14.50, got %v", res.Cheapest)
		}
		// First row is the cheapest (target), delta 0.
		if res.Results[0].RetailerID != "target" {
			t.Fatalf("expected target first, got %q", res.Results[0].RetailerID)
		}
		if res.Results[0].DeltaToCheapest == nil || *res.Results[0].DeltaToCheapest != 0 {
			t.Fatalf("expected delta 0 for cheapest, got %v", res.Results[0].DeltaToCheapest)
		}
		// Second row delta = 19.99 - 14.50 = 5.49.
		if res.Results[1].DeltaToCheapest == nil {
			t.Fatalf("expected non-nil delta for second row")
		}
		if d := *res.Results[1].DeltaToCheapest; d < 5.48 || d > 5.50 {
			t.Fatalf("expected delta ~5.49, got %v", d)
		}
	})

	t.Run("asin lookup matches asins array", func(t *testing.T) {
		insertProduct(t, db, "444", "amazon", map[string]any{
			"asins": []string{"B0CJMX93TS"}, "current_price": 22.00, "product_name": "Widget",
		})
		out := runNovelCmd(t, newNovelCompareCmd, dbPath, "B0CJMX93TS", "--lookup-type", "asin")
		var res compareResult
		if err := json.Unmarshal([]byte(out), &res); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, out)
		}
		if res.Count != 1 || res.Results[0].RetailerID != "amazon" {
			t.Fatalf("expected single amazon row, got %s", out)
		}
	})
}
