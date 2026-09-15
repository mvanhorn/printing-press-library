// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// summarizeCorpus is shaped so that every quantity summarizeTrials derives has
// a zero, a one and a more-than-one case present at once, because a fixture
// where each trial carries exactly one of something cannot tell a per-trial
// tally from a per-entry one.
//
//	phases:   one trial with none, three with one, one with two
//	sponsors: one blank, one appearing three times, two appearing once
//
// Sponsor counts are deliberately distinct (3, 1, 1) rather than tied, so an
// assertion on the top entry does not depend on how counter.top breaks ties.
func summarizeCorpus() []Trial {
	return []Trial{
		{NCTID: "NCT01", Phases: []string{"PHASE3"}, Sponsor: "Pfizer"},
		{NCTID: "NCT02", Phases: []string{"PHASE3"}, Sponsor: "Pfizer"},
		{NCTID: "NCT03", Phases: []string{"PHASE1", "PHASE2"}, Sponsor: "Pfizer"},
		{NCTID: "NCT04", Phases: []string{"PHASE2"}, Sponsor: "NIH"},
		{NCTID: "NCT05", Phases: nil, Sponsor: ""},
		{NCTID: "NCT06", Phases: []string{}, Sponsor: "Novartis"},
	}
}

func phaseCount(entries []rankedEntry, label string) int {
	for _, e := range entries {
		if e.Label == label {
			return e.Count
		}
	}
	return 0
}

// TestSummarizeTrialsSampleSizeCountsEveryTrial pins the one number a reader
// compares the distribution against. It counts trials, not phase entries, so a
// corpus carrying eight phase entries across six trials must still report six.
func TestSummarizeTrialsSampleSizeCountsEveryTrial(t *testing.T) {
	t.Parallel()

	p := &drugProfile{}
	summarizeTrials(p, summarizeCorpus(), 5)

	if p.SampleSize != 6 {
		t.Errorf("SampleSize = %d, want 6", p.SampleSize)
	}
}

// TestSummarizeTrialsReachesTallyPhases is the reason this file exists. The
// generated compare_test.go rebuilds the phase loop in its own body and so
// never reaches tallyPhases, where the phaseless guard added upstream lives.
// Two trials here carry no phase at all; if the production path did not run
// through tallyPhases they would enter no bucket and the N/A count would be
// zero rather than two.
func TestSummarizeTrialsReachesTallyPhases(t *testing.T) {
	t.Parallel()

	p := &drugProfile{}
	summarizeTrials(p, summarizeCorpus(), 5)

	if got := phaseCount(p.PhaseDistribution, "N/A"); got != 2 {
		t.Errorf("N/A bucket = %d, want 2 (the two trials with no phase)", got)
	}

	want := map[string]int{"Phase 1": 1, "Phase 2": 2, "Phase 3": 2}
	for label, n := range want {
		if got := phaseCount(p.PhaseDistribution, label); got != n {
			t.Errorf("%s = %d, want %d", label, got, n)
		}
	}
}

// TestSummarizeTrialsDistributionCountsEntries states the contract the ledger
// records: the distribution sums to the number of phase entries, which may
// exceed the sample. NCT03 runs in two phases and legitimately contributes to
// both, so six trials produce seven entries. A sum equal to SampleSize would
// mean a multi-phase trial had been silently collapsed.
func TestSummarizeTrialsDistributionCountsEntries(t *testing.T) {
	t.Parallel()

	p := &drugProfile{}
	summarizeTrials(p, summarizeCorpus(), 5)

	sum := 0
	for _, e := range p.PhaseDistribution {
		sum += e.Count
	}
	if sum != 7 {
		t.Errorf("phase entries = %d, want 7 (6 trials, one of them in two phases)", sum)
	}
}

// TestSummarizeTrialsSponsorTally covers the counter compare uses for
// sponsors. The blank sponsor matters: a trial with no sponsor posted must not
// become an empty-labelled row in a ranking a human reads.
func TestSummarizeTrialsSponsorTally(t *testing.T) {
	t.Parallel()

	p := &drugProfile{}
	summarizeTrials(p, summarizeCorpus(), 5)

	if len(p.TopSponsors) == 0 {
		t.Fatal("TopSponsors is empty")
	}
	if p.TopSponsors[0].Label != "Pfizer" || p.TopSponsors[0].Count != 3 {
		t.Errorf("top sponsor = %+v, want Pfizer x3", p.TopSponsors[0])
	}
	for _, e := range p.TopSponsors {
		if e.Label == "" {
			t.Errorf("a blank sponsor reached the ranking: %+v", p.TopSponsors)
		}
	}
}

// TestSummarizeTrialsHonoursTopSponsors checks that the --top-sponsors flag
// reaches the ranking. The corpus holds three named sponsors, so a limit of
// two must drop one; without the parameter being threaded through, all three
// would appear.
func TestSummarizeTrialsHonoursTopSponsors(t *testing.T) {
	t.Parallel()

	p := &drugProfile{}
	summarizeTrials(p, summarizeCorpus(), 2)

	if len(p.TopSponsors) != 2 {
		t.Errorf("TopSponsors length = %d, want 2", len(p.TopSponsors))
	}
}

// TestSummarizeTrialsEmptyCorpus pins the empty case. Nothing here may panic,
// and SampleSize must read zero rather than being left untouched, because a
// caller printing a profile cannot distinguish "not computed" from "computed
// as zero" once the field is rendered.
func TestSummarizeTrialsEmptyCorpus(t *testing.T) {
	t.Parallel()

	p := &drugProfile{SampleSize: 99}
	summarizeTrials(p, nil, 5)

	if p.SampleSize != 0 {
		t.Errorf("SampleSize = %d, want 0", p.SampleSize)
	}
	if len(p.PhaseDistribution) != 0 {
		t.Errorf("PhaseDistribution = %+v, want empty", p.PhaseDistribution)
	}
	if len(p.TopSponsors) != 0 {
		t.Errorf("TopSponsors = %+v, want empty", p.TopSponsors)
	}
}
