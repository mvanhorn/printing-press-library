// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// These cases close mutation-coverage gaps in `verify`: each one was proved by
// applying the operator change by hand and watching the named test fail, then
// reverting it and watching it pass again. Every number asserted here is
// either MEASURED at run time from a committed fixture in this same test, or
// read from the manifest struct under inspection — none is transcribed from a
// comment or a document.

package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// TestVerifySumInvariantFailsOnAnySingleManifestDisagreement pins the OR in
// the sum-invariant manifest comparison.
//
// The leg compares three independent counts against the manifest — passed,
// eligible and ineligible — and any ONE of them disagreeing is a failure. An
// AND there would need all three to move together before it refused, so a
// manifest that had drifted on exactly one count (the realistic case: NEPRA
// republishes a year with one more DELICENSED row, moving eligible and
// ineligible but not passed) would sail through the gate.
//
// The observed counts are not hard-coded. They are read from the leg's own
// facts on a report-only run over the FY2023-24 fixture, then perturbed by one
// at a time, so the assertion is "disagreement refuses" rather than a
// transcribed triple.
func TestVerifySumInvariantFailsOnAnySingleManifestDisagreement(t *testing.T) {
	w, err := nepraparse.ParseWorkbook(verifyWorkbookFixture(t, "2023-24"), "2023-24")
	if err != nil {
		t.Fatal(err)
	}
	s := verifyMustSurface(t, "gen-2023-24")

	// Measure the three counts with no manifest attached: with Internals nil
	// the leg is report-only and simply states what it observed.
	reportOnly := s
	reportOnly.Internals = nil
	base := verifySumInvariantLeg(reportOnly, w)
	if base.Verdict != verifyReported {
		t.Fatalf("with no internals the leg must be report-only, got %s", base.Verdict)
	}
	obsPassed := verifyFactInt(t, base, "passed")
	obsEligible := verifyFactInt(t, base, "eligible")
	obsIneligible := verifyFactInt(t, base, "ineligible")
	if obsEligible <= 0 || obsIneligible <= 0 {
		t.Fatalf("the FY2023-24 fixture must have eligible and ineligible rows to perturb; "+
			"measured %d/%d with %d ineligible", obsPassed, obsEligible, obsIneligible)
	}

	// A manifest that agrees on all three passes.
	agreeing := verifyInternals{
		InternalsAsOf: s.Internals.InternalsAsOf,
		SumPassed:     obsPassed, SumEligible: obsEligible, Ineligible: obsIneligible,
	}
	withAgreeing := s
	withAgreeing.Internals = &agreeing
	if got := verifySumInvariantLeg(withAgreeing, w); got.Verdict != verifyPass {
		t.Fatalf("a manifest matching the measured counts must pass, got %s (facts %v)",
			got.Verdict, got.Facts)
	}

	// And a manifest that disagrees on exactly ONE of the three refuses.
	for _, tc := range []struct {
		name  string
		mutTo verifyInternals
	}{
		{"passed only", verifyInternals{SumPassed: obsPassed - 1, SumEligible: obsEligible, Ineligible: obsIneligible}},
		{"eligible only", verifyInternals{SumPassed: obsPassed, SumEligible: obsEligible + 1, Ineligible: obsIneligible}},
		{"ineligible only", verifyInternals{SumPassed: obsPassed, SumEligible: obsEligible, Ineligible: obsIneligible + 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.mutTo
			in.InternalsAsOf = s.Internals.InternalsAsOf
			drifted := s
			drifted.Internals = &in
			leg := verifySumInvariantLeg(drifted, w)
			if leg.Verdict != verifyFail {
				t.Fatalf("manifest %d/%d with %d ineligible against measured %d/%d with %d: "+
					"verdict = %s, want fail. One count disagreeing is enough; requiring all "+
					"three to move would let a single-count drift through the gate.",
					in.SumPassed, in.SumEligible, in.Ineligible,
					obsPassed, obsEligible, obsIneligible, leg.Verdict)
			}
			problem, ok := leg.Facts["problem"].(string)
			if !ok || !strings.Contains(problem, "observed") {
				t.Fatalf("a refusal must carry the manifest-vs-observed problem string; facts = %v", leg.Facts)
			}
		})
	}
}

// TestVerifyRowFloorLegSeparatesNoFloorFromAMetFloor pins the <= 0 guard.
//
// RowFloor == 0 is the sentinel for "no floor was ever measured for this
// surface", and the leg must then report rather than assert: a zero floor that
// every row count clears would be an assertion that always passes, dressed up
// as a gate. Flip the comparison and the two states swap — the four sheets
// that DO carry a measured floor stop being asserted, and an unfloored surface
// starts returning pass as if it had been checked.
func TestVerifyRowFloorLegSeparatesNoFloorFromAMetFloor(t *testing.T) {
	s := verifyMustSurface(t, "hydel")
	if s.RowFloor <= 0 {
		t.Fatalf("the hydel surface must carry a measured row floor; got %d", s.RowFloor)
	}
	// Measure the extracted row count from the committed sheet fixture.
	res := verifySheetLegs(s, "https://nepra.org.pk"+s.Path, verifyOKFetch(sheetFixture(t, "hydel")))
	rows := verifyFactInt(t, verifyLegByName(t, res, "row_floor"), "rows")

	met := verifyRowFloorLeg(s, rows)
	if met.Verdict != verifyPass {
		t.Fatalf("%d extracted rows against a measured floor of %d: verdict = %s, want pass. "+
			"A surface with a recorded floor must be ASSERTED, not reported.",
			rows, s.RowFloor, met.Verdict)
	}

	unfloored := s
	unfloored.RowFloor = 0
	none := verifyRowFloorLeg(unfloored, rows)
	if none.Verdict != verifyReported {
		t.Fatalf("with no recorded floor the leg must report, got %s (facts %v). A zero floor is "+
			"an absent measurement, not a floor of zero that everything clears.",
			none.Verdict, none.Facts)
	}
	if !strings.Contains(none.Note, "no row floor") {
		t.Fatalf("the report-only note must say no floor was recorded; got %q", none.Note)
	}
	if _, present := none.Facts["above_floor_by"]; present {
		t.Fatalf("no floor was recorded, so there is nothing to be above: facts = %v", none.Facts)
	}
	// And a real floor that is NOT met still fails, so the pass above is not
	// coming from a leg that cannot refuse.
	if got := verifyRowFloorLeg(s, s.RowFloor-1); got.Verdict != verifyFail {
		t.Fatalf("%d rows under a floor of %d: verdict = %s, want fail", s.RowFloor-1, s.RowFloor, got.Verdict)
	}
}

// TestVerifyManifestKeepsWorkbookAndSheetInternalsApart pins the kind switch
// in the manifest payload builder.
//
// A workbook's measured internals are table_rows/raw_cells/plants/... and a
// sheet's are grid_rows/grid_raw_cells. The two sets live in one struct, so
// the only thing keeping a workbook's numbers out of the sheet fields is that
// comparison. Flip it and gen-2023-24 reports grid_rows: 0 — a fabricated zero
// for a quantity nobody measured — while its real, measured row count
// disappears into a null.
func TestVerifyManifestKeepsWorkbookAndSheetInternalsApart(t *testing.T) {
	// Measure both kinds from the committed fixtures first.
	gen := verifyMustSurface(t, "gen-2023-24")
	genRes := verifyWorkbookLegs(gen, "https://nepra.org.pk"+gen.Path,
		verifyOKFetch(verifyWorkbookFixture(t, "2023-24")))
	genStructure := verifyLegByName(t, genRes, "structure")
	measuredTableRows := verifyFactInt(t, genStructure, "table_rows")
	measuredRawCells := verifyFactInt(t, genStructure, "raw_cells")
	measuredPlants := verifyFactInt(t, genStructure, "plants")

	sheet := verifyMustSurface(t, "hydel")
	sheetRes := verifySheetLegs(sheet, "https://nepra.org.pk"+sheet.Path,
		verifyOKFetch(sheetFixture(t, "hydel")))
	sheetStructure := verifyLegByName(t, sheetRes, "structure")
	measuredGridRows := verifyFactInt(t, sheetStructure, "grid_rows")
	measuredGridCells := verifyFactInt(t, sheetStructure, "raw_cells")

	p := verifyBuildManifestPayload()
	entry := func(id string) verifyManifestEntry {
		for _, e := range p.Surfaces {
			if e.Surface == id {
				return e
			}
		}
		t.Fatalf("the manifest payload has no surface %q", id)
		return verifyManifestEntry{}
	}

	g := entry("gen-2023-24")
	if g.Kind != verifyKindWorkbook {
		t.Fatalf("gen-2023-24 kind = %q", g.Kind)
	}
	for _, f := range []struct {
		key string
		got *int
	}{
		{"table_rows", g.TableRows}, {"raw_cells", g.RawCells}, {"plants", g.Plants},
		{"physical_width", g.PhysicalWidth}, {"nbsp_0xa0_bytes", g.NBSPBytes},
		{"sum_gwh_passed", g.SumPassed}, {"sum_gwh_eligible", g.SumEligible},
		{"ineligible_rows", g.Ineligible},
	} {
		if f.got == nil {
			t.Fatalf("gen-2023-24.%s is null, but this year's internals WERE measured; a workbook's "+
				"numbers must land in the workbook fields", f.key)
		}
	}
	if g.SigmaSumGWh == nil {
		t.Fatal("gen-2023-24.sigma_sum_gwh is null, but this year's internals were measured")
	}
	if *g.TableRows != measuredTableRows || *g.RawCells != measuredRawCells || *g.Plants != measuredPlants {
		t.Fatalf("manifest gen-2023-24 reports %d rows / %d cells / %d plants; the fixture measures "+
			"%d / %d / %d", *g.TableRows, *g.RawCells, *g.Plants,
			measuredTableRows, measuredRawCells, measuredPlants)
	}
	if g.GridRows != nil {
		t.Fatalf("gen-2023-24.grid_rows = %d, want null: no grid row count was ever measured for a "+
			"workbook, and a 0 here would be a number nobody observed", *g.GridRows)
	}
	if g.GridRawCells != nil {
		t.Fatalf("gen-2023-24.grid_raw_cells = %d, want null", *g.GridRawCells)
	}

	h := entry("hydel")
	if h.Kind != verifyKindSheet {
		t.Fatalf("hydel kind = %q", h.Kind)
	}
	if h.GridRows == nil || h.GridRawCells == nil {
		t.Fatalf("hydel's grid internals were measured and must be present: grid_rows=%v grid_raw_cells=%v",
			h.GridRows, h.GridRawCells)
	}
	if *h.GridRows != measuredGridRows || *h.GridRawCells != measuredGridCells {
		t.Fatalf("manifest hydel reports %d grid rows / %d raw cells; the fixture measures %d / %d",
			*h.GridRows, *h.GridRawCells, measuredGridRows, measuredGridCells)
	}
	for _, f := range []struct {
		key string
		got *int
	}{
		{"table_rows", h.TableRows}, {"raw_cells", h.RawCells}, {"plants", h.Plants},
		{"physical_width", h.PhysicalWidth}, {"nbsp_0xa0_bytes", h.NBSPBytes},
		{"sum_gwh_passed", h.SumPassed}, {"sum_gwh_eligible", h.SumEligible},
		{"ineligible_rows", h.Ineligible},
	} {
		if f.got != nil {
			t.Fatalf("hydel.%s = %d, want null: an Excel sheet has no plant panel, so a number "+
				"there is a workbook's field filled in by accident", f.key, *f.got)
		}
	}
	if h.SigmaSumGWh != nil {
		t.Fatalf("hydel.sigma_sum_gwh = %v, want null", *h.SigmaSumGWh)
	}
}

// TestVerifyCrosscheckDeclaredGapIsDisclosedInMeta pins the promotion of the
// crosscheck's declared gap into meta.declared_gaps.
//
// meta.declared_gaps is the list a caller reads to find out what the run did
// NOT answer. The crosscheck's own block carries the same gap, but a caller
// who only reads meta — which is the whole point of meta — would otherwise be
// told nothing about the unread IEA side. Break the guard and the disclosure
// silently stops happening while the exit code stays 0.
func TestVerifyCrosscheckDeclaredGapIsDisclosedInMeta(t *testing.T) {
	srv, hits := verifyStubServer(t, http.StatusOK, "text/html", verifyWorkbookFixture(t, "2023-24"))
	defer srv.Close()
	out, code := verifyRun(t, srv.URL, "verify", "--fy", "2023-24", "--crosscheck", "iea", "--json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0; output:\n%s", code, truncate(out, 600))
	}
	if hits.Load() != 1 {
		t.Fatalf("%d requests made, want exactly 1: the NEPRA side is read and the IEA side is not",
			hits.Load())
	}
	var doc struct {
		Meta struct {
			DeclaredGaps []verifyGap `json:"declared_gaps"`
		} `json:"meta"`
		Crosscheck *verifyCrosscheck `json:"crosscheck"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("stdout is not parseable JSON: %v\n%s", err, out)
	}
	if doc.Crosscheck == nil || doc.Crosscheck.DeclaredGap == nil {
		t.Fatalf("the crosscheck block must declare its unread side: %+v", doc.Crosscheck)
	}
	want := *doc.Crosscheck.DeclaredGap
	found := false
	for _, g := range doc.Meta.DeclaredGaps {
		if g.Gap == want.Gap && g.Reason == want.Reason {
			found = true
		}
	}
	if !found {
		var got []string
		for _, g := range doc.Meta.DeclaredGaps {
			got = append(got, g.Gap)
		}
		t.Fatalf("crosscheck.declared_gap %q is missing from meta.declared_gaps %v. A gap that is "+
			"only in the crosscheck block is invisible to a caller reading meta, and this run "+
			"exited 0 while leaving a whole side of the comparison unread.", want.Gap, got)
	}
}

// TestVerifyCurtailKeepsTheAssertedSurfaces pins the empty-selection fallback.
//
// Curtailing the dogfood matrix must keep the surfaces that actually exercise
// every leg — the one asserted workbook and the smallest asserted sheet — and
// fall back to the first surface only when the keep set selects nothing.
// Inverting the guard makes the fallback fire on the normal path, so the
// bounded run silently covers one arbitrary surface instead of both kinds and
// no leg that needs asserted internals is exercised at all; and on the path
// the fallback exists for it returns an EMPTY set, which gates nothing.
func TestVerifyCurtailKeepsTheAssertedSurfaces(t *testing.T) {
	got := verifyCurtail(verifySurfaces)
	var ids []string
	kinds := map[string]bool{}
	for _, s := range got {
		ids = append(ids, s.ID)
		kinds[s.Kind] = true
		if !s.Asserted {
			t.Fatalf("curtail kept %q, whose internals are report-only; the bounded run must keep "+
				"surfaces that can actually fail a leg", s.ID)
		}
	}
	if len(got) < 2 || !kinds[verifyKindWorkbook] || !kinds[verifyKindSheet] {
		t.Fatalf("curtail kept %v; it must keep at least one workbook and one sheet so the bounded "+
			"run still exercises every leg", ids)
	}

	// The fallback: a set containing none of the keepers keeps exactly one
	// surface, never zero. An empty target set would make the whole gate a
	// no-op that exits 0 without checking anything.
	none := []verifySurface{verifyMustSurface(t, "gen-2018-19"), verifyMustSurface(t, "fca")}
	fb := verifyCurtail(none)
	if len(fb) != 1 || fb[0].ID != none[0].ID {
		t.Fatalf("curtail of a set with no keepers = %d surfaces (%+v), want exactly the first one, %q",
			len(fb), fb, none[0].ID)
	}
}
