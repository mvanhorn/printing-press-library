package cli

import (
	"testing"
	"time"
)

func TestLooksLikeSettlement(t *testing.T) {
	tests := []struct {
		name string
		desc string
		want bool
	}{
		{name: "settle all balances", desc: "Settle all balances", want: true},
		{name: "settle up", desc: " settle up ", want: true},
		{name: "payment", desc: "payment", want: true},
		{name: "paid via", desc: "paid via Venmo", want: true},
		{name: "non-settlement", desc: "Dinner", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksLikeSettlement(tc.desc); got != tc.want {
				t.Fatalf("looksLikeSettlement(%q) = %v, want %v", tc.desc, got, tc.want)
			}
		})
	}
}

func TestComputeForecastMonthlyInsideWindow(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	expenses := []Expense{
		{Description: "Rent", GroupID: 10, Cost: "100.00", Date: "2026-02-01"},
		{Description: "Rent", GroupID: 10, Cost: "110.00", Date: "2026-03-03"},
		{Description: "Rent", GroupID: 10, Cost: "120.00", Date: "2026-04-02"},
		{Description: "Rent", GroupID: 10, Cost: "130.00", Date: "2026-05-02"},
	}
	groups := map[int]string{0: "Non-group", 10: "Roommates"}

	got := computeForecast(expenses, groups, now, 35, 50)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	row := got[0]
	if row.Description != "Rent" {
		t.Fatalf("Description = %q, want %q", row.Description, "Rent")
	}
	if row.Group != "Roommates" {
		t.Fatalf("Group = %q, want %q", row.Group, "Roommates")
	}
	if row.ExpectedDate != "2026-06-01" {
		t.Fatalf("ExpectedDate = %q, want %q", row.ExpectedDate, "2026-06-01")
	}
	if row.Overdue {
		t.Fatalf("Overdue = true, want false")
	}
	if row.CadenceDays != 30 {
		t.Fatalf("CadenceDays = %d, want 30", row.CadenceDays)
	}
	if row.ExpectedAmount != 115.00 {
		t.Fatalf("ExpectedAmount = %.2f, want 115.00", row.ExpectedAmount)
	}
}

func TestComputeForecastIrregularExcluded(t *testing.T) {
	now := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	expenses := []Expense{
		{Description: "Gym", GroupID: 0, Cost: "40", Date: "2026-01-01"},
		{Description: "Gym", GroupID: 0, Cost: "40", Date: "2026-01-03"},
		{Description: "Gym", GroupID: 0, Cost: "40", Date: "2026-01-05"},
		{Description: "Gym", GroupID: 0, Cost: "40", Date: "2026-02-14"},
	}
	groups := map[int]string{0: "Non-group"}

	got := computeForecast(expenses, groups, now, 90, 50)
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
}

func TestComputeForecastOverdueIncluded(t *testing.T) {
	now := time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)
	expenses := []Expense{
		{Description: "Utilities", GroupID: 0, Cost: "90", Date: "2026-01-01"},
		{Description: "Utilities", GroupID: 0, Cost: "100", Date: "2026-01-31"},
		{Description: "Utilities", GroupID: 0, Cost: "110", Date: "2026-03-02"},
	}
	groups := map[int]string{0: "Non-group"}

	got := computeForecast(expenses, groups, now, 20, 50)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if !got[0].Overdue {
		t.Fatalf("Overdue = false, want true")
	}
	if got[0].ExpectedDate != "2026-04-01" {
		t.Fatalf("ExpectedDate = %q, want %q", got[0].ExpectedDate, "2026-04-01")
	}
}

func TestComputeForecastVeryStaleExcluded(t *testing.T) {
	// A recurring series silent for years has clearly stopped and must not
	// keep appearing as "overdue".
	now := time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC)
	expenses := []Expense{
		{Description: "Utilities", GroupID: 0, Cost: "90", Date: "2026-01-01"},
		{Description: "Utilities", GroupID: 0, Cost: "100", Date: "2026-01-31"},
		{Description: "Utilities", GroupID: 0, Cost: "110", Date: "2026-03-02"},
	}
	groups := map[int]string{0: "Non-group"}

	got := computeForecast(expenses, groups, now, 20, 50)
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0 (series silent for years should be dropped)", len(got))
	}
}

func TestComputeForecastDueTodayAtMiddayNotOverdue(t *testing.T) {
	now := time.Date(2026, 6, 1, 14, 30, 0, 0, time.UTC)
	expenses := []Expense{
		{Description: "Rent", GroupID: 10, Cost: "100.00", Date: "2026-02-01"},
		{Description: "Rent", GroupID: 10, Cost: "110.00", Date: "2026-03-03"},
		{Description: "Rent", GroupID: 10, Cost: "120.00", Date: "2026-04-02"},
		{Description: "Rent", GroupID: 10, Cost: "130.00", Date: "2026-05-02"},
	}
	groups := map[int]string{0: "Non-group", 10: "Roommates"}

	got := computeForecast(expenses, groups, now, 35, 50)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].ExpectedDate != "2026-06-01" {
		t.Fatalf("ExpectedDate = %q, want %q", got[0].ExpectedDate, "2026-06-01")
	}
	if got[0].Overdue {
		t.Fatalf("Overdue = true, want false")
	}
}

func TestComputeForecastYesterdayAtMiddayOverdue(t *testing.T) {
	now := time.Date(2026, 6, 2, 14, 30, 0, 0, time.UTC)
	expenses := []Expense{
		{Description: "Rent", GroupID: 10, Cost: "100.00", Date: "2026-02-01"},
		{Description: "Rent", GroupID: 10, Cost: "110.00", Date: "2026-03-03"},
		{Description: "Rent", GroupID: 10, Cost: "120.00", Date: "2026-04-02"},
		{Description: "Rent", GroupID: 10, Cost: "130.00", Date: "2026-05-02"},
	}
	groups := map[int]string{0: "Non-group", 10: "Roommates"}

	got := computeForecast(expenses, groups, now, 35, 50)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].ExpectedDate != "2026-06-01" {
		t.Fatalf("ExpectedDate = %q, want %q", got[0].ExpectedDate, "2026-06-01")
	}
	if !got[0].Overdue {
		t.Fatalf("Overdue = false, want true")
	}
}

func TestComputeForecastFiltersPaymentsAndSettlements(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	groups := map[int]string{0: "Non-group", 1: "Trip"}

	onlyFiltered := []Expense{
		{Description: "Settle all balances", GroupID: 1, Cost: "10", Date: "2026-01-01"},
		{Description: "paid via Venmo", GroupID: 1, Cost: "10", Date: "2026-02-01"},
		{Description: "Payment", GroupID: 1, Cost: "10", Date: "2026-03-01", Payment: true},
	}
	got := computeForecast(onlyFiltered, groups, now, 60, 50)
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}

	mixed := []Expense{
		{Description: "Internet", GroupID: 1, Cost: "50", Date: "2026-02-01"},
		{Description: "Internet", GroupID: 1, Cost: "50", Date: "2026-03-03"},
		{Description: "Internet", GroupID: 1, Cost: "50", Date: "2026-04-02"},
		{Description: "Internet", GroupID: 1, Cost: "50", Date: "2026-05-02"},
		{Description: "Settle all balances", GroupID: 1, Cost: "999", Date: "2026-05-10"},
		{Description: "paid via Venmo", GroupID: 1, Cost: "888", Date: "2026-05-12"},
		{Description: "Internet", GroupID: 1, Cost: "777", Date: "2026-05-14", Payment: true},
	}
	got = computeForecast(mixed, groups, now, 40, 50)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	if got[0].Description != "Internet" {
		t.Fatalf("Description = %q, want Internet", got[0].Description)
	}
	if got[0].ExpectedAmount != 50 {
		t.Fatalf("ExpectedAmount = %.2f, want 50.00", got[0].ExpectedAmount)
	}
}

func TestComputeForecastUndatedExpensesDoNotAffectAttribution(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	expenses := []Expense{
		{Description: "Dinner", GroupID: 10, Cost: "10.00", Date: "2026-03-01"},
		{Description: "Dinner", GroupID: 10, Cost: "20.00", Date: "2026-04-01"},
		{Description: "Dinner", GroupID: 10, Cost: "30.00", Date: "2026-05-01"},
		{Description: "Bogus label", GroupID: 999, Cost: "999.00", Date: "not-a-date"},
	}
	groups := map[int]string{0: "Non-group", 10: "Home", 999: "ShouldNotWin"}

	got := computeForecast(expenses, groups, now, 40, 50)
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	row := got[0]
	if row.Description != "Dinner" {
		t.Fatalf("Description = %q, want %q", row.Description, "Dinner")
	}
	if row.Group != "Home" {
		t.Fatalf("Group = %q, want %q", row.Group, "Home")
	}
	if row.Occurrences != 3 {
		t.Fatalf("Occurrences = %d, want 3", row.Occurrences)
	}
	if row.ExpectedAmount != 20.00 {
		t.Fatalf("ExpectedAmount = %.2f, want 20.00", row.ExpectedAmount)
	}
}
