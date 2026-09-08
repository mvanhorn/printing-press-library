// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package pbsparse

import (
	"math"
	"sort"
	"testing"
)

func loadAnnexure(t *testing.T) *Annexure {
	t.Helper()
	b := readFixture(t, "Annex_03.09.2026.xlsx")
	a, err := ParseAnnexureXLSX(b, "2026-09-03")
	if err != nil {
		t.Fatalf("ParseAnnexureXLSX: %v", err)
	}
	return a
}

// TestAnnexureStructure pins the shape against ground truth taken independently
// from the workbook's own sharedStrings before this parser existed: seventeen
// cities carrying codes 01..17, fifty-one Appendix-A items, and three stacked
// header bands (7 + 7 + 3 cities).
func TestAnnexureStructure(t *testing.T) {
	a := loadAnnexure(t)
	if got, want := len(a.Cities), 17; got != want {
		t.Errorf("cities = %d, want %d: %v", got, want, a.Cities)
	}
	if got, want := len(a.Items), 51; got != want {
		t.Errorf("items = %d, want %d", got, want)
	}
	if got, want := a.Blocks, 3; got != want {
		t.Errorf("stacked header bands = %d, want %d", got, want)
	}
}

// TestAllThreeBlocksContributeRows is the regression test for the trap that
// would otherwise be invisible: a parser reading ONE header row and applying it
// to every data row still produces 17 cities' worth of column names and wholly
// plausible numbers, but attributes roughly two thirds of the panel to the
// wrong city. Requiring rows from every band is what catches it.
func TestAllThreeBlocksContributeRows(t *testing.T) {
	a := loadAnnexure(t)
	byBlock := map[int]int{}
	citiesByBlock := map[int]map[string]bool{}
	for _, r := range a.Rows {
		byBlock[r.Block]++
		if citiesByBlock[r.Block] == nil {
			citiesByBlock[r.Block] = map[string]bool{}
		}
		citiesByBlock[r.Block][r.City] = true
	}
	if len(byBlock) != 3 {
		t.Fatalf("rows came from %d bands, want 3: %v", len(byBlock), byBlock)
	}
	for b, n := range byBlock {
		if n == 0 {
			t.Errorf("band %d contributed no rows", b)
		}
		if len(citiesByBlock[b]) == 0 {
			t.Errorf("band %d resolved no cities", b)
		}
	}
	// No city may appear in two different bands: the bands partition the cities.
	seen := map[string]int{}
	for b, cs := range citiesByBlock {
		for c := range cs {
			if prev, ok := seen[c]; ok {
				t.Errorf("city %q resolved in both band %d and band %d", c, prev, b)
			}
			seen[c] = b
		}
	}
}

// TestCityCodesResolved pins that every city carries its PBS numeric code, the
// only stable machine identifier the source provides.
func TestCityCodesResolved(t *testing.T) {
	a := loadAnnexure(t)
	codes := map[string]bool{}
	for _, r := range a.Rows {
		if r.CityCode == "" {
			t.Fatalf("city %q resolved without a code", r.City)
		}
		codes[r.CityCode] = true
	}
	if got, want := len(codes), 17; got != want {
		t.Errorf("distinct city codes = %d, want %d", got, want)
	}
	for _, want := range []string{"01", "05", "10", "16", "17"} {
		if !codes[want] {
			t.Errorf("missing expected city code %q", want)
		}
	}
}

// TestNullStatesKeptDistinct is the -16.6% regression test.
//
// Item 3 (Rice IRRI-6/9) is uncollected in three cities, written as numeric
// zero. The published National Ave. is 154.00. Averaging the zeros as prices
// gives 128.36, an error of -16.6%; excluding them gives 155.86. The parser
// must keep the states distinct so a caller can exclude them.
func TestNullStatesKeptDistinct(t *testing.T) {
	a := loadAnnexure(t)
	const item = "Rice IRRI-6/9 (Sindh/Punjab)"

	var present, zero []float64
	for _, r := range a.Rows {
		if r.ItemDesc != item || r.Stat != StatAvg {
			continue
		}
		switch r.State {
		case StatePresent:
			present = append(present, r.Num)
		case StateZero:
			zero = append(zero, 0)
		}
	}
	if len(zero) == 0 {
		t.Fatalf("expected zero-state cells for %q; the three-state distinction is not being preserved", item)
	}
	if len(present)+len(zero) != 17 {
		t.Errorf("avg cells for %q = %d present + %d zero, want 17 total", item, len(present), len(zero))
	}

	sum := 0.0
	for _, v := range present {
		sum += v
	}
	excl := sum / float64(len(present))
	incl := sum / float64(len(present)+len(zero))

	if math.Abs(excl-155.86) > 0.5 {
		t.Errorf("mean excluding zeros = %.2f, want ~155.86", excl)
	}
	if math.Abs(incl-128.36) > 0.5 {
		t.Errorf("mean including zeros as prices = %.2f, want ~128.36", incl)
	}
	// The whole point: the two differ materially, so the distinction matters.
	if rel := (incl - excl) / excl * 100; rel > -10 {
		t.Errorf("coercing zeros shifts the mean by only %.1f%%, expected about -16%%", rel)
	}
}

// TestNationalAvgCaptured pins that the PUBLISHED national average is read
// through rather than recomputed. PBS weights cities and does not publish the
// city weight vector, so a rebuilt figure cannot reproduce this column.
func TestNationalAvgCaptured(t *testing.T) {
	a := loadAnnexure(t)
	v, ok := a.NationalAvg["Rice IRRI-6/9 (Sindh/Punjab)"]
	if !ok {
		t.Fatal("no published National Ave. captured for Rice IRRI-6/9")
	}
	// 154.00 is the AVG column of the "National Average" min/avg/max triplet
	// (M:O in the third band -> 110 / 154 / 200). The adjacent "National Ave."
	// group whose sub-labels are Prv. Wk / Cor. Wk holds 153.98 and 155.51:
	// last week's and last year's national figures. Picking the wrong one of
	// those is a 0.02 numerical difference and a completely different variable.
	if math.Abs(v.Num-154.00) > 0.01 {
		t.Errorf("published national avg = %.2f, want 154.00 (the AVG of the National Average triplet, not the adjacent Prv. Wk column)", v.Num)
	}
}

// TestDerivedSeriesSeparated pins that the extra Appendix-A groups are captured
// as distinct named series rather than collapsed with the national price.
func TestDerivedSeriesSeparated(t *testing.T) {
	a := loadAnnexure(t)
	const item = "Rice IRRI-6/9 (Sindh/Punjab)"
	d, ok := a.Derived[item]
	if !ok {
		t.Fatalf("no derived series captured for %q", item)
	}
	for _, name := range []string{"national_prev_week", "national_corresponding_week"} {
		if _, ok := d[name]; !ok {
			t.Errorf("missing derived series %q; have %v", name, keysOf(d))
		}
	}
	if v, ok := d["national_prev_week"]; ok && math.Abs(v.Num-153.98) > 0.01 {
		t.Errorf("national_prev_week = %.2f, want 153.98", v.Num)
	}
	if v, ok := d["national_corresponding_week"]; ok && math.Abs(v.Num-155.51) > 0.01 {
		t.Errorf("national_corresponding_week = %.2f, want 155.51", v.Num)
	}
	// The current national average must NOT equal the previous week's value.
	if nat, ok := a.NationalAvg[item]; ok {
		if pw, ok2 := d["national_prev_week"]; ok2 && math.Abs(nat.Num-pw.Num) < 0.001 {
			t.Errorf("national avg %.2f equals prev-week %.2f: the two groups are being conflated", nat.Num, pw.Num)
		}
	}
}

func keysOf(m map[string]Value) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestCensusAccountsForEveryCell pins that the census totals the cells the
// parser actually looked at, so `coverage --state-census` cannot under-report.
func TestCensusAccountsForEveryCell(t *testing.T) {
	a := loadAnnexure(t)
	if a.Census.Total() == 0 {
		t.Fatal("census counted no cells")
	}
	if a.Census.Present == 0 {
		t.Fatal("census counted no present values")
	}
	if a.Census.Zero == 0 {
		t.Error("census counted no zero-state cells, but the fixture has them")
	}
	if a.Census.NA == 0 {
		t.Error("census counted no N.A. cells, but Appendix-B of the fixture has them")
	}
}

// TestMinAvgMaxOrdering asserts the source's own arithmetic invariant on every
// fully-present triplet. A violation is a parse failure, not a data point.
func TestMinAvgMaxOrdering(t *testing.T) {
	a := loadAnnexure(t)
	type key struct{ city, item string }
	trip := map[key]map[Stat]float64{}
	for _, r := range a.Rows {
		if !r.Present() {
			continue
		}
		k := key{r.City, r.ItemDesc}
		if trip[k] == nil {
			trip[k] = map[Stat]float64{}
		}
		trip[k][r.Stat] = r.Num
	}
	checked, bad := 0, 0
	for k, m := range trip {
		lo, okLo := m[StatMin]
		av, okAv := m[StatAvg]
		hi, okHi := m[StatMax]
		if !okLo || !okAv || !okHi {
			continue
		}
		checked++
		if lo > av+1e-9 || av > hi+1e-9 {
			bad++
			if bad <= 5 {
				t.Errorf("min<=avg<=max violated for %s / %s: %.2f, %.2f, %.2f", k.city, k.item, lo, av, hi)
			}
		}
	}
	if checked < 500 {
		t.Errorf("only %d complete triplets checked, expected many more", checked)
	}
	if bad > 0 {
		t.Errorf("%d of %d triplets violate min<=avg<=max", bad, checked)
	}
}

func TestParseAnnexureRejectsGarbage(t *testing.T) {
	if _, err := ParseAnnexureXLSX([]byte("not a zip"), "2026-09-03"); err == nil {
		t.Fatal("expected an error for a non-xlsx payload")
	}
}
