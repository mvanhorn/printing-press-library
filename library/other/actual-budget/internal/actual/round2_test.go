// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual_test

import (
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
)

// groceriesSept collects Groceries' September 2026 spending from every report
// that measures it: spending, trends, cashflow (total), budget-vs-actual and
// template status.
type groceriesSept struct {
	spending, trends, bva, templates, cashflow int64
}

func measureGroceriesSept(t *testing.T, l *actual.Ledger) groceriesSept {
	t.Helper()
	ctx := context.Background()
	sept := actual.ReportScope{From: 20260901, To: 20260930}
	var g groceriesSept
	sp, err := l.Spending(ctx, sept, "category")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range sp.Rows {
		if r.ID == "cat-groceries" {
			g.spending = r.Spent
		}
	}
	tr, err := l.Trends(ctx, actual.ReportScope{From: 20260801, To: 20260930}, "cat-groceries")
	if err != nil {
		t.Fatal(err)
	}
	if len(tr) != 1 || tr[0].Series[len(tr[0].Series)-1].Month != "2026-09" {
		t.Fatalf("trends = %+v", tr)
	}
	g.trends = tr[0].Series[len(tr[0].Series)-1].Spent
	cf, err := l.Cashflow(ctx, sept)
	if err != nil {
		t.Fatal(err)
	}
	g.cashflow = cf[0].Spending
	bva, err := l.BudgetVsActual(ctx, 202609, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range bva.Rows {
		if r.CategoryID == "cat-groceries" {
			g.bva = r.Spent
		}
	}
	ts, err := l.TemplateStatuses(ctx, "2026-09")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range ts {
		if r.CategoryID == "cat-groceries" {
			g.templates = r.Spent
		}
	}
	return g
}

// A refund into an expense category reduces spending identically in all five
// reports.
func TestRefundNettingAgreesAcrossReports(t *testing.T) {
	base := measureGroceriesSept(t, fixtureLedger(t))
	l := mutatedFixtureLedger(t,
		`INSERT INTO transactions (id,acct,category,amount,description,date,sort_order,cleared) VALUES
			('txn-r2-refund','acct-card','cat-groceries',1000,'pay-kroger',20260915,20,1)`)
	got := measureGroceriesSept(t, l)

	want := base.bva - 1000
	if base.spending != base.bva || base.trends != base.bva || base.templates != base.bva {
		t.Fatalf("baseline disagrees: %+v", base)
	}
	if got.spending != want || got.trends != want || got.bva != want || got.templates != want {
		t.Fatalf("groceries after refund = %+v, want %d everywhere", got, want)
	}
	if got.cashflow != base.cashflow-1000 {
		t.Fatalf("cashflow spending = %d, want %d", got.cashflow, base.cashflow-1000)
	}
}

// Template status uses the budget-vs-actual activity definition: off-budget
// accounts and starting balances never count.
func TestTemplateStatusesMatchBudgetVsActualActivity(t *testing.T) {
	l := mutatedFixtureLedger(t,
		`INSERT INTO transactions (id,acct,category,amount,description,date,sort_order,cleared) VALUES
			('txn-r2-offbudget','acct-broker','cat-groceries',-7000,'pay-kroger',20260912,21,1)`,
		`INSERT INTO transactions (id,acct,category,amount,description,date,starting_balance_flag,sort_order,cleared) VALUES
			('txn-r2-start','acct-checking','cat-groceries',-3000,NULL,20260902,1,22,1)`)
	ctx := context.Background()
	bva, err := l.BudgetVsActual(ctx, 202609, "")
	if err != nil {
		t.Fatal(err)
	}
	spent := map[string]int64{}
	for _, r := range bva.Rows {
		spent[r.CategoryID] = r.Spent
	}
	if spent["cat-groceries"] != 1250 {
		t.Fatalf("budget-vs-actual groceries = %d, want 1250", spent["cat-groceries"])
	}
	ts, err := l.TemplateStatuses(ctx, "2026-09")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range ts {
		if r.Spent != spent[r.CategoryID] {
			t.Errorf("%s: templates spent %d != budget-vs-actual %d", r.Category, r.Spent, spent[r.CategoryID])
		}
		if r.CategoryID == "cat-groceries" && r.BalanceHint != r.Budgeted-r.Spent {
			t.Errorf("groceries balance hint = %d, want %d", r.BalanceHint, r.Budgeted-r.Spent)
		}
	}
}

func TestPayeeSimilarityWordPrefixNeedsGenericSuffix(t *testing.T) {
	distinct := [][2]string{
		{"Delta", "Delta Dental"},
		{"Uber", "Uber Eats"},
		{"Target", "Target Optical"},
		{"Walmart", "Walmart Pharmacy"},
		{"State Farm", "State Farm Insurance"},
	}
	for _, p := range distinct {
		if s := actual.PayeeSimilarity(p[0], p[1]); s >= 0.85 {
			t.Errorf("PayeeSimilarity(%q, %q) = %.3f, want < 0.85", p[0], p[1], s)
		}
		cl := actual.ClusterPayees([]actual.Payee{{ID: "a", Name: p[0]}, {ID: "b", Name: p[1]}}, 0.85)
		if len(cl) != 0 {
			t.Errorf("%q and %q clustered at 0.85: %+v", p[0], p[1], cl)
		}
	}
	same := [][2]string{
		{"Kroger", "Kroger Fuel"},
		{"Shell", "Shell Gas Station"},
		{"Walmart", "Walmart Supermarket #1234"},
		{"Amazon", "Amazon Online"},
	}
	for _, p := range same {
		if s := actual.PayeeSimilarity(p[0], p[1]); s != 0.9 {
			t.Errorf("PayeeSimilarity(%q, %q) = %.3f, want 0.9", p[0], p[1], s)
		}
	}
	// Mixed list: only the generic-suffix and normalization variants cluster.
	cl := actual.ClusterPayees([]actual.Payee{{ID: "a", Name: "Delta"}, {ID: "b", Name: "Delta Dental"}, {ID: "u", Name: "Uber"}, {ID: "ue", Name: "UBER EATS"},
		{ID: "t", Name: "Target"}, {ID: "to", Name: "Target Optical"},
		{ID: "k", Name: "Kroger"}, {ID: "kf", Name: "KROGER FUEL #42"}, {ID: "am", Name: "Amazon"}, {ID: "am2", Name: "AMAZON MKTPLACE"}}, 0.85)
	if len(cl) != 2 {
		t.Fatalf("clusters = %+v, want Kroger and Amazon only", cl)
	}
	for _, c := range cl {
		if n := actual.NormalizePayeeName(c.Target.Name); (n != "kroger" && n != "kroger fuel" && n != "amazon") || len(c.Merge) != 1 {
			t.Errorf("unexpected cluster %+v", c)
		}
	}
}

func TestParseAmountRound2(t *testing.T) {
	cases := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"12.340", 1234, true},
		{"1,234.500", 123450, true},
		{"5.0000", 500, true},
		{"-12.300", -1230, true},
		{"(12.00)", -1200, true},
		{"($12.000)", -1200, true},
		{"12.345", 0, false},
		{"12.3401", 0, false},
		{"(-5)", 0, false},
		{"(+5)", 0, false},
		{"($-5)", 0, false},
	}
	for _, c := range cases {
		got, err := actual.ParseAmount(c.in)
		if (err == nil) != c.ok || (c.ok && got != c.want) {
			t.Errorf("ParseAmount(%q) = %d, %v; want %d ok=%v", c.in, got, err, c.want, c.ok)
		}
	}
}

func TestParseBankAmountRound2(t *testing.T) {
	cases := []struct {
		in       string
		want     int64
		ok, fail bool
	}{
		{"", 0, false, false},
		{"-", 0, false, false},
		{"--", 0, false, false},
		{" — ", 0, false, false},
		{"–", 0, false, false},
		{"12.340", 1234, true, false},
		{"1,234.500-", -123450, true, false},
		{"12.345", 0, false, true},
		{"(-5)", 0, false, true},
	}
	for _, c := range cases {
		got, ok, err := actual.ParseBankAmount(c.in)
		if (err != nil) != c.fail || ok != c.ok || got != c.want {
			t.Errorf("ParseBankAmount(%q) = %d, %v, %v; want %d ok=%v fail=%v", c.in, got, ok, err, c.want, c.ok, c.fail)
		}
	}
}

func TestParseBankCSVDashInUnusedColumn(t *testing.T) {
	m := actual.CSVMapping{DateCol: "Posted", PayeeCol: "Payee", DebitCol: "Debit", CreditCol: "Credit"}
	in := "Posted,Payee,Debit,Credit\n09/03/2026,Kroger,84.50,-\n09/04/2026,Employer,—,4000\n09/05/2026,Nobody,--,--\n"
	rows, bad, err := actual.ParseBankCSV(strings.NewReader(in), m)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].Amount != -8450 || rows[1].Amount != 400000 {
		t.Fatalf("rows = %+v", rows)
	}
	if len(bad) != 1 || bad[0].Line != 4 {
		t.Fatalf("bad = %+v, want line 4 (both columns empty)", bad)
	}
}

func TestDateTime(t *testing.T) {
	d := actual.DateTime(20260930)
	if d.Year() != 2026 || d.Month() != 9 || d.Day() != 30 || d.Location().String() != "UTC" || d.Hour() != 0 {
		t.Fatalf("DateTime = %v", d)
	}
	if actual.DateInt(d) != 20260930 {
		t.Fatalf("round trip = %d", actual.DateInt(d))
	}
}
