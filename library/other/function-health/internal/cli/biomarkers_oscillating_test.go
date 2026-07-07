// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// draw is a terse helper: value + optimal bounds. Bounds of (0,0) means the
// draw has no defined Function-optimal range (inconclusive).
func draw(value, lo, hi float64) resultRow {
	return resultRow{Value: value, OptimalLow: lo, OptimalHigh: hi}
}

func TestHasOptimal(t *testing.T) {
	if hasOptimal(draw(5, 0, 0)) {
		t.Error("draw with no bounds should report hasOptimal=false")
	}
	if !hasOptimal(draw(5, 1, 10)) {
		t.Error("draw with both bounds should report hasOptimal=true")
	}
	if !hasOptimal(draw(5, 0, 10)) {
		t.Error("draw with only an upper bound should report hasOptimal=true")
	}
}

func TestOptimalCrossings(t *testing.T) {
	tests := []struct {
		name          string
		series        []resultRow
		wantCrossings int
		wantDefined   int
	}{
		{
			name:          "above then below is one crossing",
			series:        []resultRow{draw(200, 50, 150), draw(10, 50, 150)},
			wantCrossings: 1, wantDefined: 2,
		},
		{
			name:          "above below above is two crossings",
			series:        []resultRow{draw(200, 50, 150), draw(10, 50, 150), draw(200, 50, 150)},
			wantCrossings: 2, wantDefined: 3,
		},
		{
			// THE BUG: an undefined-optimal draw between above and below must
			// NOT absorb the crossing. Pre-fix, optimalSign returned 0 for the
			// middle draw (same as in-range) and the crossing was lost.
			name:          "undefined-optimal draw between above/below still counts the crossing",
			series:        []resultRow{draw(200, 50, 150), draw(99, 0, 0), draw(10, 50, 150)},
			wantCrossings: 1, wantDefined: 2,
		},
		{
			name:          "fewer than two defined draws is inconclusive",
			series:        []resultRow{draw(200, 50, 150), draw(99, 0, 0)},
			wantCrossings: 0, wantDefined: 1,
		},
		{
			name:          "all in range is zero crossings",
			series:        []resultRow{draw(100, 50, 150), draw(120, 50, 150)},
			wantCrossings: 0, wantDefined: 2,
		},
		{
			// THE BOUNDARY-CROSSING BUG: above → in-range → above crosses the
			// upper optimal boundary twice. Pre-fix, in-range draws were treated
			// as transparent (sign 0 skipped both transitions) and this counted
			// 0 — the canonical "managed on/off a supplement" oscillation.
			name:          "above to in-range to above counts in/out crossings",
			series:        []resultRow{draw(200, 50, 150), draw(100, 50, 150), draw(200, 50, 150)},
			wantCrossings: 2, wantDefined: 3,
		},
		{
			name:          "in-range to below to in-range counts two crossings",
			series:        []resultRow{draw(100, 50, 150), draw(10, 50, 150), draw(100, 50, 150)},
			wantCrossings: 2, wantDefined: 3,
		},
		{
			name:          "single in/out crossing counts once",
			series:        []resultRow{draw(100, 50, 150), draw(200, 50, 150)},
			wantCrossings: 1, wantDefined: 2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, defined := optimalCrossings(tc.series)
			if c != tc.wantCrossings {
				t.Errorf("crossings = %d, want %d", c, tc.wantCrossings)
			}
			if len(defined) != tc.wantDefined {
				t.Errorf("defined draws = %d, want %d", len(defined), tc.wantDefined)
			}
		})
	}
}

func TestOptimalSignUnchanged(t *testing.T) {
	// optimalSign must keep conflating in-range and undefined as 0 — other call
	// sites (countOutOfRange, recommendations) rely on that. hasOptimal is the
	// new discriminator.
	if optimalSign(draw(100, 50, 150)) != 0 {
		t.Error("in-range should be sign 0")
	}
	if optimalSign(draw(5, 0, 0)) != 0 {
		t.Error("undefined-optimal should still be sign 0")
	}
	if optimalSign(draw(200, 50, 150)) != 1 {
		t.Error("above optimal should be sign +1")
	}
	if optimalSign(draw(10, 50, 150)) != -1 {
		t.Error("below optimal should be sign -1")
	}
}

func TestOptimalSignSingleBound(t *testing.T) {
	// Only a lower optimal bound (OptimalHigh == 0): a value above the floor is
	// in-range (0), never "above" (+1). This is the latent misclassification the
	// unguarded `Value > OptimalHigh` comparison produced for lower-bound-only
	// biomarkers.
	if got := optimalSign(draw(100, 50, 0)); got != 0 {
		t.Errorf("value above a lower-only bound = %d, want 0 (in-range)", got)
	}
	if got := optimalSign(draw(40, 50, 0)); got != -1 {
		t.Errorf("value below a lower-only bound = %d, want -1", got)
	}
	// Only an upper optimal bound (OptimalLow == 0): below the ceiling is
	// in-range, above it is +1.
	if got := optimalSign(draw(100, 0, 150)); got != 0 {
		t.Errorf("value below an upper-only bound = %d, want 0 (in-range)", got)
	}
	if got := optimalSign(draw(200, 0, 150)); got != 1 {
		t.Errorf("value above an upper-only bound = %d, want 1", got)
	}
}
