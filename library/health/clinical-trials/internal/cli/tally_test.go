// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// phaseTallyCorpus and phaseEntries live in phase_tally_test.go. The corpus is
// deliberately mixed — two trials with no phase, eight with one, two with two —
// because a corpus where every trial has exactly one phase cannot tell a
// per-trial tally from a per-entry one.

// tallyPhases is the extraction #1973 left open: compare, emerging and report
// tallied inside RunE behind a live client, so only recruiting's copy was
// reachable from a test. These assertions now cover the shared helper all four
// call.
func TestTallyPhasesCountsEntriesNotTrials(t *testing.T) {
	corpus := phaseTallyCorpus()
	got := tallyTotal(tallyPhases(corpus).top(8))
	want := phaseEntries(corpus)

	if want <= len(corpus) {
		t.Fatalf("corpus cannot distinguish per-entry from per-trial counting: "+
			"%d entries from %d trials", want, len(corpus))
	}
	if got != want {
		t.Errorf("tally summed to %d, want %d entries from %d trials",
			got, want, len(corpus))
	}
}

// Each assertion below names a multiplicity the corpus supplies, so a helper
// that collapsed any of them fails on the count that proves it.
func TestTallyPhasesPerBucketCounts(t *testing.T) {
	counts := map[string]int{}
	for _, e := range tallyPhases(phaseTallyCorpus()).top(8) {
		counts[e.Label] = e.Count
	}

	// Phase 1: one single-phase trial plus one half of a PHASE1/PHASE2 trial.
	// Phase 2: one single, plus halves of PHASE1/PHASE2 and PHASE2/PHASE3.
	// Phase 3: two singles plus one half of PHASE2/PHASE3.
	// N/A: two trials posted "NA" plus two with no Phases key at all — the
	// bucket merges the two registry states, which is the documented design.
	for _, c := range []struct {
		label string
		want  int
	}{
		{"Phase 1", 2},
		{"Phase 2", 3},
		{"Phase 3", 3},
		{"Phase 4", 1},
		{"Early Phase 1", 1},
		{"N/A", 4},
	} {
		if got := counts[c.label]; got != c.want {
			t.Errorf("%s = %d, want %d", c.label, got, c.want)
		}
	}
}

// A WIRING check, not a behaviour one, and the distinction matters. Once
// buildTrialListView delegates to tallyPhases both sides move together, so
// mutating the helper cannot fail this test — the two assertions above are
// what catch that. What this does catch is a caller that was left holding its
// own copy of the loop: four copies existed before this extraction, and a
// fifth reappearing is exactly the regression worth guarding.
func TestTallyPhasesMatchesBuildTrialListView(t *testing.T) {
	corpus := phaseTallyCorpus()
	view := buildTrialListView("q", "RECRUITING", len(corpus), corpus)

	direct := map[string]int{}
	for _, e := range tallyPhases(corpus).top(8) {
		direct[e.Label] = e.Count
	}
	if len(view.PhaseDistribution) != len(direct) {
		t.Fatalf("bucket count differs: view %d, helper %d",
			len(view.PhaseDistribution), len(direct))
	}
	for _, e := range view.PhaseDistribution {
		if direct[e.Label] != e.Count {
			t.Errorf("%s: view %d, helper %d", e.Label, e.Count, direct[e.Label])
		}
	}
}
