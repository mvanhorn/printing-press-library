// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// growthCounter returns a counter holding each label the given number of
// times, built through add so the fixture goes through the same trimming as
// production tallies.
func growthCounter(counts map[string]int) *counter {
	c := newCounter()
	for label, n := range counts {
		for i := 0; i < n; i++ {
			c.add(label)
		}
	}
	return c
}

// TestGrowthRoundsHalfAwayFromZero pins the growth percentage to the nearest
// integer on both sides of zero. Adding 0.5 before an int conversion only
// rounds positive values, because the conversion truncates toward zero, so a
// shrinking category lost one point: a drop from 4 to 3 read as -24.
func TestGrowthRoundsHalfAwayFromZero(t *testing.T) {
	cases := []struct {
		name          string
		recent, prior int
		want          int
	}{
		{name: "3 vs 4 is -25.0", recent: 3, prior: 4, want: -25},
		{name: "2 vs 4 is -50.0", recent: 2, prior: 4, want: -50},
		{name: "2 vs 5 is -60.0", recent: 2, prior: 5, want: -60},
		{name: "2 vs 3 is -33.3", recent: 2, prior: 3, want: -33},
		{name: "5 vs 7 is -28.6", recent: 5, prior: 7, want: -29},
		{name: "7 vs 8 is -12.5", recent: 7, prior: 8, want: -13},
		{name: "3 vs 8 is -62.5", recent: 3, prior: 8, want: -63},
		{name: "4 vs 4 is 0", recent: 4, prior: 4, want: 0},
		{name: "3 vs 2 is +50.0", recent: 3, prior: 2, want: 50},
		{name: "4 vs 3 is +33.3", recent: 4, prior: 3, want: 33},
		{name: "9 vs 8 is +12.5", recent: 9, prior: 8, want: 13},
		{name: "2 vs 1 is +100.0", recent: 2, prior: 1, want: 100},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := growth(
				growthCounter(map[string]int{"drug-x": tc.recent}),
				growthCounter(map[string]int{"drug-x": tc.prior}),
				0,
			)
			if len(got) != 1 {
				t.Fatalf("growth returned %d entries, want 1: %+v", len(got), got)
			}
			e := got[0]
			if e.Recent != tc.recent || e.Prior != tc.prior {
				t.Errorf("counts = %d/%d, want %d/%d", e.Recent, e.Prior, tc.recent, tc.prior)
			}
			if e.NewlyAdded {
				t.Errorf("NewlyAdded = true, want false for a category with a baseline")
			}
			if e.GrowthPct != tc.want {
				t.Errorf("GrowthPct = %d, want %d", e.GrowthPct, tc.want)
			}
		})
	}
}

// TestGrowthZeroBaselineIsNewlyAdded pins the no-baseline branch: a category
// absent from the prior cohort is flagged and reported as 100 rather than
// dividing by zero.
func TestGrowthZeroBaselineIsNewlyAdded(t *testing.T) {
	got := growth(
		growthCounter(map[string]int{"drug-new": 2}),
		growthCounter(map[string]int{"drug-other": 5}),
		0,
	)
	if len(got) != 1 {
		t.Fatalf("growth returned %d entries, want 1: %+v", len(got), got)
	}
	e := got[0]
	if e.Label != "drug-new" || e.Prior != 0 || !e.NewlyAdded || e.GrowthPct != 100 {
		t.Errorf("entry = %+v, want drug-new with prior 0, NewlyAdded and GrowthPct 100", e)
	}
}

// TestGrowthSkipsSingleRecentOccurrence pins the noise floor: a label needs at
// least two recent occurrences to be listed, however large its prior count,
// and a label seen only in the prior cohort is never listed.
func TestGrowthSkipsSingleRecentOccurrence(t *testing.T) {
	got := growth(
		growthCounter(map[string]int{"drug-once": 1, "drug-twice": 2}),
		growthCounter(map[string]int{"drug-once": 5, "drug-twice": 1, "drug-gone": 4}),
		0,
	)
	if len(got) != 1 || got[0].Label != "drug-twice" {
		t.Fatalf("growth = %+v, want only drug-twice", got)
	}
}
