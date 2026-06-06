// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestGoatScore(t *testing.T) {
	tests := []struct {
		name     string
		distance float64
		drift    float64
		want     float64
	}{
		{name: "drift away amplifies distance", distance: 10, drift: 2, want: 20},
		{name: "flat slope earns baseline weight", distance: 10, drift: 0, want: 1},
		{name: "drift toward optimal is clamped to baseline", distance: 10, drift: -5, want: 1},
		{name: "zero distance scores zero even when drifting", distance: 0, drift: 5, want: 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := goatScore(tc.distance, tc.drift); got != tc.want {
				t.Errorf("goatScore(%v, %v) = %v, want %v", tc.distance, tc.drift, got, tc.want)
			}
		})
	}
}

func TestGoatScoreRanksWorseningOverStable(t *testing.T) {
	// A biomarker the same distance from optimal but actively drifting away must
	// outrank one that is stable — the whole point of the goat ranking.
	worsening := goatScore(10, 3)
	stable := goatScore(10, 0)
	if worsening <= stable {
		t.Errorf("worsening score %.3f should exceed stable score %.3f", worsening, stable)
	}
}

func TestDistanceFromOptimalUsesMidpoint(t *testing.T) {
	// Optimal 50-150 → midpoint 100. A value of 130 is distance 30.
	r := draw(130, 50, 150)
	if got := distanceFromOptimal(r); got != 30 {
		t.Errorf("distanceFromOptimal = %v, want 30", got)
	}
	// No optimal bounds and no Quest range → distance 0 (uncomputable).
	if got := distanceFromOptimal(draw(130, 0, 0)); got != 0 {
		t.Errorf("distanceFromOptimal with no bounds = %v, want 0", got)
	}
}
