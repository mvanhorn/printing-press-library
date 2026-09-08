// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package mufap

import (
	"math"
	"testing"
)

func almost(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestPercentile(t *testing.T) {
	if _, ok := Percentile(nil, 50); ok {
		t.Error("empty slice must report ok=false, not a silent 0 — no funds reported is not a rate of zero")
	}
	if v, ok := Percentile([]float64{7}, 50); !ok || v != 7 {
		t.Errorf("single value = (%v,%v), want (7,true)", v, ok)
	}
	// Even count: median interpolates between the two middle values.
	if v, ok := Median([]float64{1, 2, 3, 4}); !ok || !almost(v, 2.5) {
		t.Errorf("Median([1 2 3 4]) = %v, want 2.5", v)
	}
	if v, ok := Median([]float64{5, 1, 3}); !ok || v != 3 {
		t.Errorf("Median must sort first; got %v, want 3", v)
	}
	vs := []float64{10, 20, 30, 40, 50}
	if v, _ := Percentile(append([]float64(nil), vs...), 0); v != 10 {
		t.Errorf("p0 = %v, want 10", v)
	}
	if v, _ := Percentile(append([]float64(nil), vs...), 100); v != 50 {
		t.Errorf("p100 = %v, want 50", v)
	}
}

func TestCategoryClassifiers(t *testing.T) {
	cases := []struct {
		cat     string
		mm, ann bool
	}{
		{"Money Market (Annualized Return )", true, true},
		{"Shariah Compliant Money Market (Annualized Return )", true, true},
		{"VPS-Money Market (Annualized Return )", true, true},
		{"VPS-Shariah Compliant Money Market (Annualized Return )", true, true},
		{"Equity (Absolute Return )", false, false},
		{"Income (Annualized Return )", false, true},
	}
	for _, c := range cases {
		if got := IsMoneyMarketCategory(c.cat); got != c.mm {
			t.Errorf("IsMoneyMarketCategory(%q) = %v, want %v", c.cat, got, c.mm)
		}
		if got := IsAnnualizedCategory(c.cat); got != c.ann {
			t.Errorf("IsAnnualizedCategory(%q) = %v, want %v", c.cat, got, c.ann)
		}
	}
}

func TestDeriveRate(t *testing.T) {
	rows := []map[string]string{
		{"Category": "Money Market (Annualized Return )", "30 Days": "10.50"},
		{"Category": "Money Market (Annualized Return )", "30 Days": "10.70"},
		{"Category": "Shariah Compliant Money Market (Annualized Return )", "30 Days": "10.90"},
		// Equity is an ABSOLUTE return — mixing it into a yield median would
		// compare a price change against an annualized rate.
		{"Category": "Equity (Absolute Return )", "30 Days": "-2.31"},
		// Income is annualized but is not money market.
		{"Category": "Income (Annualized Return )", "30 Days": "13.10"},
		// Did not report: must be excluded, never zero-filled.
		{"Category": "Money Market (Annualized Return )", "30 Days": "-"},
		// Out-of-band artefacts MUFAP occasionally emits.
		{"Category": "Money Market (Annualized Return )", "30 Days": "0.0000"},
		{"Category": "Money Market (Annualized Return )", "30 Days": "9999.00"},
	}
	pt := DeriveRate("2026-09-04", "30 Days", rows)
	// Equity (absolute return), Income (not money market) and the
	// non-reporting "-" are excluded by SELECTION. The 0.0000 placeholder and
	// the 9999.00 artefact are values that parsed, so they are kept and
	// flagged rather than deleted: the median is robust to them, and deleting
	// them would reshape the percentiles with nothing in the payload to say so.
	if pt.FundCount != 5 {
		t.Fatalf("FundCount = %d, want 5 (equity, income and the non-reporting row excluded by selection; the 0.0 and 9999 artefacts kept and flagged)", pt.FundCount)
	}
	if !almost(pt.MedianYield, 10.70) {
		t.Errorf("MedianYield = %v, want 10.70 — the median must be unmoved by the two artefacts it now keeps", pt.MedianYield)
	}
	if !almost(pt.Max, 9999.00) || !almost(pt.Min, 0) {
		t.Errorf("extremes = [%v,%v], want [0,9999] — artefacts must stay visible", pt.Min, pt.Max)
	}
	if pt.OutlierRows < 1 {
		t.Errorf("OutlierRows = %d, want >=1 — the 9999.00 artefact must be flagged", pt.OutlierRows)
	}
	if pt.Date != "2026-09-04" || pt.Column != "30 Days" {
		t.Errorf("provenance lost: %+v", pt)
	}
}

func TestDeriveRateEmpty(t *testing.T) {
	// A date with no money-market rows must report zero funds, not a zero rate.
	pt := DeriveRate("2026-09-06", "30 Days", []map[string]string{
		{"Category": "Equity (Absolute Return )", "30 Days": "1.10"},
	})
	if pt.FundCount != 0 {
		t.Fatalf("FundCount = %d, want 0", pt.FundCount)
	}
	if pt.MedianYield != 0 {
		t.Errorf("MedianYield = %v; an empty cross-section must leave the rate at its zero value with FundCount 0 to mark it", pt.MedianYield)
	}
}

func TestDeriveDispersion(t *testing.T) {
	rows := []map[string]string{
		{"Category": "Equity (Absolute Return )", "YTD": "10"},
		{"Category": "Equity (Absolute Return )", "YTD": "20"},
		{"Category": "Equity (Absolute Return )", "YTD": "30"},
		{"Category": "Money Market (Annualized Return )", "YTD": "999"},
	}
	d := DeriveDispersion("2026-09-04", "Equity", "YTD", rows)
	if d.FundCount != 3 {
		t.Fatalf("FundCount = %d, want 3 (money market excluded by category filter)", d.FundCount)
	}
	if !almost(d.Median, 20) {
		t.Errorf("Median = %v, want 20", d.Median)
	}
	if !almost(d.Spread, d.P90-d.P10) {
		t.Errorf("Spread %v != P90-P10 %v", d.Spread, d.P90-d.P10)
	}
}

func TestCheckAllocationPasses(t *testing.T) {
	// ABL Stock Fund, 7-2026, as actually returned by MUFAP:
	// 6.5 + 95.16 + 0.31 = 101.97 assets, 1.97 liabilities, nets to 100.
	row := map[string]float64{
		"Cashpercent":                     6.5,
		"StocksOREquitiesPercent":         95.16,
		"OtherIncludingReceivablePercent": 0.31,
		"LaibilitiesPercent":              1.97,
	}
	c := CheckAllocation("ABL Stock Fund", "7-2026", row, 0.01)
	if !c.PercentsPopulated {
		t.Fatal("PercentsPopulated = false, want true")
	}
	if !almost(c.NetPercent, 100) {
		t.Errorf("NetPercent = %v, want 100", c.NetPercent)
	}
	if !c.Pass {
		t.Errorf("Pass = false, want true (deviation %v)", c.Deviation)
	}
}

func TestCheckAllocationFailsWhenComponentsDoNotNet(t *testing.T) {
	// ABL Stock Fund, 3-2026: assets 103.82, liabilities 1.77 -> 102.05.
	// MUFAP's own TotalPercentage field still reads "100%" for this month,
	// which is exactly why the site's label cannot be used as the check.
	row := map[string]float64{
		"Cashpercent":             8.00,
		"StocksOREquitiesPercent": 95.82,
		"LaibilitiesPercent":      1.77,
	}
	c := CheckAllocation("ABL Stock Fund", "3-2026", row, 0.01)
	if c.Pass {
		t.Error("Pass = true, want false — 102.05 must not be accepted as 100")
	}
	if !almost(c.Deviation, 2.05) {
		t.Errorf("Deviation = %v, want 2.05", c.Deviation)
	}
}

func TestCheckAllocationUnpopulatedIsNotAFailure(t *testing.T) {
	// Every percent column reads 0.0 for months before roughly 2024 while the
	// amount columns stay correct. That must be reported as not-populated so
	// the caller falls back to amounts, NOT as a catastrophic invariant break.
	row := map[string]float64{
		"Cashpercent":             0,
		"StocksOREquitiesPercent": 0,
		"LaibilitiesPercent":      0,
	}
	c := CheckAllocation("ABL Stock Fund", "6-2016", row, 0.01)
	if c.PercentsPopulated {
		t.Error("PercentsPopulated = true, want false for an all-zero historical month")
	}
	if c.Pass {
		t.Error("Pass = true; an unpopulated month can neither pass nor fail the invariant")
	}
}

// deriveTestRow builds one MUFAP-shaped row: a category label plus a single
// value cell, written the way MUFAP writes it (accounting negatives included).
func deriveTestRow(category, column, value string) map[string]string {
	return map[string]string{"Category": category, column: value}
}

const deriveTestMMCat = "Money Market (Annualized Return )"

// TestOutliersAreFlaggedNotDeleted pins the single most important property of
// both derivations: a value that parses is NEVER removed from the statistic.
//
// An earlier version bounded dispersion to a fixed +/-200pp. Measured against
// the real mirror that rejected 8 of 9 VPS-Equity funds on the "3 Years"
// column -- where Pakistani equity legitimately sits at 190-305% -- and left
// only the launch artefact at -100.00 standing. It deleted the market and kept
// the artefact. A fence taken from the sample's own quartiles carries no scale,
// so the same code is correct for a 10% yield and a 300% cumulative return.
func TestOutliersAreFlaggedNotDeleted(t *testing.T) {
	// A real 3-year equity cross-section plus one launch artefact.
	vals := []string{"190.10", "212.40", "233.75", "247.20", "260.80",
		"271.30", "288.90", "305.10", "(100.00)"}
	rows := make([]map[string]string, 0, len(vals))
	for _, v := range vals {
		rows = append(rows, deriveTestRow("Equity (Absolute Return )", "3 Years", v))
	}
	d := DeriveDispersion("2026-09-04", "Equity", "3 Years", rows)

	if d.FundCount != 9 {
		t.Fatalf("FundCount = %d, want 9 — every value that parsed must remain in the statistic", d.FundCount)
	}
	if d.OutlierRows != 1 {
		t.Errorf("OutlierRows = %d, want 1 (the -100.00 launch artefact)", d.OutlierRows)
	}
	// The market, not the artefact, must set the median.
	if d.Median < 190 || d.Median > 310 {
		t.Errorf("Median = %v, want the real cross-section (190..310), not the artefact", d.Median)
	}
	// The artefact stays visible rather than being silently removed.
	if !almost(d.Min, -100) {
		t.Errorf("Min = %v, want -100 — the artefact must remain visible in the payload", d.Min)
	}
	if !almost(d.Max, 305.10) {
		t.Errorf("Max = %v, want 305.10", d.Max)
	}
	if !(d.FenceLow < d.FenceHigh) {
		t.Errorf("fences not ordered: [%v, %v]", d.FenceLow, d.FenceHigh)
	}
}

// TestDeriveRateKeepsTheLeftTail pins the rate half of the same property. The
// fixed (0, 60] band silently moved the reported p10 from 8.593 to 9.165 by
// dropping two real accounting-negative yields -- clipping exactly the tail
// that p10 exists to describe.
func TestDeriveRateKeepsTheLeftTail(t *testing.T) {
	vals := []string{"(1.92)", "(0.20)", "8.46", "9.50", "10.40", "10.52", "11.20", "11.60"}
	rows := make([]map[string]string, 0, len(vals))
	for _, v := range vals {
		rows = append(rows, deriveTestRow(deriveTestMMCat, "30 Days", v))
	}
	pt := DeriveRate("2026-09-04", "30 Days", rows)

	if pt.FundCount != 8 {
		t.Fatalf("FundCount = %d, want 8 — the negative yields are data, not parse failures", pt.FundCount)
	}
	if !almost(pt.Min, -1.92) {
		t.Errorf("Min = %v, want -1.92 — the left tail must survive into the payload", pt.Min)
	}
	// The median still tracks the policy rate despite the tail being present.
	if pt.MedianYield < 9 || pt.MedianYield > 11.5 {
		t.Errorf("MedianYield = %v, want ~10 — the median is robust to the tail it now keeps", pt.MedianYield)
	}
	if pt.P10 > pt.MedianYield {
		t.Errorf("P10 %v above median %v", pt.P10, pt.MedianYield)
	}
}

// TestBothDerivationsAgreeOnTheSameRows is the regression for the original
// finding: the two functions disagreed by two orders of magnitude on the tails
// of the same column of the same mirror. They now use one rule, so the only
// legitimate difference is which rows each selects.
func TestBothDerivationsAgreeOnTheSameRows(t *testing.T) {
	vals := []string{"(1622.22)", "(1.92)", "8.46", "9.50", "10.40", "10.52", "11.20"}
	rows := make([]map[string]string, 0, len(vals))
	for _, v := range vals {
		rows = append(rows, deriveTestRow(deriveTestMMCat, "30 Days", v))
	}
	pt := DeriveRate("2026-09-04", "30 Days", rows)
	d := DeriveDispersion("2026-09-04", "Money Market", "30 Days", rows)

	if pt.FundCount != d.FundCount {
		t.Fatalf("FundCount differs: rate %d vs dispersion %d — same rows, same selection", pt.FundCount, d.FundCount)
	}
	if !almost(pt.Min, d.Min) || !almost(pt.Max, d.Max) {
		t.Errorf("extremes differ: rate [%v,%v] vs dispersion [%v,%v]", pt.Min, pt.Max, d.Min, d.Max)
	}
	if pt.OutlierRows != d.OutlierRows {
		t.Errorf("OutlierRows differs: rate %d vs dispersion %d", pt.OutlierRows, d.OutlierRows)
	}
	if pt.OutlierRows < 1 {
		t.Errorf("OutlierRows = %d, want >=1 — the -1622.22 artefact must be flagged", pt.OutlierRows)
	}
}
