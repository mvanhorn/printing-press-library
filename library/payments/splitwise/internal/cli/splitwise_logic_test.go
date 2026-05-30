package cli

import (
	"strings"
	"testing"
)

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
	if g, ok, err := resolveSettleGroup("10", groups); !ok || err != nil || g.ID != 10 {
		t.Errorf("resolveSettleGroup(\"10\") = (%+v, %v, %v), want id 10", g, ok, err)
	}
	if g, ok, err := resolveSettleGroup("tahoe", groups); !ok || err != nil || g.ID != 10 {
		t.Errorf("resolveSettleGroup(\"tahoe\") = (%+v, %v, %v), want id 10 (case-insensitive substring)", g, ok, err)
	}
	if _, ok, err := resolveSettleGroup("nonexistent", groups); ok || err != nil {
		t.Errorf("resolveSettleGroup(\"nonexistent\") = (ok=%v, err=%v), want (false, nil)", ok, err)
	}
}

func TestResolveSettleGroup_Ambiguous(t *testing.T) {
	// Three "Shy 25" trips: a bare "Shy 25" must error, not silently pick one.
	groups := []Group{
		{ID: 1, Name: "Shy 25 Does Vegas 2021"},
		{ID: 2, Name: "Shy 25 Weekend January 2023"},
		{ID: 3, Name: "Shy 25 2023"},
	}
	if _, ok, err := resolveSettleGroup("Shy 25", groups); ok || err == nil {
		t.Errorf("resolveSettleGroup(\"Shy 25\") = (ok=%v, err=%v), want ambiguous error", ok, err)
	}
	// An exact name wins even though it is a substring of the others.
	if g, ok, err := resolveSettleGroup("Shy 25 2023", groups); !ok || err != nil || g.ID != 3 {
		t.Errorf("resolveSettleGroup(\"Shy 25 2023\") = (%+v, %v, %v), want id 3 (exact-match preference)", g, ok, err)
	}
	// Duplicate exact names remain ambiguous.
	dup := []Group{{ID: 11, Name: "ABGT500"}, {ID: 12, Name: "ABGT500"}}
	if _, ok, err := resolveSettleGroup("ABGT500", dup); ok || err == nil {
		t.Errorf("resolveSettleGroup(\"ABGT500\") with duplicate names = (ok=%v, err=%v), want ambiguous error", ok, err)
	}
}

func TestResolveSettleGroup_AmbiguousMessageCapsCandidates(t *testing.T) {
	groups := []Group{
		{ID: 1, Name: "Trip Alpha 1"},
		{ID: 2, Name: "Trip Alpha 2"},
		{ID: 3, Name: "Trip Alpha 3"},
		{ID: 4, Name: "Trip Alpha 4"},
		{ID: 5, Name: "Trip Alpha 5"},
		{ID: 6, Name: "Trip Alpha 6"},
		{ID: 7, Name: "Trip Alpha 7"},
	}
	_, ok, err := resolveSettleGroup("Alpha", groups)
	if ok || err == nil {
		t.Fatalf("resolveSettleGroup(\"Alpha\") = (ok=%v, err=%v), want ambiguous error", ok, err)
	}

	msg := err.Error()
	if !strings.Contains(msg, "matches 7 groups") {
		t.Fatalf("ambiguous error missing full match count prefix: %q", msg)
	}
	if !strings.Contains(msg, "and 2 more") {
		t.Fatalf("ambiguous error missing remainder count: %q", msg)
	}
	if strings.Count(msg, "(id ") != 5 {
		t.Fatalf("ambiguous error listed %d candidates, want exactly 5: %q", strings.Count(msg, "(id "), msg)
	}
}

func TestResolveSettleFriend(t *testing.T) {
	friends := []Friend{{ID: 1, FirstName: "Alex", LastName: "Kim"}, {ID: 2, FirstName: "Sam", LastName: "Lee"}}
	if f, ok, err := resolveSettleFriend("alex", friends); !ok || err != nil || f.ID != 1 {
		t.Errorf("resolveSettleFriend(\"alex\") = (%+v, %v, %v), want id 1", f, ok, err)
	}
	if f, ok, err := resolveSettleFriend("Lee", friends); !ok || err != nil || f.ID != 2 {
		t.Errorf("resolveSettleFriend(\"Lee\") = (%+v, %v, %v), want id 2 (last-name match)", f, ok, err)
	}
	if _, ok, err := resolveSettleFriend("nobody", friends); ok || err != nil {
		t.Errorf("resolveSettleFriend(\"nobody\") = (ok=%v, err=%v), want (false, nil)", ok, err)
	}
}

func TestResolveSettleFriend_Ambiguous(t *testing.T) {
	// Two Michaels: a bare first name must error rather than guess.
	friends := []Friend{{ID: 1, FirstName: "Michael", LastName: "Stone"}, {ID: 2, FirstName: "Michael", LastName: "Reed"}}
	if _, ok, err := resolveSettleFriend("Michael", friends); ok || err == nil {
		t.Errorf("resolveSettleFriend(\"Michael\") = (ok=%v, err=%v), want ambiguous error", ok, err)
	}
	// Full name disambiguates.
	if f, ok, err := resolveSettleFriend("Michael Reed", friends); !ok || err != nil || f.ID != 2 {
		t.Errorf("resolveSettleFriend(\"Michael Reed\") = (%+v, %v, %v), want id 2", f, ok, err)
	}
}
