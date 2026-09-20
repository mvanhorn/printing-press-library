// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// Leg-level tests: the PER, generation and capacity derivations, run offline
// against committed captures of the published documents.
//
// Every number asserted here was MEASURED by running this code against those
// captures. Where a figure could only be measured from a live fetch, the test
// says so instead of pretending an offline capture covers it.

package cli

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraper"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraxwalk"
)

// perFixtureReport parses one committed span capture.
func perFixtureReport(t *testing.T, name, fy string) *nepraper.Report {
	t.Helper()
	r, err := nepraper.ParseReliability(perSpansFixture(t, name), fy)
	if err != nil {
		t.Fatalf("ParseReliability(%s): %v", fy, err)
	}
	return r
}

func genFixtureWorkbook(t *testing.T, fy string) *nepraparse.Workbook {
	t.Helper()
	w, err := nepraparse.ParseWorkbook(conflictsGenFixture(t, fy), fy)
	if err != nil {
		t.Fatalf("ParseWorkbook(%s): %v", fy, err)
	}
	return w
}

// TestPERConflictLegReproducesTheFY2024_25Set is the flagship. Both figures,
// both tables, both PDF pages, the ratio to 1e-7 and the class, from the
// committed capture.
func TestPERConflictLegReproducesTheFY2024_25Set(t *testing.T) {
	r := perFixtureReport(t, "fy2024-25", "FY2024-25")
	entries := perConflictEntries(r)

	// Over the FOUR pages this capture holds, the set is exactly four. Over
	// the live 35-page PDF the same code returns six; the count is a property
	// of what was read, and the ledger records both.
	if len(entries) != 4 {
		var ids []string
		for _, e := range entries {
			ids = append(ids, e.ID)
		}
		t.Fatalf("conflicts from the committed capture = %d, want 4: %v", len(entries), ids)
	}

	want := map[string]struct {
		a, b           float64
		aTable, bTable string
		aPage, bPage   int
		class, ratioID string
		ratio          float64
	}{
		"FY2024-25/MEPCO/saidi":      {3547.00, 1182.56, "Table 6", "Table 18", 15, 29, "undocumented", "saidi", 2.9994249763},
		"FY2024-25/MEPCO/saifi":      {30.67, 10.23, "Table 5", "Table 17", 13, 28, "undocumented", "saifi", 2.9980449658},
		"FY2024-25/LESCO/saifi":      {28.16, 28.61, "Table 5", "Table 17", 13, 28, "transposition", "lesco", 0.9842712338},
		"FY2024-25/K-Electric/saifi": {68.46, 68.64, "Table 5", "Table 17", 13, 28, "transposition", "ke", 0.9973776224},
	}
	for _, e := range entries {
		w, ok := want[e.Key.String()]
		if !ok {
			t.Errorf("unexpected conflict %s", e.Key)
			continue
		}
		delete(want, e.Key.String())
		if e.A.Value == nil || e.B.Value == nil {
			t.Fatalf("%s: a side is not numeric", e.Key)
		}
		if *e.A.Value != w.a || *e.B.Value != w.b {
			t.Errorf("%s figures = %v vs %v, want %v vs %v", e.Key, *e.A.Value, *e.B.Value, w.a, w.b)
		}
		if e.A.Table != w.aTable || e.B.Table != w.bTable {
			t.Errorf("%s tables = %s / %s, want %s / %s", e.Key, e.A.Table, e.B.Table, w.aTable, w.bTable)
		}
		if e.A.Page != w.aPage || e.B.Page != w.bPage {
			t.Errorf("%s pages = %d / %d, want %d / %d", e.Key, e.A.Page, e.B.Page, w.aPage, w.bPage)
		}
		if e.Class != w.class {
			t.Errorf("%s class = %s, want %s", e.Key, e.Class, w.class)
		}
		if e.Ratio == nil {
			t.Errorf("%s: ratio is undefined", e.Key)
		} else if math.Abs(*e.Ratio-w.ratio) > 1e-7 {
			t.Errorf("%s ratio = %.10f, want %.10f", e.Key, *e.Ratio, w.ratio)
		}
		if e.A.Variant == e.B.Variant {
			t.Errorf("%s: both sides claim variant %q; the two tables are different kinds", e.Key, e.A.Variant)
		}
		if e.Reconciled {
			t.Errorf("%s: reconciled", e.Key)
		}
		if !e.SameKey {
			t.Errorf("%s: a same-key conflict claims same_key false", e.Key)
		}
	}
	for k := range want {
		t.Errorf("conflict %s was not reported at all", k)
	}
}

// TestPERConflictLegAgreesWithTheShippedLedger is the pairing contract that
// makes --recompute work: the live leg and the ledger must build the same ids
// and the same figures, or a reproduction would be reported as a new finding.
func TestPERConflictLegAgreesWithTheShippedLedger(t *testing.T) {
	r := perFixtureReport(t, "fy2024-25", "FY2024-25")
	derived := map[string]conflictEntry{}
	for _, e := range perConflictEntries(r) {
		derived[e.ID] = e
	}
	var paired int
	for _, e := range conflictLedger() {
		if e.Surface != conflictSurfacePER || e.Kind != conflictKindConflict {
			continue
		}
		d, ok := derived[e.ID]
		if !ok {
			continue
		}
		paired++
		if !conflictsFiguresAgree(e.A.Value, d.A.Value) || !conflictsFiguresAgree(e.B.Value, d.B.Value) {
			t.Errorf("%s: ledger has %v/%v and the document has %v/%v",
				e.ID, e.A.Value, e.B.Value, d.A.Value, d.B.Value)
		}
		if e.Class != d.Class {
			t.Errorf("%s: ledger class %q, document class %q", e.ID, e.Class, d.Class)
		}
		if e.A.Page != d.A.Page || e.B.Page != d.B.Page {
			t.Errorf("%s: ledger pages %d/%d, document pages %d/%d",
				e.ID, e.A.Page, e.B.Page, d.A.Page, d.B.Page)
		}
	}
	if paired != 4 {
		t.Errorf("ledger entries paired with the document by id = %d, want 4; a mismatch would make every "+
			"reproduction look like a new finding", paired)
	}
}

// TestPERBreaksAtDeclaredThreshold pins the break ladder, and the grouping
// rule that keeps a break inside ONE published series.
func TestPERBreaksAtDeclaredThreshold(t *testing.T) {
	r := perFixtureReport(t, "fy2024-25", "FY2024-25")

	for _, tc := range []struct {
		ratio float64
		want  int
	}{{100, 3}, {50, 4}, {5, 5}, {1000, 0}} {
		got, skips := perBreaks(r, tc.ratio)
		if len(got) != tc.want {
			var ids []string
			for _, e := range got {
				ids = append(ids, e.ID)
			}
			t.Errorf("perBreaks at %v = %d, want %d: %v", tc.ratio, len(got), tc.want, ids)
		}
		if len(skips) != 0 {
			t.Errorf("perBreaks at %v skipped %d groups: %+v", tc.ratio, len(skips), skips)
		}
	}

	want := map[string]float64{
		"IESCO/saidi": 755.1544117647,
		"IESCO/saifi": 411.2,
		"GEPCO/saidi": 109.2656128531,
	}
	for _, e := range perBreaks100(t, r) {
		key := e.Key.Entity + "/" + e.Key.Metric
		wantRatio, ok := want[key]
		if !ok {
			t.Errorf("unexpected break %s", e.ID)
			continue
		}
		delete(want, key)
		if math.Abs(*e.BreakRatioSeen-wantRatio) > 1e-6 {
			t.Errorf("%s step = %.10f, want %.10f", key, *e.BreakRatioSeen, wantRatio)
		}
		if e.SameKey {
			t.Errorf("%s: a break claims same_key", key)
		}
		// BOTH sides must come from ONE table. Comparing across tables would
		// silently manufacture the very same-key conflicts the conflict leg
		// already reports.
		if e.A.Table != e.B.Table {
			t.Errorf("%s: the step spans %s and %s; a break must stay inside one published series",
				key, e.A.Table, e.B.Table)
		}
		if e.Kind != conflictKindBreak {
			t.Errorf("%s: kind = %s", key, e.Kind)
		}
	}
	for k := range want {
		t.Errorf("break %s was not reported at threshold 100", k)
	}

	// The headline tables publish ONE year each, so they can produce no
	// adjacent pair and therefore no break. A leg that read their target or
	// breach columns as a series would emit nonsense here.
	for _, e := range perBreaks100(t, r) {
		if e.A.Table == "Table 5" || e.A.Table == "Table 6" {
			t.Errorf("%s: a break was built from a single-year headline table", e.ID)
		}
	}

	// The FY2018-19 capture holds only the T&D, SAIFI and SAIDI headline
	// pages, so it has no comparison table and must yield no break at all.
	early := perFixtureReport(t, "fy2018-19", "FY2018-19")
	if got, _ := perBreaks(early, 5); len(got) != 0 {
		t.Errorf("the FY2018-19 capture yielded %d breaks; it carries no multi-year table", len(got))
	}
	if got := perConflictEntries(early); len(got) != 0 {
		t.Errorf("the FY2018-19 capture yielded %d conflicts, want 0 on these three pages", len(got))
	}
}

func perBreaks100(t *testing.T, r *nepraper.Report) []conflictEntry {
	t.Helper()
	got, _ := perBreaks(r, 100)
	for _, e := range got {
		if e.BreakRatioSeen == nil {
			t.Fatalf("%s carries no measured step", e.ID)
		}
	}
	return got
}

// TestBallokiSumBreak pins the one published-Sum failure in the corpus, to the
// cent, with the population that makes it a finding.
func TestBallokiSumBreak(t *testing.T) {
	w := genFixtureWorkbook(t, "2020-21")
	entries := genSumEntries(w, "2020-21")
	if len(entries) != 1 {
		t.Fatalf("Sum mismatches = %d, want exactly 1", len(entries))
	}
	e := entries[0]
	if e.Key.Entity != "(NPPCL) - Balloki" {
		t.Errorf("plant = %q, want %q", e.Key.Entity, "(NPPCL) - Balloki")
	}
	if e.A.Value == nil || math.Abs(*e.A.Value-5905.65) > 0.005 {
		t.Errorf("published Sum = %v, want 5905.65", e.A.Value)
	}
	if e.B.Value == nil || math.Abs(*e.B.Value-5945.21) > 0.005 {
		t.Errorf("derived monthly sum = %v, want 5945.21", e.B.Value)
	}
	if e.AbsDiff == nil || math.Abs(*e.AbsDiff-39.56) > 0.005 {
		t.Errorf("abs_diff = %v, want 39.56", e.AbsDiff)
	}
	// The published side must be labelled published and the computed side
	// derived. The derived figure never replaces the published Sum.
	if e.A.ValueKind != "numeric" || e.A.Variant != "as_published" {
		t.Errorf("published side is %q/%q, want numeric/as_published", e.A.ValueKind, e.A.Variant)
	}
	if e.B.ValueKind != "derived" || e.B.Variant != "derived" {
		t.Errorf("computed side is %q/%q, want derived/derived", e.B.ValueKind, e.B.Variant)
	}
	// 104 of 105 eligible rows reconcile. A regression that widened the
	// tolerance would move this number.
	if e.Population == nil || e.Population.Denominator != 105 || e.Population.Flagged != 1 {
		t.Errorf("population = %+v, want 105 eligible with 1 flagged", e.Population)
	}
	if !strings.Contains(e.Note, "104 of 105") {
		t.Errorf("the note does not state the reconciling count: %q", e.Note)
	}

	// The other two captured years have NO Sum failure, which is what makes
	// Balloki a finding rather than noise.
	for _, fy := range []string{"2017-18", "2023-24"} {
		if got := genSumEntries(genFixtureWorkbook(t, fy), fy); len(got) != 0 {
			t.Errorf("FY%s produced %d Sum mismatches, want 0", fy, len(got))
		}
	}
}

// TestColumnOrderGuardsTheArithmeticLeg is the guard that keeps the leg from
// reporting utilisations as energy.
func TestColumnOrderGuardsTheArithmeticLeg(t *testing.T) {
	for _, fy := range []string{"2017-18", "2020-21", "2023-24"} {
		w := genFixtureWorkbook(t, fy)
		if err := conflictsColumnOrderGuard(w); err != nil {
			t.Errorf("FY%s: the guard refused a workbook whose order IS provable: %v", fy, err)
		}
	}

	// A workbook with the pair members swapped: the annual-total identity no
	// longer singles out the second column, so the leg must REFUSE rather
	// than report. Without this every GWh value would actually be a
	// utilisation and every number would look plausible and be wrong.
	w := genFixtureWorkbook(t, "2020-21")
	for i := range w.Plants {
		for j := range w.Plants[i].Months {
			m := &w.Plants[i].Months[j]
			m.Utilisation, m.Generation = m.Generation, m.Utilisation
		}
		w.Plants[i].Total.Utilisation, w.Plants[i].Total.Generation =
			w.Plants[i].Total.Generation, w.Plants[i].Total.Utilisation
	}
	err := conflictsColumnOrderGuard(w)
	if err == nil {
		t.Fatal("the guard accepted a workbook whose month pairs are swapped")
	}
	if !strings.Contains(err.Error(), "not provable") {
		t.Errorf("the refusal does not quote the verdict's explanation: %v", err)
	}
	if !strings.Contains(err.Error(), "pair order may have changed") {
		t.Errorf("the refusal does not carry ColumnOrderVerdict.Explanation: %v", err)
	}
}

// TestZeroLoadFactorScanIsMeasuredNotAsserted pins the scan per fiscal year
// WITH its denominator, and pins each row's own verdict.
//
// The absorb manifest's "78 of 9,180 plant-months" does not reproduce. What
// this build measures is 4 + 5 + 1 = 10 flagged of 1,164 + 1,260 + 1,416 =
// 3,840 plant-months whose two cells are both numeric.
func TestZeroLoadFactorScanIsMeasuredNotAsserted(t *testing.T) {
	want := map[string]struct{ denom, flagged, sumDenom, sumFlagged int }{
		"2017-18": {1164, 4, 97, 1},
		"2020-21": {1260, 5, 105, 1},
		"2023-24": {1416, 1, 118, 0},
	}
	totalFlagged, totalDenom := 0, 0
	var all []conflictEntry
	for fy, w := range want {
		wb := genFixtureWorkbook(t, fy)
		entries, census := genZeroLoadFactorEntries(wb, fy)
		all = append(all, entries...)
		if census.PlantMonthsBothNumeric != w.denom {
			t.Errorf("FY%s plant-months with both cells numeric = %d, want %d",
				fy, census.PlantMonthsBothNumeric, w.denom)
		}
		if census.PlantMonthsFlagged != w.flagged {
			t.Errorf("FY%s flagged plant-months = %d, want %d", fy, census.PlantMonthsFlagged, w.flagged)
		}
		if census.SumPairsBothNumeric != w.sumDenom || census.SumPairsFlagged != w.sumFlagged {
			t.Errorf("FY%s annual Sum pairs = %d flagged of %d, want %d of %d",
				fy, census.SumPairsFlagged, census.SumPairsBothNumeric, w.sumFlagged, w.sumDenom)
		}
		if got := len(entries); got != w.flagged+w.sumFlagged {
			t.Errorf("FY%s emitted %d entries, want %d", fy, got, w.flagged+w.sumFlagged)
		}
		totalFlagged += w.flagged
		totalDenom += w.denom
	}
	if totalFlagged != 10 || totalDenom != 3840 {
		t.Errorf("across the three captures: %d flagged of %d plant-months, want 10 of 3840",
			totalFlagged, totalDenom)
	}

	// The individual rows, with the implied utilisation that separates a
	// contradiction from a rounding.
	byPlantMonth := map[string]conflictEntry{}
	for _, e := range all {
		byPlantMonth[e.Key.PeriodFY+"|"+e.Key.Entity+"|"+e.A.Table] = e
	}
	for _, tc := range []struct {
		fy, plant, month string
		implied          float64
		verdict          string
	}{
		{"FY2017-18", "AES Lalpir power limited.", "Oct", 0.0037132, zlfConsistent},
		{"FY2017-18", "Neelum Jhelum Hydropower Project (WAPDA)", "Sep", 0.6363946, zlfContradiction},
		{"FY2017-18", "Artistic Wind Power (Pvt.) Limited.", "Feb", 2.8571429, zlfContradiction},
		{"FY2017-18", "Three Gorges Second Wind Farm (Private) Ltd. (TGS)", "Jun", 0, zlfUncomputable},
		{"FY2020-21", "Fatima Energy Limited. (FEL)", "Nov", 5.3356481, zlfContradiction},
		{"FY2020-21", "Fatima Energy Limited. (FEL)", "Dec", 26.0304659, zlfContradiction},
		{"FY2020-21", "Fatima Energy Limited. (FEL)", "Jan", 26.2432796, zlfContradiction},
		{"FY2020-21", "Fatima Energy Limited. (FEL)", "Feb", 26.2896825, zlfContradiction},
		{"FY2020-21", "Fatima Energy Limited. (FEL)", "Mar", 5.0627240, zlfContradiction},
		{"FY2023-24", "Helios Power (Private) Limited (HPPL)", "Jan", 9.1935484, zlfContradiction},
	} {
		key := tc.fy + "|" + tc.plant + "|published \"% age\" cell, " + tc.month
		e, ok := byPlantMonth[key]
		if !ok {
			t.Errorf("no entry for %s %s %s", tc.fy, tc.plant, tc.month)
			continue
		}
		if !strings.Contains(e.Note, "verdict: "+tc.verdict) {
			t.Errorf("%s %s: note does not carry verdict %s: %q", tc.plant, tc.month, tc.verdict, e.Note)
		}
		// The published side is a MEASURED 0.00 and keeps its number.
		if e.A.Value == nil || *e.A.Value != 0 {
			t.Errorf("%s %s: the published side is %v, want a measured 0", tc.plant, tc.month, e.A.Value)
		}
		if tc.verdict == zlfUncomputable {
			// No denominator exists, so there is NO implied utilisation and
			// no value key at all.
			if e.B.Value != nil {
				t.Errorf("%s %s: an uncomputable implied utilisation carries %v", tc.plant, tc.month, *e.B.Value)
			}
			if e.Ratio != nil || e.AbsDiff != nil {
				t.Errorf("%s %s: ratio/abs_diff exist without a second figure", tc.plant, tc.month)
			}
			continue
		}
		if e.B.Value == nil {
			t.Errorf("%s %s: no implied utilisation", tc.plant, tc.month)
			continue
		}
		if math.Abs(*e.B.Value-tc.implied) > 1e-4 {
			t.Errorf("%s %s implied utilisation = %.7f%%, want %.7f%%",
				tc.plant, tc.month, *e.B.Value, tc.implied)
		}
	}

	// Only EIGHT of the ten are contradictions. Reporting all ten as
	// contradictions is the defect this assertion exists to catch, and a bare
	// count would say exactly that.
	verdicts := map[string]int{}
	for _, e := range all {
		for _, v := range []string{zlfContradiction, zlfConsistent, zlfUncomputable} {
			if strings.Contains(e.Note, "verdict: "+v) {
				verdicts[v]++
			}
		}
	}
	if verdicts[zlfContradiction] != 9 || verdicts[zlfConsistent] != 1 || verdicts[zlfUncomputable] != 2 {
		t.Errorf("verdicts = %v, want 9 contradictions, 1 consistent and 2 uncomputable across the 12 "+
			"flagged cells (10 plant-months plus 2 annual Sum pairs)", verdicts)
	}

	// The unreproduced probe figures must appear NOWHERE.
	payload, err := json.Marshal(all)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"9,180", "9180", "78 of"} {
		if strings.Contains(string(payload), banned) {
			t.Errorf("the scan output carries %q, an unreproduced probe figure", banned)
		}
	}
}

// TestBlankIsNeverZeroInTheScan is the negative control. It is the scan-level
// twin of Census.MixedBlankZeroRows == 0.
func TestBlankIsNeverZeroInTheScan(t *testing.T) {
	for _, fy := range []string{"2017-18", "2020-21", "2023-24"} {
		w := genFixtureWorkbook(t, fy)
		_, census := genZeroLoadFactorEntries(w, fy)
		if census.BlankUtilisationWithGeneration != 0 {
			t.Errorf("FY%s: %d plant-months have an NBSP-blank utilisation alongside positive generation; "+
				"a non-zero count here means a regression coerced blank to zero",
				fy, census.BlankUtilisationWithGeneration)
		}
		if w.Census.MixedBlankZeroRows != 0 {
			t.Errorf("FY%s: nepraparse reports %d rows mixing a blank and a 0.00", fy, w.Census.MixedBlankZeroRows)
		}
	}
}

// TestCapacityLegReportsOnlyWhatItMeasured pins the definitions conflict, and
// the refusal to answer an unobserved year with a zero.
func TestCapacityLegReportsOnlyWhatItMeasured(t *testing.T) {
	w := genFixtureWorkbook(t, "2023-24")
	e := capacityEntry(w, "2023-24")

	if e.A.Value == nil || *e.A.Value != 44686 {
		t.Errorf("CapacityReported = %v, want 44686", e.A.Value)
	}
	if e.A.Plants == nil || *e.A.Plants != 133 {
		t.Errorf("CapacityReported plants = %v, want 133", e.A.Plants)
	}
	if e.B.Value == nil || *e.B.Value != 40806 {
		t.Errorf("CapacityOperating = %v, want 40806", e.B.Value)
	}
	if e.B.Plants == nil || *e.B.Plants != 120 {
		t.Errorf("CapacityOperating plants = %v, want 120", e.B.Plants)
	}

	byVariant := map[string]conflictSide{}
	for _, s := range e.Also {
		byVariant[s.Variant] = s
	}
	for _, tc := range []struct {
		variant string
		mw      float64
		plants  int
	}{
		{"capacity_non_operating_by_sentinel", 3880, 13},
		{"capacity_status_unknown", 0, 0},
		{"capacity_tickered", 8381, 21},
		{"capacity_tickered_operating", 8381 - 1292, 20},
		// The handoff's correction: the status-sentinel rows PLUS the two
		// rows whose whole monthly block is blank while a capacity is
		// published. Both 3,880 and 4,061 are correct for different
		// populations and must never be merged.
		{"capacity_not_generating_including_unreported", 4061, 15},
	} {
		s, ok := byVariant[tc.variant]
		if !ok {
			t.Errorf("no side for %s", tc.variant)
			continue
		}
		if s.Value == nil || *s.Value != tc.mw {
			t.Errorf("%s = %v MW, want %v", tc.variant, s.Value, tc.mw)
			continue
		}
		if s.Plants == nil || *s.Plants != tc.plants {
			t.Errorf("%s plants = %v, want %d", tc.variant, s.Plants, tc.plants)
		}
	}
	// The two parts must reconstruct the whole, or one of them is wrong.
	if *e.A.Value != *e.B.Value+*byVariant["capacity_non_operating_by_sentinel"].Value {
		t.Errorf("operating + non-operating != reported: %v + %v != %v",
			*e.B.Value, *byVariant["capacity_non_operating_by_sentinel"].Value, *e.A.Value)
	}
	// The status-unknown side is a MEASURED zero and keeps its number: every
	// FY2023-24 row has a recorded block status.
	if s := byVariant["capacity_status_unknown"]; s.ValueKind != "numeric" {
		t.Errorf("capacity_status_unknown value_kind = %q, want numeric: it is a measured 0, not an absence",
			s.ValueKind)
	}

	// The figures that exist and cannot be recomputed carry NO number.
	for _, tc := range []struct{ variant, raw string }{
		{"sir_2024_cppag_system", "42,512"},
		{"sir_2024_including_k_electric", "45,888"},
		{"licence_register_gross_capacity", "47,559.97"},
	} {
		s, ok := byVariant[tc.variant]
		if !ok {
			t.Errorf("no side for %s", tc.variant)
			continue
		}
		if s.ValueKind != "unavailable" {
			t.Errorf("%s value_kind = %q, want unavailable", tc.variant, s.ValueKind)
		}
		if s.Value != nil {
			t.Errorf("%s carries the number %v; it must be unavailable, never approximated", tc.variant, *s.Value)
		}
		if s.Raw != tc.raw {
			t.Errorf("%s raw = %q, want %q", tc.variant, s.Raw, tc.raw)
		}
		if len(s.Basis) < 80 {
			t.Errorf("%s carries no measured reason: %q", tc.variant, s.Basis)
		}
	}

	// The absorb manifest's own triple is refuted on measurement and must not
	// appear: citing our own superseded brief would be the one place this
	// command manufactured a conflict.
	payload, err := json.Marshal(e)
	if err != nil {
		t.Fatal(err)
	}
	for _, banned := range []string{"40,614", "40614", "44,675", "44675"} {
		if strings.Contains(string(payload), banned) {
			t.Errorf("the capacity entry carries the refuted figure %q", banned)
		}
	}
}

// TestCapacityUnobservedYearIsNotMeasured proves every megawatt goes through
// MWSum.Float64()'s second return.
//
// A fiscal year the crosswalk never observed has no population behind it, so
// it cannot answer 0 MW — and a 0 there would read as a measurement of nothing
// rather than nothing measured.
func TestCapacityUnobservedYearIsNotMeasured(t *testing.T) {
	held := nepraxwalk.HeldOutFYs()
	if len(held) == 0 {
		t.Skip("the crosswalk holds out no fiscal year, so there is no unobserved control")
	}
	w := genFixtureWorkbook(t, "2020-21")
	e := capacityEntry(w, "2020-21")
	if e.A.Value != nil {
		t.Errorf("an unobserved fiscal year answered %v MW; it must answer <not measured>", *e.A.Value)
	}
	if e.A.ValueKind != "not_reported" {
		t.Errorf("value_kind = %q, want not_reported", e.A.ValueKind)
	}
	if !strings.Contains(e.A.Raw, "not measured") {
		t.Errorf("raw = %q, want it to say the sum was not measured", e.A.Raw)
	}
	if !strings.Contains(e.A.Basis, "NOT MEASURED") {
		t.Errorf("basis does not flag the unmeasured sum: %q", e.A.Basis)
	}
	if e.Ratio != nil {
		t.Errorf("a ratio was computed from an unmeasured sum: %v", *e.Ratio)
	}
	// And it must NOT claim to be a finding of zero capacity.
	if strings.Contains(e.Note, "0 MW") {
		t.Errorf("the note reads as a finding of zero capacity: %q", e.Note)
	}
}

// TestLedgerCoversEveryKnownArtifact keeps nepraper's registry the single
// source of truth for the four documented artifacts and their Direction
// wording.
//
// MUTATION: deleting the FY2020-21 39733/39.733 artifact from the registry
// fails this test, and hard-coding the four Direction strings in the CLI
// instead of reading them fails it too, because the assertion compares against
// the registry's own value rather than a literal.
func TestLedgerCoversEveryKnownArtifact(t *testing.T) {
	ledger := conflictLedger()
	arts := nepraper.KnownArtifacts()
	if len(arts) < 4 {
		t.Fatalf("nepraper.KnownArtifacts holds %d entries, want at least 4", len(arts))
	}
	for _, a := range arts {
		key := conflictKey{
			PeriodFY: a.Key.PeriodFY,
			Entity:   string(a.Key.Entity),
			Metric:   string(a.Key.Metric),
		}
		var found *conflictEntry
		for i := range ledger {
			if ledger[i].Key == key && ledger[i].Kind == conflictKindConflict {
				found = &ledger[i]
				break
			}
		}
		if found == nil {
			t.Errorf("the ledger has no conflict entry for the documented artifact %s", a.Key)
			continue
		}
		if found.Registry == nil {
			t.Errorf("%s: the entry does not echo the registry", found.ID)
			continue
		}
		if found.Registry.Direction != a.Direction {
			t.Errorf("%s: direction = %q, want the registry's %q", found.ID, found.Registry.Direction, a.Direction)
		}
		if found.Registry.Ratio != a.Ratio {
			t.Errorf("%s: registry ratio echo = %v, want %v", found.ID, found.Registry.Ratio, a.Ratio)
		}
		if found.Direction != a.Direction {
			t.Errorf("%s: the entry's direction = %q, want the registry's %q", found.ID, found.Direction, a.Direction)
		}
		// The ratio computed from the two published figures must agree with
		// the registry's recorded one to the precision the registry states.
		if found.Ratio == nil {
			t.Errorf("%s: no ratio", found.ID)
			continue
		}
		if math.Abs(*found.Ratio-a.Ratio) > 1e-6 {
			t.Errorf("%s: ratio computed from the two figures = %.10f, registry records %.10f",
				found.ID, *found.Ratio, a.Ratio)
		}
	}

	// The ~1000x artifact's direction is an INFERENCE and must reach the
	// caller as one, never shortened into a claim.
	var saw1000 bool
	for _, e := range ledger {
		if e.Key.PeriodFY != "FY2020-21" || e.Key.Metric != "saidi" {
			continue
		}
		saw1000 = true
		if !strings.HasPrefix(e.Direction, "inferred") {
			t.Errorf("%s: direction = %q, want it to begin \"inferred\"", e.ID, e.Direction)
		}
		if !strings.Contains(e.Direction, "NOT proven by the documents") {
			t.Errorf("%s: the direction was shortened into a claim: %q", e.ID, e.Direction)
		}
	}
	if !saw1000 {
		t.Error("the ~1000x MEPCO SAIDI artifact is absent from the ledger")
	}

	// K-Electric's FY2024-25 SAIFI transposition is asserted by nepraper's own
	// conflict_survival_test but is ABSENT from its registry. The ledger
	// supplies the ratio from the two published figures and says so; a future
	// registry entry would be picked up by the loop above.
	for _, e := range ledger {
		if e.Key.Entity != "K-Electric" || e.Key.Metric != "saifi" {
			continue
		}
		if conflictRegistryFor(e.Key) == nil {
			if e.Registry != nil {
				t.Errorf("%s: the entry echoes a registry record that does not exist", e.ID)
			}
			if !strings.Contains(e.Note, "has NO entry for this key") {
				t.Errorf("%s: the note does not say the registry lacks this key: %q", e.ID, e.Note)
			}
		}
		if e.Ratio == nil || math.Abs(*e.Ratio-0.9973776224) > 1e-7 {
			t.Errorf("%s: ratio = %v, want 0.9973776224 computed from 68.46 / 68.64", e.ID, e.Ratio)
		}
	}
}

// TestLedgerIDsAreUniqueAndStable keeps --recompute's pairing sound: two
// entries with one id would silently overwrite each other.
func TestLedgerIDsAreUniqueAndStable(t *testing.T) {
	seen := map[string]bool{}
	for _, e := range conflictLedger() {
		if seen[e.ID] {
			t.Errorf("duplicate ledger id %s", e.ID)
		}
		seen[e.ID] = true
		if strings.ContainsAny(e.ID, " _") || strings.ToLower(e.ID) != e.ID {
			t.Errorf("id %q is not a stable lowercase slug", e.ID)
		}
		if !strings.HasPrefix(e.ID, e.Surface) {
			t.Errorf("id %q does not begin with its surface %q", e.ID, e.Surface)
		}
	}
}

// TestLedgerEveryEntryDeclaresItsProvenance is the honesty assertion: a reader
// must never be able to mistake a curated claim for a computed one.
func TestLedgerEveryEntryDeclaresItsProvenance(t *testing.T) {
	counts := map[string]int{}
	for _, e := range conflictLedger() {
		counts[e.Computable]++
		switch e.Computable {
		case conflictVerified:
			if !e.ReproducedByThisBuild {
				t.Errorf("%s is verified but reproduced_by_this_build is false", e.ID)
			}
			if e.MeasuredFrom == "" {
				t.Errorf("%s is verified but names no capture it was measured from", e.ID)
			}
		case conflictLiveOnly:
			if e.ReproducedByThisBuild {
				t.Errorf("%s is live_only but claims to have been reproduced", e.ID)
			}
			if !strings.Contains(e.Derivation, "not reproduced") {
				t.Errorf("%s: derivation %q does not say the figures are unreproduced", e.ID, e.Derivation)
			}
			// Both sides must say where the claim comes from, or the reader
			// cannot tell it from a measurement.
			for _, s := range []conflictSide{e.A, e.B} {
				if !strings.Contains(s.Basis, "survey") && !strings.Contains(s.Basis, "KnownArtifacts") &&
					!strings.Contains(s.Basis, "capture") {
					t.Errorf("%s: a live_only side carries no provenance: %q", e.ID, s.Basis)
				}
			}
		default:
			t.Errorf("%s: computable = %q, which is not one of the three answers", e.ID, e.Computable)
		}
		if len(e.RecomputeNeeds) == 0 {
			t.Errorf("%s names no document --recompute would read", e.ID)
		}
	}
	if counts[conflictVerified] == 0 || counts[conflictLiveOnly] == 0 {
		t.Errorf("the ledger holds %v; both classes must be present or the flag proves nothing", counts)
	}
}

// TestDaysInFiscalMonthUsesTheRealCalendar keeps the implied utilisation
// honest. A hard-coded 30 would put February out by up to 3.4%, and
// FY2023-24's February has 29 days.
func TestDaysInFiscalMonthUsesTheRealCalendar(t *testing.T) {
	for _, tc := range []struct {
		fy    string
		idx   int
		month string
		want  int
	}{
		{"FY2017-18", 0, "Jul", 31},
		{"FY2017-18", 2, "Sep", 30},
		{"FY2017-18", 7, "Feb", 28},
		{"FY2019-20", 7, "Feb", 29},
		{"FY2023-24", 7, "Feb", 29},
		{"FY2023-24", 11, "Jun", 30},
		{"2020-21", 7, "Feb", 28},
	} {
		if got := daysInFiscalMonth(tc.fy, tc.idx); got != tc.want {
			t.Errorf("daysInFiscalMonth(%s, %d=%s) = %d, want %d", tc.fy, tc.idx, tc.month, got, tc.want)
		}
	}
	// An out-of-range index has no month, so no day count is invented.
	for _, idx := range []int{-1, 12, 99} {
		if got := daysInFiscalMonth("FY2020-21", idx); got != 0 {
			t.Errorf("daysInFiscalMonth with index %d = %d, want 0", idx, got)
		}
	}
	// An implied utilisation with no denominator does not exist.
	if _, ok := zlfImplied(4.44, 0, true, "FY2017-18", 2, false); ok {
		t.Error("zlfImplied returned a utilisation for a plant with 0 MW installed")
	}
	if _, ok := zlfImplied(4.44, 969, false, "FY2017-18", 2, false); ok {
		t.Error("zlfImplied returned a utilisation for a plant whose capacity was never published")
	}
	// September 2017 has 30 days: 4.44 GWh on 969 MW is 0.6363946%.
	got, ok := zlfImplied(4.44, 969, true, "FY2017-18", 2, false)
	if !ok || math.Abs(got-0.6363946) > 1e-6 {
		t.Errorf("zlfImplied(4.44 GWh, 969 MW, Sep FY2017-18) = %v (%v), want 0.6363946%%", got, ok)
	}
}

// TestGenPathUsesTheBareFiscalYear guards the one URL detail that silently
// 404s: the token is "2020-21", never "FY2020-21", and NEPRA's own typo
// "Genenration" is load-bearing.
func TestGenPathUsesTheBareFiscalYear(t *testing.T) {
	for _, fy := range []string{"FY2020-21", "2020-21"} {
		path := conflictGenPath(fy)
		if strings.Contains(path, "FY2020-21") {
			t.Errorf("conflictGenPath(%q) = %q; the {fy} token is the BARE year", fy, path)
		}
		if !strings.Contains(path, "wise%202020-21_files") {
			t.Errorf("conflictGenPath(%q) = %q, want the encoded bare year", fy, path)
		}
		if !strings.Contains(path, "Genenration") {
			t.Errorf("conflictGenPath(%q) = %q; NEPRA's own typo is load-bearing", fy, path)
		}
	}
	if got := conflictGenURL("FY2023-24"); !strings.HasPrefix(got, "https://nepra.org.pk/") {
		t.Errorf("conflictGenURL = %q, want an absolute NEPRA URL", got)
	}
	// Only the seven published years are reachable; FY2016-17 and FY2024-25
	// are measured 404s.
	for _, fy := range []string{"FY2017-18", "FY2020-21", "FY2023-24"} {
		if !conflictsGenYearReachable(fy) {
			t.Errorf("%s should be reachable", fy)
		}
	}
	for _, fy := range []string{"FY2016-17", "FY2024-25", "FY2025-26"} {
		if conflictsGenYearReachable(fy) {
			t.Errorf("%s is a measured 404 and must not be reachable", fy)
		}
	}
}

// TestPERURLsComeFromTheRegistry keeps the fetch leg off the reliability
// command's /Standards/{path} template, which cannot reach the FY2022-23 or
// FY2024-25 reports at all.
func TestPERURLsComeFromTheRegistry(t *testing.T) {
	for _, tc := range []struct{ fy, want string }{
		{"FY2024-25", "https://nepra.org.pk/M&E/PER/Distribution/2026/PER%202024-25%20Distribution%20Companies.pdf"},
		{"FY2020-21", "https://nepra.org.pk/Standards/2022/NEPRA%20PER%202021%20Distribution%20Companies%20.pdf"},
		{"FY2022-23", "https://nepra.org.pk/M&E/PER/Distribution/PER%202022-23%20-%20DSICOs.pdf"},
	} {
		if got := conflictPERURL(tc.fy); got != tc.want {
			t.Errorf("conflictPERURL(%s) = %q, want %q", tc.fy, got, tc.want)
		}
	}
	// The FY2020-21 path ends in a trailing ENCODED SPACE before .pdf, and
	// the same URL without it returns 404. A "cleaned" path is a dead one.
	if !strings.HasSuffix(conflictPERURL("FY2020-21"), "Companies%20.pdf") {
		t.Error("the FY2020-21 URL lost its trailing %20")
	}
	// Two of the three flagship years live OUTSIDE /Standards/, which is why
	// the reliability command's template cannot reach them.
	if strings.Contains(conflictPERURL("FY2024-25"), "/Standards/") {
		t.Error("the FY2024-25 report is not under /Standards/; the registry path must be used verbatim")
	}
	if got := conflictPERURL("FY1999-00"); got != "" {
		t.Errorf("conflictPERURL for an unrecorded year = %q, want empty", got)
	}
}

// TestContentHashesAreOverExtractedData is why the artifact carries two
// hashes.
//
// A raw byte hash of this site is useless as a document identity: two
// identical fetches of one page seconds apart returned the same 74,431 bytes
// with different SHA-256s, because Cloudflare rewrites every data-cfemail
// token per response.
func TestContentHashesAreOverExtractedData(t *testing.T) {
	r := perFixtureReport(t, "fy2024-25", "FY2024-25")
	a, err := perContentHash(r)
	if err != nil {
		t.Fatal(err)
	}
	b, err := perContentHash(perFixtureReport(t, "fy2024-25", "FY2024-25"))
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Error("perContentHash is not stable across two parses of the same document")
	}
	if !strings.Contains(string(a), "3547.00") {
		t.Error("the content hash input does not carry the published figures it is meant to fingerprint")
	}

	w := genFixtureWorkbook(t, "2020-21")
	ga, err := genContentHash(w)
	if err != nil {
		t.Fatal(err)
	}
	gb, err := genContentHash(genFixtureWorkbook(t, "2020-21"))
	if err != nil {
		t.Fatal(err)
	}
	if string(ga) != string(gb) {
		t.Error("genContentHash is not stable across two parses of the same workbook")
	}

	// The artifact's two hashes must DIFFER in value: one is over the bytes,
	// the other over the extracted rows.
	art := newNepraArtifact("https://example.invalid/x", []byte("raw bytes"), a, "application/pdf", 120, 0)
	if art.SHA256Raw == art.SHA256Content {
		t.Error("sha256_raw and sha256_content are equal; they are over different things")
	}
	if art.SHA256Raw == "" || art.SHA256Content == "" {
		t.Error("an artifact is missing a hash")
	}
	payload, err := json.Marshal(art)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "http_status") {
		t.Error("the artifact carries http_status; the client never observes the status code")
	}
}
