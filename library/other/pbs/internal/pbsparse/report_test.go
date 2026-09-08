// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package pbsparse

import (
	"math"
	"testing"
)

func loadReport(t *testing.T) *Report {
	t.Helper()
	b := readFixture(t, "3.-SPI-Report-03.09.2026.xlsx")
	rep, err := ParseReportXLSX(b, "2026-09-03")
	if err != nil {
		t.Fatalf("ParseReportXLSX: %v", err)
	}
	return rep
}

// TestReportQuintiles pins the published index table, including the income
// bands, which PBS revises without a change log.
func TestReportQuintiles(t *testing.T) {
	rep := loadReport(t)
	if got, want := len(rep.Quintiles), 6; got != want {
		t.Fatalf("quintile rows = %d, want %d (Q1..Q5 plus Combined)", got, want)
	}
	byQ := map[string]QuintileIndex{}
	for _, q := range rep.Quintiles {
		byQ[q.Quintile] = q
	}
	q1, ok := byQ["Q1"]
	if !ok {
		t.Fatal("no Q1 row")
	}
	if math.Abs(q1.Index.Num-352.61) > 0.005 {
		t.Errorf("Q1 index = %.2f, want 352.61", q1.Index.Num)
	}
	if math.Abs(q1.BandHigh-17732) > 0.5 {
		t.Errorf("Q1 band high = %.0f, want 17732", q1.BandHigh)
	}
	comb, ok := byQ["COMBINED"]
	if !ok {
		t.Fatal("no Combined row")
	}
	if math.Abs(comb.Index.Num-363.42) > 0.005 {
		t.Errorf("Combined index = %.2f, want 363.42", comb.Index.Num)
	}
	// Q5's band is open-ended upward.
	if q5 := byQ["Q5"]; math.Abs(q5.BandLow-44175) > 0.5 {
		t.Errorf("Q5 band low = %.0f, want 44175", q5.BandLow)
	}
}

// TestReportTrend pins the rolling ten-week table.
func TestReportTrend(t *testing.T) {
	rep := loadReport(t)
	if got := len(rep.Trend); got != 10 {
		t.Errorf("trend points = %d, want 10", got)
	}
	last := rep.Trend[len(rep.Trend)-1]
	if last.WeekEnded != "03-09-2026" {
		t.Errorf("last trend week = %q, want 03-09-2026", last.WeekEnded)
	}
	if math.Abs(last.Combined.Num-363.42) > 0.005 {
		t.Errorf("last trend combined = %.2f, want 363.42", last.Combined.Num)
	}
}

// TestReportWeightTotalsSumTo100 is the source's own invariant: the three
// section TOTAL rows decompose household expenditure, so they must sum to 100
// on both weight columns. A deviation beyond rounding is a parse failure.
func TestReportWeightTotalsSumTo100(t *testing.T) {
	rep := loadReport(t)
	lowest, combined, sections := rep.WeightTotals()
	if sections != 3 {
		t.Fatalf("section TOTAL rows = %d, want 3 (increased/decreased/unchanged)", sections)
	}
	if math.Abs(lowest-100) > 0.01 {
		t.Errorf("lowest-quintile weights sum to %.4f, want 100.0000", lowest)
	}
	if math.Abs(combined-100) > 0.01 {
		t.Errorf("combined weights sum to %.4f, want 100.0000", combined)
	}
}

// TestReportSectionCountsAgree pins the free invariant the report states about
// itself: each heading declares how many items follow it.
func TestReportSectionCountsAgree(t *testing.T) {
	rep := loadReport(t)
	ok, detail := rep.SectionCountsAgree()
	if !ok {
		t.Errorf("section item counts disagree with declared counts: %v", detail)
	}
	if got, want := len(rep.Items), 51; got != want {
		t.Errorf("total item rows = %d, want %d", got, want)
	}
	// Measured on this release: 17 increased + 7 decreased + 27 unchanged.
	bySec := map[MovementSection]int{}
	for _, it := range rep.Items {
		bySec[it.Section]++
	}
	for sec, want := range map[MovementSection]int{
		SectionIncreased: 17, SectionDecreased: 7, SectionUnchanged: 27,
	} {
		if got := bySec[sec]; got != want {
			t.Errorf("section %s has %d items, want %d", sec, got, want)
		}
	}
}

// TestReportNationalPriceMatchesAnnexure cross-validates two independent PBS
// files for the same release: the report's national price column must equal the
// annexure's published National Average column.
//
// This is the check that caught a real defect. The annexure carries two
// adjacent groups titled "National Average" and "National Ave."; the first is
// the current week's min/avg/max, the second is the previous week's and last
// year's figures. Reading the wrong one returns 153.98 where 154.00 is correct
// — a plausible price, a different variable.
func TestReportNationalPriceMatchesAnnexure(t *testing.T) {
	rep := loadReport(t)
	ab := readFixture(t, "Annex_03.09.2026.xlsx")
	ax, err := ParseAnnexureXLSX(ab, "2026-09-03")
	if err != nil {
		t.Fatalf("annexure: %v", err)
	}

	var compared, mismatch int
	for _, it := range rep.Items {
		nat, ok := ax.NationalAvg[it.ItemDesc]
		if !ok || !nat.Present() || !it.NationalPrice.Present() {
			continue
		}
		compared++
		if math.Abs(nat.Num-it.NationalPrice.Num) > 0.01 {
			mismatch++
			if mismatch <= 6 {
				t.Errorf("national price differs for %q: annexure=%.2f report=%.2f",
					it.ItemDesc, nat.Num, it.NationalPrice.Num)
			}
		}
	}
	if compared < 40 {
		t.Errorf("only %d items cross-validated, want >= 40", compared)
	}
	if mismatch > 0 {
		t.Errorf("%d of %d items disagree between report and annexure", mismatch, compared)
	}
	t.Logf("report/annexure national-price cross-validation: compared=%d agree=%d", compared, compared-mismatch)
}

// TestReportPrevWeekIsDistinct pins that the report's previous-week column is
// kept separate from the current price, the same conflation the annexure parser
// had to be fixed for.
func TestReportPrevWeekIsDistinct(t *testing.T) {
	rep := loadReport(t)
	for _, it := range rep.Items {
		if it.ItemDesc != "Rice IRRI-6/9 (Sindh/Punjab)" {
			continue
		}
		if math.Abs(it.NationalPrice.Num-154.00) > 0.01 {
			t.Errorf("national price = %.2f, want 154.00", it.NationalPrice.Num)
		}
		if math.Abs(it.PricePrevWeek.Num-153.98) > 0.01 {
			t.Errorf("prev-week price = %.2f, want 153.98", it.PricePrevWeek.Num)
		}
		if math.Abs(it.PriceCorWeek.Num-155.51) > 0.01 {
			t.Errorf("corresponding-week price = %.2f, want 155.51", it.PriceCorWeek.Num)
		}
		return
	}
	t.Fatal("Rice IRRI-6/9 not found in report items")
}

func TestParseReportRejectsGarbage(t *testing.T) {
	if _, err := ParseReportXLSX([]byte("not a zip"), "2026-09-03"); err == nil {
		t.Fatal("expected an error for a non-xlsx payload")
	}
}
