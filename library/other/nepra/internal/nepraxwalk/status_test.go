// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package nepraxwalk

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// workbook parses one of nepraparse's full-year fixtures.
//
// Reaching into a sibling package's testdata is deliberate. crosswalk.json's
// per-year observations were originally produced by re-extracting columns 0-4
// of these very files by hand, INDEPENDENTLY of nepraparse — and that second
// extraction path is what lost the DELICENSED/DECOMMISSIONED status, because
// the status lives in the monthly block rather than in the first five columns.
// Parsing the published bytes here turns that divergent path into a checked
// redundancy: TestCrosswalkAgreesWithNepraparse fails if the curated data and
// the parser ever disagree about the same file.
func workbook(t *testing.T, name, fy string) *nepraparse.Workbook {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "nepraparse", "testdata", name))
	if err != nil {
		t.Fatalf("reading sibling fixture %s: %v", name, err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("opening gzip fixture %s: %v", name, err)
	}
	defer func() { _ = zr.Close() }()
	dec, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("decompressing %s: %v", name, err)
	}
	w, err := nepraparse.ParseWorkbook(dec, fy)
	if err != nil {
		t.Fatalf("ParseWorkbook(%s, %q): %v", name, fy, err)
	}
	return w
}

var workbookFixtures = []struct{ file, fy string }{
	{"full-fy2017-18.htm.gz", "2017-18"},
	{"full-fy2020-21.htm.gz", "2020-21"},
	{"full-fy2023-24.htm.gz", "2023-24"},
}

// TestCrosswalkAgreesWithNepraparse is the load-bearing test of this file. It
// re-parses the published workbooks with nepraparse and requires the curated
// crosswalk to agree, field by field, on every observation.
//
// It exists because the crosswalk's first cut agreed with the parser on all
// five columns it copied and was still wrong: it had no column for the
// monthly-block status at all, so 13 FY2023-24 plants totalling 3,880 MW —
// among them the DELICENSED 1,292 MW Hub Power Company — were indistinguishable
// from operating ones. Agreement on the fields you thought to copy is not
// agreement with the source.
func TestCrosswalkAgreesWithNepraparse(t *testing.T) {
	// (fy, sno) -> plant. S.No is unique and contiguous within a year, which
	// makes it a valid in-year key; it is NOT valid across years.
	type key struct {
		fy  string
		sno int
	}
	plants := map[key]nepraparse.Plant{}
	for _, f := range workbookFixtures {
		w := workbook(t, f.file, f.fy)
		for _, p := range w.Plants {
			plants[key{f.fy, p.SNo}] = p
		}
	}

	var checked int
	statuses := map[string]int{}
	for _, r := range Rows() {
		for _, o := range r.Observed {
			p, ok := plants[key{o.FY, o.SNo}]
			if !ok {
				t.Errorf("%q: observation FY%s S.No %d has no counterpart in the published workbook",
					r.CanonicalName, o.FY, o.SNo)
				continue
			}
			checked++

			if got, want := Normalize(o.PublishedName), Normalize(p.Name); got != want {
				t.Errorf("FY%s S.No %d name: crosswalk %q, workbook %q", o.FY, o.SNo, got, want)
			}
			if o.Technology != p.Technology {
				t.Errorf("FY%s S.No %d technology: crosswalk %q, workbook %q", o.FY, o.SNo, o.Technology, p.Technology)
			}
			if o.Fuel != p.Fuel {
				t.Errorf("FY%s S.No %d fuel: crosswalk %q, workbook %q", o.FY, o.SNo, o.Fuel, p.Fuel)
			}

			// The capacity cell must agree on BOTH the number and the state,
			// so an "Export to K.Electric" cell can never be curated as a
			// blank one.
			gotCap, gotOK := o.Capacity().Float64()
			wantCap, wantOK := p.InstalledCapacity.Float64()
			if gotOK != wantOK || (gotOK && gotCap != wantCap) {
				t.Errorf("FY%s S.No %d capacity: crosswalk (%v,%v), workbook (%v,%v)",
					o.FY, o.SNo, gotCap, gotOK, wantCap, wantOK)
			}
			if got, want := o.Capacity().State(), p.InstalledCapacity.State(); got != want {
				t.Errorf("FY%s S.No %d capacity state: crosswalk %s, workbook %s", o.FY, o.SNo, got, want)
			}
			// The stored state name is a checked redundancy against the state
			// re-derived from the raw string.
			if got, want := o.InstalledCapacityStateName, o.Capacity().State().String(); got != want {
				t.Errorf("FY%s S.No %d stored capacity state %q disagrees with the state derived from %q: %q",
					o.FY, o.SNo, got, o.InstalledCapacityMWRaw, want)
			}

			// THE FIELD THAT WAS MISSING.
			st, known := o.BlockStatus()
			if !known {
				t.Errorf("FY%s S.No %d (%q) has no recorded block status; operating status would be unknowable",
					o.FY, o.SNo, r.CanonicalName)
				continue
			}
			if st != p.Status {
				t.Errorf("FY%s S.No %d block status: crosswalk %s, workbook %s", o.FY, o.SNo, st, p.Status)
			}
			statuses[st.String()]++
		}
	}

	if checked != 241 {
		t.Errorf("checked %d observations, want 241 (108 FY2017-18 + 133 FY2023-24)", checked)
	}
	// Measured against the published files: the sentinels exist in FY2023-24
	// and nowhere else in the observed years.
	want := map[string]int{"numeric": 228, "delicensed": 12, "decommissioned": 1}
	for k, v := range want {
		if statuses[k] != v {
			t.Errorf("observations with block status %q = %d, want %d", k, statuses[k], v)
		}
	}
	for k, v := range statuses {
		if want[k] == 0 {
			t.Errorf("unexpected block status %q on %d observations", k, v)
		}
	}
}

// TestNonOperatingPlantsAreRepresentable is finding F stated as a test: a
// delicensed or decommissioned plant must be distinguishable from a running
// one, must keep its published capacity, and must not be reported as current.
func TestNonOperatingPlantsAreRepresentable(t *testing.T) {
	w := workbook(t, "full-fy2023-24.htm.gz", "2023-24")

	// Ground truth straight from the published file.
	type expect struct {
		status nepraparse.CellState
		mw     float64
	}
	wantByName := map[string]expect{}
	var wantMW float64
	for _, p := range w.Plants {
		if !p.Status.Status() {
			continue
		}
		mw, ok := p.InstalledCapacity.Float64()
		if !ok {
			t.Fatalf("%q is non-operating AND publishes no capacity; the fixture assumption is wrong", p.Name)
		}
		m, resolved := Resolve(p.Name)
		if !resolved {
			t.Fatalf("non-operating plant %q does not resolve in the crosswalk", p.Name)
		}
		wantByName[m.Row.CanonicalName] = expect{p.Status, mw}
		wantMW += mw
	}
	if len(wantByName) != 13 {
		t.Fatalf("non-operating FY2023-24 plants = %d, want 13", len(wantByName))
	}
	if wantMW != 3880 {
		t.Fatalf("non-operating FY2023-24 capacity = %v MW, want 3880", wantMW)
	}

	for canon, exp := range wantByName {
		r, ok := RowByCanonicalName(canon)
		if !ok {
			t.Errorf("%q: not retrievable by canonical name", canon)
			continue
		}
		// Operating status must be readable, and must say NOT operating.
		op, observed := r.OperatingInFY("2023-24")
		if !observed {
			t.Errorf("%q: not observed in FY2023-24", canon)
			continue
		}
		if op {
			t.Errorf("%q reports operating in FY2023-24, but its monthly block is %s", canon, exp.status)
		}
		if st, ok := r.StatusInFY("2023-24"); !ok || st != exp.status {
			t.Errorf("%q StatusInFY(2023-24) = (%s,%v), want (%s,true)", canon, st, ok, exp.status)
		}
		// LatestStatus is the "is it still running" accessor.
		st, fy, ok := r.LatestStatus()
		if !ok || fy != "2023-24" || st != exp.status {
			t.Errorf("%q LatestStatus() = (%s,%q,%v), want (%s,\"2023-24\",true)", canon, st, fy, ok, exp.status)
		}
		// The publication interval is still open — that is CORRECT, and is
		// exactly why IsOpenEnded must never be read as an operating claim.
		if !r.IsOpenEnded() {
			t.Errorf("%q: publication interval unexpectedly closed", canon)
		}
		// Capacity survives. Nulling it would be the opposite error.
		o, _ := r.ObservedIn("2023-24")
		if mw, ok := o.InstalledCapacityMW(); !ok || mw != exp.mw {
			t.Errorf("%q FY2023-24 capacity = (%v,%v), want (%v,true): a non-operating plant keeps its published capacity",
				canon, mw, ok, exp.mw)
		}
	}

	// An unrecorded status must read as unknown, never as operating.
	var blank Observation
	if st, known := blank.BlockStatus(); known || st != nepraparse.StateUnset {
		t.Errorf("zero-value Observation BlockStatus = (%s,%v), want (unset,false)", st, known)
	}
	if op, known := blank.Operating(); op || known {
		t.Errorf("zero-value Observation Operating = (%v,%v), want (false,false): unknown must not be operating", op, known)
	}
}

// TestPlantsForParentSeparatesLiveFromDelicensed is the analyst-facing half of
// finding F. PlantsForParent("HUBC") used to return six look-alike plants, one
// of which had been delicensed — 1,292 MW of the 8,381 MW this crosswalk
// attributes to any PSX ticker, or 15.4%.
func TestPlantsForParentSeparatesLiveFromDelicensed(t *testing.T) {
	pp, ok := PlantsForParent("HUBC")
	if !ok {
		t.Fatal("PlantsForParent(HUBC) not found")
	}
	if pp.AsOfFY != "2023-24" {
		t.Errorf("AsOfFY = %q, want 2023-24; a fleet summary without an as-of year is uninterpretable", pp.AsOfFY)
	}
	if len(pp.Plants) != 6 {
		t.Fatalf("HUBC plants = %d, want 6", len(pp.Plants))
	}
	if pp.OperatingPlants != 5 || pp.NonOperatingPlants != 1 {
		t.Errorf("HUBC operating/non-operating = %d/%d, want 5/1", pp.OperatingPlants, pp.NonOperatingPlants)
	}
	if pp.StatusUnknownPlants != 0 || pp.NotObservedPlants != 0 {
		t.Errorf("HUBC unknown/not-observed = %d/%d, want 0/0", pp.StatusUnknownPlants, pp.NotObservedPlants)
	}

	// The delicensed plant is named, keeps its capacity, and is flagged.
	var found bool
	for _, p := range pp.Plants {
		if p.CanonicalName != "Hub Power Company (HUBCO)" {
			if !p.Operating || !p.StatusKnown {
				t.Errorf("%q: operating=%v known=%v, want true/true", p.CanonicalName, p.Operating, p.StatusKnown)
			}
			continue
		}
		found = true
		if p.Operating {
			t.Error("Hub Power Company (HUBCO) reports operating; it is DELICENSED in FY2023-24")
		}
		if !p.StatusKnown {
			t.Error("Hub Power Company (HUBCO) status is unknown; it is recorded as delicensed")
		}
		if p.StatusName != "delicensed" {
			t.Errorf("HUBCO status = %q, want delicensed", p.StatusName)
		}
		if mw, ok := p.InstalledCapacity.Float64(); !ok || mw != 1292 {
			t.Errorf("HUBCO capacity = (%v,%v), want (1292,true): delicensing does not erase installed capacity", mw, ok)
		}
	}
	if !found {
		t.Fatal("Hub Power Company (HUBCO) missing from HUBC's plants")
	}

	// The sums a caller should actually use.
	op, opOK := pp.CapacityOperating.Float64()
	nonOp, nonOpOK := pp.CapacityNonOperating.Float64()
	if !opOK || !nonOpOK {
		t.Fatalf("HUBC capacity sums unmeasured: operating=%s non-operating=%s", pp.CapacityOperating, pp.CapacityNonOperating)
	}
	if nonOp != 1292 {
		t.Errorf("HUBC non-operating capacity = %v MW, want 1292", nonOp)
	}
	if op != 1320+225+84+330+330 {
		t.Errorf("HUBC operating capacity = %v MW, want %v", op, 1320+225+84+330+330)
	}
	if pp.CapacityOperating.Plants() != 5 || pp.CapacityNonOperating.Plants() != 1 {
		t.Errorf("HUBC sum plant counts = %d/%d, want 5/1", pp.CapacityOperating.Plants(), pp.CapacityNonOperating.Plants())
	}

	// ResolveTicker names the hazard too, so a caller who never reaches
	// PlantsForParent still cannot read Plants as a live fleet.
	tr, err := ResolveTicker("HUBC")
	if err != nil {
		t.Fatalf("ResolveTicker(HUBC): %v", err)
	}
	if len(tr.NonOperatingPlants) != 1 || tr.NonOperatingPlants[0] != "Hub Power Company (HUBCO)" {
		t.Errorf("ResolveTicker(HUBC).NonOperatingPlants = %v, want [Hub Power Company (HUBCO)]", tr.NonOperatingPlants)
	}

	// An operator with no delicensed plant must report a MEASURED zero, not
	// an unmeasured one: absence of delicensing is a finding.
	other, ok := PlantsForParent("KAPCO")
	if ok {
		if mw, measured := other.CapacityNonOperating.Float64(); !measured || mw != 0 {
			t.Errorf("KAPCO non-operating capacity = (%v,%v), want (0,true)", mw, measured)
		}
	}
}

// TestCapacityCellStatesAreDistinguishable is finding H: four materially
// different capacity cells that the old (float64, bool) accessor collapsed
// into one indistinguishable (0, false).
func TestCapacityCellStatesAreDistinguishable(t *testing.T) {
	tests := []struct {
		raw       string
		state     nepraparse.CellState
		mw        float64
		published bool
		why       string
	}{
		{"1,292", nepraparse.StateNumeric, 1292, true, "thousands separator"},
		{"0", nepraparse.StateNumeric, 0, true, "a MEASURED zero is data"},
		{"362", nepraparse.StateNumeric, 362, true, "plain integer"},
		{"", nepraparse.StateNotReported, 0, false, "blank: 11 FY2017-18 plants under construction"},
		{" ", nepraparse.StateNotReported, 0, false, "an 'empty' workbook cell is NBSP"},
		{"   ", nepraparse.StateNotReported, 0, false, "whitespace only"},
		{"Export\n  to K.Electric", nepraparse.StateExportToKElectric, 0, false, "3 FY2020-21 rows: a disposition, not a blank"},
		{"Export to K.Electric", nepraparse.StateExportToKElectric, 0, false, "collapsed spelling"},
		{"DELICENSED", nepraparse.StateDelicensed, 0, false, "sentinel in the capacity column"},
		{"banana", nepraparse.StateUnknownText, 0, false, "unmodelled text must be visible, not a silent zero"},
	}
	seen := map[nepraparse.CellState]bool{}
	for _, tc := range tests {
		o := Observation{InstalledCapacityMWRaw: tc.raw}
		if got := o.Capacity().State(); got != tc.state {
			t.Errorf("Capacity(%q).State() = %s, want %s (%s)", tc.raw, got, tc.state, tc.why)
		}
		mw, ok := o.InstalledCapacityMW()
		if ok != tc.published || (ok && mw != tc.mw) {
			t.Errorf("InstalledCapacityMW(%q) = (%v,%v), want (%v,%v)", tc.raw, mw, ok, tc.mw, tc.published)
		}
		seen[tc.state] = true
	}
	// The point of the finding: a blank and a status cell must not be the
	// same answer.
	blank := Observation{InstalledCapacityMWRaw: ""}
	export := Observation{InstalledCapacityMWRaw: "Export to K.Electric"}
	if blank.Capacity().State() == export.Capacity().State() {
		t.Error("a blank capacity cell and an 'Export to K.Electric' cell report the same state")
	}
	if len(seen) < 5 {
		t.Errorf("covered %d distinct capacity states, want at least 5", len(seen))
	}
}

// TestMWSumCannotFabricateAZero guards the type that fixes finding G,
// including its JSON path — the identical defect to the one that made
// json.Marshal of a nepraparse.Value emit {} and report 3,478 MW as nothing.
func TestMWSumCannotFabricateAZero(t *testing.T) {
	var zero MWSum
	if mw, ok := zero.Float64(); ok {
		t.Errorf("zero-value MWSum reports a measured %v MW; the zero value must not be a measurement", mw)
	}
	if zero.String() != "<not measured>" {
		t.Errorf("zero-value MWSum renders as %q, want %q", zero.String(), "<not measured>")
	}

	// An unmeasured sum must not carry a number through JSON.
	b, err := json.Marshal(zero)
	if err != nil {
		t.Fatalf("marshalling unmeasured MWSum: %v", err)
	}
	if bytes.Contains(b, []byte(`"mw"`)) {
		t.Errorf("unmeasured MWSum serialised with an mw field: %s", b)
	}
	if bytes.Equal(b, []byte("{}")) {
		t.Fatalf("MWSum serialised as {} — every field is unexported, so it needs its own MarshalJSON")
	}
	var back MWSum
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshalling unmeasured MWSum: %v", err)
	}
	if _, ok := back.Float64(); ok {
		t.Error("an unmeasured MWSum came back from JSON as a measurement")
	}

	// A measured sum must survive the round trip exactly, zero included.
	for _, tc := range []struct {
		mw     float64
		plants int
	}{{0, 0}, {3880, 13}, {44686.5, 133}} {
		in := MWSum{mw: tc.mw, measured: true, plants: tc.plants}
		b, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("marshalling %v: %v", in, err)
		}
		var out MWSum
		if err := json.Unmarshal(b, &out); err != nil {
			t.Fatalf("unmarshalling %s: %v", b, err)
		}
		mw, ok := out.Float64()
		if !ok {
			t.Errorf("%s came back unmeasured", b)
			continue
		}
		if math.Abs(mw-tc.mw) > 1e-9 || out.Plants() != tc.plants {
			t.Errorf("round trip of (%v,%d) gave (%v,%d)", tc.mw, tc.plants, mw, out.Plants())
		}
	}

	// A malformed pair must be refused rather than silently defaulted.
	for _, bad := range []string{
		`{"measured":true,"plants":3}`,
		`{"measured":false,"mw":3880,"plants":13}`,
	} {
		var out MWSum
		if err := json.Unmarshal([]byte(bad), &out); err == nil {
			t.Errorf("json %s was accepted; want an error", bad)
		}
	}
}

// TestCoverageReportSerialises checks the whole report survives JSON, since
// that is the CLI's output path and MWSum's unexported fields make it the
// exact shape of value that has silently serialised to {} before.
func TestCoverageReportSerialises(t *testing.T) {
	rep := Coverage("2023-24", fixtureNames(t, "2023-24"))
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("marshalling CoverageReport: %v", err)
	}
	var back CoverageReport
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("unmarshalling CoverageReport: %v", err)
	}
	for _, c := range []struct {
		label string
		a, b  MWSum
	}{
		{"CapacityReported", rep.CapacityReported, back.CapacityReported},
		{"CapacityOperating", rep.CapacityOperating, back.CapacityOperating},
		{"CapacityNonOperating", rep.CapacityNonOperating, back.CapacityNonOperating},
		{"CapacityTickered", rep.CapacityTickered, back.CapacityTickered},
	} {
		wantMW, wantOK := c.a.Float64()
		gotMW, gotOK := c.b.Float64()
		if wantOK != gotOK || wantMW != gotMW || c.a.Plants() != c.b.Plants() {
			t.Errorf("%s round trip: (%v,%v,%d) -> (%v,%v,%d)",
				c.label, wantMW, wantOK, c.a.Plants(), gotMW, gotOK, c.b.Plants())
		}
	}
	// And the held-out year's unmeasured sums must stay unmeasured in JSON.
	held := Coverage("2020-21", fixtureNames(t, "2023-24"))
	hb, err := json.Marshal(held)
	if err != nil {
		t.Fatalf("marshalling held-out report: %v", err)
	}
	var heldBack CoverageReport
	if err := json.Unmarshal(hb, &heldBack); err != nil {
		t.Fatalf("unmarshalling held-out report: %v", err)
	}
	if mw, ok := heldBack.CapacityReported.Float64(); ok {
		t.Errorf("held-out CapacityReported survived JSON as a measured %v MW", mw)
	}
}

// TestNonGeneratingCapacityHasThreePopulations pins a distinction that has now
// been got wrong twice, in opposite directions.
//
// An adversarial review reported "4,061 MW of non-operating capacity summed as
// current". A later pass measured the DELICENSED/DECOMMISSIONED rows at
// 3,880.00 MW, recorded 4,061 as an error, and — testing the wrong predicate
// (Plant.Status == StateNotReported, which is never true for these rows) —
// additionally recorded that the two not-reported rows "sum to 0 MW".
//
// BOTH figures are right, for different populations, and 0 MW was simply wrong:
//
//	3,880.00 MW  13 rows  monthly block is a DELICENSED/DECOMMISSIONED sentinel
//	  181.00 MW   2 rows  block status is ordinary NUMERIC, every cell NOT REPORTED
//	--------------------  (Reshma Power 97.00 + Gulf Powergen 84.00)
//	4,061.00 MW  15 rows  all capacity that generated nothing in FY2023-24
//
// The two are genuinely different facts: a delicensed plant has STOPPED, while
// a fully-blank plant is licensed and reported nothing — which is why
// nepraparse counts them in separate census fields (StatusRows and
// FullyNotReportedRows) and why a capacity report must not merge them.
func TestNonGeneratingCapacityHasThreePopulations(t *testing.T) {
	w := workbook(t, "full-fy2023-24.htm.gz", "2023-24")

	var statusMW, blankMW float64
	var statusRows, blankRows int
	for _, p := range w.Plants {
		mw, ok := p.InstalledCapacity.Float64()
		if !ok {
			continue
		}
		if p.Status.Status() {
			statusMW += mw
			statusRows++
			continue
		}
		blank := 0
		for _, o := range p.Months {
			if o.Generation.State() == nepraparse.StateNotReported {
				blank++
			}
		}
		if p.Total.Generation.State() == nepraparse.StateNotReported {
			blank++
		}
		// 12 months plus the annual Sum.
		if blank == 13 {
			blankMW += mw
			blankRows++
		}
	}

	for _, c := range []struct {
		label    string
		gotRows  int
		wantRows int
		gotMW    float64
		wantMW   float64
	}{
		{"status-sentinel rows", statusRows, 13, statusMW, 3880},
		{"fully-blank-block rows", blankRows, 2, blankMW, 181},
		{"all non-generating", statusRows + blankRows, 15, statusMW + blankMW, 4061},
	} {
		if c.gotRows != c.wantRows {
			t.Errorf("%s: %d rows, want %d", c.label, c.gotRows, c.wantRows)
		}
		if c.gotMW != c.wantMW {
			t.Errorf("%s: %.2f MW, want %.2f", c.label, c.gotMW, c.wantMW)
		}
	}

	// The census must agree, so the two populations stay separately countable.
	if got := w.Census.StatusRows; got != 13 {
		t.Errorf("Census.StatusRows = %d, want 13", got)
	}
	if got := w.Census.FullyNotReportedRows; got != 2 {
		t.Errorf("Census.FullyNotReportedRows = %d, want 2", got)
	}
	// And the wrong predicate must stay demonstrably wrong: these rows do NOT
	// carry StateNotReported as their block status.
	for _, p := range w.Plants {
		if p.Status == nepraparse.StateNotReported {
			t.Errorf("%q has block status not_reported; the 0 MW measurement came from expecting this", p.Name)
		}
	}
}

// TestStatusVocabularyStartsInFY2022_23 pins when NEPRA began publishing the
// DELICENSED/DECOMMISSIONED vocabulary at all.
//
// nepraparse's CellState doc comments said both sentinels were "FY2023-24
// only". That was an over-generalisation from the three-year sample those
// comments were written against (FY2017-18, FY2020-21, FY2023-24), which
// happens to skip the first year the vocabulary appears. Measured over all
// seven reachable years, FY2022-23 carries 11 delicensed rows and 1
// decommissioned row totalling 2,588.00 MW.
//
// It matters beyond bookkeeping: anyone reading the old comment would build a
// capacity or fleet series that silently treats FY2022-23's 12 non-generating
// plants as operating.
func TestStatusVocabularyStartsInFY2022_23(t *testing.T) {
	// Only FY2021-22..FY2023-24 are needed to bracket the first appearance,
	// and these three sheets are committed under internal/cli/testdata by the
	// capacity command.
	for _, tc := range []struct {
		fy                 string
		wantDelicensed     int
		wantDecommissioned int
		wantStatusMW       float64
	}{
		{"2021-22", 0, 0, 0},
		{"2022-23", 11, 1, 2588},
		{"2023-24", 12, 1, 3880},
	} {
		t.Run(tc.fy, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join("..", "cli", "testdata", "workbook-fy"+tc.fy+".htm.gz"))
			if err != nil {
				t.Skipf("sheet not committed: %v", err)
			}
			zr, err := gzip.NewReader(bytes.NewReader(raw))
			if err != nil {
				t.Fatalf("gzip: %v", err)
			}
			defer func() { _ = zr.Close() }()
			dec, err := io.ReadAll(zr)
			if err != nil {
				t.Fatalf("decompress: %v", err)
			}
			w, err := nepraparse.ParseWorkbook(dec, tc.fy)
			if err != nil {
				t.Fatalf("ParseWorkbook(%s): %v", tc.fy, err)
			}
			var del, decom int
			var statusMW float64
			for _, p := range w.Plants {
				switch p.Status {
				case nepraparse.StateDelicensed:
					del++
				case nepraparse.StateDecommissioned:
					decom++
				}
				if p.Status.Status() {
					mw, _ := p.InstalledCapacity.Float64()
					statusMW += mw
				}
			}
			if del != tc.wantDelicensed {
				t.Errorf("FY%s delicensed rows = %d, want %d", tc.fy, del, tc.wantDelicensed)
			}
			if decom != tc.wantDecommissioned {
				t.Errorf("FY%s decommissioned rows = %d, want %d", tc.fy, decom, tc.wantDecommissioned)
			}
			if statusMW != tc.wantStatusMW {
				t.Errorf("FY%s status capacity = %.2f MW, want %.2f", tc.fy, statusMW, tc.wantStatusMW)
			}
			// The census must agree with the row scan.
			if w.Census.StatusRows != del+decom {
				t.Errorf("FY%s census.StatusRows = %d, want %d", tc.fy, w.Census.StatusRows, del+decom)
			}
		})
	}
}
