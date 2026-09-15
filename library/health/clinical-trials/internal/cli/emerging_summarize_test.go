// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"testing"
	"time"
)

// emergingCorpus builds a sample whose dates are relative to the year the test
// runs in, because summarizeEmerging derives its cutoff from time.Now. Fixed
// years would drift out of the recent window as the calendar moves and the
// suite would start failing on a date rather than on a change to the code.
//
// With recentYears=3 the cutoff is currentYear-3. The corpus holds:
//
//	3 trials in the recent cohort, one of them with an unparseable date
//	2 trials in the prior cohort
//	interventions and conditions at zero, one and more than one per trial
//	one country appearing in both cohorts, one in neither
func emergingCorpus() []Trial {
	y := time.Now().Year()
	recent := fmt.Sprintf("%d-04-01", y-1)
	prior := fmt.Sprintf("%d-04-01", y-8)

	return []Trial{
		{
			NCTID: "NCT01", StartDate: recent,
			Interventions: []string{"Semaglutide", "Metformin"},
			Conditions:    []string{"Obesity"},
			Countries:     []string{"United States"},
		},
		{
			NCTID: "NCT02", StartDate: recent,
			Interventions: []string{"Semaglutide (1.0 mg)"},
			Conditions:    []string{"Obesity", "Type 2 Diabetes"},
			Countries:     []string{"United States", "Denmark"},
		},
		{
			NCTID: "NCT03", StartDate: "",
			Interventions: []string{"semaglutide"},
			Conditions:    []string{"obesity"},
			Countries:     nil,
		},
		{
			NCTID: "NCT04", StartDate: prior,
			Interventions: []string{"Metformin"},
			Conditions:    []string{"Type 2 Diabetes"},
			Countries:     []string{"United States"},
		},
		{
			NCTID: "NCT05", StartDate: prior,
			Interventions: nil,
			Conditions:    nil,
			Countries:     []string{"Japan"},
		},
	}
}

func growthFor(entries []growthEntry, label string) (growthEntry, bool) {
	for _, e := range entries {
		if e.Label == label {
			return e, true
		}
	}
	return growthEntry{}, false
}

func rankFor(entries []rankedEntry, label string) int {
	for _, e := range entries {
		if e.Label == label {
			return e.Count
		}
	}
	return 0
}

// TestSummarizeEmergingUnparsedDateCountsAsRecent pins the cohort rule for a
// trial with no usable start date. NCT03 has an empty StartDate; it must land
// in the recent cohort, not the prior one. Sending it to prior would inflate
// the baseline every growth percentage is measured against, so this decides
// numbers a reader acts on rather than a label.
func TestSummarizeEmergingUnparsedDateCountsAsRecent(t *testing.T) {
	t.Parallel()

	v := summarizeEmerging(emergingCorpus(), 3, 10)

	if v.RecentCohort != 3 {
		t.Errorf("RecentCohort = %d, want 3 (two dated, one unparsed)", v.RecentCohort)
	}
	if v.PriorCohort != 2 {
		t.Errorf("PriorCohort = %d, want 2", v.PriorCohort)
	}
	if v.RecentCohort+v.PriorCohort != v.SampleSize {
		t.Errorf("cohorts sum to %d, SampleSize is %d - every trial must land in exactly one",
			v.RecentCohort+v.PriorCohort, v.SampleSize)
	}
}

// TestSummarizeEmergingNormalizesBeforeTallying covers the interaction between
// normalizeCategoryToken and the cohort counters. Three spellings of one drug
// appear across three trials - "Semaglutide", "Semaglutide (1.0 mg)" and
// "semaglutide" - and all three must collapse into one recent bucket. Without
// normalization each would be its own label, every count would be 1, and the
// growth table would report three different one-off entries instead of one
// real trend.
func TestSummarizeEmergingNormalizesBeforeTallying(t *testing.T) {
	t.Parallel()

	v := summarizeEmerging(emergingCorpus(), 3, 10)

	e, ok := growthFor(v.FastestGrowing, "semaglutide")
	if !ok {
		t.Fatalf("semaglutide missing from FastestGrowing: %+v", v.FastestGrowing)
	}
	if e.Recent != 3 {
		t.Errorf("semaglutide recent = %d, want 3 (three spellings collapsed)", e.Recent)
	}
}

// TestSummarizeEmergingNewCategoryIsFlagged pins what happens when a label has
// no prior presence. Dividing by a zero baseline is undefined, so the code
// assigns 100 and sets NewlyAdded rather than producing an infinity or a
// silently capped number. The flag is what lets a renderer say "new" instead
// of a percentage that would read as a measured change.
func TestSummarizeEmergingNewCategoryIsFlagged(t *testing.T) {
	t.Parallel()

	v := summarizeEmerging(emergingCorpus(), 3, 10)

	e, ok := growthFor(v.FastestGrowing, "semaglutide")
	if !ok {
		t.Fatal("semaglutide missing from FastestGrowing")
	}
	if !e.NewlyAdded {
		t.Error("semaglutide has no prior-cohort trials and must be flagged NewlyAdded")
	}
	if e.Prior != 0 {
		t.Errorf("semaglutide prior = %d, want 0", e.Prior)
	}
	if e.GrowthPct != 100 {
		t.Errorf("GrowthPct = %d, want 100 for a category with no baseline", e.GrowthPct)
	}
}

// TestSummarizeEmergingDropsSingleOccurrences guards the rc < 2 rule in growth.
// One appearance is not a trend, and a table listing every single-trial label
// would bury the real movement. "metformin" appears once in each cohort, so it
// must not reach the table at all - not as a zero-growth row, not as an entry
// with Recent 1.
func TestSummarizeEmergingDropsSingleOccurrences(t *testing.T) {
	t.Parallel()

	v := summarizeEmerging(emergingCorpus(), 3, 10)

	if e, ok := growthFor(v.FastestGrowing, "metformin"); ok {
		t.Errorf("metformin reached the table with %+v; one recent occurrence is noise", e)
	}
}

// TestSummarizeEmergingGeographySpansBothCohorts pins that countries are
// tallied across the whole sample rather than per cohort. "United States"
// appears twice in the recent cohort and once in the prior one; the hotspot
// count must be 3. A per-cohort split would report 2, which answers a
// different question than the field's name asks.
func TestSummarizeEmergingGeographySpansBothCohorts(t *testing.T) {
	t.Parallel()

	v := summarizeEmerging(emergingCorpus(), 3, 10)

	if got := rankFor(v.GeographicHotspots, "United States"); got != 3 {
		t.Errorf("United States = %d, want 3 (two recent, one prior)", got)
	}
	if got := rankFor(v.GeographicHotspots, "Japan"); got != 1 {
		t.Errorf("Japan = %d, want 1 (prior cohort only)", got)
	}
}

// TestSummarizeEmergingNoteOrderPrefersNoMatch is the reason the two note
// branches must stay in their current order. An empty sample satisfies BOTH
// conditions: len(trials) == 0 and priorN == 0. Only the ordering decides
// which message the reader sees, and reporting "all sampled trials fall in the
// recent cohort" for a sample with no trials would describe a cohort that does
// not exist.
func TestSummarizeEmergingNoteOrderPrefersNoMatch(t *testing.T) {
	t.Parallel()

	v := summarizeEmerging(nil, 3, 10)

	want := "no trials matched; try a broader category term"
	if v.Note != want {
		t.Errorf("Note = %q, want %q", v.Note, want)
	}
}

// TestSummarizeEmergingNoteFiresOnAllRecent covers the second branch on its
// own, so the ordering test above cannot pass merely because the second branch
// never fires. Every trial here is recent, so priorN is zero with a non-empty
// sample - the only shape that reaches it.
func TestSummarizeEmergingNoteFiresOnAllRecent(t *testing.T) {
	t.Parallel()

	y := time.Now().Year()
	trials := []Trial{
		{NCTID: "NCT01", StartDate: fmt.Sprintf("%d-01-01", y)},
		{NCTID: "NCT02", StartDate: fmt.Sprintf("%d-01-01", y-1)},
	}

	v := summarizeEmerging(trials, 3, 10)

	if v.PriorCohort != 0 {
		t.Fatalf("PriorCohort = %d, want 0 - fixture does not reach the branch", v.PriorCohort)
	}
	if v.Note == "" {
		t.Error("an all-recent sample must carry the baseline advisory")
	}
	if v.Note == "no trials matched; try a broader category term" {
		t.Error("a non-empty sample must not report no match")
	}
}

// TestSummarizeEmergingEmptyCorpus checks the empty case does not panic and
// reports zeros rather than leaving fields untouched, because a rendered zero
// and an absent value read the same to a consumer once they reach JSON.
func TestSummarizeEmergingEmptyCorpus(t *testing.T) {
	t.Parallel()

	v := summarizeEmerging(nil, 3, 10)

	if v.SampleSize != 0 || v.RecentCohort != 0 || v.PriorCohort != 0 {
		t.Errorf("sample %d, recent %d, prior %d - all want 0",
			v.SampleSize, v.RecentCohort, v.PriorCohort)
	}
	if len(v.FastestGrowing) != 0 || len(v.GeographicHotspots) != 0 {
		t.Errorf("growth %+v, geo %+v - both want empty", v.FastestGrowing, v.GeographicHotspots)
	}
}
