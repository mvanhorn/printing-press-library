// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestSlopePerRound(t *testing.T) {
	tests := []struct {
		name string
		rows []resultRow
		want float64
	}{
		{name: "rising 10/round", rows: []resultRow{draw(100, 50, 150), draw(110, 50, 150), draw(120, 50, 150)}, want: 10},
		{name: "falling 10/round", rows: []resultRow{draw(120, 50, 150), draw(110, 50, 150), draw(100, 50, 150)}, want: -10},
		{name: "flat", rows: []resultRow{draw(120, 50, 150), draw(120, 50, 150)}, want: 0},
		{name: "single draw is slope 0", rows: []resultRow{draw(120, 50, 150)}, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := slopePerRound(tc.rows, 0); got != tc.want {
				t.Errorf("slopePerRound = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestDriftAway covers the projection onto "moving away from the optimal
// midpoint" (optimal 50-150 → midpoint 100). Positive drift = worse (drifting
// away), negative = better (drifting toward optimal).
func TestDriftAway(t *testing.T) {
	tests := []struct {
		name        string
		rows        []resultRow
		wantWorse   bool // drift > 0
		wantBetter  bool // drift < 0
		wantNeutral bool // drift == 0
	}{
		{
			name:      "above midpoint and rising = worse",
			rows:      []resultRow{draw(160, 50, 150), draw(180, 50, 150), draw(200, 50, 150)},
			wantWorse: true,
		},
		{
			name:      "below midpoint and falling = worse",
			rows:      []resultRow{draw(40, 50, 150), draw(30, 50, 150), draw(20, 50, 150)},
			wantWorse: true,
		},
		{
			name:       "above midpoint and falling = better",
			rows:       []resultRow{draw(200, 50, 150), draw(180, 50, 150), draw(160, 50, 150)},
			wantBetter: true,
		},
		{
			name:        "flat = neutral",
			rows:        []resultRow{draw(120, 50, 150), draw(120, 50, 150)},
			wantNeutral: true,
		},
		{
			name:        "no optimal bounds = neutral",
			rows:        []resultRow{draw(40, 0, 0), draw(30, 0, 0)},
			wantNeutral: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := driftAway(tc.rows, 0)
			switch {
			case tc.wantWorse && got <= 0:
				t.Errorf("driftAway = %v, want > 0 (worse)", got)
			case tc.wantBetter && got >= 0:
				t.Errorf("driftAway = %v, want < 0 (better)", got)
			case tc.wantNeutral && got != 0:
				t.Errorf("driftAway = %v, want 0 (neutral)", got)
			}
		})
	}
}
