// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func monthlyRec(beginningOfMonth string, tradeCount, volume float64) map[string]any {
	return map[string]any{
		"beginningOfMonth":    beginningOfMonth,
		"totalTradeCount":     tradeCount,
		"totalVolumeQuantity": volume,
	}
}

func TestComputeLiquidityTrend(t *testing.T) {
	t.Parallel()

	windowStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)

	t.Run("insufficient data below 2 records", func(t *testing.T) {
		t.Parallel()
		records := []map[string]any{
			monthlyRec("2026-01-01", 100, 1000),
		}
		_, trend, note := computeLiquidityTrend(records, windowStart, windowEnd)
		if trend != "insufficient_data" {
			t.Fatalf("trend = %q, want insufficient_data", trend)
		}
		if note == "" {
			t.Fatalf("expected a non-empty note for insufficient data")
		}
	})

	t.Run("deteriorating: fewer trades and flat-or-larger volume in the back half of the window", func(t *testing.T) {
		t.Parallel()
		// Window midpoint is 2026-04-01T12:00Z-ish. Three months of higher
		// trade counts before it, two months of fewer trades but
		// flat-or-larger volume after it.
		records := []map[string]any{
			monthlyRec("2026-01-01", 500, 1000),
			monthlyRec("2026-02-01", 500, 1000),
			monthlyRec("2026-03-01", 500, 1000),
			monthlyRec("2026-05-01", 300, 1500),
			monthlyRec("2026-06-01", 300, 1500),
		}
		_, trend, _ := computeLiquidityTrend(records, windowStart, windowEnd)
		if trend != "deteriorating" {
			t.Fatalf("trend = %q, want deteriorating", trend)
		}
	})

	t.Run("improving: more trades in the back half of the window", func(t *testing.T) {
		t.Parallel()
		records := []map[string]any{
			monthlyRec("2026-01-01", 100, 1000),
			monthlyRec("2026-05-01", 400, 1000),
			monthlyRec("2026-06-01", 400, 1000),
		}
		_, trend, _ := computeLiquidityTrend(records, windowStart, windowEnd)
		if trend != "improving" {
			t.Fatalf("trend = %q, want improving", trend)
		}
	})

	t.Run("avg trades per month sums total trade count across distinct months", func(t *testing.T) {
		t.Parallel()
		records := []map[string]any{
			monthlyRec("2026-01-01", 100, 1000),
			monthlyRec("2026-02-01", 200, 1000),
		}
		avg, _, _ := computeLiquidityTrend(records, windowStart, windowEnd)
		if avg != 150.0 {
			t.Fatalf("avg trades/month = %v, want 150.0", avg)
		}
	})

	t.Run("no note when trend is computable", func(t *testing.T) {
		t.Parallel()
		records := []map[string]any{
			monthlyRec("2026-01-01", 100, 1000),
			monthlyRec("2026-02-01", 100, 1000),
		}
		_, _, note := computeLiquidityTrend(records, windowStart, windowEnd)
		if note != "" {
			t.Fatalf("expected no note when trend is computable, got %q", note)
		}
	})
}

func TestSumNumericField(t *testing.T) {
	t.Parallel()
	records := []map[string]any{
		{"totalTradeCount": 100.0},
		{"totalTradeCount": 200.0},
		{"otherField": "ignored"},
	}
	if got := sumNumericField(records, "totalTradeCount"); got != 300.0 {
		t.Fatalf("sumNumericField = %v, want 300.0", got)
	}
}
