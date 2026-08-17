package cli

import (
	"encoding/json"
	"testing"
)

func TestCompactListFieldsPreservesCommerceFields(t *testing.T) {
	input := []map[string]any{
		{
			"item_id":     "398130961247",
			"title":       "NVIDIA GeForce RTX 5090 Founders Edition",
			"price":       3999.99,
			"sold_price":  3825.00,
			"currency":    "USD",
			"condition":   "new",
			"seller":      "example-seller",
			"bids":        float64(12),
			"best_offer":  true,
			"auction":     true,
			"buy_it_now":  false,
			"time_left":   "1h 12m",
			"ends_at":     "2026-07-02T20:00:00Z",
			"sold_date":   "2026-07-01T00:00:00Z",
			"url":         "https://www.ebay.com/itm/398130961247",
			"image_url":   "https://i.ebayimg.com/images/example.webp",
			"location":    "Madison, Wisconsin",
			"shipping":    "Free shipping",
			"description": "large verbose body should not survive compact list output",
		},
	}

	var got []map[string]any
	if err := json.Unmarshal(compactListFields(input), &got); err != nil {
		t.Fatalf("unmarshal compact output: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected one compact row, got %d", len(got))
	}
	row := got[0]
	for _, key := range []string{
		"item_id", "title", "price", "sold_price", "currency", "condition",
		"seller", "bids", "best_offer", "auction", "buy_it_now", "time_left",
		"ends_at", "sold_date", "url", "image_url", "location", "shipping",
	} {
		if _, ok := row[key]; !ok {
			t.Fatalf("compact output dropped commerce field %q; row=%v", key, row)
		}
	}
	if _, ok := row["description"]; ok {
		t.Fatalf("compact output kept verbose field description; row=%v", row)
	}
}

func TestCompactListFieldsPreservesCompSummaryFields(t *testing.T) {
	input := []map[string]any{{
		"mean": 3200.0, "median": 3150.0, "sample_size": float64(24),
		"p25": 2900.0, "p75": 3500.0, "std_dev": 180.0,
		"notes": "verbose diagnostic",
	}}

	var got []map[string]any
	if err := json.Unmarshal(compactListFields(input), &got); err != nil {
		t.Fatalf("unmarshal compact output: %v", err)
	}
	row := got[0]
	for _, key := range []string{"mean", "median", "sample_size", "p25", "p75", "std_dev"} {
		if _, ok := row[key]; !ok {
			t.Fatalf("compact output dropped comp field %q; row=%v", key, row)
		}
	}
	if _, ok := row["notes"]; ok {
		t.Fatalf("compact output kept verbose field notes; row=%v", row)
	}
}
