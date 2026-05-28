package cli

import "testing"

func TestParseAmount(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"84.00", 84.0},
		{"  12.50 ", 12.5},
		{"-7.25", -7.25},
		{"0", 0},
		{"", 0},
		{"not-a-number", 0},
	}
	for _, c := range cases {
		if got := parseAmount(c.in); got != c.want {
			t.Errorf("parseAmount(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestFriendDisplayName(t *testing.T) {
	cases := []struct {
		f    Friend
		want string
	}{
		{Friend{FirstName: "Alex", LastName: "Kim"}, "Alex Kim"},
		{Friend{FirstName: "Sam", LastName: ""}, "Sam"},
		{Friend{FirstName: "", LastName: "Lee"}, "Lee"},
		{Friend{}, ""},
	}
	for _, c := range cases {
		if got := friendDisplayName(c.f); got != c.want {
			t.Errorf("friendDisplayName(%+v) = %q, want %q", c.f, got, c.want)
		}
	}
}

func TestStripHTML(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"You paid <strong>Alex</strong> $5.00", "You paid Alex $5.00"},
		{"line one<br>line two", "line one line two"},
		{`<font color="#5bc5a7">You paid $10.00</font>`, "You paid $10.00"},
		{"Bill &amp; Ted", "Bill & Ted"},
		{"  plain text  ", "plain text"},
		{"", ""},
	}
	for _, c := range cases {
		if got := stripHTML(c.in); got != c.want {
			t.Errorf("stripHTML(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseSplitwiseDate(t *testing.T) {
	if _, ok := parseSplitwiseDate("2026-05-20T18:30:00Z"); !ok {
		t.Errorf("parseSplitwiseDate(RFC3339) ok = false, want true")
	}
	if _, ok := parseSplitwiseDate("garbage"); ok {
		t.Errorf("parseSplitwiseDate(garbage) ok = true, want false")
	}
}

func TestOldestExpenseForFriend(t *testing.T) {
	expenses := []Expense{
		{ID: 1, Date: "2026-03-01T00:00:00Z", Description: "older", Users: []ExpenseUser{{UserID: 42}}},
		{ID: 2, Date: "2026-05-01T00:00:00Z", Description: "newer", Users: []ExpenseUser{{UserID: 42}}},
		{ID: 3, Date: "2026-01-01T00:00:00Z", Description: "other-friend", Users: []ExpenseUser{{UserID: 99}}},
	}
	when, _, found, parsed := oldestExpenseForFriend(expenses, 42)
	if !found || !parsed {
		t.Fatalf("oldestExpenseForFriend(42) found=%v parsed=%v, want both true", found, parsed)
	}
	if when.Month() != 3 {
		t.Errorf("oldest month = %d, want 3 (the March expense is oldest for friend 42)", when.Month())
	}
	if _, _, found, _ := oldestExpenseForFriend(expenses, 7); found {
		t.Errorf("oldestExpenseForFriend(7) found = true, want false (no matching expense)")
	}
	// Unparseable date: found but not parsed (must not fabricate an age).
	bad := []Expense{{ID: 9, Date: "not-a-date", Users: []ExpenseUser{{UserID: 5}}}}
	if _, _, found, parsed := oldestExpenseForFriend(bad, 5); !found || parsed {
		t.Errorf("unparseable date: found=%v parsed=%v, want found=true parsed=false", found, parsed)
	}
}

func TestResolveSettleGroup(t *testing.T) {
	groups := []Group{{ID: 10, Name: "Tahoe Trip"}, {ID: 20, Name: "Apartment"}}
	if g, ok := resolveSettleGroup("10", groups); !ok || g.ID != 10 {
		t.Errorf("resolveSettleGroup(\"10\") = (%+v, %v), want id 10", g, ok)
	}
	if g, ok := resolveSettleGroup("tahoe", groups); !ok || g.ID != 10 {
		t.Errorf("resolveSettleGroup(\"tahoe\") = (%+v, %v), want id 10 (case-insensitive substring)", g, ok)
	}
	if _, ok := resolveSettleGroup("nonexistent", groups); ok {
		t.Errorf("resolveSettleGroup(\"nonexistent\") ok = true, want false")
	}
}

func TestResolveSettleFriend(t *testing.T) {
	friends := []Friend{{ID: 1, FirstName: "Alex", LastName: "Kim"}, {ID: 2, FirstName: "Sam", LastName: "Lee"}}
	if f, ok := resolveSettleFriend("alex", friends); !ok || f.ID != 1 {
		t.Errorf("resolveSettleFriend(\"alex\") = (%+v, %v), want id 1", f, ok)
	}
	if f, ok := resolveSettleFriend("Lee", friends); !ok || f.ID != 2 {
		t.Errorf("resolveSettleFriend(\"Lee\") = (%+v, %v), want id 2 (last-name match)", f, ok)
	}
	if _, ok := resolveSettleFriend("nobody", friends); ok {
		t.Errorf("resolveSettleFriend(\"nobody\") ok = true, want false")
	}
}
