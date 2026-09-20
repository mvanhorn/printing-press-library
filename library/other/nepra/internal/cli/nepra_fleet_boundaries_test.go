// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// HAND-AUTHORED boundary tests for the `fleet` panel: each one pins a branch
// whose two sides produce different output, so flipping the operator at that
// site turns this file red. Every number here was MEASURED by running the
// command against the embedded crosswalk — see the comment above each value
// for the selection it came from.

package cli

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraxwalk"
)

// ---------------------------------------------------------------------------
// 1. An acronym that already reaches its parent's WHOLE fleet gets no scope
//    note. nepra_fleet.go:568 fires the note only when the parent has strictly
//    more plants than the acronym reached; with ENGRO there is nothing wider
//    to point at, and a note saying "reaches 1; its parent has 1" would invite
//    a second, identical query.
// ---------------------------------------------------------------------------

func TestFleetAcronymCoveringWholeFleetHasNoScopeNote(t *testing.T) {
	// Measured: ResolveTicker("EPTPL") is an acronym for PSX ENGRO, and
	// PlantsForParent("ENGRO") holds exactly the same single plant.
	stdout, stderr, err := runFleet(t, "--parent", "EPTPL")
	if err != nil {
		t.Fatalf("returned %v", err)
	}
	env := decodeFleet(t, stdout)
	r := env.Results.Resolution
	if r.TokenKind != "nepra_acronym" {
		t.Fatalf("token_kind = %q, want nepra_acronym (this test must exercise the acronym branch)", r.TokenKind)
	}
	if r.PSXTicker != "ENGRO" {
		t.Fatalf("psx_ticker = %q, want ENGRO", r.PSXTicker)
	}
	// The acronym's reach and the parent's fleet are the same one plant, which
	// is what makes the note pointless here.
	if len(env.Results.Plants) != 1 || env.Results.Plants[0].CanonicalName != "Engro Powergen Thar (Private) Limited. (EPTPL)" {
		t.Fatalf("plants = %v, want exactly [Engro Powergen Thar (Private) Limited. (EPTPL)]", fleetPlantNames(env))
	}
	pp, ok := nepraxwalk.PlantsForParent("ENGRO")
	if !ok || len(pp.Plants) != len(env.Results.Plants) {
		t.Fatalf("PlantsForParent(ENGRO) = %d plants (ok=%v), want the same %d the acronym reached",
			len(pp.Plants), ok, len(env.Results.Plants))
	}
	if r.ScopeNote != "" {
		t.Errorf("scope_note = %q, want empty: the acronym already covers the whole %d-plant fleet, so there is no wider fleet to send the caller to",
			r.ScopeNote, len(pp.Plants))
	}
	if strings.Contains(stderr, "SCOPE:") {
		t.Errorf("human channel printed a SCOPE line for a fleet-wide acronym:\n%s", stderr)
	}
}

// ---------------------------------------------------------------------------
// 2. Overstatement is withheld when operating capacity is zero.
//    nepra_fleet.go:817 returns nil unless BOTH sums are measurements AND the
//    denominator is above zero. Engro's FY2017-18 cell is a published numeric
//    0 MW (the Thar block was not yet commissioned), so reported and operating
//    are both MEASURED zeros and the ratio has no value — not a 0.0%.
// ---------------------------------------------------------------------------

func TestFleetOverstatementWithheldOnAZeroDenominator(t *testing.T) {
	stdout, _, err := runFleet(t, "--parent", "ENGRO")
	if err != nil {
		t.Fatalf("returned %v", err)
	}
	env := decodeFleet(t, stdout)

	// Measured for --parent ENGRO: FY2017-18 reported 0.00 MW over 1 plant and
	// operating 0.00 MW over 1 plant. Both are measurements, so the guard that
	// withholds the ratio is the denominator one.
	f18 := fleetFYByLabel(t, env, "2017-18")
	wantMeasured(t, "FY2017-18 capacity_reported", f18.CapacityReported, 0, 1)
	wantMeasured(t, "FY2017-18 capacity_operating", f18.CapacityOperating, 0, 1)
	if f18.OverstatementPct != nil {
		t.Errorf("FY2017-18 overstatement = %v, want ABSENT: reported/operating over a zero denominator is not a number",
			*f18.OverstatementPct)
	}

	// Control, same command: with a positive measured denominator the ratio IS
	// reported, and for Engro's single plant it is a measured 0 (660/660).
	f24 := fleetFYByLabel(t, env, "2023-24")
	wantMeasured(t, "FY2023-24 capacity_operating", f24.CapacityOperating, 660, 1)
	if f24.OverstatementPct == nil {
		t.Fatal("FY2023-24 carries no overstatement, but both sums are measured and operating is 660 MW")
	}
	if got := *f24.OverstatementPct; got != 0 {
		t.Errorf("FY2023-24 overstatement = %v, want a measured 0 (reported 660 == operating 660)", got)
	}
}

// ---------------------------------------------------------------------------
// 3. The as-of cross-check needs BOTH capacity sums to agree.
//    nepra_fleet.go:851 ANDs the operating and non-operating comparisons. If
//    either one alone could pass the assertion, the cross-check would sign off
//    on a fleet whose delicensed megawatts had drifted — the single error this
//    panel exists to catch.
// ---------------------------------------------------------------------------

func TestFleetAsOfCrossCheckNeedsBothSumsToAgree(t *testing.T) {
	pp, ok := nepraxwalk.PlantsForParent("HUBC")
	if !ok {
		t.Fatal("PlantsForParent(HUBC) is not in the crosswalk")
	}
	names := make([]string, 0, len(pp.Plants))
	for _, p := range pp.Plants {
		names = append(names, p.CanonicalName)
	}
	sort.Strings(names)
	latest := nepraxwalk.LatestObservedFY()
	fys := []string{latest}

	// Baseline: the real fleet's two computations agree, so the assertion passes.
	base := fleetNamedAssertion(t, fleetAssertions(names, fys, latest, pp, true, fleetResults{}), "as_of_cross_check")
	if !base.OK {
		t.Fatalf("as_of_cross_check is already failing on the committed crosswalk, so this test cannot tell a drift apart: %s", base.Detail)
	}

	// Two foreign-but-MEASURED sums to swap in, taken by running Coverage over
	// a different selection (Kotri Power Station, as of the same fiscal year).
	// Measured: HUBC operating is 2289.00 MW over 5 plants and non-operating
	// 1292.00 MW over 1; Kotri's are 0.00 MW over 0 and 174.00 MW over 1.
	foreign := nepraxwalk.Coverage(latest, []string{"Kotri Power Station"})
	if fleetSameSum(foreign.CapacityOperating, pp.CapacityOperating) ||
		fleetSameSum(foreign.CapacityNonOperating, pp.CapacityNonOperating) {
		t.Fatalf("the foreign sums must differ from HUBC's to perturb anything: foreign op %s / non-op %s vs HUBC op %s / non-op %s",
			foreign.CapacityOperating, foreign.CapacityNonOperating, pp.CapacityOperating, pp.CapacityNonOperating)
	}

	for _, tc := range []struct {
		name  string
		drift func(nepraxwalk.ParentPlants) nepraxwalk.ParentPlants
	}{
		{"only the non-operating sum drifts", func(p nepraxwalk.ParentPlants) nepraxwalk.ParentPlants {
			p.CapacityNonOperating = foreign.CapacityNonOperating
			return p
		}},
		{"only the operating sum drifts", func(p nepraxwalk.ParentPlants) nepraxwalk.ParentPlants {
			p.CapacityOperating = foreign.CapacityOperating
			return p
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := fleetNamedAssertion(t, fleetAssertions(names, fys, latest, tc.drift(pp), true, fleetResults{}), "as_of_cross_check")
			if got.OK {
				t.Errorf("as_of_cross_check passed with one capacity sum disagreeing; one matching sum must not excuse the other: %s", got.Detail)
			}
		})
	}
}

func fleetNamedAssertion(t *testing.T, as []fleetAssertion, name string) fleetAssertion {
	t.Helper()
	for _, a := range as {
		if a.Name == name {
			return a
		}
	}
	t.Fatalf("no %q assertion in %v", name, as)
	return fleetAssertion{}
}

// ---------------------------------------------------------------------------
// 4. The absent-parent hint matches probe tokens on WORD boundaries, digits
//    included. nepra_fleet.go:1146 counts 0-9 as word bytes, so "BQPS1" — a
//    real K-Electric unit designation — is not the token "BQPS" and must not
//    pull K-Electric's absence note onto an unrelated decline.
// ---------------------------------------------------------------------------

func TestFleetAbsentParentHintTreatsDigitsAsWordBytes(t *testing.T) {
	// Positive control: the bare probe token DOES pull the absence record, so
	// a missing hint below means the boundary rule fired, not that the hint is
	// broken or unreachable.
	stdout, _, err := runFleet(t, "--plant", "SITE")
	if err == nil {
		t.Fatal("an unknown plant name must be declined")
	}
	hit := decodeRefusal(t, stdout)
	if hit.Error.Kind != "unknown_plant_name" {
		t.Fatalf("kind = %q, want unknown_plant_name", hit.Error.Kind)
	}
	if !strings.Contains(hit.Error.AbsentParentHint, "K-Electric Limited") {
		t.Fatalf("bare probe token SITE produced no K-Electric absence hint: %q", hit.Error.AbsentParentHint)
	}

	// The boundary case: a digit glued to the token is part of the word.
	for _, q := range []string{"BQPS1", "BQPS2", "SITE1"} {
		stdout, _, err := runFleet(t, "--plant", q)
		if err == nil {
			t.Fatalf("%q must be declined", q)
		}
		r := decodeRefusal(t, stdout)
		if r.Error.Kind != "unknown_plant_name" {
			t.Errorf("%q: kind = %q, want unknown_plant_name", q, r.Error.Kind)
		}
		if r.Error.AbsentParentHint != "" {
			t.Errorf("%q: absent_parent_hint fired on a digit-suffixed token, so a probe token matched mid-word: %q",
				q, r.Error.AbsentParentHint)
		}
		if r.Error.CounterFact != "" {
			t.Errorf("%q: counter_fact rides along with the hint and must be absent too: %q", q, r.Error.CounterFact)
		}
	}
}

// TestFleetContainsWordBoundaryBytes pins the boundary rule directly on both
// sides of the needle, which the CLI path above can only reach from the right.
func TestFleetContainsWordBoundaryBytes(t *testing.T) {
	for _, tc := range []struct {
		haystack, needle string
		want             bool
	}{
		{"SITE", "SITE", true},
		{"OLD SITE PLANT", "SITE", true},
		{"SITE-1", "SITE", true},   // '-' is not a word byte
		{"SITES", "SITE", false},   // letter after
		{"OFFSITE", "SITE", false}, // letter before
		{"SITE1", "SITE", false},   // digit after
		{"1SITE", "SITE", false},   // digit before
		{"BQPS9", "BQPS", false},
		{"9BQPS", "BQPS", false},
		// Production upper-cases both sides before calling this, so the
		// lower-case half of the rule is only reachable from here. It is still
		// the rule: a caller that skips the fold must not get a mid-word hit.
		{"site", "site", true},
		{"old site plant", "site", true},
		{"sites", "site", false},
		{"offsite", "site", false},
		{"site1", "site", false},
		{"1site", "site", false},
	} {
		if got := fleetContainsWord(tc.haystack, tc.needle); got != tc.want {
			t.Errorf("fleetContainsWord(%q, %q) = %v, want %v", tc.haystack, tc.needle, got, tc.want)
		}
	}
}

// ---------------------------------------------------------------------------
// 5. The "no published number" line prints when ANY of the three unpublished
//    populations is non-empty. nepra_fleet_render.go:247 ORs them; requiring
//    all three at once would hide the only case the committed crosswalk
//    actually has — 11 blank capacity cells in FY2017-18, none of which is a
//    status sentinel or unmodelled text.
// ---------------------------------------------------------------------------

func TestFleetHumanReportsBlankCapacityCellsAlone(t *testing.T) {
	// Measured for --plant "Gulpur Hydropower Project": FY2017-18 has 1 blank
	// capacity cell, 0 status sentinels and 0 unmodelled-text cells, and
	// FY2023-24 has none of the three.
	stdout, _, err := runFleetHuman(t, "--plant", "Gulpur Hydropower Project")
	if err != nil {
		t.Fatalf("returned %v", err)
	}
	const want = "cells with no published number: 1 blank, 0 status sentinel, 0 unmodelled text"
	if !strings.Contains(stdout, want) {
		t.Errorf("human FY panel does not report the blank cell; one non-empty population is enough to print the line.\nwant substring: %s\ngot:\n%s", want, stdout)
	}
	if n := strings.Count(stdout, "cells with no published number"); n != 1 {
		t.Errorf("the line appears %d times, want 1 (only FY2017-18 has an unpublished cell for this plant)", n)
	}
	// The JSON panel carries the same three counts, which is where the
	// rendered line comes from.
	jsonOut, _, err := runFleet(t, "--plant", "Gulpur Hydropower Project")
	if err != nil {
		t.Fatalf("json run returned %v", err)
	}
	env := decodeFleetCells(t, jsonOut)
	f18 := fleetCellCountsByFY(t, env, "2017-18")
	if f18.CapacityNotReported != 1 || f18.CapacityStatusCell != 0 || f18.CapacityUnknownText != 0 {
		t.Errorf("FY2017-18 unpublished-cell counts = %d blank / %d sentinel / %d unmodelled, want 1 / 0 / 0",
			f18.CapacityNotReported, f18.CapacityStatusCell, f18.CapacityUnknownText)
	}
	// FY2023-24 has all three at zero, and prints no line at all — that is the
	// other side of the same branch.
	f24 := fleetCellCountsByFY(t, env, "2023-24")
	if f24.CapacityNotReported != 0 || f24.CapacityStatusCell != 0 || f24.CapacityUnknownText != 0 {
		t.Errorf("FY2023-24 unpublished-cell counts = %d / %d / %d, want 0 / 0 / 0",
			f24.CapacityNotReported, f24.CapacityStatusCell, f24.CapacityUnknownText)
	}
}

// fleetCellCounts mirrors just the three unpublished-cell counters of a fiscal
// year column; the shared envelope in fleet_test.go does not decode them.
type fleetCellCounts struct {
	FY                  string `json:"fy"`
	CapacityNotReported int    `json:"capacity_not_reported"`
	CapacityStatusCell  int    `json:"capacity_status_cell"`
	CapacityUnknownText int    `json:"capacity_unknown_text"`
}

type fleetCellCountsEnvelope struct {
	Results struct {
		FiscalYears []fleetCellCounts `json:"fiscal_years"`
	} `json:"results"`
}

func decodeFleetCells(t *testing.T, stdout string) fleetCellCountsEnvelope {
	t.Helper()
	var env fleetCellCountsEnvelope
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("stdout is not the fleet envelope: %v\n%s", err, stdout)
	}
	return env
}

func fleetCellCountsByFY(t *testing.T, env fleetCellCountsEnvelope, fy string) fleetCellCounts {
	t.Helper()
	for _, c := range env.Results.FiscalYears {
		if c.FY == fy {
			return c
		}
	}
	t.Fatalf("no FY%s column in the panel", fy)
	return fleetCellCounts{}
}

// ---------------------------------------------------------------------------
// 2b. The same guard, one clause at a time. nepra_fleet.go:817 refuses on
//     THREE independent grounds, and CoverageReport can never hand the CLI a
//     measured numerator beside an unmeasured denominator (one `measurable`
//     flag sets every sum in a report), so the measuredness clauses are only
//     reachable by calling the helper with sums from two different reports —
//     which is exactly what a caller comparing years would do.
// ---------------------------------------------------------------------------

func TestFleetOverstatementNeedsBothSumsMeasured(t *testing.T) {
	pp, ok := nepraxwalk.PlantsForParent("HUBC")
	if !ok {
		t.Fatal("PlantsForParent(HUBC) is not in the crosswalk")
	}
	names := make([]string, 0, len(pp.Plants))
	for _, p := range pp.Plants {
		names = append(names, p.CanonicalName)
	}
	latest := nepraxwalk.LatestObservedFY()
	held := nepraxwalk.HeldOutFYs()
	if len(held) == 0 {
		t.Fatal("the crosswalk holds out no fiscal year, so there is no unmeasured sum to test with")
	}

	// Measured: FY2023-24 over the HUBC fleet is a real measurement in both
	// lines; the held-out year is a measurement in neither.
	obs := nepraxwalk.Coverage(latest, names)
	out := nepraxwalk.Coverage(held[0], names)
	if _, ok := obs.CapacityReported.Float64(); !ok {
		t.Fatalf("FY%s reported is not measured, so it cannot serve as the measured side", latest)
	}
	op, okOp := obs.CapacityOperating.Float64()
	if !okOp || op <= 0 {
		t.Fatalf("FY%s operating = %v (measured=%v), want a positive measurement", latest, op, okOp)
	}
	if _, ok := out.CapacityReported.Float64(); ok {
		t.Fatalf("FY%s is held out and must be unmeasured", held[0])
	}

	// An unmeasured numerator over a perfectly good denominator is still not a
	// ratio: it would read as -100%, i.e. "this fleet publishes nothing".
	if got := fleetOverstatement(out.CapacityReported, obs.CapacityOperating); got != nil {
		t.Errorf("overstatement over an UNMEASURED reported sum = %v%%, want nil: an unobserved year has no numerator", *got)
	}
	// And a measured numerator over an unmeasured denominator.
	if got := fleetOverstatement(obs.CapacityReported, out.CapacityOperating); got != nil {
		t.Errorf("overstatement over an UNMEASURED operating sum = %v%%, want nil: an unobserved year has no denominator", *got)
	}
	// Control: both from the observed year, and the ratio is reported.
	if got := fleetOverstatement(obs.CapacityReported, obs.CapacityOperating); got == nil {
		t.Error("overstatement over two measured sums is nil, so the guard above proves nothing")
	}
}

// ---------------------------------------------------------------------------
// 5b. The same line, one counter at a time. The committed crosswalk has only
//     blank cells (11, all in FY2017-18), so the status-sentinel and
//     unmodelled-text counters can only be driven through the renderer
//     directly. Either one alone must print the line: an unmodelled sentinel
//     that prints nothing is how a parser gap becomes a silent zero.
// ---------------------------------------------------------------------------

func TestFleetHumanReportsEachUnpublishedCellPopulationAlone(t *testing.T) {
	for _, tc := range []struct {
		name string
		col  fleetFiscalYear
		want string
	}{
		{
			name: "blank cells only",
			col:  fleetFiscalYear{FY: "2023-24", FYObserved: true, CoverageClass: fleetCoverageObserved, CapacityNotReported: 2},
			want: "cells with no published number: 2 blank, 0 status sentinel, 0 unmodelled text",
		},
		{
			name: "status sentinel only",
			col:  fleetFiscalYear{FY: "2023-24", FYObserved: true, CoverageClass: fleetCoverageObserved, CapacityStatusCell: 3},
			want: "cells with no published number: 0 blank, 3 status sentinel, 0 unmodelled text",
		},
		{
			name: "unmodelled text only",
			col:  fleetFiscalYear{FY: "2023-24", FYObserved: true, CoverageClass: fleetCoverageObserved, CapacityUnknownText: 1},
			want: "cells with no published number: 0 blank, 0 status sentinel, 1 unmodelled text",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := fleetRenderColumn(t, tc.col)
			if !strings.Contains(got, tc.want) {
				t.Errorf("renderer dropped the unpublished-cell line.\nwant substring: %s\ngot:\n%s", tc.want, got)
			}
		})
	}
	// Control: with all three empty there is no line to print.
	got := fleetRenderColumn(t, fleetFiscalYear{FY: "2023-24", FYObserved: true, CoverageClass: fleetCoverageObserved})
	if strings.Contains(got, "cells with no published number") {
		t.Errorf("renderer printed an unpublished-cell line for a column with none:\n%s", got)
	}
}

// fleetRenderColumn renders the human panel for one synthetic fiscal-year
// column and returns stdout. The capacity sums are left as zero-value MWSums,
// which render as "<not measured>" — this exercises the counters, and asserts
// nothing about megawatts.
func fleetRenderColumn(t *testing.T, col fleetFiscalYear) string {
	t.Helper()
	cmd := &cobra.Command{}
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	fleetWriteHuman(cmd, fleetResults{
		Selector:    fleetSelector{Kind: "parent", Value: "synthetic"},
		AsOf:        fleetAsOf{AsOfFY: col.FY},
		FiscalYears: []fleetFiscalYear{col},
	})
	return out.String()
}
