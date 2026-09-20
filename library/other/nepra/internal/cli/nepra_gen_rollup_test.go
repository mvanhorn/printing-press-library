// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// The rollup's whole job is to sum without inventing. Every plant count here
// was re-derived from the committed fixture by a shell/regex pass that does
// not use nepraparse at all, so a bug in the extractor cannot make these
// numbers agree with themselves.

package cli

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

func genTestRollup(t *testing.T, fy, kind string) []genRollupRow {
	t.Helper()
	w, _ := genWorkbook(t, fy)
	return genRollup(w, kind, genTestURL)
}

func genRollupGroup(rows []genRollupRow, group string) []genRollupRow {
	var out []genRollupRow
	for _, r := range rows {
		if r.Group == group {
			out = append(out, r)
		}
	}
	return out
}

// TestGenRollupTechnologyFY2023_24 pins the nine technology groups and their
// plant counts. The nine values and their counts were re-derived outside the
// extractor; THERMAL 46 / WIND 36 / HYDEL-(WAPDA) 12 / BIOGAS 9 /
// THERMAL- COAL 8 / SOLAR 8 / HYDEL-(IPP) 7 / NUCLEAR 6 / Coal 1 = 133.
func TestGenRollupTechnologyFY2023_24(t *testing.T) {
	rows := genTestRollup(t, "2023-24", genRollupTechnology)
	if len(rows) != 108 {
		t.Fatalf("rows = %d, want 108 (9 groups x 12)", len(rows))
	}
	want := map[string]int{
		"THERMAL": 46, "WIND": 36, "HYDEL-(WAPDA)": 12, "BIOGAS": 9,
		"THERMAL- COAL": 8, "SOLAR": 8, "HYDEL-(IPP)": 7, "NUCLEAR": 6, "Coal": 1,
	}
	got := map[string]int{}
	months := map[string]int{}
	total := 0
	for _, r := range rows {
		if _, seen := got[r.Group]; !seen {
			got[r.Group] = r.Plants
			total += r.Plants
		}
		months[r.Group]++
		if r.Rollup != genRollupTechnology || r.FY != "2023-24" {
			t.Errorf("%s: rollup/fy = %q/%q", r.Group, r.Rollup, r.FY)
		}
	}
	if len(got) != 9 {
		t.Errorf("groups = %d, want 9: %v", len(got), got)
	}
	for g, n := range want {
		if got[g] != n {
			t.Errorf("group %q plants = %d, want %d", g, got[g], n)
		}
	}
	for g, n := range months {
		if n != 12 {
			t.Errorf("group %q has %d months, want 12", g, n)
		}
	}
	if total != 133 {
		t.Errorf("group plant counts sum to %d, want 133", total)
	}
	// "THERMAL- COAL" carries an upstream space after the hyphen and "Coal"
	// is the only mixed-case entry. Neither is re-spelled, and they are NOT
	// merged with each other or with fuel "COAL".
	if _, ok := got["THERMAL- COAL"]; !ok {
		t.Error("THERMAL- COAL lost its upstream space")
	}
	if _, ok := got["Coal"]; !ok {
		t.Error("the mixed-case Coal group was folded into another group")
	}
}

// TestGenRollupEmitsNoUtilisation pins the refusal in the output shape itself.
// The assertion is on the ABSENCE of the key from the marshalled row, not on a
// nil value: a null utilisation field would still invite a consumer to average
// it once it was populated.
func TestGenRollupEmitsNoUtilisation(t *testing.T) {
	for _, kind := range []string{genRollupTechnology, genRollupFuel} {
		rows := genTestRollup(t, "2023-24", kind)
		for i, r := range rows {
			b, err := json.Marshal(r)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(b, &obj); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			for _, banned := range []string{
				"utilisation_pct", "utilisation", "utilization", "utilisation_state",
				"load_factor", "pct", "percent", "capacity_factor",
			} {
				if _, ok := obj[banned]; ok {
					t.Fatalf("%s row %d carries %q; a rolled-up percentage is an invented number", kind, i, banned)
				}
			}
		}
		for _, col := range genRollupHeader {
			if strings.Contains(col, "util") || strings.Contains(col, "pct") || strings.Contains(col, "factor") {
				t.Errorf("%s header column %q is a rolled-up percentage", kind, col)
			}
		}
	}
}

// TestGenRollupRefusesToInventZero is the rollup's central honesty test.
//
// Technology "Coal" and fuel "Lignite Coal" are both exactly one plant —
// Lakhra, 150 MW installed / 30 MW dependable, DELICENSED — so neither group
// has a single summable cell in any of the twelve months. 0 GWh would be a
// fabrication; gwh_measured false with the key absent is the measurement.
//
// MUTATION-CHECKED: initialising the sum to 0 and marking it measured
// unconditionally fails this test.
func TestGenRollupRefusesToInventZero(t *testing.T) {
	for _, tc := range []struct {
		kind, group string
	}{
		{genRollupTechnology, "Coal"},
		{genRollupFuel, "Lignite Coal"},
	} {
		rows := genRollupGroup(genTestRollup(t, "2023-24", tc.kind), tc.group)
		if len(rows) != 12 {
			t.Fatalf("%s/%s: %d rows, want 12", tc.kind, tc.group, len(rows))
		}
		for _, r := range rows {
			if r.GWhMeasured {
				t.Errorf("%s/%s %s: gwh_measured = true with %d summable cells", tc.kind, tc.group, r.Month, r.GWhCellsSummed)
			}
			if r.GWh != nil {
				t.Errorf("%s/%s %s: gwh = %v, want no number at all", tc.kind, tc.group, r.Month, *r.GWh)
			}
			b, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(b, &obj); err != nil {
				t.Fatal(err)
			}
			if _, ok := obj["gwh"]; ok {
				t.Errorf("%s/%s %s: the gwh key is present on an unmeasured sum", tc.kind, tc.group, r.Month)
			}
			if r.GWhCellsSummed != 0 || r.CellsDelicensed != 1 {
				t.Errorf("%s/%s %s: summed %d, delicensed %d; want 0 and 1", tc.kind, tc.group, r.Month, r.GWhCellsSummed, r.CellsDelicensed)
			}
			if r.Plants != 1 || r.PlantsActive != 0 || r.PlantsDelicensed != 1 {
				t.Errorf("%s/%s %s: plants %d active %d delicensed %d; want 1/0/1", tc.kind, tc.group, r.Month, r.Plants, r.PlantsActive, r.PlantsDelicensed)
			}
			// The capacity is real and stays real. Only the GENERATION is
			// unmeasured; a delicensed plant's megawatts still exist on
			// paper.
			if r.InstalledMWNonOperating == nil || *r.InstalledMWNonOperating != 150 || !r.InstalledMWNonOperatingMeasured {
				t.Errorf("%s/%s %s: non-operating capacity = %v, want a measured 150 MW", tc.kind, tc.group, r.Month, r.InstalledMWNonOperating)
			}
			// The ACTIVE capacity of a group with no active plants is a
			// measured 0 over 0 plants, which is a real finding and not the
			// same as unmeasured.
			if !r.InstalledMWActiveMeasured || r.InstalledMWActive == nil || *r.InstalledMWActive != 0 {
				t.Errorf("%s/%s %s: active capacity = %v/%t, want a measured 0", tc.kind, tc.group, r.Month, r.InstalledMWActive, r.InstalledMWActiveMeasured)
			}
		}
	}
}

// TestGenRollupFuelFY2023_24 pins the sixteen fuel groups.
//
// BAGASSE is 8 against technology BIOGAS 9, because one BIOGAS plant burns
// "BAGASSE/COAL" — which is its own group and is not merged into either
// BAGASSE or COAL. That is what "keyed on the published string" means.
func TestGenRollupFuelFY2023_24(t *testing.T) {
	rows := genTestRollup(t, "2023-24", genRollupFuel)
	if len(rows) != 192 {
		t.Fatalf("rows = %d, want 192 (16 groups x 12)", len(rows))
	}
	want := map[string]int{
		"WIND": 36, "HYDEL": 19, "RFO": 16, "SOLAR": 8, "RLNG/HSD": 8,
		"Natural Gas/ Furnace Oil": 8, "COAL": 8, "BAGASSE": 8, "GAS": 6,
		"NUCLEAR": 6, "Natural Gas": 4, "RLNG": 2, "RFO/RLNG/HSD": 1,
		"Lignite Coal": 1, "GAS/HSD": 1, "BAGASSE/COAL": 1,
	}
	got := map[string]int{}
	total := 0
	for _, r := range rows {
		if _, seen := got[r.Group]; !seen {
			got[r.Group] = r.Plants
			total += r.Plants
		}
	}
	if len(got) != 16 {
		t.Errorf("groups = %d, want 16: %v", len(got), got)
	}
	for g, n := range want {
		if got[g] != n {
			t.Errorf("fuel %q plants = %d, want %d", g, got[g], n)
		}
	}
	if total != 133 {
		t.Errorf("fuel plant counts sum to %d, want 133", total)
	}
	tech := genTestRollup(t, "2023-24", genRollupTechnology)
	biogas := 0
	for _, r := range tech {
		if r.Group == "BIOGAS" {
			biogas = r.Plants
			break
		}
	}
	if biogas != 9 || got["BAGASSE"] != 8 || got["BAGASSE/COAL"] != 1 {
		t.Errorf("BIOGAS %d vs BAGASSE %d + BAGASSE/COAL %d; the 9-vs-8 gap is the one dual-fuel plant",
			biogas, got["BAGASSE"], got["BAGASSE/COAL"])
	}
}

// TestGenRollupSumsOnlyNumericCells pins that a partial sum is visible as
// partial and that the sums are real.
//
// THERMAL is the one FY2023-24 technology group with members in four classes
// at once: 32 active, 11 delicensed, 1 decommissioned and 2 listed_no_data.
// Its monthly GWh is therefore a sum over 32 of 46 cells, and the other 14 are
// counted by the reason they could not be summed.
func TestGenRollupSumsOnlyNumericCells(t *testing.T) {
	rows := genRollupGroup(genTestRollup(t, "2023-24", genRollupTechnology), "THERMAL")
	if len(rows) != 12 {
		t.Fatalf("THERMAL rows = %d, want 12", len(rows))
	}
	for _, r := range rows {
		if r.Plants != 46 {
			t.Errorf("THERMAL %s: plants = %d, want 46", r.Month, r.Plants)
		}
		if r.PlantsActive != 32 || r.PlantsDelicensed != 11 || r.PlantsDecommissioned != 1 || r.PlantsListedNoData != 2 {
			t.Errorf("THERMAL %s: classes = %d/%d/%d/%d, want 32/11/1/2",
				r.Month, r.PlantsActive, r.PlantsDelicensed, r.PlantsDecommissioned, r.PlantsListedNoData)
		}
		if r.GWhCellsSummed != 32 {
			t.Errorf("THERMAL %s: summed %d cells, want 32", r.Month, r.GWhCellsSummed)
		}
		// The 14 unsummable cells are accounted for by reason, and the six
		// buckets must add back to the plant count. This is the rollup's own
		// census-balance check.
		accounted := r.GWhCellsSummed + r.CellsNotReported + r.CellsDelicensed +
			r.CellsDecommissioned + r.CellsExportToKElectric + r.CellsUnknownText
		if accounted != r.Plants {
			t.Errorf("THERMAL %s: %d cells accounted for, want %d", r.Month, accounted, r.Plants)
		}
		if r.CellsDelicensed != 11 || r.CellsDecommissioned != 1 || r.CellsNotReported != 2 {
			t.Errorf("THERMAL %s: unsummable = %d delicensed, %d decommissioned, %d not_reported; want 11/1/2",
				r.Month, r.CellsDelicensed, r.CellsDecommissioned, r.CellsNotReported)
		}
		if !r.GWhMeasured || r.GWh == nil || *r.GWh <= 0 {
			t.Errorf("THERMAL %s: gwh = %v, want a measured positive sum", r.Month, r.GWh)
		}
	}
}

// TestGenRollupTotalsAgreeWithThePlantGrain proves the rollup did not lose or
// duplicate energy: summing the plant-month rows by technology must reproduce
// the rollup's own monthly figures exactly.
func TestGenRollupTotalsAgreeWithThePlantGrain(t *testing.T) {
	for _, fy := range []string{"2017-18", "2020-21", "2023-24"} {
		w, _ := genWorkbook(t, fy)
		plantRows := genRows(w, genTestURL)
		byKey := map[string]float64{}
		cells := map[string]int{}
		for _, r := range plantRows {
			if r.GWh == nil {
				continue
			}
			k := r.Technology + "\x00" + r.Month
			byKey[k] += *r.GWh
			cells[k]++
		}
		for _, r := range genRollup(w, genRollupTechnology, genTestURL) {
			k := r.Group + "\x00" + r.Month
			if cells[k] != r.GWhCellsSummed {
				t.Errorf("FY%s %s %s: rollup summed %d cells, plant grain has %d numeric", fy, r.Group, r.Month, r.GWhCellsSummed, cells[k])
			}
			if r.GWh == nil {
				if cells[k] != 0 {
					t.Errorf("FY%s %s %s: rollup has no number but the plant grain has %d", fy, r.Group, r.Month, cells[k])
				}
				continue
			}
			if math.Abs(*r.GWh-byKey[k]) > 1e-6 {
				t.Errorf("FY%s %s %s: rollup %.6f vs plant grain %.6f", fy, r.Group, r.Month, *r.GWh, byKey[k])
			}
		}
	}
}

// TestGenRollupCapacityBucketsPartitionTheGroup pins that the four capacity
// buckets and the six plant-class counts each account for the whole group.
func TestGenRollupCapacityBucketsPartitionTheGroup(t *testing.T) {
	for _, fy := range []string{"2017-18", "2020-21", "2023-24"} {
		for _, kind := range []string{genRollupTechnology, genRollupFuel} {
			for _, r := range genTestRollup(t, fy, kind) {
				classes := r.PlantsActive + r.PlantsDelicensed + r.PlantsDecommissioned +
					r.PlantsListedNoData + r.PlantsExportToKElectric + r.PlantsUnmodelledBlock
				if classes != r.Plants {
					t.Errorf("FY%s %s/%s %s: classes sum to %d, want %d plants", fy, kind, r.Group, r.Month, classes, r.Plants)
				}
				// An unmeasured bucket must carry NO number, and a measured
				// one must carry one. There is no third state.
				for _, b := range []struct {
					name     string
					mw       *float64
					measured bool
				}{
					{"active", r.InstalledMWActive, r.InstalledMWActiveMeasured},
					{"non_operating", r.InstalledMWNonOperating, r.InstalledMWNonOperatingMeasured},
					{"listed_no_data", r.InstalledMWListedNoData, r.InstalledMWListedNoDataMeasured},
					{"unmodelled_block", r.InstalledMWUnmodelledBlock, r.InstalledMWUnmodelledBlockMeasured},
				} {
					if b.measured != (b.mw != nil) {
						t.Errorf("FY%s %s/%s %s: %s measured=%t but mw=%v", fy, kind, r.Group, r.Month, b.name, b.measured, b.mw)
					}
				}
			}
		}
	}
}

// TestGenRollupFY2017_18UnmeasuredCapacity pins the counter-case in the
// rollup: FY2017-18's eleven all-blank rows publish no capacity, so the
// listed_no_data bucket of every group holding one of them is UNMEASURED
// rather than 0 MW.
func TestGenRollupFY2017_18UnmeasuredCapacity(t *testing.T) {
	rows := genTestRollup(t, "2017-18", genRollupTechnology)
	unmeasured := 0
	for _, r := range rows {
		if r.PlantsListedNoData > 0 && !r.InstalledMWListedNoDataMeasured {
			unmeasured++
		}
		if r.PlantsListedNoData > 0 && r.InstalledMWListedNoData != nil {
			t.Errorf("FY2017-18 %s %s: listed_no_data capacity = %v, but those rows publish no capacity", r.Group, r.Month, *r.InstalledMWListedNoData)
		}
	}
	if unmeasured == 0 {
		t.Error("FY2017-18 must have at least one group whose listed_no_data capacity is unmeasured")
	}
}

// TestGenRollupKeepsUnknownVocabulary pins that a group value outside the
// measured vocabulary is KEPT as its own group, flagged, and never folded into
// a neighbour.
//
// No published year contains one — Technology is the same 9 values and Fuel the
// same 16 in every sampled year — so the case is reproduced by editing one
// Technology cell in the FY2023-24 bytes. Folding an unrecognised technology
// into the nearest known one would move a plant's whole year of generation
// into a category NEPRA never put it in.
func TestGenRollupKeepsUnknownVocabulary(t *testing.T) {
	body := genFixture(t, "2023-24")
	// Tarbela is the first data row, technology HYDEL-(WAPDA).
	edited := bytes.Replace(body, []byte(">HYDEL-(WAPDA)<"), []byte(">GEOTHERMAL<"), 1)
	if bytes.Equal(edited, body) {
		t.Fatal("the fixture has no HYDEL-(WAPDA) cell to edit; this test is not testing anything")
	}
	w, err := nepraparse.ParseWorkbook(edited, "2023-24")
	if err != nil {
		t.Fatalf("an unknown vocabulary value must not be fatal: %v", err)
	}

	rows := genRollup(w, genRollupTechnology, genTestURL)
	groups := map[string]int{}
	for _, r := range rows {
		groups[r.Group] = r.Plants
	}
	if len(groups) != 10 {
		t.Fatalf("groups = %d, want 10 (the unknown value is its own group): %v", len(groups), groups)
	}
	if groups["GEOTHERMAL"] != 1 {
		t.Errorf("GEOTHERMAL plants = %d, want 1", groups["GEOTHERMAL"])
	}
	if groups["HYDEL-(WAPDA)"] != 11 {
		t.Errorf("HYDEL-(WAPDA) plants = %d, want 11 after one row moved out", groups["HYDEL-(WAPDA)"])
	}
	total := 0
	for _, n := range groups {
		total += n
	}
	if total != 133 {
		t.Errorf("group plant counts sum to %d, want 133; a plant was dropped or duplicated", total)
	}
	if len(rows) != 120 {
		t.Errorf("rows = %d, want 120 (10 groups x 12)", len(rows))
	}

	// And it is FLAGGED, so a consumer is not left to notice on their own.
	y, _ := genYearByLabel("2023-24")
	plantRows := genRows(w, genTestURL)
	meta := genBuildMeta(w, y, "plant-month", len(plantRows), edited, genTestURL, "text/html", genRowsJSON(plantRows))
	var flagged bool
	for _, warn := range meta.Warnings {
		if strings.Contains(warn, "unknown Technology") && strings.Contains(warn, "GEOTHERMAL") {
			flagged = true
		}
	}
	if !flagged {
		t.Errorf("no warning names the unknown technology: %v", meta.Warnings)
	}
	// The per-row known flags say so too.
	for _, r := range plantRows {
		if r.Technology == "GEOTHERMAL" && r.TechnologyKnown {
			t.Errorf("%s reports technology_known=true for an unknown value", r.Plant)
		}
		if r.Technology == "HYDEL-(WAPDA)" && !r.TechnologyKnown {
			t.Errorf("%s reports technology_known=false for a known value", r.Plant)
		}
	}
}

// TestGenRollupKeepsAnEmptyVocabularyCell pins the one group value that
// carries no information at all. An empty Technology is indistinguishable from
// an absent column and an unclassified plant, so it must not pass silently:
// the plant keeps its rows under an empty group name and the parse warns.
//
// nepraparse's Technologies()/Fuels() accessors SKIP empty values, which is
// why the rollup keys on the plant's own field instead of iterating those
// lists — doing it the other way would have dropped the plant entirely.
func TestGenRollupKeepsAnEmptyVocabularyCell(t *testing.T) {
	body := genFixture(t, "2023-24")
	edited := bytes.Replace(body, []byte(">HYDEL-(WAPDA)<"), []byte("><"), 1)
	if bytes.Equal(edited, body) {
		t.Fatal("the fixture has no HYDEL-(WAPDA) cell to blank")
	}
	w, err := nepraparse.ParseWorkbook(edited, "2023-24")
	if err != nil {
		t.Fatalf("a blank vocabulary cell must not be fatal: %v", err)
	}
	// Technologies() reports 9 — the nine known values, with the empty one
	// skipped — while the workbook now holds TEN distinct technology
	// strings. That gap is the whole point: iterating the accessor would
	// have dropped the blank-technology plant's twelve rows.
	if got := len(w.Technologies()); got != 9 {
		t.Fatalf("Technologies() = %d, want 9 with the empty value skipped", got)
	}
	rows := genRollup(w, genRollupTechnology, genTestURL)
	total, groups := 0, map[string]bool{}
	for _, r := range rows {
		if !groups[r.Group] {
			groups[r.Group] = true
			total += r.Plants
		}
	}
	if total != 133 {
		t.Errorf("group plant counts sum to %d, want 133; the blank-technology plant was dropped", total)
	}
	if len(groups) != 10 {
		t.Errorf("groups = %d, want 10 (nine named plus the empty one)", len(groups))
	}
	if !groups[""] {
		t.Error("the blank technology did not survive as its own group")
	}
	y, _ := genYearByLabel("2023-24")
	plantRows := genRows(w, genTestURL)
	meta := genBuildMeta(w, y, "plant-month", len(plantRows), edited, genTestURL, "text/html", genRowsJSON(plantRows))
	var flagged bool
	for _, warn := range meta.Warnings {
		if strings.Contains(warn, "EMPTY Technology") {
			flagged = true
		}
	}
	if !flagged {
		t.Errorf("no warning names the empty technology: %v", meta.Warnings)
	}
}
