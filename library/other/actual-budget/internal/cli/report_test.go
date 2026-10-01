// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"strings"
	"testing"
)

type s2Spending struct {
	Total int64  `json:"total"`
	By    string `json:"by"`
	Rows  []struct {
		Name  string  `json:"name"`
		ID    string  `json:"id"`
		Spent int64   `json:"spent"`
		Count int     `json:"count"`
		Share float64 `json:"share"`
	} `json:"rows"`
}

func TestReportSpendingByCategory(t *testing.T) {
	s3Fixture(t)
	var res s2Spending
	s3RunJSON(t, &res, "report", "spending", "--month", "2026-08")
	got := map[string]int64{}
	for _, r := range res.Rows {
		got[r.Name] = r.Spent
	}
	want := map[string]int64{"Rent": 150000, "Groceries": 12230, "Dining Out": 4894, "Car Fund": 4000, "Uncategorized": 2599}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("spending = %v, want %v (split parent must not be double-counted; transfers excluded)", got, want)
	}
	if res.Total != 173723 || res.Rows[0].Name != "Rent" || res.Rows[0].Share < 0.86 || res.Rows[0].Share > 0.87 {
		t.Fatalf("total/sort/share wrong: %+v", res)
	}
}

func TestReportSpendingByPayeeAndFilters(t *testing.T) {
	s3Fixture(t)
	var res s2Spending
	s3RunJSON(t, &res, "report", "spending", "--month", "2026-08", "--by", "payee", "--account", "Visa Card")
	got := map[string]int64{}
	for _, r := range res.Rows {
		got[r.Name] = r.Spent
	}
	if got["Amazon"] != 6000 || got["Kroger"] != 11730 {
		t.Fatalf("by payee = %v", got)
	}
	if _, ok := got["Oak Street Apartments"]; ok {
		t.Fatal("--account Visa Card leaked checking rent")
	}
	var empty s2Spending
	s3RunJSON(t, &empty, "report", "spending", "--month", "2025-01")
	if empty.Total != 0 || len(empty.Rows) != 0 {
		t.Fatalf("empty month = %+v", empty)
	}
	if _, _, err := s3Run(t, "report", "spending", "--by", "mood"); s3ExitCode(err) != 2 {
		t.Fatalf("bad --by: code %d, want 2", s3ExitCode(err))
	}
}

func TestReportCashflow(t *testing.T) {
	s3Fixture(t)
	var rows []struct {
		Month    string `json:"month"`
		Income   int64  `json:"income"`
		Spending int64  `json:"spending"`
		Net      int64  `json:"net"`
	}
	s3RunJSON(t, &rows, "report", "cashflow", "--month", "2026-08", "--months", "3")
	if len(rows) != 3 || rows[0].Month != "2026-06" || rows[2].Month != "2026-08" {
		t.Fatalf("months = %+v", rows)
	}
	if rows[0].Income != 0 {
		t.Fatalf("starting balance counted as income: %+v", rows[0])
	}
	aug := rows[2]
	if aug.Income != 400000 || aug.Spending != 173723 || aug.Net != 400000-173723 {
		t.Fatalf("2026-08 = %+v", aug)
	}
}

func TestReportBudgetVsActual(t *testing.T) {
	s3Fixture(t)
	var res struct {
		Month string `json:"month"`
		Rows  []struct {
			Category   string `json:"category"`
			Budgeted   int64  `json:"budgeted"`
			Spent      int64  `json:"spent"`
			Remaining  int64  `json:"remaining"`
			OverBudget bool   `json:"over_budget"`
		} `json:"rows"`
	}
	s3RunJSON(t, &res, "report", "budget-vs-actual", "--month", "2026-09")
	found := 0
	for _, r := range res.Rows {
		switch r.Category {
		case "Rent":
			found++
			if r.Budgeted != 150000 || r.Spent != 165000 || !r.OverBudget || r.Remaining != -15000 {
				t.Errorf("Rent = %+v", r)
			}
		case "Groceries":
			found++
			if r.Budgeted != 40000 || r.Spent != 1250 || r.OverBudget {
				t.Errorf("Groceries = %+v (txn-kr-6 is uncategorized)", r)
			}
		case "Salary":
			t.Error("income category listed")
		}
	}
	if found != 2 || res.Month != "2026-09" {
		t.Fatalf("rows = %+v", res)
	}
}

func TestReportNetWorth(t *testing.T) {
	s3Fixture(t)
	var rows []struct {
		Month    string `json:"month"`
		AsOf     string `json:"as_of"`
		NetWorth int64  `json:"net_worth"`
		Accounts []struct {
			Name    string `json:"name"`
			Balance int64  `json:"balance"`
		} `json:"accounts"`
	}
	s3RunJSON(t, &rows, "report", "net-worth", "--month", "2026-09", "--months", "4", "--by-account")
	if len(rows) != 4 || rows[0].AsOf != "2026-06-30" || rows[0].NetWorth != 4000000 {
		t.Fatalf("net worth = %+v", rows)
	}
	bal := map[string]int64{}
	for _, a := range rows[0].Accounts {
		bal[a.Name] = a.Balance
	}
	if bal["Brokerage"] != 2500000 || bal["Checking"] != 500000 || bal["Savings"] != 1000000 {
		t.Fatalf("June balances = %v", bal)
	}
}

func TestReportTrendsGroceries(t *testing.T) {
	s3Fixture(t)
	var rows []struct {
		Category string `json:"category"`
		Average  int64  `json:"average"`
		Latest   int64  `json:"latest"`
		Delta    int64  `json:"delta"`
		Series   []struct {
			Month string `json:"month"`
			Spent int64  `json:"spent"`
		} `json:"series"`
	}
	s3RunJSON(t, &rows, "report", "trends", "--category", "groceries", "--month", "2026-09", "--months", "3")
	if len(rows) != 1 || rows[0].Category != "Groceries" {
		t.Fatalf("rows = %+v", rows)
	}
	var series []int64
	for _, p := range rows[0].Series {
		series = append(series, p.Spent)
	}
	if !reflect.DeepEqual(series, []int64{29169, 12230, 1250}) || rows[0].Series[0].Month != "2026-07" {
		t.Fatalf("groceries series = %+v", rows[0].Series)
	}
	if rows[0].Latest != 1250 || rows[0].Average != 14216 || rows[0].Delta != 1250-14216 {
		t.Fatalf("avg/latest/delta = %+v", rows[0])
	}
	if _, _, err := s3Run(t, "report", "trends", "--category", "Vacations"); s3ExitCode(err) != 3 {
		t.Fatalf("unknown category: code %d, want 3", s3ExitCode(err))
	}
}

func TestReportRecurring(t *testing.T) {
	s3Fixture(t)
	var rows []struct {
		Payee        string `json:"payee"`
		Occurrences  int    `json:"occurrences"`
		AvgAmount    int64  `json:"avg_amount"`
		CadenceDays  int    `json:"cadence_days"`
		NextExpected string `json:"next_expected"`
		Scheduled    bool   `json:"scheduled"`
	}
	s3RunJSON(t, &rows, "report", "recurring", "--month", "2026-09")
	var rent bool
	for _, r := range rows {
		if r.Payee == "Chipotle" {
			t.Fatalf("Chipotle detected at default threshold: %+v", r)
		}
		if r.Payee == "Oak Street Apartments" {
			rent = true
			if !r.Scheduled || r.Occurrences != 3 || r.CadenceDays != 31 || r.NextExpected != "2026-10-04" {
				t.Fatalf("rent = %+v", r)
			}
		}
	}
	if !rent {
		t.Fatalf("rent not detected: %+v", rows)
	}
	out, _, err := s3Run(t, "report", "recurring", "--month", "2026-09", "--min-occurrences", "4")
	if err != nil || out != "[]\n" {
		t.Fatalf("min 4: out = %q err = %v, want []", out, err)
	}
}

func TestReportNoMirror(t *testing.T) {
	s3NoMirror(t)
	for _, sub := range []string{"cashflow", "net-worth", "trends", "recurring"} {
		out, _, err := s3Run(t, "report", sub)
		if err != nil || out != "[]\n" {
			t.Errorf("report %s: out = %q err = %v, want []", sub, out, err)
		}
	}
	var res s2Spending
	s3RunJSON(t, &res, "report", "spending", "--month", "2026-08")
	if res.Total != 0 || res.Rows == nil || len(res.Rows) != 0 {
		t.Fatalf("spending without mirror = %+v", res)
	}
}

func TestReportRejectsLive(t *testing.T) {
	s3Fixture(t)
	if _, _, err := s3Run(t, "report", "spending", "--data-source", "live"); s3ExitCode(err) != 2 {
		t.Fatalf("--data-source live: code %d, want 2", s3ExitCode(err))
	}
}

func TestReportAmbiguousNamesAreUsageErrors(t *testing.T) {
	s3Fixture(t)
	s3Exec(t,
		`INSERT INTO accounts (id,name,offbudget,closed,sort_order) VALUES ('acct-checking2','checking',0,0,9)`,
		`INSERT INTO categories (id,name,is_income,cat_group,sort_order) VALUES ('cat-groc2','GROCERIES',0,'grp-living',9)`)
	for _, args := range [][]string{
		{"report", "spending", "--month", "2026-08", "--account", "Checking"},
		{"report", "budget-vs-actual", "--month", "2026-09", "--account", "Checking"},
		{"ledger", "uncategorized", "--account", "Checking"},
		{"report", "trends", "--month", "2026-09", "--category", "groceries"},
	} {
		_, _, err := s3Run(t, args...)
		if s3ExitCode(err) != 2 {
			t.Fatalf("%v: code %d (%v), want 2", args, s3ExitCode(err), err)
		}
		if !strings.Contains(err.Error(), "ids:") {
			t.Fatalf("%v: error does not list ids: %v", args, err)
		}
	}
	// An exact id still resolves.
	var res s2Spending
	s3RunJSON(t, &res, "report", "spending", "--month", "2026-08", "--account", "acct-checking")
	if res.Total != 150000 {
		t.Fatalf("--account acct-checking total = %d, want 150000 (rent only)", res.Total)
	}
}

func TestReportHumanTablesKeepColumnOrder(t *testing.T) {
	s3Fixture(t)
	out, _, err := s3RunHuman(t, "report", "net-worth", "--month", "2026-08", "--months", "2", "--by-account")
	if err != nil {
		t.Fatal(err)
	}
	header := strings.Fields(strings.SplitN(out, "\n", 2)[0])
	want := []string{"month", "net_worth", "change", "Checking", "Visa", "Card", "Savings", "Brokerage"}
	if !reflect.DeepEqual(header, want) {
		t.Fatalf("net-worth header = %q, want %q\n%s", header, want, out)
	}
	out, _, err = s3RunHuman(t, "report", "trends", "--month", "2026-09", "--months", "3")
	if err != nil {
		t.Fatal(err)
	}
	header = strings.Fields(strings.SplitN(out, "\n", 2)[0])
	want = []string{"category", "2026-07", "2026-08", "2026-09", "average", "latest", "delta"}
	if !reflect.DeepEqual(header, want) {
		t.Fatalf("trends header = %q, want %q\n%s", header, want, out)
	}
	if !strings.Contains(out, "Rent") {
		t.Fatalf("trends rows missing:\n%s", out)
	}
}
