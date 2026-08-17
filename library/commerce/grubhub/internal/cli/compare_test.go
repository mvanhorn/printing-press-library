// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func sampleRows() []restaurantRow {
	return []restaurantRow{
		{ID: "1", Name: "Bravo", DeliveryFeeCents: 300, MinimumCents: 1000, ETAMinutes: 25, Rating: 4.2, DistanceMiles: 1.0, Deals: 0},
		{ID: "2", Name: "Alpha", DeliveryFeeCents: 0, MinimumCents: 1500, ETAMinutes: 40, Rating: 4.9, DistanceMiles: 0.3, Deals: 2},
		{ID: "3", Name: "Cosmo", DeliveryFeeCents: 100, MinimumCents: 0, ETAMinutes: 15, Rating: 3.5, DistanceMiles: 2.5, Deals: 1},
	}
}

func TestSortRows(t *testing.T) {
	cases := []struct {
		key     string
		wantTop string
	}{
		{"fee", "Alpha"},      // fee 0
		{"min", "Cosmo"},      // minimum 0
		{"eta", "Cosmo"},      // 15m
		{"rating", "Alpha"},   // 4.9
		{"distance", "Alpha"}, // 0.3mi
		{"deals", "Alpha"},    // 2 deals
		{"name", "Alpha"},     // alphabetical
	}
	for _, tc := range cases {
		rows := sampleRows()
		sortRows(rows, tc.key)
		if rows[0].Name != tc.wantTop {
			t.Errorf("sort %q: top = %s, want %s", tc.key, rows[0].Name, tc.wantTop)
		}
	}
}

func TestFilterComparison(t *testing.T) {
	rows := sampleRows()
	// max-fee $1.00 keeps fee<=100 (Alpha 0, Cosmo 100), drops Bravo 300.
	got := filterComparison(rows, 1.0, 0, 0)
	if len(got) != 2 {
		t.Fatalf("max-fee filter kept %d, want 2", len(got))
	}
	// eta-under 20 keeps only Cosmo (15).
	got = filterComparison(rows, 0, 0, 20)
	if len(got) != 1 || got[0].Name != "Cosmo" {
		t.Errorf("eta-under filter = %+v, want only Cosmo", got)
	}
	// max-min $12 keeps minimum<=1200 (Bravo 1000, Cosmo 0), drops Alpha 1500.
	got = filterComparison(rows, 0, 12.0, 0)
	if len(got) != 2 {
		t.Errorf("max-min filter kept %d, want 2", len(got))
	}
}
