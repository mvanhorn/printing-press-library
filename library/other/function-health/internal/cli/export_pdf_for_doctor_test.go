// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// fixtureRows builds a small multi-category, multi-status result set.
//
//	Iron        (Nutrients)      latest out-of-optimal (above)
//	ApoB        (Heart)          latest in-optimal
//	ALT         (Liver)          latest out-of-optimal (below)
func fixtureRows() []resultRow {
	return []resultRow{
		{BiomarkerName: "Iron", Category: "Nutrients", DrawDate: "2024-01-01", Value: 100, OptimalLow: 50, OptimalHigh: 150},
		{BiomarkerName: "Iron", Category: "Nutrients", DrawDate: "2025-01-01", Value: 200, OptimalLow: 50, OptimalHigh: 150}, // above -> out
		{BiomarkerName: "ApoB", Category: "Heart", DrawDate: "2025-01-01", Value: 70, OptimalLow: 0, OptimalHigh: 90},        // in range
		{BiomarkerName: "ALT", Category: "Liver", DrawDate: "2025-01-01", Value: 5, OptimalLow: 10, OptimalHigh: 40},         // below -> out
	}
}

func biomarkerSet(rows []resultRow) map[string]bool {
	m := map[string]bool{}
	for _, r := range rows {
		m[r.BiomarkerName] = true
	}
	return m
}

func TestFilterResultRows(t *testing.T) {
	tests := []struct {
		name         string
		oor          bool
		section      string
		wantMarkers  []string
		wantNoMarker []string
	}{
		{name: "default keeps all", wantMarkers: []string{"Iron", "ApoB", "ALT"}},
		{name: "out-of-range only", oor: true, wantMarkers: []string{"Iron", "ALT"}, wantNoMarker: []string{"ApoB"}},
		{name: "section case-insensitive substring", section: "liver", wantMarkers: []string{"ALT"}, wantNoMarker: []string{"Iron", "ApoB"}},
		{name: "section + out-of-range combined", oor: true, section: "Nutrients", wantMarkers: []string{"Iron"}, wantNoMarker: []string{"ApoB", "ALT"}},
		{name: "section + out-of-range excludes in-range in section", oor: true, section: "Heart", wantMarkers: nil, wantNoMarker: []string{"ApoB"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := biomarkerSet(filterResultRows(fixtureRows(), tc.oor, tc.section))
			for _, m := range tc.wantMarkers {
				if !got[m] {
					t.Errorf("expected biomarker %q in result, got set %v", m, got)
				}
			}
			for _, m := range tc.wantNoMarker {
				if got[m] {
					t.Errorf("did not expect biomarker %q in result, got set %v", m, got)
				}
			}
		})
	}
}

func TestFilterResultRowsDefaultUnchanged(t *testing.T) {
	in := fixtureRows()
	out := filterResultRows(in, false, "")
	if len(out) != len(in) {
		t.Fatalf("default filter changed row count: got %d want %d", len(out), len(in))
	}
}

func TestFilterResultRowsOutOfRangeUsesLatestDraw(t *testing.T) {
	// A biomarker that WAS out of range historically but is now in range must
	// not appear under --out-of-range (the filter keys off the latest draw).
	rows := []resultRow{
		{BiomarkerName: "Glucose", Category: "Metabolic", DrawDate: "2024-01-01", Value: 200, OptimalLow: 70, OptimalHigh: 99}, // historically high
		{BiomarkerName: "Glucose", Category: "Metabolic", DrawDate: "2025-01-01", Value: 85, OptimalLow: 70, OptimalHigh: 99},  // now optimal
	}
	if got := biomarkerSet(filterResultRows(rows, true, "")); got["Glucose"] {
		t.Errorf("Glucose is optimal in its latest draw; should be excluded by --out-of-range")
	}
}

func TestCountOutOfRange(t *testing.T) {
	// fixtureRows: Iron latest above (out), ApoB in-optimal, ALT below (out).
	if got := countOutOfRange(fixtureRows()); got != 2 {
		t.Errorf("countOutOfRange(fixtureRows) = %d, want 2", got)
	}
}

func TestCountOutOfRangeUnnamedBiomarkersDoNotCollapse(t *testing.T) {
	// Two distinct out-of-optimal biomarkers carrying only an ID (no name) must
	// not collapse into a single "" bucket — that would undercount to 1.
	rows := []resultRow{
		{BiomarkerID: "id-a", DrawDate: "2024-01-01", Value: 200, OptimalLow: 50, OptimalHigh: 150}, // above
		{BiomarkerID: "id-b", DrawDate: "2024-01-01", Value: 5, OptimalLow: 10, OptimalHigh: 40},     // below
		{BiomarkerID: "id-c", BiomarkerName: "ApoB", DrawDate: "2024-01-01", Value: 70, OptimalLow: 0, OptimalHigh: 90}, // in-optimal
	}
	if got := countOutOfRange(rows); got != 2 {
		t.Errorf("countOutOfRange with unnamed biomarkers = %d, want 2", got)
	}
}

func TestPdfScopeLabel(t *testing.T) {
	cases := []struct {
		oor     bool
		section string
		want    string
	}{
		{false, "", "Complete lab history"},
		{true, "", "Out-of-optimal biomarkers"},
		{false, "Liver", "Section: Liver"},
		{true, "Liver", "Out-of-optimal biomarkers — section: Liver"},
	}
	for _, c := range cases {
		if got := pdfScopeLabel(c.oor, c.section); got != c.want {
			t.Errorf("pdfScopeLabel(%v,%q) = %q, want %q", c.oor, c.section, got, c.want)
		}
	}
}
