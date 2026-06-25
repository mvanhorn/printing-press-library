// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

// catRows: two cardiovascular biomarkers (ApoB in-optimal, LDL out) measured
// across three requisition rounds, plus one unrelated metabolic row.
func catRows() []resultRow {
	rows := []resultRow{}
	for i, req := range []string{"R1", "R2", "R3"} {
		date := []string{"2022-01-01", "2023-01-01", "2024-01-01"}[i]
		rows = append(rows,
			resultRow{BiomarkerName: "ApoB", Category: "Cardiovascular", RequisitionID: req, DrawDate: date, Value: 70, OptimalLow: 0, OptimalHigh: 90},
			resultRow{BiomarkerName: "LDL", Category: "Cardiovascular", RequisitionID: req, DrawDate: date, Value: 200, OptimalLow: 50, OptimalHigh: 150},
		)
	}
	rows = append(rows, resultRow{BiomarkerName: "Glucose", Category: "Metabolic", RequisitionID: "R1", DrawDate: "2022-01-01", Value: 90, OptimalLow: 70, OptimalHigh: 99})
	return rows
}

func TestAggregateCategoryTrendCountsDistinctBiomarkers(t *testing.T) {
	points, distinct := aggregateCategoryTrend(catRows(), "cardiovascular")

	// THE BUG: distinct must be 2 (ApoB, LDL), NOT 6 (2 biomarkers × 3 rounds).
	if distinct != 2 {
		t.Errorf("distinctBiomarkers = %d, want 2 (per-biomarker, not per draw-row)", distinct)
	}
	if len(points) != 3 {
		t.Fatalf("rounds = %d, want 3", len(points))
	}
	// Sorted oldest → newest.
	if points[0].DrawDate != "2022-01-01" || points[2].DrawDate != "2024-01-01" {
		t.Errorf("rounds not sorted oldest→newest: %v", points)
	}
	// Each round: 2 biomarkers, 1 in-optimal (ApoB), 1 out (LDL) → 50%.
	for _, p := range points {
		if p.Total != 2 || p.InOptimal != 1 || p.PercentOptimal != 50 {
			t.Errorf("round %s = total %d, in-optimal %d, pct %.1f; want 2/1/50", p.DrawDate, p.Total, p.InOptimal, p.PercentOptimal)
		}
	}
}

func TestAggregateCategoryTrendSubstringAndNoMatch(t *testing.T) {
	// Substring match ("cardio" ⊂ "cardiovascular").
	if points, distinct := aggregateCategoryTrend(catRows(), "cardio"); len(points) != 3 || distinct != 2 {
		t.Errorf("substring match = %d rounds, %d distinct; want 3, 2", len(points), distinct)
	}
	// No category match → no rounds, zero distinct (drives the not-found error).
	if points, distinct := aggregateCategoryTrend(catRows(), "thyroid"); len(points) != 0 || distinct != 0 {
		t.Errorf("no-match = %d rounds, %d distinct; want 0, 0", len(points), distinct)
	}
}

func TestPercentBar(t *testing.T) {
	tests := []struct {
		pct        float64
		width      int
		wantFilled int
	}{
		{50, 24, 12},
		{0, 10, 0},
		{100, 10, 10},
		{150, 10, 10}, // clamped to width
		{-5, 10, 0},   // clamped to zero
	}
	for _, tc := range tests {
		bar := percentBar(tc.pct, tc.width)
		if got := strings.Count(bar, "█"); got != tc.wantFilled {
			t.Errorf("percentBar(%.0f, %d) filled = %d, want %d (bar=%q)", tc.pct, tc.width, got, tc.wantFilled, bar)
		}
		if got := len([]rune(bar)); got != tc.width {
			t.Errorf("percentBar(%.0f, %d) width = %d runes, want %d", tc.pct, tc.width, got, tc.width)
		}
	}
}
