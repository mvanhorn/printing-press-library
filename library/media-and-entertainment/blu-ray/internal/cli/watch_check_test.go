// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// TestWatchEntryEvalDeal exercises the `watch check` alert decision in isolation
// from the deals scrape + SQLite persistence: a deal alerts when it is at/below a
// set target price OR strictly below the running historical low, and the running
// low advances so a later, higher deal in the same scan can't re-fire a new-low
// alert.
func TestWatchEntryEvalDeal(t *testing.T) {
	tests := []struct {
		name       string
		entry      watchEntry
		sale       float64
		wantAlert  bool
		wantLow    float64
		wantHasLow bool
	}{
		{"target hit exactly", watchEntry{target: 15}, 15, true, 15, true},
		{"target undercut", watchEntry{target: 15}, 12.5, true, 12.5, true},
		{"above target, no prior low, no alert", watchEntry{target: 15}, 19.99, false, 19.99, true},
		{"no target, first sighting sets low but does not alert", watchEntry{}, 20, false, 20, true},
		{"no target, new historical low alerts", watchEntry{prevLow: 20, hasPrevLow: true}, 18, true, 18, true},
		{"no target, equal to prior low is not a new low", watchEntry{prevLow: 18, hasPrevLow: true}, 18, false, 18, true},
		{"no target, above prior low: no alert, low unchanged", watchEntry{prevLow: 18, hasPrevLow: true}, 25, false, 18, true},
		{"target hit and new low together", watchEntry{target: 15, prevLow: 20, hasPrevLow: true}, 12, true, 12, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alert, got := tt.entry.evalDeal(tt.sale)
			if alert != tt.wantAlert {
				t.Errorf("evalDeal(%.2f) alert = %v, want %v", tt.sale, alert, tt.wantAlert)
			}
			if got.prevLow != tt.wantLow || got.hasPrevLow != tt.wantHasLow {
				t.Errorf("evalDeal(%.2f) low = (%.2f, %v), want (%.2f, %v)",
					tt.sale, got.prevLow, got.hasPrevLow, tt.wantLow, tt.wantHasLow)
			}
		})
	}
}

// TestWatchEntryEvalDeal_RunningLowDedup proves the scan-local running low: given
// a prior low of $20 and deals of $18 then $19 in one scan, only $18 fires a
// new-low alert; $19 must not (it is above the running low of $18). Guards the
// PR #634 follow-up fix against a regression that would double-alert.
func TestWatchEntryEvalDeal_RunningLowDedup(t *testing.T) {
	e := watchEntry{prevLow: 20, hasPrevLow: true} // no target set
	alert1, e := e.evalDeal(18)
	if !alert1 {
		t.Fatalf("first deal $18 should alert as a new low below $20")
	}
	alert2, e := e.evalDeal(19)
	if alert2 {
		t.Errorf("second deal $19 should NOT alert (running low is $18); got an alert")
	}
	if e.prevLow != 18 {
		t.Errorf("running low = %.2f, want 18 (unchanged by the $19 row)", e.prevLow)
	}
}
