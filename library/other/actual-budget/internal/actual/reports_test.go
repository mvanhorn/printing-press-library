// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package actual_test

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual/actualtest"
)

func txnIDs(txns []actual.Txn) []string {
	ids := make([]string, 0, len(txns))
	for _, t := range txns {
		ids = append(ids, t.ID)
	}
	sort.Strings(ids)
	return ids
}

func TestSearchTxns(t *testing.T) {
	l := fixtureLedger(t)
	ctx := context.Background()
	cases := []struct {
		name  string
		query string
		f     actual.TxnFilter
		limit int
		want  []string
	}{
		{"payee incl merged", "kroger", actual.TxnFilter{}, 0, []string{"txn-kr-1", "txn-kr-2", "txn-kr-3", "txn-kr-4", "txn-kr-5", "txn-kr-6"}},
		{"case-insensitive notes", "BURRITO", actual.TxnFilter{}, 0, []string{"txn-ch-1"}},
		{"all terms must match", "kroger deli", actual.TxnFilter{}, 0, []string{"txn-kr-5"}},
		{"category name", "car fund", actual.TxnFilter{}, 0, []string{"txn-split-2"}},
		{"imported text", "cincinnati", actual.TxnFilter{From: 20260801, To: 20260831}, 0, []string{"txn-kr-4", "txn-kr-5"}},
		{"account filter excludes", "kroger", actual.TxnFilter{AccountID: actualtest.AcctChecking}, 0, []string{}},
		{"limit", "kroger", actual.TxnFilter{}, 2, []string{"txn-kr-5", "txn-kr-6"}},
		{"no match", "zzznomatch", actual.TxnFilter{}, 0, []string{}},
		{"blank query", "   ", actual.TxnFilter{}, 0, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := l.SearchTxns(ctx, tc.query, tc.f, tc.limit)
			if err != nil {
				t.Fatal(err)
			}
			if ids := txnIDs(got); !reflect.DeepEqual(ids, tc.want) {
				t.Fatalf("SearchTxns(%q) = %v, want %v", tc.query, ids, tc.want)
			}
		})
	}
}

func TestFindDuplicates(t *testing.T) {
	l := fixtureLedger(t)
	ctx := context.Background()
	cases := []struct {
		name string
		opts actual.DuplicateOptions
		want [][2]string
	}{
		{"default window", actual.DuplicateOptions{Days: 3, MinConfidence: 0.6}, [][2]string{{"txn-dup-a", "txn-dup-b"}}},
		{"zero days", actual.DuplicateOptions{Days: 0, MinConfidence: 0.6}, nil},
		{"wide window still excludes split and amount-mismatch", actual.DuplicateOptions{Days: 20, MinConfidence: 0.6}, [][2]string{{"txn-dup-a", "txn-dup-b"}}},
		{"range outside", actual.DuplicateOptions{Days: 3, From: 20260701, To: 20260731, MinConfidence: 0.6}, nil},
		{"high threshold", actual.DuplicateOptions{Days: 3, MinConfidence: 0.99}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := l.FindDuplicates(ctx, tc.opts)
			if err != nil {
				t.Fatal(err)
			}
			var pairs [][2]string
			for _, p := range got {
				pairs = append(pairs, [2]string{p.AID, p.BID})
				if p.Confidence <= 0 || p.Confidence > 1 {
					t.Fatalf("confidence out of range: %+v", p)
				}
				if p.AID == "txn-ch-2" || p.BID == "txn-ch-2" {
					t.Fatalf("txn-ch-2 paired: %+v", p)
				}
			}
			if !reflect.DeepEqual(pairs, tc.want) {
				t.Fatalf("pairs = %v, want %v", pairs, tc.want)
			}
		})
	}
}

func TestTextSimilarity(t *testing.T) {
	cases := []struct {
		a, b string
		min  float64
		max  float64
	}{
		{"Kroger", "KROGER", 1, 1},
		{"Amazon", "Amazon.com", 0.5, 0.7},
		{"Chipotle", "Netflix", 0, 0.5},
		{"", "x", 0, 0},
	}
	for _, tc := range cases {
		if s := actual.TextSimilarity(tc.a, tc.b); s < tc.min || s > tc.max {
			t.Errorf("TextSimilarity(%q,%q) = %v, want [%v,%v]", tc.a, tc.b, s, tc.min, tc.max)
		}
	}
}

func TestCheckReadOnlySQL(t *testing.T) {
	cases := []struct {
		q        string
		ok, wrap bool
	}{
		{"SELECT 1", true, true},
		{"  select name from accounts;  ", true, true},
		{"WITH x AS (SELECT 1) SELECT * FROM x", true, true},
		{"-- comment\nSELECT 1", true, true},
		{"SELECT ';' AS semi", true, true},
		{"PRAGMA table_info(accounts)", true, false},
		{"DELETE FROM accounts", false, false},
		{"SELECT 1; DELETE FROM accounts", false, false},
		{"PRAGMA writable_schema = 1", false, false},
		{"ATTACH DATABASE 'x' AS y", false, false},
		{"", false, false},
		// Read-only bypass regressions: a quote inside a comment must not
		// hide a statement separator, and connection-state keywords are
		// rejected even inside a single statement.
		{"SELECT 1 WHERE 0) /* ' */ ; PRAGMA query_only=0; VACUUM INTO '/tmp/x.db'; SELECT * FROM (SELECT 1 /* ' */", false, false},
		{"SELECT 1 WHERE 0 /* ' */ ; PRAGMA query_only=0; ATTACH '/tmp/x.db' AS z; CREATE TABLE z.t AS SELECT 1 /* ' */", false, false},
		{"SELECT 1 WHERE 0 -- '\n; SELECT 2", false, false},
		{"SELECT 1 /* unterminated", false, false},
		{"SELECT 'unterminated", false, false},
		{"VACUUM INTO '/tmp/x.db'", false, false},
		{"WITH x AS (SELECT 1) SELECT * FROM x, pragma_query_only", true, true},
		{"SELECT load_extension('x')", false, false},
		{"SELECT 'attach; vacuum' AS s, replace(name, 'a', 'b') FROM accounts", true, true},
		{"SELECT [delete] FROM t -- trailing ' comment", true, true},
		{"PRAGMA table_info(accounts); ATTACH 'x' AS y", false, false},
		// TCL-style parameter tokens swallow quotes in SQLite's lexer.
		{"SELECT $a(') WHERE 0) ; PRAGMA query_only=0; ATTACH '/tmp/x.db' AS z; CREATE TABLE z.t(x); SELECT * FROM (SELECT $b(')", false, false},
		{"SELECT @a(') ; ATTACH 'x' AS y; SELECT @b(')", false, false},
		{"SELECT :a(') ; ATTACH 'x' AS y; SELECT :b(')", false, false},
		{"SELECT $a(\"); ATTACH 'x' AS y; SELECT $b(\")", false, false},
		{"SELECT ?", false, false},
		{"SELECT json_extract(x, '$.a') FROM (SELECT '{}' AS x)", true, true},
	}
	for _, tc := range cases {
		run, wrap, err := actual.CheckReadOnlySQL(tc.q)
		if err == nil && strings.Contains(run, "--") {
			t.Errorf("CheckReadOnlySQL(%q) returned a statement that still has a comment: %q", tc.q, run)
		}
		if (err == nil) != tc.ok || wrap != tc.wrap {
			t.Errorf("CheckReadOnlySQL(%q) = wrap %v err %v, want ok %v wrap %v", tc.q, wrap, err, tc.ok, tc.wrap)
		}
		if err != nil && !errors.Is(err, actual.ErrNotReadOnlySQL) {
			t.Errorf("CheckReadOnlySQL(%q) error not ErrNotReadOnlySQL: %v", tc.q, err)
		}
	}
}

func TestQueryRows(t *testing.T) {
	l := fixtureLedger(t)
	rows, cols, err := l.QueryRows(context.Background(), "SELECT COUNT(*) AS n FROM accounts WHERE tombstone = 0")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["n"] != int64(4) || !reflect.DeepEqual(cols, []string{"n"}) {
		t.Fatalf("rows = %v cols = %v", rows, cols)
	}
}

func TestSpending(t *testing.T) {
	l := fixtureLedger(t)
	ctx := context.Background()
	aug := actual.ReportScope{From: 20260801, To: 20260831}
	res, err := l.Spending(ctx, aug, "category")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	var sum int64
	for _, r := range res.Rows {
		got[r.Name] = r.Spent
		sum += r.Spent
	}
	want := map[string]int64{"Rent": 150000, "Groceries": 12230, "Dining Out": 4894, "Car Fund": 4000, "Uncategorized": 2599}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("spending by category = %v, want %v", got, want)
	}
	if res.Total != sum || res.Total != 173723 {
		t.Fatalf("total = %d (sum %d), want 173723", res.Total, sum)
	}
	if res.Rows[0].Name != "Rent" {
		t.Fatalf("not sorted by spent desc: %+v", res.Rows)
	}

	cases := []struct {
		by   string
		name string
		want int64
	}{
		{"payee", "Amazon", 6000},  // split children, parent not double-counted
		{"payee", "Kroger", 11730}, // kr-4 + kr-5
		{"group", "Living", 17124},
		{"group", "Bills", 154000},
	}
	for _, tc := range cases {
		r, err := l.Spending(ctx, aug, tc.by)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, row := range r.Rows {
			if row.Name == tc.name {
				found = true
				if row.Spent != tc.want {
					t.Errorf("by %s %s = %d, want %d", tc.by, tc.name, row.Spent, tc.want)
				}
			}
		}
		if !found {
			t.Errorf("by %s: %s missing in %+v", tc.by, tc.name, r.Rows)
		}
	}
	if _, err := l.Spending(ctx, aug, "bogus"); err == nil {
		t.Fatal("expected error for bad grouping")
	}
}

func TestCashflow(t *testing.T) {
	l := fixtureLedger(t)
	got, err := l.Cashflow(context.Background(), actual.ReportScope{From: 20260601, To: 20260930})
	if err != nil {
		t.Fatal(err)
	}
	want := []actual.CashflowMonth{
		{Month: "2026-06", Income: 0, Spending: 0, Net: 0},
		{Month: "2026-07", Income: 400000, Spending: 150000 + 8450 + 9120 + 7600 + 3999, Net: 400000 - 179169},
		{Month: "2026-08", Income: 400000, Spending: 173723, Net: 400000 - 173723},
		{Month: "2026-09", Income: 400000, Spending: 165000 + 8800 + 2210 + 1250 + 4210 + 4210, Net: 400000 - 185680},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("cashflow = %+v\nwant %+v", got, want)
	}
}

func TestBudgetVsActual(t *testing.T) {
	l := fixtureLedger(t)
	res, err := l.BudgetVsActual(context.Background(), 202609, "")
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]actual.BudgetLine{}
	for _, r := range res.Rows {
		by[r.Category] = r
	}
	cases := []actual.BudgetLine{
		{CategoryID: "cat-rent", Category: "Rent", Group: "Bills", Budgeted: 150000, Spent: 165000, Remaining: -15000, OverBudget: true},
		{CategoryID: "cat-groceries", Category: "Groceries", Group: "Living", Budgeted: 40000, Spent: 1250, Remaining: 38750},
		{CategoryID: "cat-dining", Category: "Dining Out", Group: "Living", Budgeted: 10000, Spent: 0, Remaining: 10000},
		{CategoryID: "cat-car", Category: "Car Fund", Group: "Bills", Budgeted: 20000, Spent: 0, Remaining: 20000},
	}
	for _, want := range cases {
		if got := by[want.Category]; got != want {
			t.Errorf("%s = %+v, want %+v", want.Category, got, want)
		}
	}
	if _, ok := by["Salary"]; ok {
		t.Error("income category included")
	}
	if res.TotalBudgeted != 220000 || res.TotalSpent != 166250 {
		t.Errorf("totals = %d/%d", res.TotalBudgeted, res.TotalSpent)
	}
}

func TestNetWorth(t *testing.T) {
	l := fixtureLedger(t)
	got, err := l.NetWorth(context.Background(), []int{202605, 202606, 202607}, true)
	if err != nil {
		t.Fatal(err)
	}
	if got[0].NetWorth != 0 || got[1].NetWorth != 4000000 || got[1].AsOf != "2026-06-30" {
		t.Fatalf("net worth = %+v", got)
	}
	// July: +400000 salary, -150000 rent, -(8450+9120+7600+3999) card spending.
	if want := int64(4000000 + 400000 - 150000 - 29169); got[2].NetWorth != want || got[2].Change != want-4000000 {
		t.Fatalf("July net worth = %d change %d, want %d", got[2].NetWorth, got[2].Change, want)
	}
	if len(got[1].Accounts) != 4 {
		t.Fatalf("by-account breakdown = %+v", got[1].Accounts)
	}
}

func TestTrends(t *testing.T) {
	l := fixtureLedger(t)
	rows, err := l.Trends(context.Background(), actual.ReportScope{From: 20260701, To: 20260930}, "cat-groceries")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %+v", rows)
	}
	r := rows[0]
	want := []actual.TrendPoint{{Month: "2026-07", Spent: 29169}, {Month: "2026-08", Spent: 12230}, {Month: "2026-09", Spent: 1250}}
	if !reflect.DeepEqual(r.Series, want) {
		t.Fatalf("series = %+v, want %+v", r.Series, want)
	}
	if r.Average != 14216 || r.Latest != 1250 || r.Delta != 1250-14216 {
		t.Fatalf("avg/latest/delta = %d/%d/%d", r.Average, r.Latest, r.Delta)
	}
	all, err := l.Trends(context.Background(), actual.ReportScope{From: 20260701, To: 20260930}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 5 || all[0].Category != "Rent" {
		t.Fatalf("all trends = %+v", all)
	}
}

func TestRecurring(t *testing.T) {
	l := fixtureLedger(t)
	ctx := context.Background()
	scope := actual.ReportScope{From: 20251001, To: 20260930}
	cases := []struct {
		min     int
		want    []string
		missing []string
	}{
		{3, []string{"Oak Street Apartments"}, []string{"Chipotle", "Kroger", "Amazon"}},
		{4, nil, []string{"Oak Street Apartments"}},
	}
	for _, tc := range cases {
		got, err := l.Recurring(ctx, scope, tc.min)
		if err != nil {
			t.Fatal(err)
		}
		var names []string
		for _, r := range got {
			names = append(names, r.Payee)
		}
		if !reflect.DeepEqual(names, tc.want) {
			t.Fatalf("min %d: recurring = %v, want %v", tc.min, names, tc.want)
		}
		if tc.min == 3 {
			r := got[0]
			if !r.Scheduled || r.Occurrences != 3 || r.CadenceDays != 31 || r.LastDate != "2026-09-03" || r.NextExpected != "2026-10-04" || r.AvgAmount != 155000 {
				t.Fatalf("rent = %+v", r)
			}
		}
	}
}

func TestMonthsBetween(t *testing.T) {
	if got := actual.MonthsBetween(20251115, 20260210); !reflect.DeepEqual(got, []int{202511, 202512, 202601, 202602}) {
		t.Fatalf("MonthsBetween = %v", got)
	}
	if got := actual.MonthsBetween(20260301, 20260201); len(got) != 0 {
		t.Fatalf("reversed range = %v", got)
	}
	if actual.MonthEnd(202602) != 20260228 || actual.AddMonths(202601, -2) != 202511 {
		t.Fatal("month helpers wrong")
	}
}

// Extra rows (applied to a writable copy, so the shared fixture is
// unchanged): a categorized on-budget -> off-budget transfer and a refund.
var transferAndRefundRows = []string{
	`INSERT INTO payees (id,name,transfer_acct) VALUES ('pay-xfer-broker','','acct-broker')`,
	`INSERT INTO payee_mapping (id,targetId) VALUES ('pay-xfer-broker','pay-xfer-broker')`,
	`INSERT INTO transactions (id,acct,category,amount,description,date,transferred_id,sort_order,cleared) VALUES
		('txn-xbrk-out','acct-checking','cat-car',-20000,'pay-xfer-broker',20260921,'txn-xbrk-in',13,1),
		('txn-xbrk-in','acct-broker',NULL,20000,'pay-xfer-checking',20260921,'txn-xbrk-out',13,1),
		('txn-refund','acct-card','cat-groceries',1000,'pay-kroger',20260920,NULL,14,1)`,
}

func TestCategorizedTransferAndRefund(t *testing.T) {
	l := mutatedFixtureLedger(t, transferAndRefundRows...)
	ctx := context.Background()
	sept := actual.ReportScope{From: 20260901, To: 20260930}

	// Cashflow spending is net activity: base Sept spending 185680, plus the
	// 20000 categorized transfer to Brokerage, minus the 1000 refund.
	cf, err := l.Cashflow(ctx, sept)
	if err != nil {
		t.Fatal(err)
	}
	if len(cf) != 1 || cf[0].Income != 400000 || cf[0].Spending != 185680+20000-1000 {
		t.Fatalf("cashflow = %+v", cf)
	}

	bva, err := l.BudgetVsActual(ctx, 202609, "")
	if err != nil {
		t.Fatal(err)
	}
	spent := map[string]int64{}
	for _, r := range bva.Rows {
		spent[r.CategoryID] = r.Spent
	}
	if spent["cat-car"] != 20000 || spent["cat-groceries"] != 1250-1000 || spent["cat-rent"] != 165000 {
		t.Fatalf("budget vs actual spent = %v", spent)
	}

	sp, err := l.Spending(ctx, sept, "category")
	if err != nil {
		t.Fatal(err)
	}
	car := int64(0)
	for _, r := range sp.Rows {
		if r.ID == "cat-car" {
			car = r.Spent
		}
	}
	if car != 20000 {
		t.Fatalf("spending Car Fund = %d, want 20000 (categorized transfer to off-budget)", car)
	}

	// templates status and budget-vs-actual must agree on per-category spent.
	ts, err := l.TemplateStatuses(ctx, "2026-09")
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) == 0 {
		t.Fatal("no template statuses")
	}
	for _, s := range ts {
		if s.Spent != spent[s.CategoryID] {
			t.Errorf("%s: templates spent %d != budget-vs-actual spent %d", s.Category, s.Spent, spent[s.CategoryID])
		}
	}

	// An on-budget <-> on-budget transfer (no category) is never spending.
	aug, err := l.BudgetVsActual(ctx, 202608, "")
	if err != nil {
		t.Fatal(err)
	}
	base := fixtureLedger(t)
	want, err := base.BudgetVsActual(ctx, 202608, "")
	if err != nil {
		t.Fatal(err)
	}
	if aug.TotalSpent != want.TotalSpent {
		t.Fatalf("August spent changed: %d vs %d", aug.TotalSpent, want.TotalSpent)
	}
}

func TestResolveIDs(t *testing.T) {
	l := mutatedFixtureLedger(t,
		`INSERT INTO accounts (id,name,offbudget,closed,sort_order) VALUES ('acct-checking2','CHECKING',0,0,9)`,
		`INSERT INTO categories (id,name,is_income,cat_group,sort_order) VALUES ('cat-groc2','groceries',0,'grp-living',9)`)
	ctx := context.Background()
	cases := []struct {
		name    string
		resolve func(context.Context, string) (string, error)
		in      string
		want    string
		wantErr error
		ids     []string
	}{
		{"exact account id wins", l.ResolveAccountID, "acct-checking", "acct-checking", nil, nil},
		{"unique account name", l.ResolveAccountID, "visa card", actualtest.AcctCard, nil, nil},
		{"ambiguous account name", l.ResolveAccountID, "Checking", "", actual.ErrAmbiguousName, []string{"acct-checking", "acct-checking2"}},
		{"tombstoned account", l.ResolveAccountID, "Old Account", "", sql.ErrNoRows, nil},
		{"exact category id wins", l.ResolveCategoryID, "cat-groc2", "cat-groc2", nil, nil},
		{"unique category name", l.ResolveCategoryID, "DINING OUT", actualtest.CatDining, nil, nil},
		{"ambiguous category name", l.ResolveCategoryID, "Groceries", "", actual.ErrAmbiguousName, []string{"cat-groc2", "cat-groceries"}},
		{"tombstoned category", l.ResolveCategoryID, "Food (old)", "", sql.ErrNoRows, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.resolve(ctx, tc.in)
			if tc.wantErr == nil {
				if err != nil || got != tc.want {
					t.Fatalf("resolve(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
				}
				return
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("resolve(%q) err = %v, want %v", tc.in, err, tc.wantErr)
			}
			for _, id := range tc.ids {
				if !strings.Contains(err.Error(), id) {
					t.Errorf("error %q does not list id %s", err, id)
				}
			}
		})
	}
}
