// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package mufap

import (
	"os"
	"path/filepath"
	"testing"
)

func readNetSalesFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "netsales", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

// TestParseTableReturnsZeroRowsOnNetSalesPages pins the reason netsales.go
// exists at all. ParseTable flattens the spanning title row into its header
// list, so its row-matching count is wrong and it yields ZERO rows with NO
// error. If a future change makes ParseTable handle these pages, this test
// fails and netsales.go's own header resolution can be reconsidered -- but
// until then, silently swapping the parsers would silently empty the series.
func TestParseTableReturnsZeroRowsOnNetSalesPages(t *testing.T) {
	for _, name := range []string{"monthly-2024-05.html", "investor-2024-05.html"} {
		tbl, err := ParseTable(readNetSalesFixture(t, name))
		if err != nil {
			t.Fatalf("%s: ParseTable errored (%v); it is expected to succeed-but-empty", name, err)
		}
		if len(tbl.Rows) != 0 {
			t.Fatalf("%s: ParseTable returned %d rows; netsales.go's premise no longer holds", name, len(tbl.Rows))
		}
	}
}

// TestParseNetSalesGroundTruth asserts against counts measured independently
// and stored in research.db by a separate acquisition script, so the parser is
// checked against an outside observation rather than against itself.
//
//	month     rows  reporting  residSales  residRed  bound  reconciles
//	2020-06     39          0           -         -      -  Total row all dashes
//	2024-05     39         14         0.0       0.0    7.5  yes
//	2025-06     40          8         0.0       0.0    4.5  yes
//	2025-12     40          8         2.0       1.0    4.5  yes (fixed 1.5 said no)
//	2026-04     40          7         0.0       2.0    4.0  yes (fixed 1.5 said no)
func TestParseNetSalesGroundTruth(t *testing.T) {
	cases := []struct {
		file        string
		period      string
		wantRows    int
		wantReport  int
		wantTotal   bool
		wantChecked bool
		wantOK      bool
	}{
		// 2020-06 DOES render a Total row; every figure in it is a dash. That
		// is why wantTotal is true while wantChecked is false.
		{"monthly-2020-06.html", "2020-06", 39, 0, true, false, false},
		{"monthly-2024-05.html", "2024-05", 39, 14, true, true, true},
		{"monthly-2025-06.html", "2025-06", 40, 8, true, true, true},
		{"monthly-2025-12.html", "2025-12", 40, 8, true, true, true},
		{"monthly-2026-04.html", "2026-04", 40, 7, true, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.period, func(t *testing.T) {
			ns, err := ParseNetSales(readNetSalesFixture(t, tc.file), tc.period)
			if err != nil {
				t.Fatalf("ParseNetSales: %v", err)
			}
			if len(ns.Rows) != tc.wantRows {
				t.Errorf("rows = %d, want %d (research.db ground truth)", len(ns.Rows), tc.wantRows)
			}
			if ns.Reporting != tc.wantReport {
				t.Errorf("reporting = %d, want %d (research.db ground truth)", ns.Reporting, tc.wantReport)
			}
			if (ns.Total != nil) != tc.wantTotal {
				t.Errorf("total present = %v, want %v", ns.Total != nil, tc.wantTotal)
			}
			if ns.Invariant.Checked != tc.wantChecked {
				t.Errorf("invariant checked = %v, want %v", ns.Invariant.Checked, tc.wantChecked)
			}
			if ns.Invariant.Checked && ns.Invariant.OK != tc.wantOK {
				t.Errorf("invariant ok = %v, want %v (residS=%.1f residR=%.1f bound=%.1f)",
					ns.Invariant.OK, tc.wantOK,
					ns.Invariant.ResidSales, ns.Invariant.ResidRedemptions, ns.Invariant.RoundingBound)
			}
			if ns.Period != tc.period {
				t.Errorf("period = %q, want %q", ns.Period, tc.period)
			}
		})
	}
}

// TestNetSalesTotalRowIsNotData is the trap that would double-count a month.
func TestNetSalesTotalRowIsNotData(t *testing.T) {
	ns, err := ParseNetSales(readNetSalesFixture(t, "monthly-2024-05.html"), "2024-05")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range ns.Rows {
		if r.Sector == "Total" || r.SectorID == 100 {
			t.Fatalf("Total row leaked into Rows: %+v", r)
		}
	}
	if ns.Total == nil || ns.Total.Sales == nil {
		t.Fatal("Total row was not captured separately")
	}
	if got := *ns.Total.Sales; got != 68656 {
		t.Errorf("total sales = %v, want 68656 (research.db)", got)
	}
	if got := *ns.Total.NetSales; got != 25523 {
		t.Errorf("total net = %v, want 25523 (research.db)", got)
	}
}

// TestNetSalesVPSDuplicationDropped pins the ~13% double-count.
func TestNetSalesVPSDuplicationDropped(t *testing.T) {
	ns, err := ParseNetSales(readNetSalesFixture(t, "monthly-2026-04.html"), "2026-04")
	if err != nil {
		t.Fatal(err)
	}
	if ns.VPSDuplicatesDropped == 0 {
		t.Fatal("expected at least one duplicated pension row to be dropped")
	}
	// After de-duplication no (sector, category) pair may repeat.
	seen := map[string]int{}
	for _, r := range ns.Rows {
		seen[r.Sector+"|"+r.Category]++
	}
	for k, n := range seen {
		if n > 1 && !netSalesPension[splitSector(k)] {
			t.Errorf("non-pension pair %q appears %d times after de-duplication", k, n)
		}
	}
}

func splitSector(k string) string {
	for i := 0; i < len(k); i++ {
		if k[i] == '|' {
			return k[:i]
		}
	}
	return k
}

// TestNetSalesDashIsMissingNotZero is the convention that is INVERTED from CDC.
func TestNetSalesDashIsMissingNotZero(t *testing.T) {
	ns, err := ParseNetSales(readNetSalesFixture(t, "monthly-2026-04.html"), "2026-04")
	if err != nil {
		t.Fatal(err)
	}
	var missing, zero int
	for _, r := range ns.Rows {
		if r.NetSales == nil {
			missing++
		} else if *r.NetSales == 0 {
			zero++
		}
	}
	if missing == 0 {
		t.Fatal("expected non-reporting categories to parse as nil, not 0")
	}
	// 40 rows, 7 reporting -> 33 non-reporting.
	if missing != 33 {
		t.Errorf("missing net = %d, want 33 (research.db: 40 rows, 7 reporting)", missing)
	}
}

// TestNetSalesRoundingAwareInvariant is the correction to a fixed tolerance.
// 2026-04's sales residual is exactly 0 while its redemption residual is 2.0,
// so a one-sided check would also have passed it for the wrong reason.
func TestNetSalesRoundingAwareInvariant(t *testing.T) {
	ns, err := ParseNetSales(readNetSalesFixture(t, "monthly-2026-04.html"), "2026-04")
	if err != nil {
		t.Fatal(err)
	}
	inv := ns.Invariant
	if !inv.Checked || !inv.OK {
		t.Fatalf("2026-04 must reconcile under the rounding bound: %+v", inv)
	}
	if abs(inv.ResidSales) != 0 {
		t.Errorf("resid sales = %.1f, want 0.0", inv.ResidSales)
	}
	if abs(inv.ResidRedemptions) != 2 {
		t.Errorf("resid redemptions = %.1f, want 2.0", inv.ResidRedemptions)
	}
	if inv.RoundingBound <= 2 {
		t.Errorf("rounding bound = %.1f, must exceed the 2.0 residual it forgives", inv.RoundingBound)
	}
	// A fixed 1.5 tolerance would have rejected this month.
	if abs(inv.ResidRedemptions) < 1.5 {
		t.Error("fixture no longer exercises the false-alarm case")
	}
}

// TestParseNetSalesInvestorNineClasses pins the taxonomy width and the column
// arithmetic: 3 keys + 2 per class.
func TestParseNetSalesInvestorNineClasses(t *testing.T) {
	want := []string{
		"Individuals", "Banking & Financial Institutions", "Provident fund",
		"Gratuity fund", "Pension fund", "Public Limited Companies",
		"Associated Companies", "Fund of funds", "Other",
	}
	for _, name := range []string{"investor-2024-05.html", "investor-2026-04.html", "investor-2020-06.html"} {
		inv, err := ParseNetSalesInvestor(readNetSalesFixture(t, name), "2024-05")
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(inv.Classes) != 9 {
			t.Fatalf("%s: %d investor classes, want 9: %q", name, len(inv.Classes), inv.Classes)
		}
		for i := range want {
			if inv.Classes[i] != want[i] {
				t.Errorf("%s: class[%d] = %q, want %q", name, i, inv.Classes[i], want[i])
			}
		}
		if len(inv.Rows)%9 != 0 {
			t.Errorf("%s: %d rows is not a multiple of 9 classes", name, len(inv.Rows))
		}
	}
}

// TestParseNetSalesInvestorHasNoNetColumn guards the derivation rule.
func TestParseNetSalesInvestorHasNoNetColumn(t *testing.T) {
	inv, err := ParseNetSalesInvestor(readNetSalesFixture(t, "investor-2024-05.html"), "2024-05")
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Total) != 9 {
		t.Fatalf("Total row should yield one entry per class, got %d", len(inv.Total))
	}
	for _, r := range inv.Rows {
		if r.Sector == "Total" {
			t.Fatalf("Total leaked into Rows: %+v", r)
		}
	}
	if inv.Note == "" {
		t.Error("the partial-slice warning must travel with the data")
	}
}

// TestNetSalesChallengeIsAnError is the difference between "no data this
// month" and "we were blocked". Cloudflare challenges these paths
// intermittently, so conflating the two would silently punch holes in a panel.
func TestNetSalesChallengeIsAnError(t *testing.T) {
	doc := readNetSalesFixture(t, "challenge-403.html")
	if !IsChallengePage(doc) {
		t.Fatal("challenge fixture not detected as a challenge")
	}
	if _, err := ParseNetSales(doc, "2026-05"); err == nil {
		t.Fatal("ParseNetSales returned no error on a challenge page")
	}
	if _, err := ParseNetSalesInvestor(doc, "2026-05"); err == nil {
		t.Fatal("ParseNetSalesInvestor returned no error on a challenge page")
	}
	// And a real page must not be mistaken for one.
	if IsChallengePage(readNetSalesFixture(t, "monthly-2024-05.html")) {
		t.Fatal("a real page was classified as a challenge")
	}
}

// TestNetSalesEmptyMonthIsNotAnError: MUFAP renders a page for every month,
// so an empty month is a real observation and must not read as a failure.
func TestNetSalesEmptyMonthIsNotAnError(t *testing.T) {
	ns, err := ParseNetSales(readNetSalesFixture(t, "monthly-2020-06.html"), "2020-06")
	if err != nil {
		t.Fatalf("an empty month must parse, not error: %v", err)
	}
	if !ns.HasTotalRow {
		t.Error("2020-06 does render a Total row (all dashes); HasTotalRow should be true")
	}
	if ns.Total == nil {
		t.Fatal("Total should be captured even when blank")
	}
	if ns.Total.Sales != nil || ns.Total.Redemptions != nil || ns.Total.NetSales != nil {
		t.Errorf("2020-06 Total figures should all be nil (dashes), got %+v", *ns.Total)
	}
	if ns.Reporting != 0 {
		t.Errorf("reporting = %d, want 0", ns.Reporting)
	}
	if len(ns.Rows) == 0 {
		t.Error("the category scaffold rows should still be present")
	}
	if ns.Invariant.Checked {
		t.Error("invariant must not claim to have checked a month with no Total")
	}
	if ns.Note == "" {
		t.Error("an empty month must say so in-band")
	}
}

// TestNetSalesTitleConfirmsThePeriod: the page is served for any (Month,Year),
// so the rendered title is the only in-band confirmation of what came back.
func TestNetSalesTitleConfirmsThePeriod(t *testing.T) {
	ns, err := ParseNetSales(readNetSalesFixture(t, "monthly-2025-12.html"), "2025-12")
	if err != nil {
		t.Fatal(err)
	}
	if ns.Title != "December 2025 (in PKR millions)" {
		t.Errorf("title = %q, want the December 2025 header", ns.Title)
	}
}

func TestNetSalesPeriodFormat(t *testing.T) {
	if got := NetSalesPeriod(5, 2026); got != "2026-05" {
		t.Errorf("NetSalesPeriod(5, 2026) = %q, want 2026-05", got)
	}
	if got := NetSalesPeriod(12, 2025); got != "2025-12" {
		t.Errorf("NetSalesPeriod(12, 2025) = %q, want 2025-12", got)
	}
}
