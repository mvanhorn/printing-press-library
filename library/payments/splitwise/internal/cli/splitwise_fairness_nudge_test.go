package cli

import (
	"strings"
	"testing"
)

func TestSelectNudgeExpense(t *testing.T) {
	friendID := 2
	youID := 1
	deleted := "2026-01-01"

	expenses := []Expense{
		{ID: 10, Description: "Deleted", Cost: "10.00", CurrencyCode: "USD", Date: "2026-05-01", DeletedAt: &deleted, Users: []ExpenseUser{{UserID: friendID, OwedShare: "10.00"}, {UserID: youID, PaidShare: "10.00"}}},
		{ID: 11, Description: "Payment", Cost: "11.00", CurrencyCode: "USD", Date: "2026-05-02", Payment: true, Users: []ExpenseUser{{UserID: friendID, OwedShare: "11.00"}, {UserID: youID, PaidShare: "11.00"}}},
		{ID: 12, Description: "Not member", Cost: "12.00", CurrencyCode: "USD", Date: "2026-05-03", Users: []ExpenseUser{{UserID: 999, OwedShare: "12.00"}, {UserID: youID, PaidShare: "12.00"}}},
		{ID: 13, Description: "Friend owes zero", Cost: "13.00", CurrencyCode: "USD", Date: "2026-05-04", Users: []ExpenseUser{{UserID: friendID, OwedShare: "0"}, {UserID: youID, PaidShare: "13.00"}}},
		{ID: 14, Description: "You paid older", Cost: "14.00", CurrencyCode: "USD", Date: "2026-05-05", Users: []ExpenseUser{{UserID: friendID, OwedShare: "14.00"}, {UserID: youID, PaidShare: "14.00"}}},
		{ID: 15, Description: "You paid newest", Cost: "15.00", CurrencyCode: "USD", Date: "2026-05-20", Users: []ExpenseUser{{UserID: friendID, OwedShare: "15.00"}, {UserID: youID, PaidShare: "15.00"}}},
		{ID: 16, Description: "Friend owes fallback", Cost: "16.00", CurrencyCode: "USD", Date: "2026-05-25", Users: []ExpenseUser{{UserID: friendID, OwedShare: "16.00"}, {UserID: youID, PaidShare: "0"}}},
	}

	got, ok := selectNudgeExpense(expenses, friendID, youID)
	if !ok {
		t.Fatalf("selectNudgeExpense returned ok=false")
	}
	if got.ID != 15 {
		t.Fatalf("selected expense id=%d, want 15", got.ID)
	}

	fallbackOnly := []Expense{
		{ID: 21, Description: "Fallback older", Cost: "21.00", CurrencyCode: "USD", Date: "2026-04-01", Users: []ExpenseUser{{UserID: friendID, OwedShare: "21.00"}, {UserID: youID, PaidShare: "0"}}},
		{ID: 22, Description: "Fallback newest", Cost: "22.00", CurrencyCode: "USD", Date: "2026-05-01", Users: []ExpenseUser{{UserID: friendID, OwedShare: "22.00"}, {UserID: youID, PaidShare: "0"}}},
	}
	got, ok = selectNudgeExpense(fallbackOnly, friendID, youID)
	if !ok {
		t.Fatalf("fallback selectNudgeExpense returned ok=false")
	}
	if got.ID != 22 {
		t.Fatalf("fallback selected expense id=%d, want 22", got.ID)
	}

	none := []Expense{
		{ID: 31, Description: "No debt", Cost: "31.00", CurrencyCode: "USD", Date: "2026-05-01", Users: []ExpenseUser{{UserID: friendID, OwedShare: "0"}, {UserID: youID, PaidShare: "31.00"}}},
		{ID: 32, Description: "Payment", Cost: "32.00", CurrencyCode: "USD", Date: "2026-05-02", Payment: true, Users: []ExpenseUser{{UserID: friendID, OwedShare: "32.00"}, {UserID: youID, PaidShare: "32.00"}}},
	}
	_, ok = selectNudgeExpense(none, friendID, youID)
	if ok {
		t.Fatalf("expected no candidate, got ok=true")
	}
}

func TestBuildNudgeMessage(t *testing.T) {
	friendID := 2
	e := Expense{Description: "Dinner", Cost: "42.50", CurrencyCode: "USD", Users: []ExpenseUser{{UserID: friendID, OwedShare: "10.63"}, {UserID: 1, PaidShare: "42.50"}}}
	custom := "Ping when you can, thanks"
	if got := buildNudgeMessage("Alex", friendID, e, custom); got != custom {
		t.Fatalf("custom message = %q, want %q", got, custom)
	}

	got := buildNudgeMessage("Alex", friendID, e, "")
	// The default reminder quotes the friend's SHARE (10.63), not the expense
	// total (42.50) — sending the total would overstate what they owe on a split.
	for _, part := range []string{"Alex", "Dinner", "10.63", "USD", "your share"} {
		if !strings.Contains(got, part) {
			t.Fatalf("default message %q missing %q", got, part)
		}
	}
	if strings.Contains(got, "42.50") {
		t.Fatalf("default message %q must quote the friend's share, not the expense total", got)
	}
}

func TestFindExpenseByID(t *testing.T) {
	expenses := []Expense{
		{ID: 10, Description: "A"},
		{ID: 20, Description: "B"},
	}
	got, ok := findExpenseByID(expenses, 20)
	if !ok || got.ID != 20 {
		t.Fatalf("findExpenseByID(20) = (%+v, %v), want id 20, true", got, ok)
	}
	if _, ok := findExpenseByID(expenses, 99); ok {
		t.Fatalf("findExpenseByID(99) returned ok=true, want false")
	}
}

// TestNudgeAcceptsMultiWordPositional guards the multi-word friend-name path:
// the MCP command-mirror whitespace-splits a quoted friend name into several
// positionals (["Tahoe","Trip"]). cobra.ExactArgs(1) rejected that before RunE
// could rejoin them; the validator must now accept 2+ positionals so the inline
// join can resolve the full name.
func TestNudgeAcceptsMultiWordPositional(t *testing.T) {
	cmd := newFairnessNudgeCmd(&rootFlags{})
	if cmd.Args == nil {
		t.Fatal("nudge command has no Args validator")
	}
	if err := cmd.Args(cmd, []string{"Tahoe", "Trip"}); err != nil {
		t.Errorf("nudge rejected a split multi-word name (2 positionals): %v", err)
	}
	// Zero positionals must still be rejected (MinimumNArgs(1)).
	if err := cmd.Args(cmd, []string{}); err == nil {
		t.Error("nudge accepted zero positionals, want a missing-argument error")
	}
}

func TestNudgeExpenseProblem(t *testing.T) {
	friendID := 2
	owes := []ExpenseUser{{UserID: friendID, OwedShare: "10"}, {UserID: 1, PaidShare: "10"}}
	deleted := "2026-01-01"

	// valid target → no problem
	if got := nudgeExpenseProblem(Expense{Users: owes}, friendID); got != "" {
		t.Fatalf("valid target problem=%q, want empty", got)
	}
	// deleted → flagged
	if got := nudgeExpenseProblem(Expense{DeletedAt: &deleted, Users: owes}, friendID); !strings.Contains(got, "deleted") {
		t.Fatalf("deleted problem=%q, want 'deleted'", got)
	}
	// payment/settlement → flagged
	if got := nudgeExpenseProblem(Expense{Payment: true, Users: owes}, friendID); !strings.Contains(got, "payment") {
		t.Fatalf("payment problem=%q, want 'payment'", got)
	}
	// friend not a member → flagged (prevents a wrong-amount reminder)
	notMember := []ExpenseUser{{UserID: 999, OwedShare: "10"}, {UserID: 1, PaidShare: "10"}}
	if got := nudgeExpenseProblem(Expense{Users: notMember}, friendID); !strings.Contains(got, "owed share") {
		t.Fatalf("non-member problem=%q, want 'owed share'", got)
	}
	// friend on it but owes zero → flagged
	owesZero := []ExpenseUser{{UserID: friendID, OwedShare: "0"}, {UserID: 1, PaidShare: "10"}}
	if got := nudgeExpenseProblem(Expense{Users: owesZero}, friendID); got == "" {
		t.Fatalf("zero-owed problem empty, want flagged")
	}
}
