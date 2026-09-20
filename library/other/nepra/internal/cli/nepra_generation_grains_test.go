// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Tests for `generation plants` / `generation monthly`.
//
// Every assertion here pins a HAZARD that a naive implementation of these two
// grains actually hits, not a restatement of the code. They run offline
// against the committed workbook fixtures.

package cli

import (
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// TestPlantGrainIsOneRowPerPlant pins the grain itself. `gen` emits twelve
// rows per plant; this command must emit exactly one, for EVERY plant
// whatever its class, so a delicensed plant cannot vanish.
func TestPlantGrainIsOneRowPerPlant(t *testing.T) {
	w, _ := genWorkbook(t, "2023-24")
	rows := genPlantRows(w, w.Plants, genTestURL)
	if len(rows) != len(w.Plants) {
		t.Fatalf("plant rows = %d, want %d (one per plant)", len(rows), len(w.Plants))
	}
	if len(rows) != 133 {
		t.Fatalf("FY2023-24 plant rows = %d, want the published 133", len(rows))
	}
	seen := map[string]int{}
	for _, r := range rows {
		seen[r.Plant]++
	}
	for name, n := range seen {
		if n != 1 {
			t.Fatalf("plant %q appears %d times at the plant grain", name, n)
		}
	}
}

// TestPlantGrainKeepsCapacityOfADelicensedPlant pins the independence of
// capacity and the monthly block. Kotri publishes 174/120 MW while all 26 of
// its monthly cells read DELICENSED; nulling capacity from the block status
// would delete a published figure.
func TestPlantGrainKeepsCapacityOfADelicensedPlant(t *testing.T) {
	w, _ := genWorkbook(t, "2023-24")
	rows := genPlantRows(w, w.Plants, genTestURL)
	var got *genPlantRow
	for i := range rows {
		if strings.Contains(rows[i].Plant, "Kotri") {
			got = &rows[i]
			break
		}
	}
	if got == nil {
		t.Fatal("Kotri Power Station is absent from the plant grain")
	}
	if got.RowClass != "delicensed" {
		t.Fatalf("Kotri row_class = %q, want delicensed", got.RowClass)
	}
	if got.InstalledMW == nil || *got.InstalledMW != 174 {
		t.Fatalf("Kotri installed_mw = %v, want 174 — a delicensed plant keeps its published capacity", got.InstalledMW)
	}
	if got.DependableMW == nil || *got.DependableMW != 120 {
		t.Fatalf("Kotri dependable_mw = %v, want 120", got.DependableMW)
	}
	// The other half of the contract: it generated nothing, and that must
	// read as absence rather than as a measured zero.
	if got.SumGWhReported != nil {
		t.Fatalf("Kotri sum_gwh_reported = %v, want absent: the cell is a DELICENSED sentinel, not a 0", *got.SumGWhReported)
	}
	if got.MonthsReported != 0 {
		t.Fatalf("Kotri months_reported = %d, want 0", got.MonthsReported)
	}
}

// TestPlantGrainNeverInventsAZeroCapacity pins the converse. FY2017-18's
// eleven listed_no_data plants publish NO capacity number at all; emitting
// 0.00 for them would fabricate eleven plants' worth of capacity.
func TestPlantGrainNeverInventsAZeroCapacity(t *testing.T) {
	w, _ := genWorkbook(t, "2017-18")
	rows := genPlantRows(w, w.Plants, genTestURL)
	n := 0
	for _, r := range rows {
		if r.RowClass != "listed_no_data" {
			continue
		}
		n++
		if r.InstalledMW != nil {
			t.Fatalf("%s is listed_no_data but installed_mw = %v; the cell publishes no number", r.Plant, *r.InstalledMW)
		}
		if r.InstalledMWState != "not_reported" {
			t.Fatalf("%s installed_mw_state = %q, want not_reported", r.Plant, r.InstalledMWState)
		}
	}
	if n != 11 {
		t.Fatalf("FY2017-18 listed_no_data plants = %d, want the measured 11", n)
	}
}

// TestFilterIsExactAndNeverAPrefix is the vocabulary hazard. "THERMAL" and
// "THERMAL- COAL" are two different published technologies — the space after
// the hyphen is NEPRA's own — so a prefix or substring match silently folds
// eight extra plants into THERMAL, and a whitespace-normalising match would
// accept "THERMAL-COAL", which NEPRA never published.
func TestFilterIsExactAndNeverAPrefix(t *testing.T) {
	w, _ := genWorkbook(t, "2023-24")

	thermal, err := genFilterPlants(w, genPlantFilter{Technology: "THERMAL"})
	if err != nil {
		t.Fatalf("--technology THERMAL: %v", err)
	}
	if len(thermal) != 46 {
		t.Fatalf("THERMAL matched %d plants, want 46; %d would mean THERMAL- COAL was swallowed", len(thermal), 46+8)
	}
	coal, err := genFilterPlants(w, genPlantFilter{Technology: "THERMAL- COAL"})
	if err != nil {
		t.Fatalf("--technology 'THERMAL- COAL': %v", err)
	}
	if len(coal) != 8 {
		t.Fatalf("THERMAL- COAL matched %d plants, want 8", len(coal))
	}
	for _, p := range thermal {
		if p.Technology != "THERMAL" {
			t.Fatalf("THERMAL selection leaked %q", p.Technology)
		}
	}

	// The normalised spelling is not data and must be refused, not repaired.
	if _, err := genFilterPlants(w, genPlantFilter{Technology: "THERMAL-COAL"}); err == nil {
		t.Fatal("--technology THERMAL-COAL was accepted; NEPRA publishes no such value and a repaired guess is indistinguishable from a real one")
	}
}

// TestFilterMatchIsCaseInsensitiveButWholeCell allows the ergonomic case fold
// while still refusing a partial cell.
func TestFilterMatchIsCaseInsensitiveButWholeCell(t *testing.T) {
	if !genFilterMatches("THERMAL", "thermal") {
		t.Fatal("case-insensitive whole-cell match was rejected")
	}
	if genFilterMatches("THERMAL- COAL", "THERMAL") {
		t.Fatal("a prefix matched: --technology THERMAL would swallow THERMAL- COAL")
	}
	if genFilterMatches("THERMAL", "THERM") {
		t.Fatal("a substring matched")
	}
}

// TestFilterErrorNamesTheYearsOwnVocabulary pins the error's usefulness: a
// rejection has to say what this year does publish, and distinguish "absent
// from this year" from "not a NEPRA value at all".
func TestFilterErrorNamesTheYearsOwnVocabulary(t *testing.T) {
	w, _ := genWorkbook(t, "2023-24")
	_, err := genFilterPlants(w, genPlantFilter{Fuel: "PLUTONIUM"})
	if err == nil {
		t.Fatal("an unpublished fuel was accepted")
	}
	msg := err.Error()
	for _, want := range []string{"PLUTONIUM", "not published in this fiscal year", "This year publishes:"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("error %q does not contain %q", msg, want)
		}
	}
	if !strings.Contains(msg, "not in NEPRA's published vocabulary in any sampled year") {
		t.Fatalf("error %q does not distinguish an unknown value from a merely absent one", msg)
	}
}

// TestFilterOnAPublishedButUnknownValueStillWorks pins the contract genRollup
// already states for an out-of-vocabulary group: a value the year publishes
// is filterable even when it is outside the measured vocabulary. Validation
// reads the YEAR, not only nepraparse's constant lists.
func TestFilterOnAPublishedButUnknownValueStillWorks(t *testing.T) {
	w, _ := genWorkbook(t, "2023-24")
	for _, tech := range w.Technologies() {
		if nepraparse.KnownTechnology(tech) {
			continue
		}
		got, err := genFilterPlants(w, genPlantFilter{Technology: tech})
		if err != nil {
			t.Fatalf("published-but-unknown technology %q was refused: %v", tech, err)
		}
		if len(got) == 0 {
			t.Fatalf("published technology %q matched nothing", tech)
		}
	}
}

// TestMonthlyGrainIsGenRowsUnchanged pins the claim the help text makes: an
// unfiltered `generation monthly` is the same rows as `gen`, from the same
// builder. If these ever diverge, one of the two is wrong.
func TestMonthlyGrainIsGenRowsUnchanged(t *testing.T) {
	w, _ := genWorkbook(t, "2023-24")
	all := genRows(w, genTestURL)
	if len(all) != len(w.Plants)*genMonthsPerPlant {
		t.Fatalf("genRows emitted %d rows, want %d", len(all), len(w.Plants)*genMonthsPerPlant)
	}
	selected, err := genFilterPlants(w, genPlantFilter{})
	if err != nil {
		t.Fatalf("empty filter: %v", err)
	}
	if len(selected) != len(w.Plants) {
		t.Fatalf("empty filter selected %d of %d plants; it must select every plant", len(selected), len(w.Plants))
	}
}

// TestMonthlyFilterKeepsAllTwelveMonths pins that selection is BY PLANT. A
// matched plant keeps all twelve of its rows, so a filtered extract is still
// a complete year for the plants it contains.
func TestMonthlyFilterKeepsAllTwelveMonths(t *testing.T) {
	w, _ := genWorkbook(t, "2023-24")
	selected, err := genFilterPlants(w, genPlantFilter{Technology: "NUCLEAR"})
	if err != nil {
		t.Fatalf("--technology NUCLEAR: %v", err)
	}
	keep := map[string]bool{}
	for _, p := range selected {
		keep[p.Name] = true
	}
	count := map[string]int{}
	for _, r := range genRows(w, genTestURL) {
		if keep[r.Plant] {
			count[r.Plant]++
		}
	}
	if len(count) != len(selected) {
		t.Fatalf("%d plants survived the row filter, want %d", len(count), len(selected))
	}
	for name, n := range count {
		if n != genMonthsPerPlant {
			t.Fatalf("plant %q kept %d monthly rows, want %d", name, n, genMonthsPerPlant)
		}
	}
}

// TestGrainAssertionDoesNotFailAFilteredRun is the --strict hazard. On
// FY2023-24 `--technology Coal` is ONE plant and it is DELICENSED, so the
// whole-workbook assertion rows == plants*12 would report a failure and exit
// 1 under --strict on an answer that is entirely correct. The grain
// assertion must be measured against the SELECTION.
func TestGrainAssertionDoesNotFailAFilteredRun(t *testing.T) {
	w, _ := genWorkbook(t, "2023-24")
	selected, err := genFilterPlants(w, genPlantFilter{Technology: "Coal"})
	if err != nil {
		t.Fatalf("--technology Coal: %v", err)
	}
	if len(selected) != 1 {
		t.Fatalf("FY2023-24 --technology Coal matched %d plants, want 1 (Lakhra)", len(selected))
	}

	plantAssert := genGrainAssertion(genGrainPlant, len(selected), len(selected))
	if !plantAssert.OK || !plantAssert.Asserts {
		t.Fatalf("plant-grain assertion failed on a correct filtered run: %+v", plantAssert)
	}
	monthlyAssert := genGrainAssertion(genGrainPlantMonth, len(selected)*genMonthsPerPlant, len(selected))
	if !monthlyAssert.OK || !monthlyAssert.Asserts {
		t.Fatalf("monthly-grain assertion failed on a correct filtered run: %+v", monthlyAssert)
	}
	// And it must still be able to FAIL, or it is decoration.
	if bad := genGrainAssertion(genGrainPlant, len(selected)+1, len(selected)); bad.OK {
		t.Fatal("the plant-grain assertion passed on a wrong row count")
	}
	if bad := genGrainAssertion(genGrainPlantMonth, 1, len(selected)); bad.OK {
		t.Fatal("the monthly-grain assertion passed on a wrong row count")
	}
}

// TestMonthsPerPlantIsNotDerivedFromMonthlyCells pins the off-by-one that the
// obvious derivation produces: MonthlyCells is PairCount*2 == 26 and
// PairCount is 13 because the thirteenth pair is the annual Sum, not a month.
func TestMonthsPerPlantIsNotDerivedFromMonthlyCells(t *testing.T) {
	if genMonthsPerPlant != 12 {
		t.Fatalf("genMonthsPerPlant = %d, want 12", genMonthsPerPlant)
	}
	if nepraparse.MonthlyCells/2 == genMonthsPerPlant {
		t.Fatalf("MonthlyCells/2 == %d now equals genMonthsPerPlant; the comment explaining why they differ is stale",
			nepraparse.MonthlyCells/2)
	}
}

// TestPlantHeaderCarriesNoUtilisationAggregate pins the refusal structurally.
// This grain has capacity in MW and generation in GWh on one row, which is
// exactly where a capacity factor looks easy and is wrong: the honest
// denominator is hours-per-month and this corpus publishes it nowhere.
func TestPlantHeaderCarriesNoUtilisationAggregate(t *testing.T) {
	for _, h := range genPlantRowHeader {
		for _, banned := range []string{"utilisation", "utilization", "capacity_factor", "load_factor"} {
			if strings.Contains(h, banned) {
				t.Fatalf("plant grain emits column %q; no aggregate utilisation may be emitted at any grain", h)
			}
		}
	}
}

// TestPlantRowCellsMatchHeaderOrder pins the literal header against the
// literal Cells order. They are two hand-written lists and nothing else
// checks that they agree.
func TestPlantRowCellsMatchHeaderOrder(t *testing.T) {
	w, _ := genWorkbook(t, "2023-24")
	rows := genPlantRows(w, w.Plants, genTestURL)
	if len(rows) == 0 {
		t.Fatal("no rows")
	}
	if got, want := len(rows[0].Cells()), len(genPlantRowHeader); got != want {
		t.Fatalf("Cells() has %d values, header has %d columns", got, want)
	}
	// An absent number must render as "", never "0" and never "null".
	//
	// THIS RUNS OVER FY2017-18 AS WELL, AND THAT IS THE POINT. Checked only
	// against FY2023-24 the absent-capacity branch NEVER FIRES — every one of
	// that year's 133 plants publishes an installed capacity — so the
	// assertion was vacuous and would have passed against a build that
	// rendered nil as "0". FY2017-18 publishes eleven plants with no capacity
	// number at all, which is what makes the check real. The counters below
	// fail the test if either case stops being exercised.
	absentCap, nilReconcile := 0, 0
	for _, fy := range []string{"2023-24", "2017-18"} {
		fw, _ := genWorkbook(t, fy)
		for _, r := range genPlantRows(fw, fw.Plants, genTestURL) {
			cells := r.Cells()
			if len(cells) != len(genPlantRowHeader) {
				t.Fatalf("FY%s %s: Cells() has %d values, header has %d", fy, r.Plant, len(cells), len(genPlantRowHeader))
			}
			if r.InstalledMW == nil {
				absentCap++
				if cells[7] != "" {
					t.Fatalf("FY%s %s: absent installed_mw rendered as %q", fy, r.Plant, cells[7])
				}
			}
			if r.SumReconciles == nil {
				nilReconcile++
				if cells[15] != "" {
					t.Fatalf("FY%s %s: nil sum_reconciles rendered as %q, want the empty string not \"false\"",
						fy, r.Plant, cells[15])
				}
			}
		}
	}
	if absentCap == 0 {
		t.Fatal("no plant in either fiscal year has an absent installed_mw, so the absent-number assertion " +
			"never fired and this test would pass against a build that rendered nil as \"0\"")
	}
	if nilReconcile == 0 {
		t.Fatal("no plant has a nil sum_reconciles, so that assertion never fired either")
	}
	t.Logf("exercised %d absent capacities and %d nil reconcile verdicts across two fiscal years",
		absentCap, nilReconcile)
}

// TestBothGrainsAreRegisteredUnderGeneration pins the wiring. cobra's Find
// returns a NIL error for a missing nested name, so a hook that checks only
// err would silently publish these at top level instead.
func TestBothGrainsAreRegisteredUnderGeneration(t *testing.T) {
	for _, leaf := range []string{"plants", "monthly"} {
		cmd, rest, err := RootCmd().Find([]string{"generation", leaf})
		if err != nil {
			t.Fatalf("Find(generation %s): %v", leaf, err)
		}
		if len(rest) != 0 {
			t.Fatalf("generation %s did not resolve: Find left %v unconsumed (it fell through to the parent)", leaf, rest)
		}
		if cmd.Name() != leaf {
			t.Fatalf("Find(generation %s) resolved to %q", leaf, cmd.Name())
		}
		if parent := cmd.Parent(); parent == nil || parent.Name() != "generation" {
			t.Fatalf("%s is not a child of generation", leaf)
		}
		for _, ann := range []string{"mcp:read-only", "pp:happy-args", "pp:typed-exit-codes", "pp:novel-hand-coded"} {
			if cmd.Annotations[ann] == "" {
				t.Fatalf("generation %s is missing the %s annotation", leaf, ann)
			}
		}
		if cmd.Annotations["pp:novel-scaffold"] != "" {
			t.Fatalf("generation %s still carries the novel-scaffold annotation", leaf)
		}
	}
}

// TestGrainMetaAssertsAgainstTheSelectionNotTheWorkbook exercises the CALL
// SITE, not the assertion helper.
//
// This test exists because a mutation that passed len(w.Plants) instead of
// len(selected) into genGrainAssertion SURVIVED the first test pass:
// TestGrainAssertionDoesNotFailAFilteredRun calls the helper directly with
// correct arguments, so it can never see a wiring bug. Under that mutation a
// filtered run reports a failed completeness assertion and exits 1 under
// --strict on an answer that is entirely correct.
func TestGrainMetaAssertsAgainstTheSelectionNotTheWorkbook(t *testing.T) {
	w, body := genWorkbook(t, "2023-24")
	year, ok := genYearByLabel("2023-24")
	if !ok {
		t.Fatal("FY2023-24 is not catalogued")
	}
	filter := genPlantFilter{Technology: "Coal"}
	selected, err := genFilterPlants(w, filter)
	if err != nil {
		t.Fatalf("--technology Coal: %v", err)
	}
	if len(selected) != 1 || len(selected) == len(w.Plants) {
		t.Fatalf("FY2023-24 --technology Coal selected %d of %d plants, want 1 — the filter must not be a no-op or this test asserts nothing",
			len(selected), len(w.Plants))
	}

	rows := genPlantRows(w, selected, genTestURL)
	meta := genGrainMeta(w, year, genGrainPlant, genGrainPlant, filter, selected,
		len(rows), body, genTestURL, "text/html", genRowsJSON(rows))

	var got *genAssertion
	for i := range meta.Assertions {
		if meta.Assertions[i].Name == "rows_equal_plants_matched" {
			got = &meta.Assertions[i]
		}
	}
	if got == nil {
		t.Fatal("the grain assertion is absent from meta")
	}
	if !got.Asserts {
		t.Fatal("the grain assertion asserts nothing")
	}
	if !got.OK {
		t.Fatalf("the grain assertion FAILED on a correct filtered run: expected=%v actual=%v — it is measuring against the whole workbook, not the selection",
			got.Expected, got.Actual)
	}
	if got.Expected == nil || *got.Expected != len(selected) {
		t.Fatalf("expected=%v, want the SELECTION size %d", got.Expected, len(selected))
	}

	// The rest of the meta block must still describe the WHOLE year, or the
	// column-order proof and the census silently become claims about one plant.
	if meta.Plants != len(w.Plants) {
		t.Fatalf("meta.plants = %d, want the whole year's %d", meta.Plants, len(w.Plants))
	}
	if meta.PlantsMatched != len(selected) {
		t.Fatalf("meta.plants_matched = %d, want %d", meta.PlantsMatched, len(selected))
	}
	if !meta.ColumnOrder.PctFirst {
		t.Fatal("the percent-then-GWh proof was computed over the FILTERED subset; it must come from the whole workbook")
	}
	if meta.Artifact.SkippedRows != len(w.Plants)-len(selected) {
		t.Fatalf("artifact.skipped_rows = %d, want %d excluded plants", meta.Artifact.SkippedRows, len(w.Plants)-len(selected))
	}
}
