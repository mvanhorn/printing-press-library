package cli

import (
	"testing"
	"time"
)

func TestComputeFairnessAnchors(t *testing.T) {
	youID := 1
	carol := Friend{ID: 4, FirstName: "Carol", LastName: "EDCO", Balance: []Balance{{CurrencyCode: "USD", Amount: "100.00"}}}
	bob := Friend{ID: 3, FirstName: "Bob", LastName: "Brown", Balance: []Balance{{CurrencyCode: "USD", Amount: "0.00"}}}
	dave := Friend{ID: 5, FirstName: "Dave", LastName: "New", Balance: []Balance{{CurrencyCode: "USD", Amount: "0.00"}}}
	erin := Friend{ID: 6, FirstName: "Erin", LastName: "Carrier", Balance: []Balance{{CurrencyCode: "USD", Amount: "10.00"}}}
	frank := Friend{ID: 7, FirstName: "Frank", LastName: "FX", Balance: []Balance{{CurrencyCode: "USD", Amount: "30.00"}, {CurrencyCode: "EUR", Amount: "20.00"}}}
	nudger := Friend{ID: 8, FirstName: "Nadia", LastName: "Nudge", Balance: []Balance{{CurrencyCode: "USD", Amount: "40.00"}}}
	chaser := Friend{ID: 9, FirstName: "Chase", LastName: "Risk", Balance: []Balance{{CurrencyCode: "USD", Amount: "60.00"}}}

	friends := []Friend{carol, bob, dave, erin, frank, nudger, chaser}
	groups := []Group{
		{ID: 42, Name: "Trip", Members: []GroupMember{{ID: youID, FirstName: "You"}, {ID: carol.ID, FirstName: "Carol"}}, SimplifiedDebts: []SimplifiedDebt{{From: carol.ID, To: youID, Amount: "100.00", CurrencyCode: "USD"}}},
	}

	expenses := []Expense{
		// A. Carol write-off shape (open, old, no payment)
		{ID: 1, Description: "E1", CurrencyCode: "USD", Date: "2025-01-10", Payment: false, Users: []ExpenseUser{{UserID: carol.ID, PaidShare: "0", OwedShare: "50"}, {UserID: youID, PaidShare: "100", OwedShare: "50"}}},
		{ID: 2, Description: "E2", CurrencyCode: "USD", Date: "2025-03-10", Payment: false, Users: []ExpenseUser{{UserID: carol.ID, PaidShare: "0", OwedShare: "50"}, {UserID: youID, PaidShare: "100", OwedShare: "50"}}},
		// B. Bob closed episode
		{ID: 3, Description: "X1", CurrencyCode: "USD", Date: "2025-12-01", Payment: false, Users: []ExpenseUser{{UserID: bob.ID, PaidShare: "0", OwedShare: "20"}, {UserID: youID, PaidShare: "40", OwedShare: "20"}}},
		{ID: 4, Description: "X2", CurrencyCode: "USD", Date: "2025-12-20", Payment: true, Users: []ExpenseUser{{UserID: bob.ID, PaidShare: "20", OwedShare: "0"}}},
		// D. Erin carrier
		{ID: 5, Description: "C1", CurrencyCode: "USD", Date: "2025-06-01", Payment: false, Users: []ExpenseUser{{UserID: erin.ID, PaidShare: "80", OwedShare: "20"}}},
		{ID: 6, Description: "C2", CurrencyCode: "USD", Date: "2025-06-15", Payment: false, Users: []ExpenseUser{{UserID: erin.ID, PaidShare: "60", OwedShare: "20"}}},
		// E. Frank has history for collectability lens
		{ID: 13, Description: "F1", CurrencyCode: "USD", Date: "2025-09-01", Payment: false, Users: []ExpenseUser{{UserID: frank.ID, PaidShare: "0", OwedShare: "30"}, {UserID: youID, PaidShare: "30", OwedShare: "0"}}},
		// F. nudge (~45)
		{ID: 7, Description: "N1", CurrencyCode: "USD", Date: "2025-08-01", Payment: false, Users: []ExpenseUser{{UserID: nudger.ID, PaidShare: "0", OwedShare: "40"}, {UserID: youID, PaidShare: "40", OwedShare: "0"}}},
		{ID: 8, Description: "N2", CurrencyCode: "USD", Date: "2025-08-15", Payment: true, Users: []ExpenseUser{{UserID: nudger.ID, PaidShare: "40", OwedShare: "0"}}},
		{ID: 9, Description: "N3", CurrencyCode: "USD", Date: "2025-08-20", Payment: false, Users: []ExpenseUser{{UserID: nudger.ID, PaidShare: "0", OwedShare: "40"}, {UserID: youID, PaidShare: "40", OwedShare: "0"}}},
		// F. chase (~70)
		{ID: 10, Description: "R1", CurrencyCode: "USD", Date: "2025-02-01", Payment: false, Users: []ExpenseUser{{UserID: chaser.ID, PaidShare: "0", OwedShare: "60"}, {UserID: youID, PaidShare: "60", OwedShare: "0"}}},
		{ID: 11, Description: "R2", CurrencyCode: "USD", Date: "2025-05-31", Payment: true, Users: []ExpenseUser{{UserID: chaser.ID, PaidShare: "60", OwedShare: "0"}}},
		{ID: 12, Description: "R3", CurrencyCode: "USD", Date: "2025-06-01", Payment: false, Users: []ExpenseUser{{UserID: chaser.ID, PaidShare: "0", OwedShare: "60"}, {UserID: youID, PaidShare: "60", OwedShare: "0"}}},
	}

	tests := []struct {
		name string
		now  time.Time
		opts fairnessOpts
		fn   func(t *testing.T, res fairnessResult)
	}{
		{
			name: "A write-off risk exact score",
			now:  time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
			opts: fairnessOpts{by: "risk", writeOffDays: 365, ghostDays: 180, minEpisodes: 1},
			fn: func(t *testing.T, res fairnessResult) {
				carol := findFairnessPerson(t, res.People, 4)
				if !carol.HasHistory {
					t.Fatalf("Carol HasHistory=false")
				}
				if carol.Paid != 0 || carol.Owed != 100 {
					t.Fatalf("Carol paid/owed got %.2f/%.2f", carol.Paid, carol.Owed)
				}
				if carol.CarryRatio == nil || *carol.CarryRatio != 0 {
					t.Fatalf("Carol carry ratio got %v", carol.CarryRatio)
				}
				if carol.Role != "rider" {
					t.Fatalf("Carol role=%q", carol.Role)
				}
				if carol.OutstandingTotal != 100 {
					t.Fatalf("Carol outstanding=%.2f", carol.OutstandingTotal)
				}
				if carol.DebtAgeDays == nil || *carol.DebtAgeDays < 365 {
					t.Fatalf("Carol debt age=%v", carol.DebtAgeDays)
				}
				if carol.LastSettledDays != nil {
					t.Fatalf("Carol last settled expected nil, got %v", *carol.LastSettledDays)
				}
				if carol.AvgLatencyDays != nil {
					t.Fatalf("Carol avg latency expected nil, got %v", *carol.AvgLatencyDays)
				}
				if carol.RiskScore == nil || *carol.RiskScore != 90 {
					t.Fatalf("Carol risk score got %v", carol.RiskScore)
				}
				if carol.RiskTier != "write_off" {
					t.Fatalf("Carol tier=%q", carol.RiskTier)
				}
				if res.WriteOffTotal != 100 {
					t.Fatalf("WriteOffTotal=%.2f", res.WriteOffTotal)
				}
			},
		},
		{
			name: "B settled excluded from risk",
			now:  time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC),
			opts: fairnessOpts{by: "risk", writeOffDays: 365, ghostDays: 180, minEpisodes: 1},
			fn: func(t *testing.T, res fairnessResult) {
				assertAbsentPerson(t, res.People, 3)
			},
		},
		{
			name: "B collectability includes settled",
			now:  time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC),
			opts: fairnessOpts{by: "collectability", writeOffDays: 365, ghostDays: 180, minEpisodes: 1},
			fn: func(t *testing.T, res fairnessResult) {
				bob := findFairnessPerson(t, res.People, 3)
				if !bob.HasHistory {
					t.Fatalf("Bob HasHistory=false")
				}
				if bob.AvgLatencyDays == nil || *bob.AvgLatencyDays != 19 {
					t.Fatalf("Bob avg latency=%v", bob.AvgLatencyDays)
				}
				if bob.LastSettledDays == nil || *bob.LastSettledDays != 5 {
					t.Fatalf("Bob last settled=%v", bob.LastSettledDays)
				}
				if bob.DebtAgeDays != nil {
					t.Fatalf("Bob debt age expected nil, got %v", *bob.DebtAgeDays)
				}
				if bob.OutstandingTotal != 0 {
					t.Fatalf("Bob outstanding=%.2f", bob.OutstandingTotal)
				}
			},
		},
		{
			name: "C new member counting",
			now:  time.Date(2025, 12, 25, 0, 0, 0, 0, time.UTC),
			opts: fairnessOpts{by: "risk", writeOffDays: 365, ghostDays: 180, minEpisodes: 1},
			fn: func(t *testing.T, res fairnessResult) {
				if res.NewMembers != 1 {
					t.Fatalf("NewMembers=%d", res.NewMembers)
				}
				assertAbsentPerson(t, res.People, 5)
			},
		},
		{
			name: "D carrier leads contribution",
			now:  time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
			opts: fairnessOpts{by: "contribution", writeOffDays: 365, ghostDays: 180, minEpisodes: 1},
			fn: func(t *testing.T, res fairnessResult) {
				erin := findFairnessPerson(t, res.People, 6)
				if erin.Paid != 140 || erin.Owed != 40 || erin.Net != 100 {
					t.Fatalf("Erin paid/owed/net %.2f/%.2f/%.2f", erin.Paid, erin.Owed, erin.Net)
				}
				if erin.CarryRatio == nil || *erin.CarryRatio != 3.5 {
					t.Fatalf("Erin ratio=%v", erin.CarryRatio)
				}
				if erin.Role != "carrier" {
					t.Fatalf("Erin role=%q", erin.Role)
				}
				if len(res.People) < 2 {
					t.Fatalf("need at least 2 people")
				}
				if res.People[0].UserID != 6 {
					t.Fatalf("expected Erin first, got user_id=%d", res.People[0].UserID)
				}
			},
		},
		{
			name: "E multi currency all",
			now:  time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
			opts: fairnessOpts{by: "collectability", writeOffDays: 365, ghostDays: 180, minEpisodes: 1},
			fn: func(t *testing.T, res fairnessResult) {
				frank := findFairnessPerson(t, res.People, 7)
				if frank.OutstandingByCurrency["USD"] != 30 || frank.OutstandingByCurrency["EUR"] != 20 || frank.OutstandingTotal != 50 {
					t.Fatalf("Frank outstanding map=%v total=%.2f", frank.OutstandingByCurrency, frank.OutstandingTotal)
				}
			},
		},
		{
			name: "E multi currency USD filter",
			now:  time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
			opts: fairnessOpts{by: "collectability", writeOffDays: 365, ghostDays: 180, minEpisodes: 1, currency: "USD"},
			fn: func(t *testing.T, res fairnessResult) {
				frank := findFairnessPerson(t, res.People, 7)
				if len(frank.OutstandingByCurrency) != 1 || frank.OutstandingByCurrency["USD"] != 30 || frank.OutstandingTotal != 30 {
					t.Fatalf("Frank USD-filter map=%v total=%.2f", frank.OutstandingByCurrency, frank.OutstandingTotal)
				}
			},
		},
		{
			name: "F nudge vs chase",
			now:  time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
			opts: fairnessOpts{by: "risk", writeOffDays: 365, ghostDays: 180, minEpisodes: 1},
			fn: func(t *testing.T, res fairnessResult) {
				n := findFairnessPerson(t, res.People, 8)
				c := findFairnessPerson(t, res.People, 9)
				if n.RiskTier != "nudge" {
					t.Fatalf("nudger tier=%q score=%v", n.RiskTier, n.RiskScore)
				}
				if c.RiskTier != "chase" {
					t.Fatalf("chaser tier=%q score=%v", c.RiskTier, c.RiskScore)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := computeFairness(youID, friends, groups, expenses, tt.now, tt.opts)
			tt.fn(t, res)
		})
	}
}

func TestComputeFairnessGroupScope(t *testing.T) {
	youID := 1
	friends := []Friend{{ID: 4, FirstName: "Carol", LastName: "EDCO"}}
	groups := []Group{{
		ID:   42,
		Name: "Trip",
		Members: []GroupMember{
			{ID: youID, FirstName: "You"},
			{ID: 4, FirstName: "Carol"},
		},
		SimplifiedDebts: []SimplifiedDebt{{From: 4, To: youID, Amount: "100.00", CurrencyCode: "USD"}},
	}}
	expenses := []Expense{{ID: 1, GroupID: 42, CurrencyCode: "USD", Date: "2025-01-10", Users: []ExpenseUser{{UserID: 4, PaidShare: "0", OwedShare: "50"}}}}
	res := computeFairness(youID, friends, groups, expenses, time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC), fairnessOpts{by: "risk", writeOffDays: 365, ghostDays: 180, minEpisodes: 1, groupID: 42, groupScoped: true})
	if !res.GroupCaveat {
		t.Fatalf("GroupCaveat=false")
	}
	if res.Scope != "group:Trip" {
		t.Fatalf("scope=%q", res.Scope)
	}
	if len(res.People) != 1 || res.People[0].UserID != 4 {
		t.Fatalf("unexpected people: %+v", res.People)
	}
}

func TestClampUnit(t *testing.T) {
	cases := []struct {
		in   float64
		want float64
	}{
		{in: -1, want: 0},
		{in: 0.5, want: 0.5},
		{in: 2, want: 1},
	}
	for _, tc := range cases {
		got := clampUnit(tc.in)
		if got != tc.want {
			t.Fatalf("clampUnit(%.2f)=%.2f want %.2f", tc.in, got, tc.want)
		}
	}
}

func TestEpisodeMetricsTwoCycles(t *testing.T) {
	now := time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)
	entries := []subjectEvent{
		{date: mustDate(t, "2025-01-01"), payment: false},
		{date: mustDate(t, "2025-01-11"), payment: true},
		{date: mustDate(t, "2025-02-01"), payment: false},
		{date: mustDate(t, "2025-02-21"), payment: true},
	}
	m := episodeMetrics(now, entries, 2)
	if m.avgLatencyDays == nil || *m.avgLatencyDays != 15 {
		t.Fatalf("avg latency=%v", m.avgLatencyDays)
	}
	if m.debtAgeDays != nil {
		t.Fatalf("debt age expected nil, got %v", *m.debtAgeDays)
	}
	if m.lastSettledDays == nil || *m.lastSettledDays <= 0 {
		t.Fatalf("last settled=%v", m.lastSettledDays)
	}
}

func TestClassifyRoleBoundaries(t *testing.T) {
	cases := []struct {
		name       string
		hasHistory bool
		ratio      *float64
		want       string
	}{
		{name: "new", hasHistory: false, ratio: ptrFloat(1), want: "new"},
		{name: "rider lower", hasHistory: true, ratio: ptrFloat(0.89), want: "rider"},
		{name: "even low boundary", hasHistory: true, ratio: ptrFloat(0.90), want: "even"},
		{name: "even high boundary", hasHistory: true, ratio: ptrFloat(1.10), want: "even"},
		{name: "carrier", hasHistory: true, ratio: ptrFloat(1.11), want: "carrier"},
		{name: "nil ratio rider", hasHistory: true, ratio: nil, want: "rider"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyRole(tc.hasHistory, tc.ratio); got != tc.want {
				t.Fatalf("role=%q want %q", got, tc.want)
			}
		})
	}
}

func findFairnessPerson(t *testing.T, people []fairnessPerson, id int) fairnessPerson {
	t.Helper()
	for _, p := range people {
		if p.UserID == id {
			return p
		}
	}
	t.Fatalf("person %d not found", id)
	return fairnessPerson{}
}

func assertAbsentPerson(t *testing.T, people []fairnessPerson, id int) {
	t.Helper()
	for _, p := range people {
		if p.UserID == id {
			t.Fatalf("person %d unexpectedly present", id)
		}
	}
}

func ptrFloat(v float64) *float64 { return &v }

func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, ok := parseSplitwiseDate(s)
	if !ok {
		t.Fatalf("bad date %q", s)
	}
	return d
}
