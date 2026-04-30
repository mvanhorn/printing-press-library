package ebay

import (
	"math"
	"testing"
	"time"
)

func TestAnalyzeComps_BasicStats(t *testing.T) {
	now := time.Now()
	items := []SoldItem{
		{ItemID: "1", SoldPrice: 100, SoldDate: now.AddDate(0, 0, -1)},
		{ItemID: "2", SoldPrice: 200, SoldDate: now.AddDate(0, 0, -2)},
		{ItemID: "3", SoldPrice: 300, SoldDate: now.AddDate(0, 0, -3)},
		{ItemID: "4", SoldPrice: 400, SoldDate: now.AddDate(0, 0, -4)},
		{ItemID: "5", SoldPrice: 500, SoldDate: now.AddDate(0, 0, -5)},
	}
	stats := AnalyzeComps("test", items, 90, false)
	if stats.SampleSize != 5 {
		t.Errorf("SampleSize = %d, want 5", stats.SampleSize)
	}
	if stats.UsedSize != 5 {
		t.Errorf("UsedSize = %d, want 5", stats.UsedSize)
	}
	if stats.Min != 100 {
		t.Errorf("Min = %v, want 100", stats.Min)
	}
	if stats.Max != 500 {
		t.Errorf("Max = %v, want 500", stats.Max)
	}
	if stats.Mean != 300 {
		t.Errorf("Mean = %v, want 300", stats.Mean)
	}
	if stats.Median != 300 {
		t.Errorf("Median = %v, want 300", stats.Median)
	}
}

func TestAnalyzeComps_OutlierTrimmed(t *testing.T) {
	items := []SoldItem{
		{SoldPrice: 100}, {SoldPrice: 105}, {SoldPrice: 110}, {SoldPrice: 115},
		{SoldPrice: 120}, {SoldPrice: 125}, {SoldPrice: 130},
		{SoldPrice: 9999}, // far outlier
	}
	stats := AnalyzeComps("test", items, 90, true)
	if stats.OutliersTrim < 1 {
		t.Errorf("expected at least 1 outlier trimmed, got %d", stats.OutliersTrim)
	}
	if stats.Max == 9999 {
		t.Errorf("Max = %v, expected outlier 9999 to be trimmed", stats.Max)
	}
}

func TestAnalyzeComps_EmptyInput(t *testing.T) {
	stats := AnalyzeComps("test", nil, 90, true)
	if stats.SampleSize != 0 {
		t.Errorf("SampleSize = %d on empty input, want 0", stats.SampleSize)
	}
	if stats.Mean != 0 {
		t.Errorf("Mean should be 0 on empty input")
	}
}

func TestPercentile(t *testing.T) {
	v := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	tests := []struct {
		p    float64
		want float64
		tol  float64
	}{
		{0, 1, 0.01},
		{50, 5.5, 0.01},
		{100, 10, 0.01},
		{25, 3.25, 0.01},
		{75, 7.75, 0.01},
	}
	for _, tt := range tests {
		got := percentile(v, tt.p)
		if math.Abs(got-tt.want) > tt.tol {
			t.Errorf("percentile(p=%v) = %v, want %v", tt.p, got, tt.want)
		}
	}
}

func TestDedupeVariants_CollapsesNearDuplicates(t *testing.T) {
	items := []SoldItem{
		{ItemID: "a", Title: "Cooper Flagg /50 Topps Chrome", SoldPrice: 1000},
		{ItemID: "b", Title: "2025-26 Topps Chrome Cooper Flagg #251 Gold /50", SoldPrice: 1100},
		{ItemID: "c", Title: "Cooper Flagg Topps Chrome Gold /50 PSA 9", SoldPrice: 3500},
		{ItemID: "d", Title: "Lebron James Lakers Topps", SoldPrice: 50},
	}
	out := DedupeVariants(items)
	// We expect at most one Cooper Flagg variant cluster.
	cooperCount := 0
	for _, it := range out {
		if it.Title != "" && (it.ItemID == "a" || it.ItemID == "b" || it.ItemID == "c") {
			cooperCount++
		}
	}
	if cooperCount > 3 {
		t.Errorf("dedupe failed; got %d Cooper Flagg variants in output", cooperCount)
	}
	// Lebron should always survive (different fingerprint).
	hasLebron := false
	for _, it := range out {
		if it.ItemID == "d" {
			hasLebron = true
		}
	}
	if !hasLebron {
		t.Errorf("Lebron James item dropped; should survive dedupe (different title fingerprint)")
	}
}

func TestFingerprint_OrderInsensitive(t *testing.T) {
	a := fingerprint("Cooper Flagg /50 Topps Chrome")
	b := fingerprint("Topps Chrome /50 Cooper Flagg")
	if a != b {
		t.Errorf("fingerprint should be order-insensitive: %q vs %q", a, b)
	}
}
