// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// EVERY NUMBER ASSERTED HERE WAS MEASURED by running this code over the
// committed fixtures, not copied from a document. Where a research figure
// disagreed with what the parser produces, the parser wins and the
// supersession is recorded in the comment beside the assertion.

package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraxwalk"
)

// capacityFixture returns one workbook's UNDECODED bytes.
//
// The fixtures are byte copies of the live sheets captured on 2026-09-10:
// workbook-fy2023-24.htm.gz decodes to 493,187 bytes with md5
// 044cc6caaa71d0c17502892d6f275f75, byte-identical to a live fetch made the
// same day. They are committed under internal/cli/testdata rather than read
// across from internal/nepraparse/testdata, matching the events convention.
func capacityFixture(t *testing.T, fy string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "workbook-fy"+fy+".htm.gz"))
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

func capacityParse(t *testing.T, fy string) (*nepraparse.Workbook, []byte) {
	t.Helper()
	body := capacityFixture(t, fy)
	w, err := nepraparse.ParseWorkbook(body, fy)
	if err != nil {
		t.Fatalf("ParseWorkbook(FY%s): %v", fy, err)
	}
	return w, body
}

func capacitySurface(t *testing.T, fy string) capacityAsOfSurface {
	t.Helper()
	for _, s := range capacityAsOfCatalogue {
		if s.FiscalYear == fy {
			return s
		}
	}
	t.Fatalf("no catalogue surface for FY%s", fy)
	return capacityAsOfSurface{}
}

// mustMW fails when a sum is unmeasured, so a test can never silently
// compare against a zero that means "nothing measured this".
func mustMW(t *testing.T, label string, c *capacityMW) (float64, int) {
	t.Helper()
	if c == nil {
		t.Fatalf("%s: no sum at all", label)
	}
	n, ok := c.Float64()
	if !ok {
		t.Fatalf("%s: sum is UNMEASURED, so there is no number to compare", label)
	}
	return n, c.Plants()
}

func capacityBucketOf(t *testing.T, rep capacityReport, key string) capacityBucket {
	t.Helper()
	for _, b := range rep.Buckets {
		if b.Key == key {
			return b
		}
	}
	t.Fatalf("no bucket %q in %d buckets", key, len(rep.Buckets))
	return capacityBucket{}
}

// ---------------------------------------------------------------------------

// TestCapacityStatusFourMembers pins the partition that is this command's
// whole reason to exist. MUTATION: replace capacityStatusOf's all-blank
// branch with `return capStatusActive` and both the counts and the names
// fail.
func TestCapacityStatusFourMembers(t *testing.T) {
	w, _ := capacityParse(t, "2023-24")
	counts := map[capacityStatus]int{}
	var noData []string
	for _, p := range w.Plants {
		st := capacityStatusOf(p)
		counts[st]++
		if st == capStatusListedNoData {
			noData = append(noData, p.Name)
		}
	}
	want := map[capacityStatus]int{
		capStatusActive: 118, capStatusDelicensed: 12,
		capStatusDecommissioned: 1, capStatusListedNoData: 2,
	}
	for st, n := range want {
		if counts[st] != n {
			t.Errorf("FY2023-24 %s = %d plants, want %d", st, counts[st], n)
		}
	}
	if counts[capStatusUndetermined] != 0 {
		t.Errorf("FY2023-24 status_undetermined = %d, want 0", counts[capStatusUndetermined])
	}
	wantNames := []string{
		"Reshma Power Generation (Private) Limited. (RPGPL)",
		"Gulf Powergen (Private) Limited. (GPPL)",
	}
	if strings.Join(noData, "|") != strings.Join(wantNames, "|") {
		t.Errorf("listed_no_data plants = %q, want %q", noData, wantNames)
	}
}

// TestCapacityInstalledByStatusFY2324 uses EXACT float comparison, not a
// tolerance: every FY2023-24 capacity cell is a plain integer.
//
// SUPERSEDES the research's 40,614 MW active and 44,675 MW total. Measured
// here over the same 118 active rows: 40,625.00 and 44,686.00, an 11.00 MW
// difference the research's own note flags as "my derivation, not a
// NEPRA-published figure".
func TestCapacityInstalledByStatusFY2324(t *testing.T) {
	w, _ := capacityParse(t, "2023-24")
	rep, err := capacityGroupLive(w, "status")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		key    string
		mw     float64
		plants int
	}{
		{"active", 40625.00, 118},
		{"delicensed", 3740.00, 12},
		{"decommissioned", 140.00, 1},
		{"listed_no_data", 181.00, 2},
	} {
		b := capacityBucketOf(t, rep, c.key)
		mw, n := mustMW(t, c.key+" installed", b.Installed)
		if mw != c.mw || n != c.plants {
			t.Errorf("%s installed = %.2f over %d plants, want %.2f over %d", c.key, mw, n, c.mw, c.plants)
		}
		switch {
		case b.Plants == nil:
			t.Errorf("%s bucket carries no plants count", c.key)
		case *b.Plants != c.plants:
			t.Errorf("%s bucket plants = %d, want %d", c.key, *b.Plants, c.plants)
		}
	}
	mw, n := mustMW(t, "total installed", &rep.Totals.Installed)
	if mw != 44686.00 || n != 133 {
		t.Errorf("total installed = %.2f over %d, want 44686.00 over 133", mw, n)
	}
}

// TestCapacityDependableByStatusFY2324 exists because the SKILL.md promise
// names DEPENDABLE capacity and no figure for it exists anywhere in the
// research: it is measurable only from column 5.
func TestCapacityDependableByStatusFY2324(t *testing.T) {
	w, _ := capacityParse(t, "2023-24")
	rep, err := capacityGroupLive(w, "status")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		key string
		mw  float64
	}{
		{"active", 37929.00},
		{"delicensed", 2735.00},
		{"decommissioned", 129.00},
		{"listed_no_data", 181.00},
	} {
		mw, _ := mustMW(t, c.key+" dependable", capacityBucketOf(t, rep, c.key).Dependable)
		if mw != c.mw {
			t.Errorf("%s dependable = %.2f, want %.2f", c.key, mw, c.mw)
		}
	}
	mw, _ := mustMW(t, "total dependable", rep.Totals.Dependable)
	if mw != 40974.00 {
		t.Errorf("total dependable = %.2f, want 40974.00", mw)
	}
}

// TestCapacityPartitionCloses recomputes the identity FROM THE REPORT rather
// than restating literals, so it catches a bucket silently dropping or
// double-counting a plant in any year.
func TestCapacityPartitionCloses(t *testing.T) {
	for _, fy := range []string{"2017-18", "2018-19", "2019-20", "2020-21", "2021-22", "2022-23", "2023-24"} {
		w, _ := capacityParse(t, fy)
		rep, err := capacityGroupLive(w, "status")
		if err != nil {
			t.Fatal(err)
		}
		var inst, dep float64
		var plants int
		for _, b := range rep.Buckets {
			if b.Plants != nil {
				plants += *b.Plants
			}
			if n, ok := b.Installed.Float64(); ok {
				inst += n
			}
			if n, ok := b.Dependable.Float64(); ok {
				dep += n
			}
		}
		wantInst, wantInstN := mustMW(t, fy+" total installed", &rep.Totals.Installed)
		wantDep, _ := mustMW(t, fy+" total dependable", rep.Totals.Dependable)
		if inst != wantInst {
			t.Errorf("FY%s installed members sum to %.2f, total says %.2f", fy, inst, wantInst)
		}
		if dep != wantDep {
			t.Errorf("FY%s dependable members sum to %.2f, total says %.2f", fy, dep, wantDep)
		}
		if plants != rep.Totals.PlantRows || plants != len(w.Plants) {
			t.Errorf("FY%s members hold %d plants, totals say %d, workbook has %d",
				fy, plants, rep.Totals.PlantRows, len(w.Plants))
		}
		if wantInstN > len(w.Plants) {
			t.Errorf("FY%s installed sum claims %d plants over a %d-row file", fy, wantInstN, len(w.Plants))
		}
	}
}

// TestCapacityAssertionsAgainstCensus checks the identities against
// nepraparse's INDEPENDENT counters, per member as well as in total: a total
// of 13 non-operating rows passes unchanged if delicensed and decommissioned
// are swapped, so each is pinned against its own cell count.
func TestCapacityAssertionsAgainstCensus(t *testing.T) {
	w, body := capacityParse(t, "2023-24")
	rep, err := capacityGroupLive(w, "status")
	if err != nil {
		t.Fatal(err)
	}
	a := capacityAssertLive(w, len(body), capacitySurface(t, "2023-24"), rep)
	if len(a.Failed) != 0 {
		t.Fatalf("FY2023-24 assertions failed: %v", a.Failed)
	}
	c := w.Census
	if c.Rows != 133 || c.StatusRows != 13 || c.FullyNotReportedRows != 2 || c.MixedBlankZeroRows != 0 {
		t.Errorf("Census = {Rows:%d StatusRows:%d FullyNotReported:%d MixedBlankZero:%d}, want {133 13 2 0}",
			c.Rows, c.StatusRows, c.FullyNotReportedRows, c.MixedBlankZeroRows)
	}
	if c.CapacityCells != 266 || c.CapacityPresent != 266 || c.CapacityNotReported != 0 || c.CapacityStatus != 0 {
		t.Errorf("Census capacity = {%d cells, %d present, %d not-reported, %d status}, want {266 266 0 0}",
			c.CapacityCells, c.CapacityPresent, c.CapacityNotReported, c.CapacityStatus)
	}
	if !c.Balanced() {
		t.Error("Census.Balanced() = false")
	}
	// Per-member, against a counter this command does not compute.
	if got := c.Delicensed / nepraparse.MonthlyCells; got != 12 {
		t.Errorf("Census.Delicensed = %d cells = %d rows, want 12", c.Delicensed, got)
	}
	if got := c.Decommissioned / nepraparse.MonthlyCells; got != 1 {
		t.Errorf("Census.Decommissioned = %d cells = %d rows, want 1", c.Decommissioned, got)
	}
	for _, name := range []string{"census_balanced", "status_rows_match_census", "delicensed_rows_match_census",
		"decommissioned_rows_match_census", "fully_not_reported_rows_match_census",
		"buckets_partition_plant_rows", "no_unmodelled_status_sentinels",
		"installed_plus_dependable_cells_accounted"} {
		blob, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(blob, []byte(`"`+name+`":true`)) {
			t.Errorf("assertion %q is not true in %s", name, blob)
		}
	}
}

// TestCapacityUnmodelledSentinelIsNotActive is the FY2022-23 finding.
//
// MEASURED: Reshma Power Generation and Gulf Powergen each carry
// `<td colspan=26>DELICENSE</td>` — the upstream typo, no trailing D — in a
// file that spells the other eleven "DELICENSED". nepraparse.statusFor
// matches "DELICENSED" only, so blockStatus() returns StateNumeric and a
// naive read calls both plants ACTIVE, putting 181.00 MW of delicensed
// capacity into the active total. They must land in status_undetermined
// instead, with the verbatim text, and the assertion must FAIL loudly.
//
// MUTATION: return capStatusActive whenever numeric == 0 && blank != cells
// and active becomes 118 plants / 41,948.00 MW and the assertion goes quiet.
func TestCapacityUnmodelledSentinelIsNotActive(t *testing.T) {
	w, body := capacityParse(t, "2022-23")
	rep, err := capacityGroupLive(w, "status")
	if err != nil {
		t.Fatal(err)
	}
	b := capacityBucketOf(t, rep, "status_undetermined")
	if b.Plants == nil || *b.Plants != 2 {
		t.Fatalf("status_undetermined plants = %v, want 2", b.Plants)
	}
	if mw, _ := mustMW(t, "undetermined installed", b.Installed); mw != 181.00 {
		t.Errorf("status_undetermined installed = %.2f, want 181.00", mw)
	}
	if got := strings.Join(b.UnmodelledCellText, "|"); got != "DELICENSE" {
		t.Errorf("unmodelled_cell_text = %q, want [DELICENSE]", b.UnmodelledCellText)
	}
	if b.OperatingKnown == nil || *b.OperatingKnown {
		t.Errorf("status_undetermined operating_known = %v, want false", b.OperatingKnown)
	}
	if b.Operating != nil {
		t.Error("status_undetermined must OMIT `operating`: an absent value must not read as not-operating")
	}
	// The active bucket must NOT hold those 181.00 MW.
	mw, n := mustMW(t, "FY2022-23 active", capacityBucketOf(t, rep, "active").Installed)
	if mw != 41767.00 || n != 116 {
		t.Errorf("FY2022-23 active = %.2f over %d plants, want 41767.00 over 116 "+
			"(41948.00 over 118 is the wrong answer that includes the DELICENSE pair)", mw, n)
	}
	if rep.StatusEnumComplete {
		t.Error("status_enum_complete must be false when a fifth member fired")
	}
	if got := strings.Join(rep.StatusEnum, ","); got != "active,delicensed,decommissioned,listed_no_data,status_undetermined" {
		t.Errorf("status_enum = %q", got)
	}
	// The incompleteness must be reported under EVERY grouping, not only
	// --by status: a fifth member hidden in a nested split while
	// status_enum_complete says true would be the worst of both.
	for _, by := range []string{"technology", "system"} {
		other, gerr := capacityGroupLive(w, by)
		if gerr != nil {
			t.Fatal(gerr)
		}
		if other.StatusEnumComplete {
			t.Errorf("--by %s must also report status_enum_complete false for FY2022-23", by)
		}
		if len(other.StatusEnum) != 5 {
			t.Errorf("--by %s status_enum = %v, want 5 members", by, other.StatusEnum)
		}
		oa := capacityAssertLive(w, len(body), capacitySurface(t, "2022-23"), other)
		if len(oa.Failed) != 1 {
			t.Errorf("--by %s must carry the same single failure, got %v", by, oa.Failed)
		}
	}

	a := capacityAssertLive(w, len(body), capacitySurface(t, "2022-23"), rep)
	if a.NoUnmodelledStatusSentinels == nil || *a.NoUnmodelledStatusSentinels {
		t.Error("no_unmodelled_status_sentinels must be false for FY2022-23")
	}
	if len(a.Failed) != 1 || !strings.Contains(a.Failed[0], "DELICENSE") {
		t.Errorf("expected exactly one failure naming DELICENSE, got %v", a.Failed)
	}
	// And the identities that CANNOT catch it must still pass, which is the
	// point: agreement with the Census is not confirmation here.
	if a.StatusRowsMatchCensus == nil || !*a.StatusRowsMatchCensus {
		t.Error("status_rows_match_census should still pass; it shares the blind spot")
	}
}

// TestCapacityStrictEscalates mirrors nepra_events.go: the payload is
// written FIRST and is never suppressed, and --strict only changes the exit.
func TestCapacityStrictEscalates(t *testing.T) {
	w, body := capacityParse(t, "2023-24")
	rep, err := capacityGroupLive(w, "status")
	if err != nil {
		t.Fatal(err)
	}
	// Inject a floor above the fixture's measured 133 rows / 493,187 bytes.
	inflated := capacityAsOfSurface{
		AsOf: "2024-06-30", FiscalYear: "2023-24", State: "reachable",
		PlantRowFloor: 999, DecodedByteFloor: 99999999, FloorsMeasured: true,
	}
	a := capacityAssertLive(w, len(body), inflated, rep)
	if len(a.Failed) != 2 {
		t.Fatalf("expected 2 floor failures, got %v", a.Failed)
	}
	results, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	meta := capacityMeta(inflated, "status", "live", rep, a)

	for _, strict := range []bool{false, true} {
		var out, errBuf bytes.Buffer
		cmd := &cobra.Command{Use: "capacity"}
		cmd.SetOut(&out)
		cmd.SetErr(&errBuf)
		emitErr := capacityEmit(cmd, &rootFlags{asJSON: true}, meta, results, rep, a, strict)
		if !bytes.Contains(out.Bytes(), []byte("40625")) {
			t.Errorf("strict=%v: the payload must ALWAYS be written; stdout was %q", strict, out.String())
		}
		if !strings.Contains(errBuf.String(), "COMPLETENESS") {
			t.Errorf("strict=%v: the failure must be reported on stderr", strict)
		}
		if strict && emitErr == nil {
			t.Error("--strict must return a non-nil error after writing the payload")
		}
		if !strict && emitErr != nil {
			t.Errorf("without --strict the command must succeed, got %v", emitErr)
		}
	}
}

// TestCapacityTechnologyStatusMatrix pins the nine published technology
// strings and, above all, that "Coal" and "THERMAL- COAL" stay APART. In
// FY2023-24 they carry materially different facts: "Coal" is one DELICENSED
// plant at 150.00 MW installed / 30.00 dependable, "THERMAL- COAL" is 8
// active plants at 7,260.00 MW.
func TestCapacityTechnologyStatusMatrix(t *testing.T) {
	w, _ := capacityParse(t, "2023-24")
	rep, err := capacityGroupLive(w, "technology")
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Buckets) != 9 {
		t.Fatalf("technology buckets = %d, want 9", len(rep.Buckets))
	}
	for _, b := range rep.Buckets {
		if b.Known == nil || !*b.Known {
			t.Errorf("technology %q known = %v, want true (all 9 are in nepraparse.KnownTechnologies)", b.Key, b.Known)
		}
		if !nepraparse.KnownTechnology(b.Key) {
			t.Errorf("technology key %q is not a member of nepraparse.KnownTechnologies", b.Key)
		}
	}

	thermal := capacityBucketOf(t, rep, "THERMAL")
	if mw, n := mustMW(t, "THERMAL installed", thermal.Installed); mw != 20346.00 || n != 46 {
		t.Errorf("THERMAL installed = %.2f over %d, want 20346.00 over 46", mw, n)
	}
	if mw, _ := mustMW(t, "THERMAL dependable", thermal.Dependable); mw != 17399.00 {
		t.Errorf("THERMAL dependable = %.2f, want 17399.00", mw)
	}
	wantSplit := map[string][3]float64{
		// key -> installed, dependable, plants
		"active":         {16435.00, 14384.00, 32},
		"delicensed":     {3590.00, 2705.00, 11},
		"decommissioned": {140.00, 129.00, 1},
		"listed_no_data": {181.00, 181.00, 2},
	}
	seen := 0
	for _, n := range thermal.ByStatus {
		want, ok := wantSplit[n.Key]
		if !ok {
			continue
		}
		seen++
		inst, plants := mustMW(t, "THERMAL/"+n.Key+" installed", n.Installed)
		dep, _ := mustMW(t, "THERMAL/"+n.Key+" dependable", n.Dependable)
		if inst != want[0] || dep != want[1] || float64(plants) != want[2] {
			t.Errorf("THERMAL/%s = %.2f / %.2f over %d, want %.2f / %.2f over %.0f",
				n.Key, inst, dep, plants, want[0], want[1], want[2])
		}
	}
	if seen != 4 {
		t.Errorf("THERMAL by_status covered %d of the 4 members", seen)
	}

	// The test that catches a well-meaning normaliser merging them.
	coal := capacityBucketOf(t, rep, "Coal")
	thermalCoal := capacityBucketOf(t, rep, "THERMAL- COAL")
	if mw, n := mustMW(t, "Coal installed", coal.Installed); mw != 150.00 || n != 1 {
		t.Errorf("Coal installed = %.2f over %d, want 150.00 over 1", mw, n)
	}
	if mw, _ := mustMW(t, "Coal dependable", coal.Dependable); mw != 30.00 {
		t.Errorf("Coal dependable = %.2f, want 30.00", mw)
	}
	if mw, n := mustMW(t, "THERMAL- COAL installed", thermalCoal.Installed); mw != 7260.00 || n != 8 {
		t.Errorf("THERMAL- COAL installed = %.2f over %d, want 7260.00 over 8", mw, n)
	}
	for _, n := range coal.ByStatus {
		switch n.Key {
		case "delicensed":
			if mw, _ := mustMW(t, "Coal/delicensed", n.Installed); mw != 150.00 {
				t.Errorf("Coal/delicensed installed = %.2f, want 150.00", mw)
			}
		case "active":
			if n.Plants == nil || *n.Plants != 0 {
				t.Errorf("Coal/active plants = %v, want 0", n.Plants)
			}
			if _, measured := n.Installed.Float64(); measured {
				t.Error("Coal/active must be UNMEASURED, never a measured 0 MW")
			}
		}
	}
	// The nine rows must sum to the whole file.
	var sum float64
	for _, b := range rep.Buckets {
		if n, ok := b.Installed.Float64(); ok {
			sum += n
		}
	}
	if sum != 44686.00 {
		t.Errorf("technology rows sum to %.2f, want 44686.00", sum)
	}
}

// TestCapacityExportToKElectricIsNotZero is the null-discipline test for a
// bucket that has plants but no numbers.
//
// MUTATION: derive the bucket's `measured` flag from plants-in-bucket
// instead of plants-with-a-number and this fails, because the bucket then
// reports a measured 0 MW for three plants.
func TestCapacityExportToKElectricIsNotZero(t *testing.T) {
	w, _ := capacityParse(t, "2020-21")
	rep, err := capacityGroupLive(w, "system")
	if err != nil {
		t.Fatal(err)
	}
	exp := capacityBucketOf(t, rep, "export_to_k_electric")
	if exp.Plants == nil || *exp.Plants != 3 {
		t.Fatalf("export_to_k_electric plants = %v, want 3", exp.Plants)
	}
	if _, measured := exp.Installed.Float64(); measured {
		t.Error("export_to_k_electric installed must be UNMEASURED: the sentinel occupies the capacity column")
	}
	if _, measured := exp.Dependable.Float64(); measured {
		t.Error("export_to_k_electric dependable must be UNMEASURED")
	}
	blob, err := json.Marshal(exp)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(blob, &decoded); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"installed", "dependable"} {
		sum, ok := decoded[key].(map[string]any)
		if !ok {
			t.Fatalf("%s is not an object in %s", key, blob)
		}
		if _, present := sum["mw"]; present {
			t.Errorf("%s must carry NO `mw` key at all when unmeasured: %s", key, blob)
		}
	}
	if exp.NotReported != 3 || exp.StatusCell != 3 {
		t.Errorf("export bucket cell counts = %d not-reported / %d status, want 3 / 3 "+
			"(the installed cell holds the sentinel, the dependable cell is blank)", exp.NotReported, exp.StatusCell)
	}
	// The three plants and their names.
	var names []string
	for _, p := range w.Plants {
		if p.InstalledCapacity.State() == nepraparse.StateExportToKElectric {
			names = append(names, p.Name)
		}
	}
	want := "Tenaga Generasi Ltd.|Hydrochina Dawood Power (Pvt.) Ltd. (HDPPL)|Zephyr Power (Pvt.) Ltd."
	if strings.Join(names, "|") != want {
		t.Errorf("export rows = %q, want %q", names, want)
	}
	// cppag is a WHOLE-FILE attribution over every row, and the export
	// bucket is a subset of it, not a sibling in a partition.
	cppag := capacityBucketOf(t, rep, "cppag")
	if cppag.Plants == nil || *cppag.Plants != len(w.Plants) {
		t.Errorf("cppag plants = %v, want %d", cppag.Plants, len(w.Plants))
	}
	// And the K-Electric entry carries no numeric key at all.
	kel := capacityBucketOf(t, rep, "k_electric_own_fleet")
	if kel.Installed != nil || kel.Dependable != nil || kel.Plants != nil {
		t.Error("k_electric_own_fleet must carry NO installed/dependable/plants key: absent is not zero")
	}
	if kel.State != string(nepraxwalk.StatusAbsentFromDataset) {
		t.Errorf("k_electric_own_fleet state = %q", kel.State)
	}
	if kel.AbsenceNote == "" || len(kel.ProbeTokens) != 7 || kel.Caveat == "" {
		t.Errorf("k_electric_own_fleet must carry the verbatim absence note, 7 probe tokens and the KEL caveat; got %d tokens", len(kel.ProbeTokens))
	}
}

// TestCapacityLocalPathDeclaresItsGap pins the offline degradation precisely
// so that nobody later mistakes 40,806.00 MW for active capacity. It is
// 181.00 MW ABOVE the live active figure BY CONSTRUCTION: Reshma Power and
// Gulf Powergen both carry block_status "numeric" in crosswalk.json.
func TestCapacityLocalPathDeclaresItsGap(t *testing.T) {
	rep, err := capacityGroupLocal("2023-24", "status")
	if err != nil {
		t.Fatal(err)
	}
	if rep.StatusEnumComplete {
		t.Error("the offline path must report status_enum_complete = false")
	}
	if got := strings.Join(rep.StatusEnum, ","); got != "reporting_or_silent,delicensed,decommissioned" {
		t.Errorf("offline status_enum = %q", got)
	}
	if rep.StatusEnumGap == nil {
		t.Fatal("the offline path must carry a status_enum_gap")
	}
	if rep.StatusEnumGap.MissingMember != "listed_no_data" || rep.StatusEnumGap.Derivable {
		t.Errorf("status_enum_gap = %+v, want listed_no_data / derivable false", *rep.StatusEnumGap)
	}
	for _, want := range []string{"Reshma Power Generation", "Gulf Powergen", "181.00 MW", "block_status"} {
		if !strings.Contains(rep.StatusEnumGap.Reason, want) {
			t.Errorf("status_enum_gap reason must name %q", want)
		}
	}
	b := capacityBucketOf(t, rep, "reporting_or_silent")
	mw, n := mustMW(t, "reporting_or_silent", b.Installed)
	// 40,806.00 = 40,625.00 live active + 181.00 of no-data capacity that
	// the crosswalk cannot split out. It is NOT active capacity.
	if mw != 40806.00 || n != 120 {
		t.Errorf("reporting_or_silent = %.2f over %d plants, want 40806.00 over 120", mw, n)
	}
	if b.Operating != nil {
		t.Error("reporting_or_silent must OMIT `operating`: the bucket mixes reporting and silent plants")
	}
	// Dependable is ABSENT offline, not zero.
	if b.Dependable != nil {
		t.Error("the offline path must emit NO dependable key at all")
	}
	if b.DependableGap == "" || rep.Totals.DependableGap == "" {
		t.Error("the absent dependable column must be explained, not silently missing")
	}
	if mw, _ := mustMW(t, "offline delicensed", capacityBucketOf(t, rep, "delicensed").Installed); mw != 3740.00 {
		t.Errorf("offline delicensed = %.2f, want 3740.00", mw)
	}
	if mw, _ := mustMW(t, "offline decommissioned", capacityBucketOf(t, rep, "decommissioned").Installed); mw != 140.00 {
		t.Errorf("offline decommissioned = %.2f, want 140.00", mw)
	}
	mw, n = mustMW(t, "offline total", &rep.Totals.Installed)
	if mw != 44686.00 || n != 133 {
		t.Errorf("offline total = %.2f over %d, want 44686.00 over 133", mw, n)
	}
	if rep.Totals.Dependable != nil {
		t.Error("offline totals must carry no dependable key")
	}
	// The offline ledger cannot derive an active figure and must say so
	// rather than emit one.
	for _, r := range rep.Reconciliation {
		if r.Label == capacityLedgerWorkbookActive {
			if r.State != "unavailable" || r.InstalledMW != nil || r.PublishedAs != "" {
				t.Errorf("offline workbook_active = %+v, want state unavailable with no number and no published_as", r)
			}
		}
	}
}

// TestCapacityLocalRefusesUnobservedFY: a zero-megawatt report for a year
// the crosswalk never observed would be a fabrication, so it is an error.
// The companion assertion protects the fix-G guarantee underneath it.
func TestCapacityLocalRefusesUnobservedFY(t *testing.T) {
	if _, err := capacityGroupLocal("2020-21", "status"); err == nil {
		t.Fatal("capacityGroupLocal(2020-21) must refuse: FY2020-21 is held out of the crosswalk")
	} else {
		for _, want := range []string{"2017-18", "2023-24", "2020-21"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("the refusal must name %q; got %v", want, err)
			}
		}
	}
	if got := strings.Join(nepraxwalk.ObservedFYs(), ","); got != "2017-18,2023-24" {
		t.Errorf("nepraxwalk.ObservedFYs() = %q, want 2017-18,2023-24", got)
	}
	if got := strings.Join(nepraxwalk.HeldOutFYs(), ","); got != "2020-21" {
		t.Errorf("nepraxwalk.HeldOutFYs() = %q, want 2020-21", got)
	}
	// fix-G: every sum for an unobserved year must be UNMEASURED, never 0.
	cov := nepraxwalk.Coverage("2020-21", nil)
	for label, sum := range map[string]nepraxwalk.MWSum{
		"CapacityReported":     cov.CapacityReported,
		"CapacityOperating":    cov.CapacityOperating,
		"CapacityNonOperating": cov.CapacityNonOperating,
	} {
		if _, measured := sum.Float64(); measured {
			t.Errorf("Coverage(2020-21).%s must be unmeasured for a year with no observations", label)
		}
	}
}

// TestCapacityAsOfValidation asserts the exit CODES, not merely that an
// error happened. Exit 2 means "this source cannot be asked that"; exit 3
// means "fair question, no document".
func TestCapacityAsOfValidation(t *testing.T) {
	for _, c := range []struct {
		asOf     string
		wantFY   string
		wantCode int
		mustSay  []string
	}{
		{asOf: "2024-06-30", wantFY: "2023-24"},
		{asOf: "2018-06-30", wantFY: "2017-18"},
		{asOf: "2021-06-30", wantFY: "2020-21"},
		{asOf: "2023-06-30", wantFY: "2022-23"},
		{asOf: "2024-03-15", wantCode: 2, mustSay: []string{"no in-year capacity series"}},
		{asOf: "2024-06-31", wantCode: 2, mustSay: []string{"not a calendar date"}},
		{asOf: "June 2024", wantCode: 2, mustSay: []string{"not a calendar date"}},
		{asOf: "", wantCode: 2, mustSay: []string{"--as-of is required"}},
		{asOf: "2016-06-30", wantCode: 2, mustSay: []string{"outside the catalogued range"}},
		{asOf: "2025-06-30", wantCode: 3, mustSay: []string{"SIR Data 2025.htm", "2023-24", "37c27f4decfbad45c80078a997f13c19"}},
		{asOf: "2017-06-30", wantCode: 3, mustSay: []string{"FY2017-18"}},
		{asOf: "2026-06-30", wantCode: 3, mustSay: []string{"no workbook is published"}},
	} {
		surface, err := capacityResolveAsOf(c.asOf)
		if c.wantCode == 0 {
			if err != nil {
				t.Errorf("--as-of %q: unexpected error %v", c.asOf, err)
				continue
			}
			if surface.FiscalYear != c.wantFY {
				t.Errorf("--as-of %q -> FY%s, want FY%s", c.asOf, surface.FiscalYear, c.wantFY)
			}
			continue
		}
		if err == nil {
			t.Errorf("--as-of %q: expected exit %d, got no error", c.asOf, c.wantCode)
			continue
		}
		var typed *cliError
		if !errors.As(err, &typed) {
			t.Errorf("--as-of %q: error is not a typed cliError: %v", c.asOf, err)
			continue
		}
		if typed.code != c.wantCode {
			t.Errorf("--as-of %q: exit %d, want %d (%v)", c.asOf, typed.code, c.wantCode, err)
		}
		for _, want := range c.mustSay {
			// Case-insensitive: the messages SHOUT the load-bearing clause.
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(want)) {
				t.Errorf("--as-of %q: message must contain %q; got %v", c.asOf, want, err)
			}
		}
	}
}

// TestCapacityPassesFYToParseWorkbook is the decoy defence. `SIR Data
// 2025.htm` returns HTTP 200 and frames the FY2023-24 sheet, so a
// filename-driven fetch would republish FY2023-24 under a 2025 heading.
// A NON-EMPTY fy makes the parser cross-check against the header band.
func TestCapacityPassesFYToParseWorkbook(t *testing.T) {
	body := capacityFixture(t, "2020-21")
	if _, err := nepraparse.ParseWorkbook(body, "2023-24"); !errors.Is(err, nepraparse.ErrFiscalYearMismatch) {
		t.Fatalf("claiming FY2023-24 over FY2020-21 bytes must be ErrFiscalYearMismatch, got %v", err)
	}
	// And the command hands the fiscal year in, rather than "".
	src, err := os.ReadFile("capacity.go")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(src, []byte("nepraparse.ParseWorkbook(body, surface.FiscalYear)")) {
		t.Error("capacity.go must call ParseWorkbook with the resolved fiscal year, never \"\"")
	}
}

// TestCapacityReconciliationLedgerHasNoPhantomNumbers walks the DECODED
// document rather than string-matching, and checks that no derived total
// anywhere equals a figure this CLI never measured.
//
// MUTATION: add "installed_mw": 42512 to the SIR row and this fails.
func TestCapacityReconciliationLedgerHasNoPhantomNumbers(t *testing.T) {
	w, _ := capacityParse(t, "2023-24")
	rep, err := capacityGroupLive(w, "status")
	if err != nil {
		t.Fatal(err)
	}
	blob, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Reconciliation []map[string]any `json:"reconciliation"`
	}
	if err := json.Unmarshal(blob, &doc); err != nil {
		t.Fatal(err)
	}
	unavailable := 0
	for _, row := range doc.Reconciliation {
		if row["state"] != "unavailable" {
			continue
		}
		unavailable++
		for _, banned := range []string{"installed_mw", "dependable_mw", "mw", "plants"} {
			if _, present := row[banned]; present {
				t.Errorf("unavailable ledger row %v carries a %q key", row["label"], banned)
			}
		}
		for _, required := range []string{"published_as", "reason", "source"} {
			if s, _ := row[required].(string); s == "" {
				t.Errorf("unavailable ledger row %v has no %q", row["label"], required)
			}
		}
	}
	// A year this build has read no SIR figure for must NOT reuse the
	// FY2023-24 strings: that would misdate an attribution.
	other, err := capacityGroupLive(mustParseFY(t, "2021-22"), "status")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range other.Reconciliation {
		if strings.HasPrefix(r.Label, "sir_2024") {
			t.Errorf("a FY2021-22 report must not carry the FY2023-24 ledger row %q", r.Label)
		}
		if r.Label == "sir_2021-22_system" {
			if r.PublishedAs != "" {
				t.Errorf("sir_2021-22_system must carry NO published_as: no figure was read; got %q", r.PublishedAs)
			}
			if r.Reason == "" || r.Source == "" {
				t.Errorf("sir_2021-22_system must still say why it is absent: %+v", r)
			}
		}
	}
	if unavailable != 3 {
		t.Errorf("expected 3 unavailable ledger rows, got %d", unavailable)
	}

	// No number anywhere in the document may be one of the figures this CLI
	// cannot measure, or a sum involving them.
	banned := map[float64]string{
		42512: "SIR CPPA-G system", 45888: "SIR including K-Electric", 47559.97: "licence register gross",
		88400: "42512+45888", 90574: "42512+... phantom sum", 92447.97: "45888+... phantom sum",
	}
	var walk func(any)
	walk = func(v any) {
		switch t2 := v.(type) {
		case map[string]any:
			for _, vv := range t2 {
				walk(vv)
			}
		case []any:
			for _, vv := range t2 {
				walk(vv)
			}
		case float64:
			if label, bad := banned[t2]; bad {
				t.Errorf("the document contains %v, which is the %s figure this CLI never measured", t2, label)
			}
		}
	}
	var any0 any
	if err := json.Unmarshal(blob, &any0); err != nil {
		t.Fatal(err)
	}
	walk(any0)
}

// TestCapacityCatalogueMakesNoRequest runs the command with no flags against
// a base URL that refuses every connection. A floor of 0 in the output is a
// failure: a year with no measurement carries NO floor key.
func mustParseFY(t *testing.T, fy string) *nepraparse.Workbook {
	t.Helper()
	w, _ := capacityParse(t, fy)
	return w
}

func TestCapacityCatalogueMakesNoRequest(t *testing.T) {
	// 127.0.0.1:1 refuses instantly; if the catalogue branch made a request
	// the command would fail rather than print.
	t.Setenv("NEPRA_BASE_URL", "http://127.0.0.1:1")
	var out bytes.Buffer
	cmd := RootCmd()
	cmd.SetArgs([]string{"capacity", "--json"})
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("capacity with no selector must succeed without a request: %v", err)
	}
	var env struct {
		Meta struct {
			Source      string `json:"source"`
			RequestMade bool   `json:"request_made"`
		} `json:"meta"`
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("decoding catalogue output %q: %v", out.String(), err)
	}
	if env.Meta.Source != "catalogue" || env.Meta.RequestMade {
		t.Errorf("meta = %+v, want source catalogue and request_made false", env.Meta)
	}
	if len(env.Results) != 10 {
		t.Fatalf("catalogue entries = %d, want 10 (7 reachable + 3 unavailable)", len(env.Results))
	}
	reachable, withFloors := 0, 0
	for _, e := range env.Results {
		if e["state"] == "reachable" {
			reachable++
		}
		measured, _ := e["floors_measured"].(bool)
		_, hasRows := e["plant_row_floor"]
		_, hasBytes := e["decoded_byte_floor"]
		if measured {
			withFloors++
			if !hasRows || !hasBytes {
				t.Errorf("%v claims floors_measured but omits one: %v", e["as_of"], e)
			}
			if e["plant_row_floor"].(float64) == 0 || e["decoded_byte_floor"].(float64) == 0 {
				t.Errorf("%v carries a ZERO floor, which asserts something: %v", e["as_of"], e)
			}
		} else if hasRows || hasBytes {
			t.Errorf("%v has floors_measured false but still carries a floor key: %v", e["as_of"], e)
		}
		if e["state"] == "unavailable" {
			if s, _ := e["reason"].(string); s == "" {
				t.Errorf("%v is unavailable with no reason", e["as_of"])
			}
		}
	}
	if reachable != 7 {
		t.Errorf("reachable years = %d, want 7", reachable)
	}
	if withFloors != 7 {
		t.Errorf("years with measured floors = %d, want 7", withFloors)
	}
	if !strings.Contains(out.String(), "SIR Data 2025.htm") {
		t.Error("the catalogue must name the FY2024-25 decoy")
	}
}

// TestCapacityAllYearsMeetTheirCatalogueFloors re-derives every catalogued
// floor from its own committed fixture, so a floor can never drift away from
// the file it was measured on.
func TestCapacityAllYearsMeetTheirCatalogueFloors(t *testing.T) {
	for _, s := range capacityAsOfCatalogue {
		if s.State != "reachable" {
			continue
		}
		if !s.FloorsMeasured {
			t.Errorf("FY%s is reachable with no measured floor", s.FiscalYear)
			continue
		}
		w, body := capacityParse(t, s.FiscalYear)
		if len(body) != s.DecodedByteFloor {
			t.Errorf("FY%s fixture decodes to %d bytes, catalogue floor says %d",
				s.FiscalYear, len(body), s.DecodedByteFloor)
		}
		if len(w.Plants) != s.PlantRowFloor {
			t.Errorf("FY%s fixture has %d plant rows, catalogue floor says %d",
				s.FiscalYear, len(w.Plants), s.PlantRowFloor)
		}
		if w.FiscalYear.Label() != s.FiscalYear {
			t.Errorf("FY%s fixture's own header band says %s", s.FiscalYear, w.FiscalYear.Label())
		}
		if w.Charset != "windows-1252" || !w.CharsetDeclared {
			t.Errorf("FY%s charset = %q declared=%v, want windows-1252 declared",
				s.FiscalYear, w.Charset, w.CharsetDeclared)
		}
	}
}

// TestCapacityMWNeverMarshalsUnmeasuredZero is the null-discipline unit.
func TestCapacityMWNeverMarshalsUnmeasuredZero(t *testing.T) {
	var zero capacityMW
	blob, err := json.Marshal(zero)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, []byte(`"mw"`)) {
		t.Errorf("an unpopulated capacityMW must emit NO mw key, got %s", blob)
	}
	if _, ok := zero.Float64(); ok {
		t.Error("an unpopulated capacityMW must not report a number")
	}
	// A MEASURED zero is data and must survive.
	var measuredZero capacityMW
	measuredZero.add(nepraparse.ParseValue("0"))
	blob, err = json.Marshal(measuredZero)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(blob, []byte(`"mw":0`)) {
		t.Errorf("a measured 0.00 must be emitted as a number, got %s", blob)
	}
	n, ok := measuredZero.Float64()
	if !ok || n != 0 || measuredZero.Plants() != 1 {
		t.Errorf("measured zero = %.2f ok=%v plants=%d, want 0.00 true 1", n, ok, measuredZero.Plants())
	}
	// A status sentinel adds nothing and leaves the sum unmeasured.
	var sentinel capacityMW
	sentinel.add(nepraparse.ParseValue("Export to K.Electric"))
	if _, ok := sentinel.Float64(); ok {
		t.Error("a status sentinel must not make a sum measured")
	}
}

// TestCapacityProvenanceHashesContentNotMarkup: the raw hash is not a
// document identity, because NEPRA's edge rewrites per-response tokens.
func TestCapacityProvenanceHashesContentNotMarkup(t *testing.T) {
	content := []byte(`[{"key":"active","mw":40625}]`)
	a := newNepraArtifact("https://nepra.org.pk/x", []byte(`<a data-cfemail="aaaa">x</a>`), content, "text/html", 133, 0)
	b := newNepraArtifact("https://nepra.org.pk/x", []byte(`<a data-cfemail="bbbb">x</a>`), content, "text/html", 133, 0)
	if a.SHA256Content != b.SHA256Content {
		t.Error("sha256_content must be blind to per-response markup tokens")
	}
	if a.SHA256Raw == b.SHA256Raw {
		t.Error("sha256_raw must differ when the bytes differ")
	}
	blob, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, []byte("http_status")) {
		t.Errorf("the artifact must not record an http_status it never observed: %s", blob)
	}
}

// TestCapacityLiveEndToEnd drives the whole command over a local server
// serving the real FY2023-24 bytes, so the fetch, the fiscal-year
// cross-check, the grouping, the assertions and the envelope are all
// exercised together.
func TestCapacityLiveEndToEnd(t *testing.T) {
	body := capacityFixture(t, "2023-24")
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	t.Setenv("NEPRA_BASE_URL", srv.URL)

	var out, errBuf bytes.Buffer
	cmd := RootCmd()
	cmd.SetArgs([]string{"capacity", "--as-of", "2024-06-30", "--json", "--no-cache"})
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("capacity --as-of 2024-06-30: %v (stderr %q)", err, errBuf.String())
	}
	if want := "/publications/State of Industry Reports/Detail of Generation/" +
		"List of Companies Genenration wise 2023-24_files/sheet001.htm"; gotPath != want {
		t.Errorf("requested path = %q, want %q (the upstream typo Genenration is load-bearing)", gotPath, want)
	}

	var env struct {
		Meta struct {
			AsOf               string   `json:"as_of"`
			FiscalYear         string   `json:"fiscal_year"`
			BandLabel          string   `json:"band_label_read_from_file"`
			Source             string   `json:"source"`
			Charset            string   `json:"charset"`
			CharsetDeclared    bool     `json:"charset_declared"`
			StatusEnum         []string `json:"status_enum"`
			StatusEnumComplete bool     `json:"status_enum_complete"`
			Warnings           []string `json:"warnings"`
			Assertions         struct {
				Failed    []string `json:"failed"`
				ByteFloor int      `json:"byte_floor"`
			} `json:"assertions"`
			Artifact struct {
				URL           string `json:"url"`
				Bytes         int    `json:"bytes"`
				Rows          int    `json:"rows"`
				SHA256Raw     string `json:"sha256_raw"`
				SHA256Content string `json:"sha256_content"`
			} `json:"artifact"`
		} `json:"meta"`
		Results capacityReportJSON `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("decoding %q: %v", out.String(), err)
	}
	if env.Meta.AsOf != "2024-06-30" || env.Meta.FiscalYear != "2023-24" {
		t.Errorf("meta as_of/fiscal_year = %q/%q", env.Meta.AsOf, env.Meta.FiscalYear)
	}
	// The year in the answer is the year the DOCUMENT states, not the year
	// the URL asked for.
	if env.Meta.BandLabel != "FY 2023-24" {
		t.Errorf("band_label_read_from_file = %q, want \"FY 2023-24\"", env.Meta.BandLabel)
	}
	if env.Meta.Source != "live" || env.Meta.Charset != "windows-1252" || !env.Meta.CharsetDeclared {
		t.Errorf("meta source/charset = %q/%q declared=%v", env.Meta.Source, env.Meta.Charset, env.Meta.CharsetDeclared)
	}
	if len(env.Meta.Assertions.Failed) != 0 {
		t.Errorf("assertions failed: %v", env.Meta.Assertions.Failed)
	}
	if env.Meta.Assertions.ByteFloor != 493187 {
		t.Errorf("byte_floor = %d, want 493187", env.Meta.Assertions.ByteFloor)
	}
	if env.Meta.Artifact.Bytes != len(body) || env.Meta.Artifact.Rows != 133 {
		t.Errorf("artifact = %d bytes / %d rows, want %d / 133", env.Meta.Artifact.Bytes, env.Meta.Artifact.Rows, len(body))
	}
	if env.Meta.Artifact.SHA256Raw == "" || env.Meta.Artifact.SHA256Content == "" {
		t.Error("the artifact must carry both hashes")
	}
	if !strings.Contains(env.Meta.Artifact.URL, "Genenration%20wise%202023-24_files") {
		t.Errorf("artifact url = %q", env.Meta.Artifact.URL)
	}
	// Parser warnings travel verbatim; FY2023-24 publishes exactly one.
	if len(env.Meta.Warnings) != 1 || !strings.Contains(env.Meta.Warnings[0], "Warsak") {
		t.Errorf("meta.warnings = %v, want the single Warsak residue warning", env.Meta.Warnings)
	}
	if strings.Join(env.Meta.StatusEnum, ",") != "active,delicensed,decommissioned,listed_no_data" ||
		!env.Meta.StatusEnumComplete {
		t.Errorf("status_enum = %v complete=%v", env.Meta.StatusEnum, env.Meta.StatusEnumComplete)
	}
	if got := env.Results.Totals.Installed.MW; got == nil || *got != 44686 {
		t.Errorf("totals installed mw = %v, want 44686", got)
	}
	if got := env.Results.Totals.Dependable.MW; got == nil || *got != 40974 {
		t.Errorf("totals dependable mw = %v, want 40974", got)
	}
	if len(env.Results.Buckets) != 4 {
		t.Fatalf("buckets = %d, want 4", len(env.Results.Buckets))
	}
	if env.Results.Buckets[3].Key != "listed_no_data" {
		t.Errorf("bucket[3] = %q, want listed_no_data", env.Results.Buckets[3].Key)
	}
	if env.Results.Buckets[3].Operating != nil {
		t.Error("listed_no_data must OMIT `operating` entirely")
	}
	if env.Results.Buckets[3].OperatingKnown == nil || *env.Results.Buckets[3].OperatingKnown {
		t.Error("listed_no_data must carry operating_known:false")
	}
}

// capacityReportJSON is a decode-side view of the emitted document, so the
// tests read the WIRE shape rather than the Go structs that produced it.
type capacityReportJSON struct {
	Buckets []struct {
		Key            string `json:"key"`
		Operating      *bool  `json:"operating"`
		OperatingKnown *bool  `json:"operating_known"`
		Installed      struct {
			Measured bool     `json:"measured"`
			MW       *float64 `json:"mw"`
			Plants   int      `json:"plants"`
		} `json:"installed"`
	} `json:"buckets"`
	Totals struct {
		PlantRows int `json:"plant_rows"`
		Installed struct {
			MW *float64 `json:"mw"`
		} `json:"installed"`
		Dependable struct {
			MW *float64 `json:"mw"`
		} `json:"dependable"`
	} `json:"totals"`
}

// TestCapacityByValidation pins the grouping vocabulary. `fuel` is
// deliberately absent: `gen --rollup technology|fuel` owns fuel rollups and
// a second implementation would drift.
func TestCapacityByValidation(t *testing.T) {
	for _, in := range []string{"", "status", "STATUS", " system ", "technology"} {
		if _, err := capacityNormaliseBy(in); err != nil {
			t.Errorf("--by %q must be accepted: %v", in, err)
		}
	}
	for _, in := range []string{"fuel", "stat", "tech", "System2"} {
		_, err := capacityNormaliseBy(in)
		if err == nil {
			t.Errorf("--by %q must be rejected", in)
			continue
		}
		if !strings.Contains(err.Error(), "status, system, technology") {
			t.Errorf("--by %q rejection must list the accepted values: %v", in, err)
		}
	}
	if _, err := capacityNormaliseBy("fuel"); err == nil || !strings.Contains(err.Error(), "gen --rollup") {
		t.Errorf("--by fuel must point at `gen --rollup`: %v", err)
	}
}
