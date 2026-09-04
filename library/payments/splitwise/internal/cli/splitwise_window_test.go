package cli

import (
	"errors"
	"testing"
	"time"
)

func TestExpenseWindow(t *testing.T) {
	now := time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC)
	expenses := []Expense{
		{Date: "2026-07-01"},
		{Date: "2026-08-10T08:00:00Z"},
		{Date: "2026-08-20"},
		{Date: "2026-09-01T18:00:00Z"},
	}
	tests := []struct {
		name       string
		since      string
		until      string
		wantCount  int
		wantUsage2 bool
	}{
		{name: "no window", wantCount: 4},
		{name: "since only duration", since: "30d", wantCount: 3},
		{name: "since and until dates", since: "2026-08-01", until: "2026-08-20", wantCount: 2},
		{name: "invalid value", since: "nonsense", wantUsage2: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			window, err := parseExpenseWindow(tt.since, tt.until, now)
			if err != nil {
				if !tt.wantUsage2 {
					t.Fatalf("parseExpenseWindow() error = %v", err)
				}
				var typed *cliError
				if wrapped := usageErr(err); !errors.As(wrapped, &typed) || typed.code != 2 {
					t.Fatalf("usageErr(parse error) = %#v, want cliError code 2", wrapped)
				}
				return
			}
			if tt.wantUsage2 {
				t.Fatal("parseExpenseWindow() succeeded, want invalid-value error")
			}
			got := 0
			for _, expense := range expenses {
				if expenseInWindow(expense, window) {
					got++
				}
			}
			if got != tt.wantCount {
				t.Fatalf("matching expenses = %d, want %d", got, tt.wantCount)
			}
		})
	}
}
