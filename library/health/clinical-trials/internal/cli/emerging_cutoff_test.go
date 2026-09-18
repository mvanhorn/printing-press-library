// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"testing"
	"time"
)

// The corpus in emerging_summarize_test.go places its recent trials one year
// before the current year and its prior trials eight years before, so no trial
// sits near the cohort cutoff. Changing `yr >= cutoff` to `yr > cutoff`, or
// moving the cutoff by a year, leaves that whole suite green. The tests below
// put trials on the cutoff year and on each side of it.
//
// Dates are relative to the year the test runs in, for the same reason as the
// existing corpus: summarizeEmerging derives its cutoff from time.Now.

func cutoffTrial(id string, year int) Trial {
	return Trial{NCTID: id, StartDate: fmt.Sprintf("%d-06-15", year)}
}

// summarizeInOneYear runs summarizeEmerging with fixtures built for the year
// read before the call, and returns that year. summarizeEmerging reads the
// clock itself, so the year is read again after the call: when both reads
// agree, the read inside the call happened between them and saw the same
// year. When they differ the run crossed a New Year boundary, and the test is
// skipped rather than failed, because the fixtures and the code would be
// measuring against different years.
func summarizeInOneYear(t *testing.T, recentYears, limit int, build func(year int) []Trial) (emergingView, int) {
	t.Helper()
	before := time.Now().Year()
	view := summarizeEmerging(build(before), recentYears, limit)
	if after := time.Now().Year(); after != before {
		t.Skipf("year changed during the test (%d to %d); cutoff cannot be pinned on this run", before, after)
	}
	return view, before
}

// TestSummarizeEmergingCutoffYearIsRecent pins the boundary: a trial that
// started in the cutoff year belongs to the recent cohort, the year before it
// does not, and the reported RecentSinceYear is that same cutoff year.
func TestSummarizeEmergingCutoffYearIsRecent(t *testing.T) {
	cases := []struct {
		name        string
		recentYears int
	}{
		{"default window", 3},
		{"one year", 1},
		{"zero years", 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			view, year := summarizeInOneYear(t, c.recentYears, 10, func(year int) []Trial {
				cutoff := year - c.recentYears
				return []Trial{
					cutoffTrial("before", cutoff-1),
					cutoffTrial("on", cutoff),
					cutoffTrial("after", cutoff+1),
				}
			})
			cutoff := year - c.recentYears

			if view.RecentSinceYear != cutoff {
				t.Errorf("RecentSinceYear = %d, want %d", view.RecentSinceYear, cutoff)
			}
			if view.RecentCohort != 2 || view.PriorCohort != 1 {
				t.Errorf("recent/prior = %d/%d, want 2/1 (cutoff %d: %d and %d recent, %d prior)",
					view.RecentCohort, view.PriorCohort, cutoff, cutoff, cutoff+1, cutoff-1)
			}
		})
	}
}

// TestSummarizeEmergingCutoffYearCountsTowardRecentGrowth checks that the
// boundary decides where a trial's interventions are tallied, not only which
// cohort counter it bumps. Two trials on the cutoff year and one the year
// before give the label a recent count of 2 and a prior count of 1.
func TestSummarizeEmergingCutoffYearCountsTowardRecentGrowth(t *testing.T) {
	const recentYears = 3
	view, _ := summarizeInOneYear(t, recentYears, 10, func(year int) []Trial {
		cutoff := year - recentYears
		return []Trial{
			{NCTID: "on-1", StartDate: fmt.Sprintf("%d-01-02", cutoff), Interventions: []string{"Tirzepatide"}},
			{NCTID: "on-2", StartDate: fmt.Sprintf("%d-12-30", cutoff), Interventions: []string{"Tirzepatide"}},
			{NCTID: "before", StartDate: fmt.Sprintf("%d-12-30", cutoff-1), Interventions: []string{"Tirzepatide"}},
		}
	})

	got, ok := growthFor(view.FastestGrowing, normalizeCategoryToken("Tirzepatide"))
	if !ok {
		t.Fatalf("Tirzepatide missing from FastestGrowing: %+v", view.FastestGrowing)
	}
	if got.Recent != 2 || got.Prior != 1 {
		t.Errorf("Tirzepatide recent/prior = %d/%d, want 2/1", got.Recent, got.Prior)
	}
}
