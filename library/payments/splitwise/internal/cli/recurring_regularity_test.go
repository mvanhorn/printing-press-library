// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// TestRecurringCadence checks the regularity gate that keeps coincidentally
// repeated descriptions (and same-trip bursts) out of the recurring report.
func TestRecurringCadence(t *testing.T) {
	cases := []struct {
		name        string
		gaps        []float64
		wantCadence int
		wantRegular bool
	}{
		{name: "steady monthly", gaps: []float64{30, 31, 29, 30}, wantCadence: 30, wantRegular: true},
		{name: "monthly with one skipped cycle (within spread)", gaps: []float64{30, 60, 30}, wantCadence: 40, wantRegular: true},
		{name: "same-trip burst (sub-cadence)", gaps: []float64{0, 1, 0}, wantCadence: 0, wantRegular: false},
		{name: "irregular across trips (spread too wide)", gaps: []float64{30, 800, 45}, wantRegular: false},
		{name: "evenly spaced but too infrequent (over annual cap)", gaps: []float64{800, 846, 820}, wantRegular: false},
		{name: "two occurrences ~11 months apart (within annual cap)", gaps: []float64{330}, wantCadence: 330, wantRegular: true},
		{name: "two occurrences years apart (over annual cap)", gaps: []float64{966}, wantRegular: false},
		{name: "zero min gap makes ratio undefined -> irregular", gaps: []float64{0, 30}, wantRegular: false},
		{name: "no gaps (single occurrence)", gaps: []float64{}, wantCadence: 0, wantRegular: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cadence, regular := recurringCadence(tc.gaps)
			if regular != tc.wantRegular {
				t.Fatalf("recurringCadence(%v) regular = %v, want %v", tc.gaps, regular, tc.wantRegular)
			}
			if tc.wantRegular && cadence != tc.wantCadence {
				t.Fatalf("recurringCadence(%v) cadence = %d, want %d", tc.gaps, cadence, tc.wantCadence)
			}
		})
	}
}

// TestIsSettlementDescription guards the settlement labels that Splitwise stores
// as non-payment expenses, which the e.Payment filter alone misses.
func TestIsSettlementDescription(t *testing.T) {
	settlements := []string{"Settle all balances", "settle up", "Payment", "Paid via Zelle", "  PAID VIA venmo "}
	for _, d := range settlements {
		if !isSettlementDescription(d) {
			t.Errorf("isSettlementDescription(%q) = false, want true", d)
		}
	}
	charges := []string{"Dinner", "Safeway", "Hotel deposit", "Uber to hotel"}
	for _, d := range charges {
		if isSettlementDescription(d) {
			t.Errorf("isSettlementDescription(%q) = true, want false", d)
		}
	}
}
