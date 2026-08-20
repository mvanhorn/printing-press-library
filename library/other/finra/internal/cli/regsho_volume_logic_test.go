// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestMatchRecordsByValue(t *testing.T) {
	t.Parallel()
	records := []map[string]any{
		{"symbolCode": "GME", "tradeReportDate": "2026-06-01"},
		{"symbolCode": "AMC", "tradeReportDate": "2026-06-01"},
		{"symbolCode": "GME", "tradeReportDate": "2026-06-02"},
	}
	got := matchRecordsByValue(records, "GME")
	if len(got) != 2 {
		t.Fatalf("matchRecordsByValue matched %d records, want 2", len(got))
	}
	// Case-insensitive match.
	got = matchRecordsByValue(records, "amc")
	if len(got) != 1 {
		t.Fatalf("case-insensitive match got %d records, want 1", len(got))
	}
}

func TestFilterRecordsByDateWindow(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	records := []map[string]any{
		{"tradeReportDate": "2026-05-30"}, // before window
		{"tradeReportDate": "2026-06-05"}, // in window
		{"tradeReportDate": "2026-06-15"}, // after window
		{"symbolCode": "GME"},             // no date field
	}
	got := filterRecordsByDateWindow(records, start, end)
	if len(got) != 1 {
		t.Fatalf("filterRecordsByDateWindow kept %d records, want 1", len(got))
	}
	if got[0]["tradeReportDate"] != "2026-06-05" {
		t.Fatalf("filterRecordsByDateWindow kept unexpected record: %v", got[0])
	}
}

func TestFilterDatesByWindow(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC)
	dates := []time.Time{
		time.Date(2026, 5, 30, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
	}
	got := filterDatesByWindow(dates, start, end)
	if len(got) != 1 {
		t.Fatalf("filterDatesByWindow kept %d dates, want 1", len(got))
	}
}

func TestDecorateShortVolumeRatio(t *testing.T) {
	t.Parallel()

	t.Run("computes ratio from confirmed ParQuantity fields", func(t *testing.T) {
		t.Parallel()
		rec := map[string]any{
			"securitiesInformationProcessorSymbolIdentifier": "GME",
			"shortParQuantity": 50.0,
			"totalParQuantity": 200.0,
		}
		got := decorateShortVolumeRatio(rec)
		ratio, ok := got["short_volume_ratio"].(float64)
		if !ok {
			t.Fatalf("expected short_volume_ratio to be set, got %v", got)
		}
		if ratio != 0.25 {
			t.Fatalf("short_volume_ratio = %v, want 0.25", ratio)
		}
	})

	t.Run("omits ratio when totalParQuantity missing", func(t *testing.T) {
		t.Parallel()
		rec := map[string]any{
			"securitiesInformationProcessorSymbolIdentifier": "GME",
			"shortParQuantity": 50.0,
		}
		got := decorateShortVolumeRatio(rec)
		if _, ok := got["short_volume_ratio"]; ok {
			t.Fatalf("expected no short_volume_ratio, got %v", got)
		}
	})

	t.Run("omits ratio when total is zero to avoid divide-by-zero", func(t *testing.T) {
		t.Parallel()
		rec := map[string]any{
			"shortParQuantity": 50.0,
			"totalParQuantity": 0.0,
		}
		got := decorateShortVolumeRatio(rec)
		if _, ok := got["short_volume_ratio"]; ok {
			t.Fatalf("expected no short_volume_ratio when total is zero, got %v", got)
		}
	})

	t.Run("ignores unrelated short/total-volume-shaped keys", func(t *testing.T) {
		t.Parallel()
		// Only shortParQuantity/totalParQuantity should drive the ratio —
		// unrelated keys that happen to contain "short"/"total"+"volume"
		// must not be picked up now that matching targets exact field names.
		rec := map[string]any{
			"shortVolume":      999.0,
			"totalVolume":      999.0,
			"shortParQuantity": 50.0,
			"totalParQuantity": 200.0,
		}
		got := decorateShortVolumeRatio(rec)
		ratio, ok := got["short_volume_ratio"].(float64)
		if !ok || ratio != 0.25 {
			t.Fatalf("short_volume_ratio = %v (ok=%v), want 0.25", ratio, ok)
		}
	})
}
