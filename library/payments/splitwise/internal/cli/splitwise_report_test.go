package cli

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/store"
)

func TestComputeReport_TableDriven(t *testing.T) {
	expenses := []Expense{
		{
			ID:           101,
			GroupID:      10,
			Description:  "Lunch",
			Cost:         "40.00",
			CurrencyCode: "USD",
			Date:         "2025-01-10T12:00:00Z",
			Payment:      false,
			Category:     Category{Name: "Food"},
			Users: []ExpenseUser{
				{UserID: 1, PaidShare: "40.00", OwedShare: "20.00", User: NestedUser{ID: 1, FirstName: "You", LastName: "Person"}},
				{UserID: 2, PaidShare: "0", OwedShare: "20.00", User: NestedUser{ID: 2, FirstName: "Alex", LastName: "Kim"}},
			},
		},
		{
			ID:           102,
			GroupID:      10,
			Description:  "Cab",
			Cost:         "20.00",
			CurrencyCode: "USD",
			Date:         "2025-01-11T12:00:00Z",
			Payment:      false,
			Category:     Category{Name: "Transport"},
			Users: []ExpenseUser{
				{UserID: 1, PaidShare: "10.00", OwedShare: "10.00", User: NestedUser{ID: 1, FirstName: "You", LastName: "Person"}},
				{UserID: 2, PaidShare: "10.00", OwedShare: "10.00", User: NestedUser{ID: 2, FirstName: "Alex", LastName: "Kim"}},
			},
		},
		{
			ID:           103,
			GroupID:      10,
			Description:  "Museum",
			Cost:         "30.00",
			CurrencyCode: "EUR",
			Date:         "2025-01-12T12:00:00Z",
			Payment:      false,
			Category:     Category{Name: "Entertainment"},
			Users: []ExpenseUser{
				{UserID: 1, PaidShare: "0", OwedShare: "15.00", User: NestedUser{ID: 1, FirstName: "You", LastName: "Person"}},
				{UserID: 2, PaidShare: "30.00", OwedShare: "15.00", User: NestedUser{ID: 2, FirstName: "Alex", LastName: "Kim"}},
			},
		},
		{
			ID:           104,
			GroupID:      11,
			Description:  "Hotel",
			Cost:         "80.00",
			CurrencyCode: "USD",
			Date:         "2025-02-01T12:00:00Z",
			Payment:      false,
			Category:     Category{Name: "Lodging"},
			Users: []ExpenseUser{
				{UserID: 1, PaidShare: "80.00", OwedShare: "40.00", User: NestedUser{ID: 1, FirstName: "You", LastName: "Person"}},
				{UserID: 3, PaidShare: "0", OwedShare: "40.00", User: NestedUser{ID: 3, FirstName: ""}},
			},
		},
		{
			ID:           105,
			GroupID:      10,
			Description:  "Settlement",
			Cost:         "10.00",
			CurrencyCode: "USD",
			Date:         "2025-01-15T12:00:00Z",
			Payment:      true,
			Category:     Category{Name: ""},
		},
		{
			ID:           106,
			GroupID:      10,
			Description:  "Deleted Meal",
			Cost:         "10.00",
			CurrencyCode: "USD",
			Date:         "2025-01-16T12:00:00Z",
			Payment:      false,
			DeletedAt:    ptr("2025-01-17T00:00:00Z"),
			Category:     Category{Name: "Food"},
		},
	}

	groups := []Group{{ID: 10, Name: "Tahoe Trip"}, {ID: 11, Name: "Ski Weekend"}}

	tests := []struct {
		name   string
		opts   reportOpts
		assert func(t *testing.T, got reportResult)
	}{
		{
			name: "group date payment deleted and currency default",
			opts: reportOpts{GroupInput: "Tahoe Trip", Since: "2025-01-10", Until: "2025-01-31", Limit: 100},
			assert: func(t *testing.T, got reportResult) {
				if got.Scope != "group:Tahoe Trip" {
					t.Fatalf("Scope=%q", got.Scope)
				}
				if got.Currency != "USD" {
					t.Fatalf("Currency=%q, want USD", got.Currency)
				}
				if got.ExcludedOtherCurrency != 1 {
					t.Fatalf("ExcludedOtherCurrency=%d, want 1", got.ExcludedOtherCurrency)
				}
				if got.ExpenseCount != 2 {
					t.Fatalf("ExpenseCount=%d, want 2", got.ExpenseCount)
				}
				if got.TotalCost != 60 {
					t.Fatalf("TotalCost=%.2f, want 60", got.TotalCost)
				}
				if got.YourPaid != 50 || got.YourOwed != 30 || got.YourNet != 20 {
					t.Fatalf("your totals paid=%.2f owed=%.2f net=%.2f", got.YourPaid, got.YourOwed, got.YourNet)
				}
				if got.PeriodStart != "2025-01-10" || got.PeriodEnd != "2025-01-11" {
					t.Fatalf("period=%s..%s", got.PeriodStart, got.PeriodEnd)
				}
				if len(got.People) != 2 {
					t.Fatalf("len(People)=%d, want 2", len(got.People))
				}
				if got.People[0].Name != "You Person" || got.People[0].Net != 20 {
					t.Fatalf("people[0]=%+v", got.People[0])
				}
				if got.People[1].Name != "Alex Kim" || got.People[1].Net != -20 {
					t.Fatalf("people[1]=%+v", got.People[1])
				}
				if len(got.Categories) != 2 {
					t.Fatalf("len(Categories)=%d, want 2", len(got.Categories))
				}
				if got.Categories[0].Name != "Food" || got.Categories[0].Total != 40 || got.Categories[0].Count != 1 {
					t.Fatalf("categories[0]=%+v", got.Categories[0])
				}
				if got.Categories[1].Name != "Transport" || got.Categories[1].Total != 20 || got.Categories[1].Count != 1 {
					t.Fatalf("categories[1]=%+v", got.Categories[1])
				}
				if len(got.Expenses) != 2 {
					t.Fatalf("len(Expenses)=%d, want 2", len(got.Expenses))
				}
				if got.Expenses[0].ID != 101 || got.Expenses[0].Payer != "You Person" {
					t.Fatalf("expense[0]=%+v", got.Expenses[0])
				}
				if got.Expenses[1].ID != 102 || got.Expenses[1].Payer != "multiple" {
					t.Fatalf("expense[1]=%+v", got.Expenses[1])
				}
			},
		},
		{
			name: "explicit currency and limit with fallback user name",
			opts: reportOpts{Currency: "USD", Limit: 1},
			assert: func(t *testing.T, got reportResult) {
				if got.Scope != "all" {
					t.Fatalf("Scope=%q, want all", got.Scope)
				}
				if got.Currency != "USD" {
					t.Fatalf("Currency=%q, want USD", got.Currency)
				}
				if got.ExcludedOtherCurrency != 1 {
					t.Fatalf("ExcludedOtherCurrency=%d, want 1", got.ExcludedOtherCurrency)
				}
				if got.ExpenseCount != 3 {
					t.Fatalf("ExpenseCount=%d, want 3", got.ExpenseCount)
				}
				if got.Truncated != true || len(got.Expenses) != 1 {
					t.Fatalf("Truncated=%v len(Expenses)=%d", got.Truncated, len(got.Expenses))
				}
				if got.People[0].Name != "You Person" || got.People[2].Name != "user 3" {
					t.Fatalf("people ordering/fallback: %+v", got.People)
				}
				if got.PeriodStart != "2025-01-10" || got.PeriodEnd != "2025-02-01" {
					t.Fatalf("period=%s..%s", got.PeriodStart, got.PeriodEnd)
				}
			},
		},
		{
			name: "limit zero means all",
			opts: reportOpts{Limit: 0},
			assert: func(t *testing.T, got reportResult) {
				if got.Truncated {
					t.Fatalf("Truncated=true, want false")
				}
				if len(got.Expenses) != got.ExpenseCount {
					t.Fatalf("len(Expenses)=%d ExpenseCount=%d", len(got.Expenses), got.ExpenseCount)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := computeReport(expenses, groups, 1, tc.opts)
			tc.assert(t, got)
		})
	}
}

func TestRenderReportMarkdown(t *testing.T) {
	r := reportResult{
		Scope:                 "group:Tahoe Trip",
		Currency:              "USD",
		PeriodStart:           "2025-01-10",
		PeriodEnd:             "2025-01-11",
		ExpenseCount:          2,
		ExcludedOtherCurrency: 1,
		TotalCost:             60,
		YourNet:               20,
		People:                []reportPerson{{Name: "You Person", Paid: 50, Owed: 30, Net: 20}},
		Categories:            []reportCategory{{Name: "Food", Total: 40, Count: 1}},
		Expenses:              []reportExpenseRow{{ID: 101, Date: "2025-01-10", Description: "Lunch", Cost: 40, CurrencyCode: "USD", Payer: "You Person"}},
	}

	md := renderReportMarkdown(r)
	mustContain := []string{
		"# Report — group:Tahoe Trip",
		"## By person",
		"## By category",
		"## Expenses",
		"| You Person | 50.00 | 30.00 | 20.00 |",
		"| Food | 40.00 | 1 |",
		"Excluded 1 expense(s) in other currencies.",
	}
	for _, want := range mustContain {
		if !strings.Contains(md, want) {
			t.Fatalf("markdown missing %q\n%s", want, md)
		}
	}
}

func TestRenderReportCSV_EscapingAndRows(t *testing.T) {
	r := reportResult{Expenses: []reportExpenseRow{
		{ID: 101, Date: "2025-01-10", Description: "Simple", Cost: 40, CurrencyCode: "USD", Payer: "You Person"},
		{ID: 102, Date: "2025-01-11", Description: "Taxi, \"airport\"", Cost: 20, CurrencyCode: "USD", Payer: "multiple"},
	}}

	out := renderReportCSV(r)
	rows, err := csv.NewReader(strings.NewReader(out)).ReadAll()
	if err != nil {
		t.Fatalf("csv parse: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows=%d, want 3", len(rows))
	}
	head := strings.Join(rows[0], ",")
	if head != "id,date,description,cost,currency,payer" {
		t.Fatalf("header=%q", head)
	}
	if rows[2][2] != "Taxi, \"airport\"" {
		t.Fatalf("escaped description mismatch: %q", rows[2][2])
	}
}

func TestRenderReportMarkdown_EscapesPipes(t *testing.T) {
	r := reportResult{
		Scope:    "all",
		Currency: "USD",
		People:   []reportPerson{{Name: "A | B", Paid: 1, Owed: 0, Net: 1}},
		Expenses: []reportExpenseRow{
			{ID: 1, Date: "2025-01-10", Description: "Dinner | drinks\nlate", Cost: 50, CurrencyCode: "USD", Payer: "You"},
		},
	}
	md := renderReportMarkdown(r)
	if !strings.Contains(md, `Dinner \| drinks late`) {
		t.Fatalf("description pipe/newline not escaped:\n%s", md)
	}
	if strings.Contains(md, "Dinner | drinks") {
		t.Fatalf("raw unescaped pipe leaked into markdown:\n%s", md)
	}
	if !strings.Contains(md, `A \| B`) {
		t.Fatalf("person-name pipe not escaped:\n%s", md)
	}
	// The expense data row must remain a single 6-cell row (7 structural pipes),
	// regardless of pipes inside the (now-escaped) description.
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "| 1 | 2025-01-10 |") {
			if structural := strings.Count(line, "|") - strings.Count(line, `\|`); structural != 7 {
				t.Fatalf("expense row has %d structural pipes, want 7: %q", structural, line)
			}
		}
	}
}

func TestComputeReport_CurrencyTieBreak(t *testing.T) {
	// 1 USD + 1 EUR is a tie; the default currency picks the alphabetically-first
	// code (EUR) deterministically, and the USD expense is excluded (not mixed).
	expenses := []Expense{
		{ID: 1, Description: "u", Cost: "10", CurrencyCode: "USD", Date: "2025-01-01", Users: []ExpenseUser{{UserID: 1, PaidShare: "10", OwedShare: "10"}}},
		{ID: 2, Description: "e", Cost: "20", CurrencyCode: "EUR", Date: "2025-01-02", Users: []ExpenseUser{{UserID: 1, PaidShare: "20", OwedShare: "20"}}},
	}
	got := computeReport(expenses, nil, 1, reportOpts{Limit: 0})
	if got.Currency != "EUR" {
		t.Fatalf("tie-break currency=%q want EUR", got.Currency)
	}
	if got.ExpenseCount != 1 || got.ExcludedOtherCurrency != 1 {
		t.Fatalf("expense_count=%d excluded=%d want 1/1", got.ExpenseCount, got.ExcludedOtherCurrency)
	}
}

func TestComputeReport_NoPayer(t *testing.T) {
	// No participant has a positive paid_share -> payer "-", distinct from a
	// genuine 2+-payer "multiple".
	expenses := []Expense{
		{ID: 1, Description: "x", Cost: "10", CurrencyCode: "USD", Date: "2025-01-01", Users: []ExpenseUser{
			{UserID: 1, PaidShare: "0", OwedShare: "5", User: NestedUser{ID: 1, FirstName: "A"}},
			{UserID: 2, PaidShare: "0", OwedShare: "5", User: NestedUser{ID: 2, FirstName: "B"}},
		}},
	}
	got := computeReport(expenses, nil, 1, reportOpts{Limit: 0})
	if len(got.Expenses) != 1 || got.Expenses[0].Payer != "-" {
		t.Fatalf("payer=%q want '-'", got.Expenses[0].Payer)
	}
}

func TestComputeReport_UnresolvableGroupYieldsEmpty(t *testing.T) {
	// A --group that matches no group must NOT silently fall through to an
	// unfiltered report; the pure function filters to nothing and scopes the label.
	expenses := []Expense{
		{ID: 1, Description: "x", Cost: "10", CurrencyCode: "USD", Date: "2025-01-01", GroupID: 7, Users: []ExpenseUser{{UserID: 1, PaidShare: "10", OwedShare: "10"}}},
	}
	groups := []Group{{ID: 7, Name: "Real Group"}}
	got := computeReport(expenses, groups, 1, reportOpts{GroupInput: "Nonexistent Group", Limit: 0})
	if got.ExpenseCount != 0 || len(got.Expenses) != 0 {
		t.Fatalf("expense_count=%d len=%d, want 0/0 (unresolvable group must filter to nothing)", got.ExpenseCount, len(got.Expenses))
	}
	if !strings.Contains(got.Scope, "no match") {
		t.Fatalf("scope=%q, want it to flag the unmatched group", got.Scope)
	}
}

func ptr(s string) *string { return &s }

// TestReportCSVTruncationWarnsOnStderr verifies that when --limit truncates
// the expense list, the CSV output path emits a note on stderr (matching
// json/md/summary) while stdout remains valid, parseable CSV.
func TestReportCSVTruncationWarnsOnStderr(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dbPath := defaultDBPath("splitwise-pp-cli")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		t.Fatalf("mkdir db dir: %v", err)
	}
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	// Seed 3 same-currency expenses so --limit 1 truncates (2 dropped).
	for i := 1; i <= 3; i++ {
		id := fmt.Sprintf("%d", i)
		body := fmt.Sprintf(
			`{"id":%d,"currency_code":"USD","cost":"10.00","date":"2025-01-0%d","description":"expense%d","users":[{"user_id":42,"owed_share":"10.00","paid_share":"10.00","user":{"id":42,"first_name":"You","last_name":"Person"}}]}`,
			i, i, i,
		)
		if err := s.Upsert("get-expenses", id, []byte(body)); err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	cmd := newReportCmd(&rootFlags{})
	var out, errBuf bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	cmd.SetArgs([]string{"--format", "csv", "--limit", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v (stderr: %s)", err, errBuf.String())
	}

	// Assertion that will FAIL before the fix: stderr must contain "truncated".
	if !strings.Contains(errBuf.String(), "truncated") {
		t.Errorf("expected CSV truncation note on stderr, got: %q", errBuf.String())
	}
	// stdout must still be valid CSV with the expected header.
	if !strings.Contains(out.String(), "id,date,description") {
		t.Errorf("expected CSV header on stdout, got: %q", out.String())
	}
	// Confirm stdout is parseable CSV.
	rows, parseErr := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if parseErr != nil {
		t.Errorf("stdout is not valid CSV: %v\nraw: %q", parseErr, out.String())
	}
	// header + 1 data row (limit=1)
	if len(rows) != 2 {
		t.Errorf("expected 2 CSV rows (header+1 data), got %d", len(rows))
	}
}
