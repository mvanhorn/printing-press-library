// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// Guards what the phase tally counts: one entry per phase a trial is posted
// under, including a synthetic N/A entry for trials posted under none.
package cli

import "testing"

// phaseTallyCorpus is a twelve-trial sample shaped like a real registry page.
// The multiplicities matter more than the labels: two trials carry no phase,
// eight carry exactly one, and two carry two. A corpus where every trial has
// exactly one phase cannot tell a per-trial tally from a per-entry one — they
// give the same number — which is how the multi-phase behaviour went unnoticed
// when this file was first written.
func phaseTallyCorpus() []Trial {
	return []Trial{
		{NCTID: "NCT00000001", Phases: []string{"PHASE1"}},
		{NCTID: "NCT00000002", Phases: []string{"PHASE2"}},
		{NCTID: "NCT00000003", Phases: []string{"PHASE3"}},
		{NCTID: "NCT00000004", Phases: []string{"PHASE3"}},
		{NCTID: "NCT00000005", Phases: []string{"PHASE4"}},
		{NCTID: "NCT00000006", Phases: []string{"EARLY_PHASE1"}},
		{NCTID: "NCT00000007", Phases: []string{"NA"}},
		{NCTID: "NCT00000008", Phases: []string{"NA"}},
		{NCTID: "NCT00000009", Phases: []string{"PHASE1", "PHASE2"}},
		{NCTID: "NCT00000010", Phases: []string{"PHASE2", "PHASE3"}},
		{NCTID: "NCT00000011"}, // observational: no Phases key
		{NCTID: "NCT00000012"}, // observational: no Phases key
	}
}

// phaseEntries is what the tally is expected to sum to: one per posted phase,
// plus one for each trial posted under no phase at all.
func phaseEntries(trials []Trial) int {
	n := 0
	for _, t := range trials {
		if len(t.Phases) == 0 {
			n++
			continue
		}
		n += len(t.Phases)
	}
	return n
}

func tallyTotal(entries []rankedEntry) int {
	total := 0
	for _, e := range entries {
		total += e.Count
	}
	return total
}

// The tally counts phase entries, not trials, and the two differ in both
// directions. A trial with no phase contributed nothing until the N/A guard
// landed; a trial posted under two phases contributes twice and always has.
// On this corpus that is 14 entries from 12 trials, so a distribution printed
// beside "12 trials" legitimately sums to more than 12.
func TestPhaseTallyCountsEveryPhaseEntry(t *testing.T) {
	corpus := phaseTallyCorpus()
	want := phaseEntries(corpus)
	if want != 14 {
		t.Fatalf("fixture drifted: phaseEntries = %d, want 14", want)
	}
	view := buildTrialListView("q", "RECRUITING", len(corpus), corpus)
	if got := tallyTotal(view.PhaseDistribution); got != want {
		t.Errorf("phase distribution sums to %d, want %d (one entry per posted phase, one for each phaseless trial)", got, want)
	}
	if got := tallyTotal(view.PhaseDistribution); got <= len(corpus) {
		t.Errorf("sum %d does not exceed the %d trials — the multi-phase trials are not being counted twice", got, len(corpus))
	}
	if view.Returned != len(corpus) {
		t.Errorf("returned = %d, want %d", view.Returned, len(corpus))
	}
}

// A trial posted under two phases lands in both buckets. Phase 1 holds the
// single-phase NCT00000001 plus the two-phase NCT00000009; Phase 3 holds the
// two single-phase trials plus the two-phase NCT00000010.
func TestMultiPhaseTrialCountsInEveryBucket(t *testing.T) {
	view := buildTrialListView("q", "RECRUITING", 12, phaseTallyCorpus())
	got := map[string]int{}
	for _, e := range view.PhaseDistribution {
		got[e.Label] = e.Count
	}
	for _, tc := range []struct {
		label string
		want  int
	}{
		{"Phase 1", 2},
		{"Phase 2", 3},
		{"Phase 3", 3},
		{"N/A", 4},
	} {
		if got[tc.label] != tc.want {
			t.Errorf("%s bucket = %d, want %d", tc.label, got[tc.label], tc.want)
		}
	}
}

// Phaseless trials land in N/A alongside the explicit "NA" ones: two of each
// in this corpus, so the bucket holds four. That is the same answer
// phaseDisplay gives a phaseless trial for the Phase column, which is why the
// summary and the printed rows agree.
func TestPhaselessTrialsCountAsNA(t *testing.T) {
	view := buildTrialListView("q", "RECRUITING", 12, phaseTallyCorpus())
	found := false
	for _, e := range view.PhaseDistribution {
		if e.Label != "N/A" {
			continue
		}
		found = true
		if e.Count != 4 {
			t.Errorf("N/A bucket = %d, want 4 (2 explicit NA + 2 phaseless)", e.Count)
		}
	}
	if !found {
		t.Error("no N/A bucket in the distribution")
	}
	if got := phaseDisplay(nil); got != "N/A" {
		t.Errorf("phaseDisplay(nil) = %q, want %q — the tally must agree with the column", got, "N/A")
	}
}

// A corpus of nothing but observational studies must still produce a
// distribution. Before the N/A guard this returned zero buckets, so the
// caller's `len(PhaseDistribution) == 0` branch reported "no distribution
// computed" for a sample that was in fact fully described by one.
func TestAllPhaselessCorpusStillTallies(t *testing.T) {
	corpus := []Trial{{NCTID: "NCT1"}, {NCTID: "NCT2"}, {NCTID: "NCT3"}}
	view := buildTrialListView("q", "RECRUITING", len(corpus), corpus)
	if len(view.PhaseDistribution) == 0 {
		t.Fatal("no phase distribution for an all-observational corpus")
	}
	if got := tallyTotal(view.PhaseDistribution); got != len(corpus) {
		t.Errorf("phase distribution sums to %d, want %d", got, len(corpus))
	}
}
