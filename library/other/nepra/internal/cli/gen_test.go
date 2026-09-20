// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// Every number asserted here was MEASURED by running this code against the
// three committed fixtures, which are the published bytes verbatim. Where the
// spec this command was built from disagreed with what the code produces, the
// measurement won and the disagreement is recorded in the comment beside it.

package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// genFixture loads one fiscal year's published sheet001.htm.
//
// The fixtures are gzipped only so the repository does not carry 1.4 MB of
// blobs; decompressed they are byte-identical to what NEPRA serves. VERIFIED
// on 2026-09-10 for FY2023-24: the live URL with Accept: */* returns exactly
// 493,187 bytes with sha256
// 0ad0cb1183895e10b7e6f761f8a683415b92b14fa028acf1092669a0760a1a02, which is
// the committed fixture's hash. That is what lets these tests assert measured
// whole-file numbers rather than approximations.
func genFixture(t *testing.T, fy string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "gen-fy"+fy+".htm.gz"))
	if err != nil {
		t.Fatalf("reading fixture %s: %v", fy, err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("gzip %s: %v", fy, err)
	}
	defer func() { _ = zr.Close() }()
	out, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompress %s: %v", fy, err)
	}
	return out
}

func genWorkbook(t *testing.T, fy string) (*nepraparse.Workbook, []byte) {
	t.Helper()
	body := genFixture(t, fy)
	w, err := nepraparse.ParseWorkbook(body, fy)
	if err != nil {
		t.Fatalf("ParseWorkbook(FY%s): %v", fy, err)
	}
	return w, body
}

const genTestURL = "https://nepra.org.pk/test/sheet001.htm"

func genTestRows(t *testing.T, fy string) []genRow {
	t.Helper()
	w, _ := genWorkbook(t, fy)
	return genRows(w, genTestURL)
}

func genTestMeta(t *testing.T, fy string) genMeta {
	t.Helper()
	w, body := genWorkbook(t, fy)
	y, ok := genYearByLabel(fy)
	if !ok {
		t.Fatalf("FY%s is not catalogued", fy)
	}
	rows := genRows(w, genTestURL)
	return genBuildMeta(w, y, "plant-month", len(rows), body, genTestURL, "text/html", genRowsJSON(rows))
}

func genRowsFor(rows []genRow, plant string) []genRow {
	var out []genRow
	for _, r := range rows {
		if r.Plant == plant {
			out = append(out, r)
		}
	}
	return out
}

// TestNovelGenHelpWires smoke-tests that the gen command resolves at runtime
// and renders useful --help output. Catches wiring regressions (missing
// AddCommand, panicking RunE on --help, etc.).
func TestNovelGenHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"gen", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("gen --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "gen"} {
		if !strings.Contains(help, want) {
			t.Fatalf("gen --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestGenIsNotAScaffold proves the TODO body is gone and the annotations a
// local-store command needs are present. The publish gate rejects a
// hand-written command that lacks pp:happy-args or pp:typed-exit-codes as
// hollow coverage, and the scaffold's mcp:read-only:false was simply wrong for
// a command that makes one GET.
func TestGenIsNotAScaffold(t *testing.T) {
	found := 0
	for _, c := range RootCmd().Commands() {
		if c.Name() != "gen" {
			continue
		}
		found++
		if c.Annotations["pp:novel-scaffold"] == "true" {
			t.Error("gen still carries pp:novel-scaffold")
		}
		for k, want := range map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--fy=2023-24",
			"pp:typed-exit-codes": "0,1,2,3,5",
			"pp:novel-hand-coded": "true",
		} {
			if got := c.Annotations[k]; got != want {
				t.Errorf("annotation %q = %q, want %q", k, got, want)
			}
		}
		for _, flag := range []string{"fy", "format", "rollup", "strict"} {
			if c.Flags().Lookup(flag) == nil {
				t.Errorf("missing --%s", flag)
			}
		}
	}
	// Exactly one. A scaffold left alongside the implementation would
	// resolve unpredictably, and preferImplementedNovelCommands is what
	// removes it.
	if found != 1 {
		t.Fatalf("%d commands named gen are registered on the root, want exactly 1", found)
	}
}

// TestGenRowsFY2023_24Shape pins the extract's shape. 133 plants x 12 months =
// 1,596 rows, and the 139 <tr> behind those 133 plants were confirmed outside
// this parser by a regex pass over the fixture's cp1252 bytes.
func TestGenRowsFY2023_24Shape(t *testing.T) {
	rows := genTestRows(t, "2023-24")
	if len(rows) != 1596 {
		t.Fatalf("rows = %d, want 1596 (133 plants x 12)", len(rows))
	}
	plants := map[string]int{}
	for _, r := range rows {
		plants[r.Plant]++
	}
	if len(plants) != 133 {
		t.Errorf("distinct plants = %d, want 133", len(plants))
	}
	for name, n := range plants {
		if n != 12 {
			t.Errorf("plant %q has %d rows, want 12", name, n)
		}
	}

	// Fiscal order, and the calendar dating that goes with it: Jul of the
	// FIRST calendar year through Jun of the SECOND. Reading column 7 as
	// January of the first year mis-dates six months of every plant.
	wantMonths := []string{"Jul", "Aug", "Sep", "Oct", "Nov", "Dec", "Jan", "Feb", "Mar", "Apr", "May", "Jun"}
	wantPeriods := []string{
		"2023-07", "2023-08", "2023-09", "2023-10", "2023-11", "2023-12",
		"2024-01", "2024-02", "2024-03", "2024-04", "2024-05", "2024-06",
	}
	first := rows[:12]
	for i, r := range first {
		if r.Month != wantMonths[i] {
			t.Errorf("row %d month = %q, want %q", i, r.Month, wantMonths[i])
		}
		if r.MonthIndex != i+1 {
			t.Errorf("row %d month_index = %d, want %d", i, r.MonthIndex, i+1)
		}
		if r.Period != wantPeriods[i] {
			t.Errorf("row %d period = %q, want %q", i, r.Period, wantPeriods[i])
		}
		if r.FY != "2023-24" {
			t.Errorf("row %d fy = %q, want 2023-24", i, r.FY)
		}
	}

	// All 25 keys on every row, always. compactListFields drops any key
	// missing from more than 20% of rows, so an `omitempty` on a numeric
	// field would silently delete a column in a year with many unreported
	// plant-months.
	for i, r := range rows {
		var obj map[string]json.RawMessage
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatalf("marshal row %d: %v", i, err)
		}
		if err := json.Unmarshal(b, &obj); err != nil {
			t.Fatalf("unmarshal row %d: %v", i, err)
		}
		if len(obj) != 25 {
			t.Fatalf("row %d has %d keys, want 25: %s", i, len(obj), b)
		}
		for _, k := range genRowHeader {
			if _, ok := obj[k]; !ok {
				t.Fatalf("row %d is missing key %q", i, k)
			}
		}
	}
	if len(genRowHeader) != 25 {
		t.Errorf("genRowHeader has %d columns, want 25", len(genRowHeader))
	}
}

// TestGenBlankIsNeverZero is one half of the blank-vs-zero pair. Reshma and
// Gulf Powergen publish a capacity and NO monthly data; twelve rows of gwh=0
// would invent 24 plant-months of zero generation.
//
// MUTATION-CHECKED: making genFloatPtr return &zero for a non-numeric Value
// fails this test.
func TestGenBlankIsNeverZero(t *testing.T) {
	rows := genTestRows(t, "2023-24")
	for _, tc := range []struct {
		plant      string
		installed  float64
		dependable float64
	}{
		{"Reshma Power Generation (Private) Limited. (RPGPL)", 97, 97},
		{"Gulf Powergen (Private) Limited. (GPPL)", 84, 84},
	} {
		got := genRowsFor(rows, tc.plant)
		if len(got) != 12 {
			t.Fatalf("%s: %d rows, want 12", tc.plant, len(got))
		}
		for _, r := range got {
			if r.GWh != nil {
				t.Errorf("%s %s: gwh = %v, want nil", tc.plant, r.Month, *r.GWh)
			}
			if r.GWhState != "not_reported" {
				t.Errorf("%s %s: gwh_state = %q, want not_reported", tc.plant, r.Month, r.GWhState)
			}
			if r.UtilisationPct != nil || r.UtilisationState != "not_reported" {
				t.Errorf("%s %s: utilisation = %v/%q, want nil/not_reported", tc.plant, r.Month, r.UtilisationPct, r.UtilisationState)
			}
			if r.SumGWhReported != nil || r.SumGWhReportedState != "not_reported" {
				t.Errorf("%s %s: sum = %v/%q, want nil/not_reported", tc.plant, r.Month, r.SumGWhReported, r.SumGWhReportedState)
			}
			// Not eligible for the Sum identity is NOT the same as failing
			// it, so sum_reconciles is null rather than false.
			if r.SumReconciles != nil {
				t.Errorf("%s %s: sum_reconciles = %v, want nil", tc.plant, r.Month, *r.SumReconciles)
			}
			if r.RowClass != genRowClassListedNoData {
				t.Errorf("%s %s: row_class = %q, want %q", tc.plant, r.Month, r.RowClass, genRowClassListedNoData)
			}
			// The trap this class exists for: nepraparse reports the block
			// status as numeric, which does NOT mean the plant reported.
			if r.BlockStatus != "numeric" {
				t.Errorf("%s %s: block_status = %q, want numeric", tc.plant, r.Month, r.BlockStatus)
			}
			if r.InstalledMW == nil || *r.InstalledMW != tc.installed || r.InstalledMWState != "numeric" {
				t.Errorf("%s %s: installed = %v/%q, want %v/numeric", tc.plant, r.Month, r.InstalledMW, r.InstalledMWState, tc.installed)
			}
			if r.DependableMW == nil || *r.DependableMW != tc.dependable {
				t.Errorf("%s %s: dependable = %v, want %v", tc.plant, r.Month, r.DependableMW, tc.dependable)
			}
		}
	}
}

// TestGenMeasuredZeroIsData is the other half. KAPCO holds 1,601 MW of
// installed capacity and published a real 0.00 GWh in all twelve FY2023-24
// months against a published Sum of 0.00 that reconciles. Nulling those would
// delete twelve measurements.
func TestGenMeasuredZeroIsData(t *testing.T) {
	rows := genTestRows(t, "2023-24")
	got := genRowsFor(rows, "Kot Addu Power Company (KAPCO)")
	if len(got) != 12 {
		t.Fatalf("KAPCO rows = %d, want 12", len(got))
	}
	for _, r := range got {
		if r.GWh == nil || *r.GWh != 0 || r.GWhState != "numeric" {
			t.Errorf("KAPCO %s: gwh = %v/%q, want 0/numeric", r.Month, r.GWh, r.GWhState)
		}
		if r.UtilisationPct == nil || *r.UtilisationPct != 0 || r.UtilisationState != "numeric" {
			t.Errorf("KAPCO %s: utilisation = %v/%q, want 0/numeric", r.Month, r.UtilisationPct, r.UtilisationState)
		}
		if r.SumGWhReported == nil || *r.SumGWhReported != 0 || r.SumGWhReportedState != "numeric" {
			t.Errorf("KAPCO %s: sum = %v/%q, want 0/numeric", r.Month, r.SumGWhReported, r.SumGWhReportedState)
		}
		if r.SumReconciles == nil || !*r.SumReconciles {
			t.Errorf("KAPCO %s: sum_reconciles = %v, want true", r.Month, r.SumReconciles)
		}
		if r.RowClass != genRowClassActive {
			t.Errorf("KAPCO %s: row_class = %q, want active", r.Month, r.RowClass)
		}
		if r.InstalledMW == nil || *r.InstalledMW != 1601 {
			t.Errorf("KAPCO %s: installed = %v, want 1601", r.Month, r.InstalledMW)
		}
		if r.DependableMW == nil || *r.DependableMW != 1345 {
			t.Errorf("KAPCO %s: dependable = %v, want 1345", r.Month, r.DependableMW)
		}
		if r.Technology != "THERMAL" || !r.TechnologyKnown || r.Fuel != "RFO/RLNG/HSD" || !r.FuelKnown {
			t.Errorf("KAPCO %s: vocabulary = %q/%q", r.Month, r.Technology, r.Fuel)
		}
	}
	// The pair is only proof together: a 0 that stays 0 and a blank that
	// stays blank, in the same file, at the same grain.
	blank := genRowsFor(rows, "Reshma Power Generation (Private) Limited. (RPGPL)")
	if blank[0].GWh != nil || got[0].GWh == nil {
		t.Fatal("blank-vs-zero pair broken: they must not render the same way")
	}
}

// TestGenStatusRowsKeepCapacity pins the independence of capacity and
// generation nullability. Every one of these plants publishes a real capacity
// while its whole monthly block is a status sentinel.
func TestGenStatusRowsKeepCapacity(t *testing.T) {
	rows := genTestRows(t, "2023-24")
	for _, tc := range []struct {
		plant      string
		class      string
		installed  float64
		dependable float64
	}{
		{"Hub Power Company (HUBCO)", "delicensed", 1292, 1200},
		{"Habibullah Coastal Power Company (PVT) Limited. (HCPC)", "decommissioned", 140, 129},
		{"Kotri Power Station", "delicensed", 174, 120},
		// A MEASURED ZERO CAPACITY, not a blank: Shahdara publishes 0.00 in
		// both capacity columns. installed_mw_state must still be numeric.
		{"Shahdara", "delicensed", 0, 0},
		{"Lakhra Power Generation Company Limited.", "delicensed", 150, 30},
	} {
		got := genRowsFor(rows, tc.plant)
		if len(got) != 12 {
			t.Fatalf("%s: %d rows, want 12", tc.plant, len(got))
		}
		for _, r := range got {
			if r.RowClass != tc.class || r.BlockStatus != tc.class {
				t.Errorf("%s %s: class/block = %q/%q, want %q", tc.plant, r.Month, r.RowClass, r.BlockStatus, tc.class)
			}
			if r.GWh != nil || r.GWhState != tc.class {
				t.Errorf("%s %s: gwh = %v/%q, want nil/%s", tc.plant, r.Month, r.GWh, r.GWhState, tc.class)
			}
			if r.SumReconciles != nil {
				t.Errorf("%s %s: sum_reconciles = %v, want nil", tc.plant, r.Month, *r.SumReconciles)
			}
			if r.InstalledMW == nil || *r.InstalledMW != tc.installed || r.InstalledMWState != "numeric" {
				t.Errorf("%s %s: installed = %v/%q, want %v/numeric", tc.plant, r.Month, r.InstalledMW, r.InstalledMWState, tc.installed)
			}
			if r.DependableMW == nil || *r.DependableMW != tc.dependable {
				t.Errorf("%s %s: dependable = %v, want %v", tc.plant, r.Month, r.DependableMW, tc.dependable)
			}
		}
	}
}

// TestGenRowClassCensus pins the class partition in all three fixture years,
// and pins that every class sums back to the plant count.
//
// MEASURED, and it differs from the spec in one place worth naming: the spec
// asserted active == SumReport(GWh).Eligible as an invariant. It holds in all
// three fixture years, but it is NOT an invariant of the corpus, so the
// shipped assertion is the containment (every eligible plant is active) with
// the residual reported. See TestGenReconcilabilitySplit.
func TestGenRowClassCensus(t *testing.T) {
	for _, tc := range []struct {
		fy   string
		want genRowClassCounts
	}{
		{"2023-24", genRowClassCounts{Active: 118, Delicensed: 12, Decommissioned: 1, ListedNoData: 2}},
		{"2020-21", genRowClassCounts{Active: 105, ExportToKElectric: 3}},
		{"2017-18", genRowClassCounts{Active: 97, ListedNoData: 11}},
	} {
		w, _ := genWorkbook(t, tc.fy)
		got := genCountRowClasses(w)
		if got != tc.want {
			t.Errorf("FY%s row classes = %+v, want %+v", tc.fy, got, tc.want)
		}
		if got.Total() != len(w.Plants) {
			t.Errorf("FY%s classes total %d, want %d plants", tc.fy, got.Total(), len(w.Plants))
		}
		if got.Active != w.CheckSum(nepraparse.FieldGWh, 0).Eligible {
			t.Errorf("FY%s active %d != GWh eligible %d", tc.fy, got.Active, w.CheckSum(nepraparse.FieldGWh, 0).Eligible)
		}
	}
}

// TestGenReconcilabilitySplit pins the invariant that replaced the spec's
// false equality: a plant whose thirteen GWh cells are all numeric MUST be
// classed active. The residual in the other direction is reported, not
// treated as a fault.
func TestGenReconcilabilitySplit(t *testing.T) {
	for _, fy := range []string{"2017-18", "2020-21", "2023-24"} {
		w, _ := genWorkbook(t, fy)
		split := genReconcilability(w)
		if split.EligibleNotActive != 0 {
			t.Errorf("FY%s eligible-but-not-active = %d, want 0", fy, split.EligibleNotActive)
		}
		if split.ActiveNotEligible != 0 {
			t.Errorf("FY%s active-but-not-eligible = %d, want 0 in the fixture years", fy, split.ActiveNotEligible)
		}
		if got := split.ActiveAndEligible; got != genCountRowClasses(w).Active {
			t.Errorf("FY%s active-and-eligible = %d, want %d", fy, got, genCountRowClasses(w).Active)
		}
	}
}

// TestGenFY2020_21ExportSentinel pins step 2 of the row_class rule. These
// three rows carry "Export to K.Electric" in the INSTALLED CAPACITY column and
// an all-blank monthly block, so without step 2 they come out listed_no_data
// and the one thing NEPRA did say about them is lost.
//
// MUTATION-CHECKED: deleting the capacity-sentinel step makes all three read
// listed_no_data and fails this test.
func TestGenFY2020_21ExportSentinel(t *testing.T) {
	rows := genTestRows(t, "2020-21")
	for _, plant := range []string{
		"Tenaga Generasi Ltd.",
		"Hydrochina Dawood Power (Pvt.) Ltd. (HDPPL)",
		"Zephyr Power (Pvt.) Ltd.",
	} {
		got := genRowsFor(rows, plant)
		if len(got) != 12 {
			t.Fatalf("%s: %d rows, want 12", plant, len(got))
		}
		for _, r := range got {
			if r.RowClass != "export_to_k_electric" {
				t.Errorf("%s %s: row_class = %q, want export_to_k_electric", plant, r.Month, r.RowClass)
			}
			if r.InstalledMW != nil || r.InstalledMWState != "export_to_k_electric" {
				t.Errorf("%s %s: installed = %v/%q, want nil/export_to_k_electric", plant, r.Month, r.InstalledMW, r.InstalledMWState)
			}
			// The monthly block is BLANK, not the sentinel: the two live in
			// different columns and must not be conflated.
			if r.GWhState != "not_reported" || r.GWh != nil {
				t.Errorf("%s %s: gwh = %v/%q, want nil/not_reported", plant, r.Month, r.GWh, r.GWhState)
			}
		}
	}
}

// TestGenMetaReproducesCensus pins nepraparse's cell-state census as it
// travels into meta, for all three fixture years.
func TestGenMetaReproducesCensus(t *testing.T) {
	for _, tc := range []struct {
		fy   string
		want genCensusMeta
	}{
		{"2023-24", genCensusMeta{
			MonthlyCellsTotal: 3458, Present: 3068, NotReported: 52, Delicensed: 312,
			Decommissioned: 26, ExportToKElectric: 0, UnknownText: 0,
			MeasuredZeros: 471, ThousandsSeparated: 68,
			NotReportedRows: 2, FullyNotReportedRows: 2, MixedBlankZeroRows: 0, StatusRows: 13,
			CapacityCells: 266, CapacityPresent: 266, CapacityNotReported: 0, CapacityStatus: 0,
			ResidueCells: 1, Balanced: true,
		}},
		{"2020-21", genCensusMeta{
			MonthlyCellsTotal: 2808, Present: 2730, NotReported: 78, Delicensed: 0,
			Decommissioned: 0, ExportToKElectric: 0, UnknownText: 0,
			MeasuredZeros: 540, ThousandsSeparated: 59,
			NotReportedRows: 3, FullyNotReportedRows: 3, MixedBlankZeroRows: 0, StatusRows: 0,
			// The three export sentinels sit in the CAPACITY columns, which
			// is why capacity_status is 3 and the monthly export count is 0.
			CapacityCells: 216, CapacityPresent: 210, CapacityNotReported: 3, CapacityStatus: 3,
			ResidueCells: 1, Balanced: true,
		}},
		{"2017-18", genCensusMeta{
			MonthlyCellsTotal: 2808, Present: 2522, NotReported: 286, Delicensed: 0,
			Decommissioned: 0, ExportToKElectric: 0, UnknownText: 0,
			MeasuredZeros: 437, ThousandsSeparated: 61,
			NotReportedRows: 11, FullyNotReportedRows: 11, MixedBlankZeroRows: 0, StatusRows: 0,
			// The eleven all-blank rows publish NO capacity either: 22 blank
			// capacity cells. The catalogue note says so, because it makes
			// FY2017-18's listed_no_data capacity unmeasured rather than 0.
			CapacityCells: 216, CapacityPresent: 194, CapacityNotReported: 22, CapacityStatus: 0,
			ResidueCells: 1, Balanced: true,
		}},
	} {
		meta := genTestMeta(t, tc.fy)
		if meta.Census != tc.want {
			t.Errorf("FY%s census =\n %+v\nwant\n %+v", tc.fy, meta.Census, tc.want)
		}
		// MixedBlankZeroRows == 0 in every year IS the evidence that blank
		// and zero are two different things upstream and not two spellings
		// of one thing.
		if meta.Census.MixedBlankZeroRows != 0 {
			t.Errorf("FY%s mixes a blank and a zero in one row; the blank-vs-zero distinction may be an artefact", tc.fy)
		}
		if meta.Rows != meta.Plants*12 {
			t.Errorf("FY%s rows %d != plants %d x 12", tc.fy, meta.Rows, meta.Plants)
		}
	}
}

// TestGenMetaStructuralFacts pins the file-shape numbers that must round-trip
// into meta so a consumer can tell one year's artefact from another's.
func TestGenMetaStructuralFacts(t *testing.T) {
	for _, tc := range []struct {
		fy                                                 string
		plants, tableRows, rawCells, width, sepRows, bytes int
	}{
		{"2023-24", 133, 139, 4965, 39, 2, 493187},
		{"2020-21", 108, 114, 4479, 40, 2, 455640},
		{"2017-18", 108, 114, 4479, 40, 2, 426282},
	} {
		meta := genTestMeta(t, tc.fy)
		if meta.Plants != tc.plants || meta.TableRows != tc.tableRows || meta.RawCells != tc.rawCells ||
			meta.PhysicalWidth != tc.width || meta.SeparatorRows != tc.sepRows {
			t.Errorf("FY%s structure = plants %d, rows %d, cells %d, width %d, sep %d; want %d/%d/%d/%d/%d",
				tc.fy, meta.Plants, meta.TableRows, meta.RawCells, meta.PhysicalWidth, meta.SeparatorRows,
				tc.plants, tc.tableRows, tc.rawCells, tc.width, tc.sepRows)
		}
		if meta.Artifact.Bytes != tc.bytes {
			t.Errorf("FY%s decoded bytes = %d, want %d", tc.fy, meta.Artifact.Bytes, tc.bytes)
		}
		if meta.LogicalColumns != 32 || meta.TableCount != 1 || meta.HeaderCellCount != 0 {
			t.Errorf("FY%s logical/tables/th = %d/%d/%d, want 32/1/0", tc.fy, meta.LogicalColumns, meta.TableCount, meta.HeaderCellCount)
		}
		if !meta.SNoContiguous || !meta.NamesUnique {
			t.Errorf("FY%s sno_contiguous/names_unique = %t/%t, want true/true", tc.fy, meta.SNoContiguous, meta.NamesUnique)
		}
		if meta.Charset != "windows-1252" || !meta.CharsetDeclared {
			t.Errorf("FY%s charset = %q declared=%t", tc.fy, meta.Charset, meta.CharsetDeclared)
		}
		if !meta.Schema.Matches || len(meta.Schema.Diff) != 0 || meta.Schema.PctLeafCount != 13 {
			t.Errorf("FY%s schema = %+v", tc.fy, meta.Schema)
		}
		if meta.Schema.BandLabel != "FY "+tc.fy {
			t.Errorf("FY%s band label = %q", tc.fy, meta.Schema.BandLabel)
		}
		// One stray ";" past the logical width in every sampled year. It is
		// reported as a warning and carried in the row's note, never dropped.
		if meta.Census.ResidueCells != 1 || len(meta.Warnings) == 0 {
			t.Errorf("FY%s residue %d warnings %d, want 1 and at least one warning", tc.fy, meta.Census.ResidueCells, len(meta.Warnings))
		}
	}
}

// TestGenColumnOrderProofTravels pins the percent-then-GWh proof. The
// asymmetry IS the proof: read the pair backwards and the two ratios swap.
func TestGenColumnOrderProofTravels(t *testing.T) {
	for _, tc := range []struct {
		fy                                string
		gwhPass, gwhElig, pctPass, pctLig int
	}{
		{"2023-24", 118, 118, 3, 118},
		{"2020-21", 104, 105, 12, 105},
		{"2017-18", 97, 97, 7, 97},
	} {
		meta := genTestMeta(t, tc.fy)
		if !meta.ColumnOrder.PctFirst {
			t.Errorf("FY%s pct_first = false; the monthly fields must not be trusted", tc.fy)
		}
		want := genColumnOrderMeta{
			PctFirst:    true,
			GWh:         genRatio{Passed: tc.gwhPass, Eligible: tc.gwhElig},
			Pct:         genRatio{Passed: tc.pctPass, Eligible: tc.pctLig},
			Explanation: meta.ColumnOrder.Explanation,
		}
		if meta.ColumnOrder != want {
			t.Errorf("FY%s column order = %+v, want %+v", tc.fy, meta.ColumnOrder, want)
		}
	}
}

// TestGenSumMismatchIsReported pins the published arithmetic disagreement that
// must never be silently corrected.
//
// FY2020-21's "(NPPCL) - Balloki" publishes twelve months totalling 5,945.21
// against a Sum of 5,905.65. Both numbers reach the caller: the months in the
// rows, the Sum in sum_gwh_reported, and the disagreement in
// meta.sum_check.mismatches with sum_reconciles false on that plant's rows.
func TestGenSumMismatchIsReported(t *testing.T) {
	meta := genTestMeta(t, "2020-21")
	if meta.SumCheck.Failed != 1 || len(meta.SumCheck.Mismatches) != 1 {
		t.Fatalf("FY2020-21 failed = %d, mismatches = %d, want 1 and 1", meta.SumCheck.Failed, len(meta.SumCheck.Mismatches))
	}
	m := meta.SumCheck.Mismatches[0]
	if m.Plant != "(NPPCL) - Balloki" || m.SNo != 30 {
		t.Errorf("mismatch = sno %d %q, want sno 30 (NPPCL) - Balloki", m.SNo, m.Plant)
	}
	if math.Abs(m.MonthlySum-5945.21) > 0.01 || math.Abs(m.Reported-5905.65) > 0.01 || math.Abs(m.Delta-39.56) > 0.01 {
		t.Errorf("mismatch numbers = months %.2f sum %.2f delta %.2f, want 5945.21 / 5905.65 / +39.56", m.MonthlySum, m.Reported, m.Delta)
	}
	if meta.SumCheck.Eligible != 105 || meta.SumCheck.Passed != 104 {
		t.Errorf("FY2020-21 sum check = %d/%d, want 104/105", meta.SumCheck.Passed, meta.SumCheck.Eligible)
	}
	if meta.SumCheck.Skipped != 3 || meta.SumCheck.SkippedByReason["not_reported"] != 3 {
		t.Errorf("FY2020-21 skipped = %d %v, want 3 all not_reported", meta.SumCheck.Skipped, meta.SumCheck.SkippedByReason)
	}

	rows := genTestRows(t, "2020-21")
	balloki := genRowsFor(rows, "(NPPCL) - Balloki")
	if len(balloki) != 12 {
		t.Fatalf("Balloki rows = %d, want 12", len(balloki))
	}
	for _, r := range balloki {
		if r.SumReconciles == nil || *r.SumReconciles {
			t.Errorf("Balloki %s: sum_reconciles = %v, want false", r.Month, r.SumReconciles)
		}
		// The PUBLISHED Sum, not a recomputed one.
		if r.SumGWhReported == nil || math.Abs(*r.SumGWhReported-5905.65) > 0.001 {
			t.Errorf("Balloki %s: sum_gwh_reported = %v, want the published 5905.65", r.Month, r.SumGWhReported)
		}
	}
	// Every other eligible plant reconciles, so the flag is discriminating
	// rather than uniformly false.
	other := 0
	for _, r := range rows {
		if r.SumReconciles != nil && *r.SumReconciles {
			other++
		}
	}
	if other != 104*12 {
		t.Errorf("reconciling rows = %d, want %d (104 plants x 12)", other, 104*12)
	}
}

// TestGenCapacitySplitIsExhaustive pins that every published megawatt is
// attributable to a class.
//
// MEASURED, AND IT CONTRADICTS THE SPEC BY 0.5 MW. The spec claimed
// published_total 44,686.5 and active 40,625.5. The code produces 44,686.0 and
// 40,625.0, and an independent pass over the fixture's cp1252 bytes — regex
// over <tr>/<td>, no nepraparse involved — sums the 133 numeric capacity cells
// to exactly 44686.0000 and the 13 status rows to exactly 3880.0000. The
// nepraxwalk test the spec cited asserts int(got) == 44686, which truncates
// and so cannot tell the two apart.
func TestGenCapacitySplitIsExhaustive(t *testing.T) {
	meta := genTestMeta(t, "2023-24")
	c := meta.CapacityMW
	for _, tc := range []struct {
		name   string
		got    genMW
		mw     float64
		plants int
	}{
		{"published_total", c.PublishedTotal, 44686, 133},
		{"active", c.Active, 40625, 118},
		{"non_operating", c.NonOperating, 3880, 13},
		{"listed_no_data", c.ListedNoData, 181, 2},
		{"export_to_k_electric", c.ExportToKElectric, 0, 0},
		{"unmodelled_block", c.UnmodelledBlock, 0, 0},
	} {
		if !tc.got.Measured || tc.got.MW == nil {
			t.Fatalf("FY2023-24 %s is unmeasured, want a measurement", tc.name)
		}
		if *tc.got.MW != tc.mw || tc.got.Plants != tc.plants || tc.got.PlantsNotNumeric != 0 {
			t.Errorf("FY2023-24 %s = %.4f MW over %d plants (%d not numeric), want %.1f over %d",
				tc.name, *tc.got.MW, tc.got.Plants, tc.got.PlantsNotNumeric, tc.mw, tc.plants)
		}
	}
	ok, checkable := genCapacityIdentity(c)
	if !checkable || !ok {
		t.Errorf("FY2023-24 capacity identity checkable=%t ok=%t, want true/true", checkable, ok)
	}

	// FY2017-18 is the counter-case, and the reason genMW carries a measured
	// flag at all: its eleven all-blank rows publish NO capacity, so
	// listed_no_data is UNMEASURED over 11 plants rather than 0 MW, and the
	// identity is therefore not checkable. Reporting 0 there would invent a
	// finding ("those plants have no capacity") the source never made.
	m17 := genTestMeta(t, "2017-18")
	if m17.CapacityMW.ListedNoData.Measured || m17.CapacityMW.ListedNoData.MW != nil {
		t.Errorf("FY2017-18 listed_no_data = %+v, want unmeasured with no mw", m17.CapacityMW.ListedNoData)
	}
	if m17.CapacityMW.ListedNoData.PlantsNotNumeric != 11 {
		t.Errorf("FY2017-18 listed_no_data not-numeric plants = %d, want 11", m17.CapacityMW.ListedNoData.PlantsNotNumeric)
	}
	if _, checkable := genCapacityIdentity(m17.CapacityMW); checkable {
		t.Error("FY2017-18 capacity identity must not be checkable while a class is unmeasured")
	}
	if pt := m17.CapacityMW.PublishedTotal; !pt.Measured || pt.MW == nil || *pt.MW != 32391 || pt.Plants != 97 || pt.PlantsNotNumeric != 11 {
		t.Errorf("FY2017-18 published_total = %+v, want a measured 32391 MW over 97 plants with 11 not numeric", pt)
	}

	// FY2020-21's export class is unmeasured for the same reason: the three
	// rows publish the sentinel INSTEAD of a number.
	m21 := genTestMeta(t, "2020-21")
	if e := m21.CapacityMW.ExportToKElectric; e.Measured || e.PlantsNotNumeric != 3 {
		t.Errorf("FY2020-21 export capacity = %+v, want unmeasured over 3 plants", e)
	}
	if pt := m21.CapacityMW.PublishedTotal; pt.MW == nil || *pt.MW != 36902 || pt.Plants != 105 {
		t.Errorf("FY2020-21 published_total = %+v, want 36902 MW over 105 plants", pt)
	}
}

// TestGenAssertionsPassOnEveryFixture runs the shipped completeness block
// against the three committed years. Anything failing here means the
// catalogue's floors and the parser disagree about the published bytes.
func TestGenAssertionsPassOnEveryFixture(t *testing.T) {
	for _, fy := range []string{"2017-18", "2020-21", "2023-24"} {
		meta := genTestMeta(t, fy)
		if len(meta.Assertions) != 10 {
			t.Errorf("FY%s has %d assertions, want 10", fy, len(meta.Assertions))
		}
		if short := genShortfalls(meta.Assertions); len(short) > 0 {
			for _, a := range short {
				t.Errorf("FY%s assertion %q failed: %s", fy, a.Name, a.Detail)
			}
		}
		// An assertion with a floor of 0 must report that it asserts
		// nothing rather than reporting that it passed.
		for _, a := range meta.Assertions {
			if a.Name == "plant_row_floor" && !a.Asserts {
				t.Errorf("FY%s has a committed fixture and must have a plant-row floor", fy)
			}
		}
		if len(meta.Refusals) != 6 {
			t.Errorf("FY%s has %d refusals, want 6", fy, len(meta.Refusals))
		}
	}
}

// TestGenCatalogueIsHonestAboutUnmeasuredFloors pins that the four years with
// no committed fixture are still floor-bearing but flagged as unpinned, and
// that an unmeasured floor asserts nothing rather than passing.
func TestGenCatalogueIsHonestAboutUnmeasuredFloors(t *testing.T) {
	if len(genYears) != 11 {
		t.Fatalf("catalogue has %d years, want 11", len(genYears))
	}
	reachable := genReachableLabels()
	want := []string{"2017-18", "2018-19", "2019-20", "2020-21", "2021-22", "2022-23", "2023-24"}
	if strings.Join(reachable, ",") != strings.Join(want, ",") {
		t.Errorf("reachable = %v, want %v", reachable, want)
	}
	pinned := 0
	for _, y := range genYears {
		if !y.Reachable {
			if y.DecodedBytes != 0 || y.PlantRows != 0 {
				t.Errorf("FY%s is unreachable but carries floors", y.Label)
			}
			continue
		}
		if y.DecodedBytes == 0 || y.PlantRows == 0 || y.StatusRows < 0 {
			t.Errorf("FY%s is reachable but a floor is unmeasured: %+v", y.Label, y)
		}
		if y.PlantRowsPinned {
			pinned++
		}
	}
	if pinned != 3 {
		t.Errorf("%d years are fixture-pinned, want 3", pinned)
	}

	// The floor-of-0 branch must SAY it asserts nothing. Exercised with a
	// catalogue entry carrying no floors, because every shipped year has
	// them.
	w, body := genWorkbook(t, "2023-24")
	rows := genRows(w, genTestURL)
	blank := genYear{Label: "9999-00", Reachable: true, StatusRows: -1}
	meta := genBuildMeta(w, blank, "plant-month", len(rows), body, genTestURL, "text/html", genRowsJSON(rows))
	for _, name := range []string{"plant_row_floor", "decoded_bytes_floor", "status_rows_year_bounded"} {
		var found bool
		for _, a := range meta.Assertions {
			if a.Name != name {
				continue
			}
			found = true
			if a.Asserts {
				t.Errorf("%s asserts against an unmeasured floor", name)
			}
			if a.Expected != nil {
				t.Errorf("%s carries an expected value with no floor: %v", name, *a.Expected)
			}
			if !strings.Contains(a.Detail, "NOT MEASURED") && !strings.Contains(a.Detail, "NO ") {
				t.Errorf("%s does not say it measured nothing: %q", name, a.Detail)
			}
		}
		if !found {
			t.Errorf("assertion %q missing", name)
		}
	}
	if short := genShortfalls(meta.Assertions); len(short) > 0 {
		t.Errorf("an unmeasured floor must not count as a shortfall: %+v", short)
	}
}

// TestGenSheetPathPreservesUpstreamEncoding pins the two things about this URL
// that look like bugs and are not, plus the reason it is not built with
// fmt.Sprintf.
func TestGenSheetPathPreservesUpstreamEncoding(t *testing.T) {
	got := genSheetPath("2023-24")
	want := "/publications/State%20of%20Industry%20Reports/Detail%20of%20Generation/" +
		"List%20of%20Companies%20Genenration%20wise%202023-24_files/sheet001.htm"
	if got != want {
		t.Errorf("genSheetPath =\n %s\nwant\n %s", got, want)
	}
	if !strings.Contains(got, "Genenration") {
		t.Error("the upstream typo Genenration is load-bearing; the correctly spelled path 404s")
	}
	if strings.Contains(got, "FY2023-24") || strings.Contains(got, "%2520") || strings.Contains(got, "%2F") {
		t.Errorf("path was re-encoded or FY-prefixed: %s", got)
	}
	// MEASURED: fmt.Sprintf on this template reinterprets NINE of its ten
	// %20 sequences as format verbs — %20o, %20I, %20R, %20o, %20G, %20o,
	// %20C, %20G, %20w — and returns
	// "/publications/State%!o(string=             2023-24)f%!I(MISSING)ndustry%!R(MISSING)eports/...",
	// with the year padded into an octal verb and the trailing %s left
	// unfilled because the argument was already consumed. go vet flags the
	// first one and nothing about the other eight. That is why the
	// substitution is strings.ReplaceAll on a {fy} token.
	if strings.Count(genSheetPathTemplate, "%20") != 10 || !strings.Contains(genSheetPathTemplate, "{fy}") {
		t.Errorf("template lost its pre-encoded spaces or its {fy} token: %s", genSheetPathTemplate)
	}
	if strings.Contains(genSheetPathTemplate, "%s") {
		t.Error("the template must not carry a Sprintf verb: the pre-encoded %20s would be reinterpreted")
	}
}
