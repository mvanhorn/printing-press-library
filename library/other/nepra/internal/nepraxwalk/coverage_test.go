// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package nepraxwalk

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// heldOut is testdata/heldout_2020-21.json: the FY2020-21 name column, which
// was deliberately NOT used to build crosswalk.json.
type heldOut struct {
	FY   string `json:"fy"`
	Rows []struct {
		SNo                    int    `json:"sno"`
		PublishedName          string `json:"published_name"`
		InstalledCapacityMWRaw string `json:"installed_capacity_mw_raw"`
	} `json:"rows"`
}

func loadHeldOut(t *testing.T) heldOut {
	t.Helper()
	b, err := os.ReadFile("testdata/heldout_2020-21.json")
	if err != nil {
		t.Fatalf("reading held-out fixture: %v", err)
	}
	var h heldOut
	if err := json.Unmarshal(b, &h); err != nil {
		t.Fatalf("parsing held-out fixture: %v", err)
	}
	return h
}

// TestCoverageFY2023_24 is the in-sample coverage measurement. It is 133/133
// BY CONSTRUCTION: the alias table was built from this workbook's own name
// column, so this test proves the table is complete and self-consistent, not
// that the resolver generalises. TestCoverageHeldOutFY2020_21 measures that.
func TestCoverageFY2023_24(t *testing.T) {
	rep := Coverage("2023-24", fixtureNames(t, "2023-24"))

	if rep.Total != 133 {
		t.Fatalf("published names = %d, want 133", rep.Total)
	}
	if rep.Resolved != 133 || rep.Declined != 0 {
		t.Errorf("resolved/declined = %d/%d, want 133/0; declined: %v", rep.Resolved, rep.Declined, rep.DeclinedNames)
	}
	if rep.DistinctPlants != 133 {
		t.Errorf("distinct plants = %d, want 133 (no two FY2023-24 rows may collapse onto one canonical plant)", rep.DistinctPlants)
	}
	// Every FY2023-24 name is a registered canonical name, so nothing should
	// need the parenthetical-strip fallback in-sample.
	if got := rep.ByKind[MatchCanonical]; got != 133 {
		t.Errorf("canonical matches = %d, want 133 (kinds: %v)", got, rep.ByKind)
	}
	if got := rep.ByKind[MatchParenStripped]; got != 0 {
		t.Errorf("paren-stripped matches = %d, want 0 in-sample", got)
	}

	// The honest numbers: identity is complete, ATTRIBUTION is partial.
	if rep.WithParent != 64 {
		t.Errorf("names with a parent asserted = %d, want 64 of 133 (48.1%%)", rep.WithParent)
	}
	if rep.WithTicker != 21 {
		t.Errorf("names with a PSX ticker asserted = %d, want 21 of 133 (15.8%%)", rep.WithTicker)
	}
	wantConf := map[Confidence]int{
		ConfidenceHigh:         29,
		ConfidenceMedium:       29,
		ConfidenceLow:          6,
		ConfidenceUnattributed: 69,
	}
	for k, want := range wantConf {
		if rep.ByConfidence[k] != want {
			t.Errorf("FY2023-24 names at confidence %q = %d, want %d", k, rep.ByConfidence[k], want)
		}
	}

	// Capacity-weighted attribution, computed from FY2023-24's own published
	// capacities. Every FY2023-24 row reports a capacity, so nothing is
	// silently dropped here.
	if rep.CapacityNotReported != 0 {
		t.Errorf("FY2023-24 plants with no published capacity = %d, want 0", rep.CapacityNotReported)
	}
	if !rep.FYObserved {
		t.Error("FY2023-24 must be an observed year")
	}
	if rep.ObservedInFY != 133 || rep.NoObservationInFY != 0 {
		t.Errorf("observed/absent in FY = %d/%d, want 133/0", rep.ObservedInFY, rep.NoObservationInFY)
	}
	if got := mustMW(t, "CapacityReported", rep.CapacityReported); int(got) != 44686 {
		t.Errorf("total published installed capacity = %d MW, want 44686", int(got))
	}
	if got := mustMW(t, "CapacityTickered", rep.CapacityTickered); int(got) != 8381 {
		t.Errorf("capacity attributed to a PSX ticker = %d MW, want 8381", int(got))
	}

	// THE OPERATING SPLIT. 13 FY2023-24 rows publish a real installed
	// capacity while their entire monthly block reads DELICENSED or
	// DECOMMISSIONED. Those megawatts exist on paper and generate nothing, so
	// they must never land in a current-capacity total. Measured
	// independently against nepraparse's typed Plant.Status: 12 delicensed +
	// 1 decommissioned, 3,880 MW.
	if rep.NonOperatingPlants != 13 {
		t.Errorf("non-operating plants = %d, want 13 (12 DELICENSED + 1 DECOMMISSIONED)", rep.NonOperatingPlants)
	}
	nonOp := mustMW(t, "CapacityNonOperating", rep.CapacityNonOperating)
	if int(nonOp) != 3880 {
		t.Errorf("non-operating capacity = %d MW, want 3880", int(nonOp))
	}
	if got := rep.CapacityNonOperating.Plants(); got != 13 {
		t.Errorf("non-operating capacity plants = %d, want 13", got)
	}
	op := mustMW(t, "CapacityOperating", rep.CapacityOperating)
	if int(op) != 44686-3880 {
		t.Errorf("operating capacity = %d MW, want %d", int(op), 44686-3880)
	}
	// The two parts must reconstruct the whole, or one of them is wrong.
	if int(op+nonOp) != 44686 {
		t.Errorf("operating %d + non-operating %d = %d MW, want 44686", int(op), int(nonOp), int(op+nonOp))
	}
	if got := mustMW(t, "CapacityStatusUnknown", rep.CapacityStatusUnknown); got != 0 {
		t.Errorf("status-unknown capacity = %v MW, want 0: every FY2023-24 row has a recorded block status", got)
	}

	// The equity-analyst hazard, as a number: HUBCO's delicensed 1,292 MW is
	// 15.4%% of all capacity this crosswalk attributes to a PSX ticker.
	tickOp := mustMW(t, "CapacityTickeredOperating", rep.CapacityTickeredOperating)
	if int(tickOp) != 8381-1292 {
		t.Errorf("operating tickered capacity = %d MW, want %d (8381 less HUBCO's delicensed 1292)", int(tickOp), 8381-1292)
	}
	if share := 100 * (8381.0 - tickOp) / 8381.0; share < 15.3 || share > 15.5 {
		t.Errorf("non-operating share of tickered capacity = %.2f%%, want ~15.4%%", share)
	}
}

// mustMW reads an MWSum that is required to be a measurement. A sum that is
// not measured carries no number, and reading one anyway is the bug this
// helper refuses to let a test commit.
func mustMW(t *testing.T, label string, s MWSum) float64 {
	t.Helper()
	mw, ok := s.Float64()
	if !ok {
		t.Fatalf("%s is not measured, want a measurement (%s)", label, s)
	}
	return mw
}

// TestCoverageFY2017_18 checks the older workbook, where the Narowal drift
// means exactly one name must resolve through the alias table rather than a
// canonical hit.
func TestCoverageFY2017_18(t *testing.T) {
	rep := Coverage("2017-18", fixtureNames(t, "2017-18"))
	if rep.Total != 108 {
		t.Fatalf("published names = %d, want 108", rep.Total)
	}
	if rep.Resolved != 108 || rep.Declined != 0 {
		t.Errorf("resolved/declined = %d/%d, want 108/0; declined: %v", rep.Resolved, rep.Declined, rep.DeclinedNames)
	}
	if got := rep.ByKind[MatchAlias]; got != 1 {
		t.Errorf("alias matches = %d, want exactly 1 (the Narowal drift); kinds: %v", got, rep.ByKind)
	}
	if got := rep.ByKind[MatchCanonical]; got != 107 {
		t.Errorf("canonical matches = %d, want 107", got)
	}
	// 11 FY2017-18 plants were still under construction and published no
	// capacity. They must be counted as not-reported, never summed as zero.
	if rep.CapacityNotReported != 11 {
		t.Errorf("plants with no published capacity = %d, want 11", rep.CapacityNotReported)
	}
	if got := mustMW(t, "CapacityReported", rep.CapacityReported); int(got) != 32391 {
		t.Errorf("total published installed capacity = %d MW, want 32391", int(got))
	}
	if rep.WithTicker != 18 {
		t.Errorf("names with a PSX ticker = %d, want 18 of 108", rep.WithTicker)
	}
	// No plant is delicensed or decommissioned in FY2017-18: the status
	// sentinels appear only in FY2023-24. So the whole reported capacity is
	// operating, and the non-operating sum is a MEASURED zero over 0 plants —
	// a real finding, and distinct from the unmeasured sums a never-observed
	// year returns.
	if rep.NonOperatingPlants != 0 {
		t.Errorf("FY2017-18 non-operating plants = %d, want 0", rep.NonOperatingPlants)
	}
	nonOp, measured := rep.CapacityNonOperating.Float64()
	if !measured {
		t.Error("FY2017-18 non-operating capacity must be MEASURED (at zero), not unmeasured: the year was observed")
	}
	if nonOp != 0 {
		t.Errorf("FY2017-18 non-operating capacity = %v MW, want 0", nonOp)
	}
	if got := rep.CapacityNonOperating.Plants(); got != 0 {
		t.Errorf("FY2017-18 non-operating plants in sum = %d, want 0", got)
	}
	if got := mustMW(t, "CapacityOperating", rep.CapacityOperating); int(got) != 32391 {
		t.Errorf("FY2017-18 operating capacity = %d MW, want all 32391", int(got))
	}
	// The 11 under-construction plants published no capacity at all and are
	// excluded from every sum rather than summed as zero.
	if got := rep.CapacityReported.Plants(); got != 108-11 {
		t.Errorf("plants contributing to the capacity sum = %d, want %d", got, 108-11)
	}
}

// TestCoverageHeldOutFY2020_21 is the honest out-of-sample number. FY2020-21
// was never used to build crosswalk.json, so this measures whether the alias
// table generalises to a year it has not seen.
//
// Measured: 107 of 108 resolve (99.1%). The single decline is a real third
// naming drift — FY2020-21 writes "(NPPCL) - Balloki" where both built-from
// years write "(NPPMCL) - Balloki" — and declining it is the correct answer:
// that string shares the exact prefix "(NPPCL) - " with the DIFFERENT 1,230 MW
// plant "(NPPCL) - Haveli Bahadar Shah", so any prefix or edit-distance rule
// would hand Balloki's history to Haveli Bahadur Shah. The drift is recorded in
// possible_duplicates for a human to promote to an alias.
func TestCoverageHeldOutFY2020_21(t *testing.T) {
	h := loadHeldOut(t)
	if h.FY != "2020-21" {
		t.Fatalf("held-out fixture fiscal year = %q, want 2020-21", h.FY)
	}
	if got, want := HeldOutFYs(), "2020-21"; len(got) != 1 || got[0] != want {
		t.Errorf("HeldOutFYs() = %v, want [%s]", got, want)
	}
	for _, fy := range ObservedFYs() {
		if fy == h.FY {
			t.Fatalf("FY%s appears in ObservedFYs(); it must stay held out for this test to mean anything", fy)
		}
	}

	names := make([]string, 0, len(h.Rows))
	for _, r := range h.Rows {
		names = append(names, r.PublishedName)
	}
	rep := Coverage(h.FY, names)

	if rep.Total != 108 {
		t.Fatalf("held-out published names = %d, want 108", rep.Total)
	}
	if rep.Resolved != 107 {
		t.Errorf("out-of-sample resolved = %d, want 107 (99.1%%)", rep.Resolved)
	}
	if rep.Declined != 1 {
		t.Errorf("out-of-sample declined = %d, want 1; declined: %v", rep.Declined, rep.DeclinedNames)
	}
	if len(rep.DeclinedNames) != 1 || rep.DeclinedNames[0] != "(NPPCL) - Balloki" {
		t.Errorf("declined names = %v, want exactly [%q]", rep.DeclinedNames, "(NPPCL) - Balloki")
	}
	// The Narowal drift is the one name that needs the alias table in this
	// year too, and it must land on the HUBC row.
	if got := rep.ByKind[MatchAlias]; got != 1 {
		t.Errorf("alias matches = %d, want 1; kinds: %v", got, rep.ByKind)
	}
	m, ok := Resolve("Narowal\n  Energy Ltd. (HUBCO)")
	if !ok || m.Row.PSXTicker != "HUBC" {
		t.Errorf("held-out Narowal resolution: ok=%v ticker=%q, want ok=true ticker=HUBC", ok, m.Row.PSXTicker)
	}

	// The decline must be a refusal, not a mis-join: the dangerous wrong
	// answer here is Haveli Bahadur Shah, which shares the "(NPPCL) - " prefix.
	if got, ok := Resolve("(NPPCL) - Balloki"); ok {
		t.Errorf("(NPPCL) - Balloki resolved to %q; it must be declined until a human confirms the alias", got.Row.CanonicalName)
	}
	haveli, ok := Resolve("(NPPCL) - Haveli\n  Bahadar Shah")
	if !ok {
		t.Fatal("(NPPCL) - Haveli Bahadar Shah must still resolve")
	}
	if o, found := haveli.Row.ObservedIn("2023-24"); !found || o.InstalledCapacityMWRaw != "1,230" {
		t.Errorf("Haveli Bahadur Shah FY2023-24 capacity = %q, want %q (it is NOT the 1,223 MW Balloki plant)", o.InstalledCapacityMWRaw, "1,230")
	}
	// ===================================================================
	// THE FABRICATED ZERO. This is the assertion whose absence let the bug
	// ship: the old report answered this very call with
	// CapacityReportedMW = 0 AND CapacityNotReported = 0 — claiming both that
	// FY2020-21 had no capacity and that nothing had gone unreported — for a
	// year whose workbook publishes 36,902 MW over 108 plants.
	//
	// FY2020-21 is held out on purpose, so NO row carries an observation for
	// it. The only correct answer is "not measured", never a number.
	// ===================================================================
	if rep.FYObserved {
		t.Error("FYObserved is true for the held-out year; capacity would then claim to be measurable")
	}
	if rep.ObservedInFY != 0 {
		t.Errorf("plants observed in the held-out FY = %d, want 0 (it was never indexed)", rep.ObservedInFY)
	}
	if rep.NoObservationInFY != 107 {
		t.Errorf("plants with no observation in the held-out FY = %d, want 107 (every resolved plant)", rep.NoObservationInFY)
	}
	for _, c := range []struct {
		label string
		sum   MWSum
	}{
		{"CapacityReported", rep.CapacityReported},
		{"CapacityOperating", rep.CapacityOperating},
		{"CapacityNonOperating", rep.CapacityNonOperating},
		{"CapacityStatusUnknown", rep.CapacityStatusUnknown},
		{"CapacityTickered", rep.CapacityTickered},
		{"CapacityTickeredOperating", rep.CapacityTickeredOperating},
	} {
		if mw, ok := c.sum.Float64(); ok {
			t.Errorf("%s reports a MEASURED %v MW for the held-out FY2020-21; want unmeasured. "+
				"The workbook publishes 36,902 MW that year, so any number here is fabricated", c.label, mw)
		}
		if c.sum.Plants() != 0 {
			t.Errorf("%s claims %d contributing plants for a year with no observations", c.label, c.sum.Plants())
		}
	}
	// The counters must not claim knowledge either: reporting 0 not-reported
	// alongside 0 MW was the second fabrication in the same answer.
	if rep.CapacityNotReported != 0 || rep.CapacityStatusCell != 0 || rep.CapacityUnknownText != 0 {
		t.Errorf("capacity-absence counters = notReported %d / statusCell %d / unknownText %d; "+
			"all must be 0 because no capacity cell was examined at all, and NoObservationInFY carries the population instead",
			rep.CapacityNotReported, rep.CapacityStatusCell, rep.CapacityUnknownText)
	}
	// The report must say so in its own words, so a human reading the output
	// cannot mistake the absence for a finding.
	if !strings.Contains(rep.CapacityBasis, "NOT a finding of zero capacity") {
		t.Errorf("CapacityBasis does not disclaim the zero: %q", rep.CapacityBasis)
	}

	// And the drift is written down for review rather than left implicit.
	var recorded bool
	for _, pd := range PossibleDuplicates() {
		if pd.Verdict == "alias_candidate_unconfirmed" {
			for _, n := range pd.Names {
				if n == "(NPPCL) - Balloki" {
					recorded = true
				}
			}
		}
	}
	if !recorded {
		t.Error("the (NPPCL) - Balloki drift is not recorded in possible_duplicates")
	}
}

// TestResolveDeclinesOutOfSampleNames pins the conservatism: plausible strings
// that are NOT registered must all be refused, including the ones a fuzzy
// matcher would happily accept.
func TestResolveDeclinesOutOfSampleNames(t *testing.T) {
	tests := []struct {
		name string
		in   string
		why  string
	}{
		{"PSX spelling of a plant's parent", "The Hub Power Company Limited", "the parent's legal name is not a published plant name"},
		{"renamed company, not the published plant name", "Lalpir Power Limited", "the workbook still says 'AES Lalpir power limited.'"},
		{"K-Electric plant by station code", "BQPS-1", "K-Electric's fleet is not in this dataset"},
		{"K-Electric plant by station name", "Bin Qasim Power Station", "K-Electric's fleet is not in this dataset"},
		{"K-Electric plant, Korangi", "Korangi Combined Cycle Power Plant", "K-Electric's fleet is not in this dataset"},
		{"KAPCO block", "Kot Addu Power Company (KAPCO) Block-II", "the generation workbook has no block-level rows"},
		{"one word of a real name", "Kohinoor", "a substring is not a name"},
		{"a bare acronym", "KEL", "acronyms go to ResolveTicker, which refuses this one"},
		{"prefix-shared different plant", "(NPPCL) - Balloki", "would mis-join to Haveli Bahadur Shah under any prefix rule"},
		{"transposed letters", "Nishat Chunain Power Ltd (NCPL)", "no edit-distance rescue"},
		{"the ambiguous FWEL base", "Foundation Wind Energy-I Ltd.", "two different farms share this string"},
		{"the ambiguous FWEL base, stripped differently", "foundation wind energy-i ltd", "same collision, folded"},
		{"a plausible but absent sibling", "Thermal Power Station Guddu 748", "not published"},
		{"empty", "", "nothing to resolve"},
		{"whitespace and NBSP only", " \n  ", "an 'empty' workbook cell"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if m, ok := Resolve(tc.in); ok {
				t.Errorf("Resolve(%q) resolved to %q, want a refusal: %s", tc.in, m.Row.CanonicalName, tc.why)
			}
		})
	}
}

// TestPossibleDuplicatesAreReviewable checks the review list is populated and
// every entry carries a verdict and reasoning, since this is the artefact a
// human is supposed to act on.
func TestPossibleDuplicatesAreReviewable(t *testing.T) {
	pds := PossibleDuplicates()
	if len(pds) != 8 {
		t.Errorf("possible_duplicates entries = %d, want 8", len(pds))
	}
	verdicts := map[string]int{}
	for _, pd := range pds {
		if len(pd.Names) == 0 {
			t.Error("a possible_duplicates entry lists no names")
		}
		if pd.Suspicion == "" {
			t.Errorf("%v carries no reasoning", pd.Names)
		}
		switch pd.Verdict {
		case "kept_separate", "merged", "granularity_unresolved", "alias_candidate_unconfirmed":
			verdicts[pd.Verdict]++
		default:
			t.Errorf("%v has unrecognised verdict %q", pd.Names, pd.Verdict)
		}
		// Anything marked kept_separate must actually be separate rows.
		if pd.Verdict == "kept_separate" {
			seen := map[string]bool{}
			for _, n := range pd.Names {
				m, ok := Resolve(n)
				if !ok {
					continue
				}
				if seen[m.Row.CanonicalName] {
					t.Errorf("%q is marked kept_separate but two of its names resolve to %q", pd.Names, m.Row.CanonicalName)
				}
				seen[m.Row.CanonicalName] = true
			}
		}
	}
	if verdicts["merged"] != 1 {
		t.Errorf("merged verdicts = %d, want exactly 1 (Narowal); every merge must be reviewable", verdicts["merged"])
	}
	if verdicts["kept_separate"] < 4 {
		t.Errorf("kept_separate verdicts = %d, want at least 4", verdicts["kept_separate"])
	}
}
