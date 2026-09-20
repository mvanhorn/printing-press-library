// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// Branch-level tests for the conflicts command's decision points: the --strict
// gate, the gen leg's content hash, the surfaces that need a workbook, the
// one-sided-absence comparison, the blank-is-not-zero guard in the
// zero-load-factor scan, and the abs_diff that must not exist when one side of
// a conflict is not a number.
//
// Every number asserted here was MEASURED by running this code against the
// committed captures in testdata/ and against the shipped ledger. No figure is
// transcribed from a comment or a document, and nothing asserts a zero for a
// value the code reports as unavailable.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraper"
)

// conflictsRecomputeEnvelope is the recompute path's wire shape. The envelope
// declared in conflicts_test.go refuses unknown fields and does not model the
// artifacts, the scan census or the recompute verdicts, which are exactly the
// blocks the live leg adds.
type conflictsRecomputeEnvelope struct {
	Meta struct {
		Source       string `json:"source"`
		Recomputed   bool   `json:"recomputed"`
		BytesFetched int    `json:"bytes_fetched"`
		EntriesTotal int    `json:"entries_total"`
	} `json:"meta"`
	Results struct {
		Entries   []conflictEntry `json:"entries"`
		Recompute []struct {
			ID         string `json:"id"`
			Verdict    string `json:"verdict"`
			Reproduced bool   `json:"reproduced"`
		} `json:"recompute"`
		Artifacts []struct {
			URL           string `json:"url"`
			Bytes         int    `json:"bytes"`
			Rows          int    `json:"rows"`
			SHA256Raw     string `json:"sha256_raw"`
			SHA256Content string `json:"sha256_content"`
		} `json:"artifacts"`
		ScanCensus []genScanCensus `json:"scan_census"`
	} `json:"results"`
}

// runConflictsOverWorkbook drives the whole command against a local server
// that serves one committed generation workbook, so the fetch, the parse, the
// content hash, the reconciliation and the envelope are all exercised offline.
func runConflictsOverWorkbook(t *testing.T, fy string, args ...string) (env conflictsRecomputeEnvelope,
	stderr string, err error) {
	t.Helper()
	body := conflictsGenFixture(t, fy)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	t.Setenv("NEPRA_BASE_URL", srv.URL)

	cmd := RootCmd()
	cmd.SetArgs(append([]string{"conflicts"}, args...))
	var so, se bytes.Buffer
	cmd.SetOut(&so)
	cmd.SetErr(&se)
	err = cmd.Execute()
	if err != nil {
		return conflictsRecomputeEnvelope{}, se.String(), err
	}
	if uerr := json.Unmarshal(so.Bytes(), &env); uerr != nil {
		t.Fatalf("conflicts %v output is not the recompute envelope: %v\n%s", args, uerr, so.String())
	}
	return env, se.String(), nil
}

// TestConflictsStrictExitsZeroWhenEveryReadEntryReproduces pins the --strict
// gate to the thing it asserts: a shortfall, not the mere presence of the
// flag.
//
// --strict is an assertion about the DOCUMENTS, so a run in which every
// selected entry reproduced must exit 0 with the flag set. MEASURED: the
// committed FY2023-24 workbook reproduces the one FY2023-24 gen/arithmetic
// ledger entry exactly, so this selection has no shortfall to report.
func TestConflictsStrictExitsZeroWhenEveryReadEntryReproduces(t *testing.T) {
	env, stderr, err := runConflictsOverWorkbook(t, "2023-24",
		"--recompute", "--strict", "--surface", "gen", "--kind", "arithmetic",
		"--fy", "2023-24", "--json", "--no-cache")
	if err != nil {
		t.Fatalf("--strict failed a run with nothing to report: %v\nstderr: %s", err, stderr)
	}
	if strings.Contains(stderr, "COMPLETENESS") {
		t.Errorf("a run with no shortfall printed a COMPLETENESS line:\n%s", stderr)
	}
	// The assertion above is only worth something if the run actually
	// recomputed something and every verdict was a reproduction.
	if !env.Meta.Recomputed || env.Meta.Source != "ledger+live" {
		t.Errorf("meta recomputed=%v source=%q, want true/\"ledger+live\"", env.Meta.Recomputed, env.Meta.Source)
	}
	if len(env.Results.Recompute) != 1 {
		t.Fatalf("recompute verdicts = %d, want 1; --strict would then be asserting over nothing",
			len(env.Results.Recompute))
	}
	for _, r := range env.Results.Recompute {
		if r.Verdict != conflictReproduced || !r.Reproduced {
			t.Errorf("%s: verdict %q reproduced=%v, want a reproduction", r.ID, r.Verdict, r.Reproduced)
		}
	}
}

// TestConflictsGenRecomputeRecordsTheArtifactItHashed pins the gen leg's
// content-hash step to the artifact it produces.
//
// The hash is what gives a derived figure its provenance, so a leg that
// skipped it — or returned early on a SUCCESSFUL hash — would answer with an
// empty document set and no error at all: a recompute that read nothing,
// reported as a clean run. Every value here was measured from the committed
// FY2023-24 capture.
func TestConflictsGenRecomputeRecordsTheArtifactItHashed(t *testing.T) {
	env, stderr, err := runConflictsOverWorkbook(t, "2023-24",
		"--recompute", "--surface", "gen", "--kind", "arithmetic",
		"--fy", "2023-24", "--json", "--no-cache")
	if err != nil {
		t.Fatalf("gen recompute: %v\nstderr: %s", err, stderr)
	}
	if len(env.Results.Artifacts) != 1 {
		t.Fatalf("artifacts = %d, want 1; a recompute that hashed no document derived nothing",
			len(env.Results.Artifacts))
	}
	a := env.Results.Artifacts[0]
	if a.Bytes != 493187 || a.Rows != 133 {
		t.Errorf("artifact = %d bytes over %d rows, want 493187 over 133", a.Bytes, a.Rows)
	}
	if a.SHA256Raw != "0ad0cb1183895e10b7e6f761f8a683415b92b14fa028acf1092669a0760a1a02" {
		t.Errorf("sha256_raw = %q", a.SHA256Raw)
	}
	if a.SHA256Content != "5ff098a5f617b926cb969dea8d0e333d14aca5d43db1f7c7b03f35bea7f1a0ed" {
		t.Errorf("sha256_content = %q", a.SHA256Content)
	}
	if env.Meta.BytesFetched != a.Bytes {
		t.Errorf("meta.bytes_fetched = %d, want the artifact's %d", env.Meta.BytesFetched, a.Bytes)
	}
	// The census proves the scan ran over the document that was hashed, with
	// its own denominator.
	if len(env.Results.ScanCensus) != 1 {
		t.Fatalf("scan_census entries = %d, want 1", len(env.Results.ScanCensus))
	}
	c := env.Results.ScanCensus[0]
	if c.FiscalYear != "FY2023-24" || c.PlantMonthsBothNumeric != 1416 || c.PlantMonthsFlagged != 1 {
		t.Errorf("census = %+v, want FY2023-24 with 1 flagged of 1416 plant-months", c)
	}
	if len(env.Results.Recompute) != 1 || !env.Results.Recompute[0].Reproduced {
		t.Errorf("recompute = %+v, want the single FY2023-24 entry reproduced", env.Results.Recompute)
	}
}

// TestConflictsYearsToReadCoversEverySurfaceThatNeedsTheWorkbook pins which
// surfaces make the generation workbook a document this run must read.
//
// The gen surface and the capacity surface BOTH live in that workbook, and the
// unselected default needs it too. A surface dropped from that test would
// refuse with "nothing to read" or fetch nothing and report a short list as a
// complete one. The year lists are MEASURED from the shipped ledger's own
// recompute_needs.
func TestConflictsYearsToReadCoversEverySurfaceThatNeedsTheWorkbook(t *testing.T) {
	ledger := conflictLedger()
	for _, tc := range []struct {
		surface string
		wantGen []string
		wantPER []string
	}{
		{"", []string{"FY2017-18", "FY2018-19", "FY2019-20", "FY2020-21", "FY2023-24"},
			[]string{"FY2018-19", "FY2019-20", "FY2020-21", "FY2024-25"}},
		{conflictSurfaceGen, []string{"FY2017-18", "FY2020-21", "FY2023-24"}, nil},
		{conflictSurfaceCapacity, []string{"FY2023-24"}, nil},
		{conflictSurfacePER, nil, []string{"FY2018-19", "FY2019-20", "FY2020-21", "FY2024-25"}},
	} {
		per, gen := conflictsYearsToRead(conflictsSelection{Surface: tc.surface, BreakRatio: 100}, ledger)
		if strings.Join(gen, ",") != strings.Join(tc.wantGen, ",") {
			t.Errorf("surface %q reads workbooks %v, want %v", tc.surface, gen, tc.wantGen)
		}
		if strings.Join(per, ",") != strings.Join(tc.wantPER, ",") {
			t.Errorf("surface %q reads PERs %v, want %v", tc.surface, per, tc.wantPER)
		}
	}
	// An explicit --fy narrows the list but must not change WHICH surfaces
	// need the workbook.
	for _, surface := range []string{"", conflictSurfaceGen, conflictSurfaceCapacity} {
		per, gen := conflictsYearsToRead(
			conflictsSelection{Surface: surface, FYs: []string{"FY2023-24"}, BreakRatio: 100}, ledger)
		if len(gen) != 1 || gen[0] != "FY2023-24" {
			t.Errorf("surface %q with --fy 2023-24 reads workbooks %v, want [FY2023-24]", surface, gen)
		}
		// FY2023-24's PER is published-but-404 in the registry, so no surface
		// turns it into a PER read.
		if len(per) != 0 {
			t.Errorf("surface %q with --fy 2023-24 reads PERs %v, want none", surface, per)
		}
	}
}

// TestConflictsFiguresAgreeRefusesOneSidedAbsence is the null discipline of the
// ledger-against-document comparison.
//
// Two absences agree: neither run measured anything, and that is not a
// disagreement. An absence against a NUMBER is not agreement and must never be
// treated as one — reporting it as agreement would mark an entry reproduced on
// the strength of a figure nobody measured. The ledger really does carry such
// entries: the two Three Gorges zero-load-factor rows have an installed
// capacity of 0 MW, so no implied utilisation exists on their B side.
func TestConflictsFiguresAgreeRefusesOneSidedAbsence(t *testing.T) {
	var oneSided int
	for _, e := range conflictLedger() {
		if (e.A.Value == nil) == (e.B.Value == nil) {
			continue
		}
		oneSided++
		present, absent := e.A.Value, e.B.Value
		if present == nil {
			present, absent = e.B.Value, e.A.Value
		}
		if conflictsFiguresAgree(absent, present) || conflictsFiguresAgree(present, absent) {
			t.Errorf("%s: an absent figure was reported as agreeing with the measured %v",
				e.ID, *present)
		}
		if !conflictsFiguresAgree(absent, absent) {
			t.Errorf("%s: two absences were reported as disagreeing; neither run measured anything", e.ID)
		}
		if !conflictsFiguresAgree(present, present) {
			t.Errorf("%s: the measured figure %v does not agree with itself", e.ID, *present)
		}
	}
	if oneSided == 0 {
		t.Fatal("no ledger entry has exactly one absent side; the assertion proved nothing")
	}
}

// TestZeroLoadFactorBlankUtilisationIsNeitherCountedNorFlagged is the
// blank-is-never-zero guard at the cell level.
//
// The three committed workbooks contain no row that pairs a blank utilisation
// with positive generation — that is what TestBlankIsNeverZeroInTheScan
// measures — so the guard's own branch is exercised here on a row built to
// have exactly that shape, with a numeric twin as the control. The only
// difference between the two cases is the utilisation cell's STATE.
func TestZeroLoadFactorBlankUtilisationIsNeitherCountedNorFlagged(t *testing.T) {
	build := func(utilisation string) *nepraparse.Workbook {
		p := nepraparse.Plant{
			SNo:               1,
			Name:              "Probe Power (Private) Limited",
			InstalledCapacity: nepraparse.ParseValue("50"),
		}
		for i := range p.Months {
			p.Months[i] = nepraparse.MonthlyObservation{Month: nepraparse.Month(i + 1)}
		}
		// Jul: generation published, utilisation as given.
		p.Months[0].Generation = nepraparse.ParseValue("3.42")
		p.Months[0].Utilisation = nepraparse.ParseValue(utilisation)
		p.Total = nepraparse.MonthlyObservation{IsTotal: true}
		return &nepraparse.Workbook{Plants: []nepraparse.Plant{p}}
	}

	// The control: a MEASURED 0.00 utilisation alongside 3.42 GWh is a real
	// finding, and it is counted in the denominator.
	entries, census := genZeroLoadFactorEntries(build("0.00"), "2023-24")
	if len(entries) != 1 {
		t.Fatalf("a published 0.00 against 3.42 GWh produced %d entries, want 1", len(entries))
	}
	if census.PlantMonthsBothNumeric != 1 || census.PlantMonthsFlagged != 1 {
		t.Errorf("control census = %d flagged of %d, want 1 of 1",
			census.PlantMonthsFlagged, census.PlantMonthsBothNumeric)
	}
	if census.BlankUtilisationWithGeneration != 0 {
		t.Errorf("control census reports %d blank-utilisation rows, want 0",
			census.BlankUtilisationWithGeneration)
	}

	// The guard: the SAME row with an unreported utilisation is not a
	// finding, is not in the denominator, and is reported as the negative
	// control it is.
	entries, census = genZeroLoadFactorEntries(build(""), "2023-24")
	if len(entries) != 0 {
		t.Errorf("an unreported utilisation produced %d entries; a blank was read as a zero and the finding "+
			"was invented: %+v", len(entries), entries)
	}
	if census.PlantMonthsBothNumeric != 0 {
		t.Errorf("an unreported utilisation was counted in the both-numeric denominator (%d); the "+
			"denominator would then include cells the scan cannot judge", census.PlantMonthsBothNumeric)
	}
	if census.PlantMonthsFlagged != 0 {
		t.Errorf("flagged plant-months = %d, want 0", census.PlantMonthsFlagged)
	}
	if census.BlankUtilisationWithGeneration != 1 {
		t.Errorf("blank_utilisation_with_generation = %d, want 1; the twin count is how a regression "+
			"coercing an unreported cell to 0.0 becomes visible", census.BlankUtilisationWithGeneration)
	}
}

// TestPERConflictWithANonNumericSideCarriesNoAbsDiff keeps a kind-mismatch
// conflict from publishing an absolute difference.
//
// nepraper computes |A-B| only when both sides are numeric, so its AbsDiff is
// the zero value for a kind mismatch. Projecting that zero into the entry
// would put "abs_diff": 0 on a pair whose difference was never computed —
// indistinguishable from two figures that are genuinely equal, which is the
// one thing a conflict cannot be.
//
// The numeric side is the MEASURED MEPCO SAIDI observation from the committed
// FY2024-25 capture; only the second side is constructed, as the truncated
// chart label this corpus is already known to contain.
func TestPERConflictWithANonNumericSideCarriesNoAbsDiff(t *testing.T) {
	fixture := perFixtureReport(t, "fy2024-25", "FY2024-25")
	var numeric *nepraper.Observation
	for i := range fixture.Observations {
		o := fixture.Observations[i]
		if o.Entity == nepraper.EntityMEPCO && o.Metric == nepraper.MetricSAIDI && o.Value.IsNumeric() {
			numeric = &fixture.Observations[i]
			break
		}
	}
	if numeric == nil {
		t.Fatal("the committed FY2024-25 capture holds no numeric MEPCO SAIDI observation to build on")
	}
	unverified := *numeric
	unverified.Value = nepraper.Value{
		Kind:   nepraper.KindUnverified,
		Raw:    "19,535.",
		Reason: "truncated Excel chart data label; trailing digits absent from the text layer",
	}
	unverified.Prov.TableLabel = "Figure 08"
	unverified.Prov.Page = numeric.Prov.Page + 1

	entries := perConflictEntries(&nepraper.Report{
		FY:           "FY2024-25",
		Observations: []nepraper.Observation{*numeric, unverified},
	})
	if len(entries) != 1 {
		t.Fatalf("a numeric figure against an unverified one produced %d conflicts, want 1", len(entries))
	}
	e := entries[0]
	if e.Class != "kind_mismatch" {
		t.Errorf("class = %q, want kind_mismatch", e.Class)
	}
	if e.A.Value == nil || *e.A.Value != numeric.Value.Num {
		t.Errorf("the numeric side = %v, want the capture's own %v", e.A.Value, numeric.Value.Num)
	}
	if e.B.Value != nil {
		t.Errorf("the unverified side carries value %v; nothing was published there", *e.B.Value)
	}
	if e.AbsDiff != nil {
		t.Errorf("abs_diff = %v on a pair with only one number; |A-B| was never computed and must be "+
			"absent, not zero", *e.AbsDiff)
	}
	if e.Ratio != nil {
		t.Errorf("ratio = %v on a pair with only one number", *e.Ratio)
	}
}
