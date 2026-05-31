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

func TestComputeFairnessSinceKeepsOutstandingDebtor(t *testing.T) {
	youID := 1
	// Olivia's only shared expense is in 2024 (before --since), but she still
	// owes you $40 via Friend.Balance. She must NOT be dropped or counted as new.
	friends := []Friend{{ID: 50, FirstName: "Olivia", LastName: "Old", Balance: []Balance{{CurrencyCode: "USD", Amount: "40.00"}}}}
	expenses := []Expense{
		{ID: 1, CurrencyCode: "USD", Date: "2024-03-01", Users: []ExpenseUser{{UserID: 50, PaidShare: "0", OwedShare: "40"}, {UserID: youID, PaidShare: "40", OwedShare: "0"}}},
	}
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	opts := fairnessOpts{by: "risk", writeOffDays: 365, ghostDays: 180, minEpisodes: 1,
		since: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), hasSince: true}
	res := computeFairness(youID, friends, nil, expenses, now, opts)
	o := findFairnessPerson(t, res.People, 50) // fails (person not found) without the fix
	if !o.HasHistory {
		t.Fatalf("Olivia HasHistory=false despite an open balance")
	}
	if o.OutstandingTotal != 40 {
		t.Fatalf("Olivia outstanding=%.2f", o.OutstandingTotal)
	}
	if res.NewMembers != 0 {
		t.Fatalf("NewMembers=%d (an outstanding debtor was wrongly counted as new)", res.NewMembers)
	}
	// Debt age uses FULL history, not the --since window, so an old silent debt
	// is still classified write_off — --since must not suppress the tier.
	if o.DebtAgeDays == nil || *o.DebtAgeDays < 365 {
		t.Fatalf("Olivia debt age=%v (should reflect the 2024 expense, not be windowed away)", o.DebtAgeDays)
	}
	if o.RiskTier != "write_off" {
		t.Fatalf("Olivia tier=%q, expected write_off (--since must not downgrade an old debt)", o.RiskTier)
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

func TestHumanizeDays(t *testing.T) {
	cases := []struct {
		in   int
		want string
	}{
		{in: -3, want: "0d"},
		{in: 0, want: "0d"},
		{in: 5, want: "5d"},
		{in: 6, want: "6d"},
		{in: 7, want: "1w"},
		{in: 19, want: "2w 5d"},
		{in: 29, want: "4w 1d"},
		{in: 30, want: "1mo"},
		{in: 48, want: "1mo 18d"},
		{in: 334, want: "11mo 4d"},
		{in: 365, want: "1y"},
		{in: 366, want: "1y 1d"},
		{in: 900, want: "2y 5mo 20d"},
		{in: 1558, want: "4y 3mo 8d"},
	}
	for _, tc := range cases {
		if got := humanizeDays(tc.in); got != tc.want {
			t.Fatalf("humanizeDays(%d)=%q want %q", tc.in, got, tc.want)
		}
	}
}

func TestAgeCell(t *testing.T) {
	if got := ageCell(nil); got != "-" {
		t.Fatalf("ageCell(nil)=%q want %q", got, "-")
	}
	z := 0
	if got := ageCell(&z); got != "0d" {
		t.Fatalf("ageCell(0)=%q want 0d", got)
	}
	n := 1558
	if got := ageCell(&n); got != "4y 3mo 8d" {
		t.Fatalf("ageCell(1558)=%q want 4y 3mo 8d", got)
	}
}

func TestProjectSettle(t *testing.T) {
	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)

	lat := 30.0
	age := 10
	daysOut, date := projectSettle(&age, &lat, now)
	if daysOut == nil || *daysOut != 20 {
		t.Fatalf("daysOut=%v want 20", daysOut)
	}
	if date == nil || *date != "2026-01-30" {
		t.Fatalf("date=%v want 2026-01-30", date)
	}

	age = 40
	daysOut, date = projectSettle(&age, &lat, now)
	if daysOut == nil || *daysOut != -10 {
		t.Fatalf("daysOut=%v want -10", daysOut)
	}
	if date == nil || *date != "2025-12-31" {
		t.Fatalf("date=%v want 2025-12-31", date)
	}

	daysOut, date = projectSettle(nil, &lat, now)
	if daysOut != nil || date != nil {
		t.Fatalf("nil debt age expected nil,nil got %v,%v", daysOut, date)
	}

	daysOut, date = projectSettle(&age, nil, now)
	if daysOut != nil || date != nil {
		t.Fatalf("nil avg latency expected nil,nil got %v,%v", daysOut, date)
	}

	rounded := 29.6
	age = 10
	daysOut, date = projectSettle(&age, &rounded, now)
	if daysOut == nil || *daysOut != 20 {
		t.Fatalf("rounded daysOut=%v want 20", daysOut)
	}
	if date == nil || *date != "2026-01-30" {
		t.Fatalf("rounded date=%v want 2026-01-30", date)
	}
}

func TestComputeFairnessCollectabilityProjectedSettle(t *testing.T) {
	youID := 1
	alex := Friend{ID: 2, FirstName: "Alex", LastName: "Late", Balance: []Balance{{CurrencyCode: "USD", Amount: "30.00"}}}
	blair := Friend{ID: 3, FirstName: "Blair", LastName: "New", Balance: []Balance{{CurrencyCode: "USD", Amount: "20.00"}}}
	// Casey is fully settled (owes 0) but has a closed episode (latency) PLUS a
	// dangling open episode (debtAge) — so projectSettle would otherwise fire.
	// The outstanding>0 gate must suppress it: a $0 balance never gets a
	// projected settle date (this was caught in live dogfood, where the only
	// people with projections were settled debtors showing "overdue by 2459d").
	casey := Friend{ID: 6, FirstName: "Casey", LastName: "Settled"}
	friends := []Friend{alex, blair, casey}

	expenses := []Expense{
		{ID: 1, Description: "A1", CurrencyCode: "USD", Date: "2025-10-01", Payment: false, Users: []ExpenseUser{{UserID: alex.ID, PaidShare: "0", OwedShare: "30"}, {UserID: youID, PaidShare: "30", OwedShare: "0"}}},
		{ID: 2, Description: "A2", CurrencyCode: "USD", Date: "2025-10-21", Payment: true, Users: []ExpenseUser{{UserID: alex.ID, PaidShare: "30", OwedShare: "0"}}},
		{ID: 3, Description: "A3", CurrencyCode: "USD", Date: "2025-12-20", Payment: false, Users: []ExpenseUser{{UserID: alex.ID, PaidShare: "0", OwedShare: "30"}, {UserID: youID, PaidShare: "30", OwedShare: "0"}}},
		{ID: 4, Description: "B1", CurrencyCode: "USD", Date: "2025-12-25", Payment: false, Users: []ExpenseUser{{UserID: blair.ID, PaidShare: "0", OwedShare: "20"}, {UserID: youID, PaidShare: "20", OwedShare: "0"}}},
		{ID: 5, Description: "C1", CurrencyCode: "USD", Date: "2025-09-01", Payment: false, Users: []ExpenseUser{{UserID: casey.ID, PaidShare: "0", OwedShare: "15"}, {UserID: youID, PaidShare: "15", OwedShare: "0"}}},
		{ID: 6, Description: "C2", CurrencyCode: "USD", Date: "2025-09-20", Payment: true, Users: []ExpenseUser{{UserID: casey.ID, PaidShare: "15", OwedShare: "0"}}},
		{ID: 7, Description: "C3", CurrencyCode: "USD", Date: "2025-12-01", Payment: false, Users: []ExpenseUser{{UserID: casey.ID, PaidShare: "0", OwedShare: "15"}, {UserID: youID, PaidShare: "15", OwedShare: "0"}}},
	}

	now := time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)
	res := computeFairness(youID, friends, nil, expenses, now, fairnessOpts{by: "collectability", writeOffDays: 365, ghostDays: 180, minEpisodes: 1})

	alexPerson := findFairnessPerson(t, res.People, 2)
	if alexPerson.ProjectedDaysOut == nil {
		t.Fatalf("Alex projected_days_out=nil")
	}
	if *alexPerson.ProjectedDaysOut >= 0 {
		t.Fatalf("Alex projected_days_out=%d want negative (overdue)", *alexPerson.ProjectedDaysOut)
	}
	if alexPerson.ProjectedSettleDate == nil {
		t.Fatalf("Alex projected_settle_date=nil")
	}

	blairPerson := findFairnessPerson(t, res.People, 3)
	if blairPerson.ProjectedDaysOut != nil || blairPerson.ProjectedSettleDate != nil {
		t.Fatalf("Blair projection expected nil,nil got %v,%v", blairPerson.ProjectedDaysOut, blairPerson.ProjectedSettleDate)
	}

	caseyPerson := findFairnessPerson(t, res.People, 6)
	if caseyPerson.OutstandingTotal != 0 {
		t.Fatalf("Casey outstanding=%.2f want 0 (test fixture expects a settled debtor)", caseyPerson.OutstandingTotal)
	}
	if caseyPerson.ProjectedDaysOut != nil || caseyPerson.ProjectedSettleDate != nil {
		t.Fatalf("Casey (settled, owes 0) projection expected nil,nil got %v,%v", caseyPerson.ProjectedDaysOut, caseyPerson.ProjectedSettleDate)
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

func TestRoleForContributionCarrierWhenPaidButOwesNothing(t *testing.T) {
	r110 := 1.10
	r200 := 2.00
	r050 := 0.50
	cases := []struct {
		name       string
		hasHistory bool
		paid, owed float64
		ratio      *float64
		want       string
	}{
		{"paid but owes nothing -> carrier", true, 50, 0, nil, "carrier"},
		{"no history -> new even if paid>0 owed==0", false, 50, 0, nil, "new"},
		{"owes nothing and paid nothing -> rider", true, 0, 0, nil, "rider"},
		{"normal rider (low ratio)", true, 5, 50, &r050, "rider"},
		{"even ratio", true, 55, 50, &r110, "even"},
		{"carrier via high ratio", true, 100, 50, &r200, "carrier"},
	}
	for _, c := range cases {
		if got := roleForContribution(c.hasHistory, c.paid, c.owed, c.ratio); got != c.want {
			t.Errorf("%s: roleForContribution(%v,%v,%v,ratio)=%q want %q", c.name, c.hasHistory, c.paid, c.owed, got, c.want)
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
