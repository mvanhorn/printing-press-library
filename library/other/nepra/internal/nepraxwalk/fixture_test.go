// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package nepraxwalk

import (
	"encoding/json"
	"os"
	"testing"
)

// fixture is testdata/published_names.json: the "Name of Companies" column of
// the FY2017-18 and FY2023-24 NEPRA generation workbooks, cp1252-decoded and
// stored verbatim (NBSP padding and embedded newlines included).
type fixture struct {
	Years map[string][]struct {
		SNo                    int    `json:"sno"`
		PublishedName          string `json:"published_name"`
		Technology             string `json:"technology"`
		Fuel                   string `json:"fuel"`
		InstalledCapacityMWRaw string `json:"installed_capacity_mw_raw"`
	} `json:"years"`
}

func loadFixture(t *testing.T) fixture {
	t.Helper()
	b, err := os.ReadFile("testdata/published_names.json")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	var f fixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatalf("parsing fixture: %v", err)
	}
	return f
}

func fixtureNames(t *testing.T, fy string) []string {
	t.Helper()
	f := loadFixture(t)
	rows, ok := f.Years[fy]
	if !ok {
		t.Fatalf("fixture has no fiscal year %q", fy)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.PublishedName)
	}
	return out
}

// TestFixtureShape pins the fixture's measured size so a later re-fetch that
// changes the source cannot quietly move every other assertion in this file.
func TestFixtureShape(t *testing.T) {
	f := loadFixture(t)
	for _, tc := range []struct {
		fy   string
		want int
	}{
		{"2017-18", 108},
		{"2023-24", 133},
	} {
		if got := len(f.Years[tc.fy]); got != tc.want {
			t.Errorf("FY%s: got %d published plant rows, want %d", tc.fy, got, tc.want)
		}
	}
	// An "empty" capacity cell in these workbooks is U+00A0 NBSP, not "". The
	// fixture stores the cp1252-decoded cell, so blanks arrive as "" here and
	// must be reported as not-measured, never as zero. FY2017-18 has 11 such
	// cells (plants under construction that year: K-2, Gulpur, Golen Gol,
	// Tarbela Ext-4, the three Tricon Boston farms, ...); FY2023-24 has none.
	blanks := map[string]int{}
	notReported := map[string]int{}
	for fy, rows := range f.Years {
		for _, r := range rows {
			if r.InstalledCapacityMWRaw == "" {
				blanks[fy]++
			}
			o := Observation{InstalledCapacityMWRaw: r.InstalledCapacityMWRaw}
			if _, reported := o.InstalledCapacityMW(); !reported {
				notReported[fy]++
			}
		}
	}
	if blanks["2017-18"] != 11 {
		t.Errorf("FY2017-18 blank installed-capacity cells: got %d, want 11", blanks["2017-18"])
	}
	if blanks["2023-24"] != 0 {
		t.Errorf("FY2023-24 blank installed-capacity cells: got %d, want 0", blanks["2023-24"])
	}
	for fy, n := range blanks {
		if notReported[fy] != n {
			t.Errorf("FY%s: InstalledCapacityMW reported %d not-measured cells, want %d; a blank must never parse as zero", fy, notReported[fy], n)
		}
	}
	// A blank must be distinguishable from a published "0": FY2017-18 S.No 32
	// and 33 publish "0" MW, which IS a measurement.
	zero := Observation{InstalledCapacityMWRaw: "0"}
	if mw, reported := zero.InstalledCapacityMW(); !reported || mw != 0 {
		t.Errorf(`published "0" parsed as (%v, %v), want (0, true)`, mw, reported)
	}
	thousands := Observation{InstalledCapacityMWRaw: "1,292"}
	if mw, reported := thousands.InstalledCapacityMW(); !reported || mw != 1292 {
		t.Errorf(`published "1,292" parsed as (%v, %v), want (1292, true)`, mw, reported)
	}
}

// TestSNoIsNotAJoinKey is the reason this package exists. Of the plant names
// common to both workbooks, none keep their S.No, because FY2023-24 was
// re-sorted by technology.
func TestSNoIsNotAJoinKey(t *testing.T) {
	f := loadFixture(t)
	sno := map[string]map[string]int{}
	for fy, rows := range f.Years {
		sno[fy] = map[string]int{}
		for _, r := range rows {
			sno[fy][Normalize(r.PublishedName)] = r.SNo
		}
	}
	var common, stable int
	for name, a := range sno["2017-18"] {
		b, ok := sno["2023-24"][name]
		if !ok {
			continue
		}
		common++
		if a == b {
			stable++
		}
	}
	if common != 107 {
		t.Errorf("names common to both workbooks: got %d, want 107", common)
	}
	if stable != 0 {
		t.Errorf("plants keeping their S.No across years: got %d, want 0 (S.No is not a join key)", stable)
	}

	// The five documented moves, asserted individually so a regression names
	// the plant that broke.
	moves := []struct {
		substr       string
		from18, to24 int
	}{
		{"AES Lalpir", 1, 36},
		{"Allai Khwar", 56, 8},
		{"Kot Addu Power Company", 9, 33},
		{"Kohinoor Energy", 12, 35},
		{"Narowal Energy", 11, 58},
	}
	for _, mv := range moves {
		got18 := findSNo(t, f, "2017-18", mv.substr)
		got24 := findSNo(t, f, "2023-24", mv.substr)
		if got18 != mv.from18 || got24 != mv.to24 {
			t.Errorf("%s: S.No moved %d->%d, want %d->%d", mv.substr, got18, got24, mv.from18, mv.to24)
		}
	}
}

func findSNo(t *testing.T, f fixture, fy, substr string) int {
	t.Helper()
	var found []int
	for _, r := range f.Years[fy] {
		if containsFold(Normalize(r.PublishedName), substr) {
			found = append(found, r.SNo)
		}
	}
	if len(found) != 1 {
		t.Fatalf("FY%s: %q matched %d rows, want exactly 1", fy, substr, len(found))
	}
	return found[0]
}

func containsFold(hay, needle string) bool {
	return len(needle) > 0 && len(hay) >= len(needle) && indexFold(hay, needle) >= 0
}

func indexFold(hay, needle string) int {
	h, n := foldKey(hay), foldKey(needle)
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}

// TestKElectricAbsentFromFixture proves the absence claim against the fetched
// data rather than asserting it in prose: K-Electric's fleet is not in these
// workbooks at all, so a query for it must never be answered with a zero.
func TestKElectricAbsentFromFixture(t *testing.T) {
	f := loadFixture(t)
	probes := []string{"BQPS", "Korangi", "SITE", "Bin Qasim", "KESC", "K-Electric", "Karachi Electric"}
	var searched int
	for _, rows := range f.Years {
		searched += len(rows)
		for _, r := range rows {
			for _, p := range probes {
				if containsFold(Normalize(r.PublishedName), p) {
					t.Errorf("probe %q unexpectedly matched published name %q", p, Normalize(r.PublishedName))
				}
			}
		}
	}
	if searched != 241 {
		t.Errorf("searched %d published names, want 241 (108 + 133)", searched)
	}
	// Lucky Electric's Bin Qasim plant IS present, under its company name, so
	// the probe list above must not be read as "nothing at Bin Qasim".
	if _, ok := Resolve("Lucky Electric Power Company Limited (LEPCL)"); !ok {
		t.Error("Lucky Electric Power Company Limited (LEPCL) should resolve; it is the Bin Qasim coal plant that IS in this dataset")
	}
}
