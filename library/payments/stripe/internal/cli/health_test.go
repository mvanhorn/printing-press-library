// Copyright 2026 chris-rodriguez. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestComputeHealthScore(t *testing.T) {
	cases := []struct {
		name        string
		in          healthInputs
		wantScore   int
		wantFactors int
	}{
		{
			name:        "pristine customer scores 100",
			in:          healthInputs{AccountAgeDays: 365, DaysSinceLastCharge: 5},
			wantScore:   100,
			wantFactors: 0,
		},
		{
			name: "single dispute deducts 25",
			in: healthInputs{
				OpenDisputes:        1,
				AccountAgeDays:      365,
				DaysSinceLastCharge: 5,
			},
			wantScore:   75,
			wantFactors: 1,
		},
		{
			name: "failed-charge deduction caps at -30",
			in: healthInputs{
				FailedChargesIn30d:  20,
				AccountAgeDays:      365,
				DaysSinceLastCharge: 5,
			},
			wantScore:   70,
			wantFactors: 1,
		},
		{
			name: "subscription unpaid + new account + stale revenue stacks",
			in: healthInputs{
				SubscriptionStatus:  "unpaid",
				AccountAgeDays:      10,
				DaysSinceLastCharge: 90,
			},
			wantScore:   60, // 100 - 25 - 5 - 10
			wantFactors: 3,
		},
		{
			name: "multi-sub bonus applies",
			in: healthInputs{
				ActiveSubscriptions: 2,
				AccountAgeDays:      365,
				DaysSinceLastCharge: 5,
			},
			wantScore:   100, // capped at 100, but factor still recorded
			wantFactors: 1,
		},
		{
			name: "score floors at 0",
			in: healthInputs{
				OpenDisputes:        10,
				FailedChargesIn30d:  20,
				SubscriptionStatus:  "unpaid",
				AccountAgeDays:      5,
				DaysSinceLastCharge: 200,
			},
			wantScore: 0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, factors := computeHealthScore(tc.in)
			if got != tc.wantScore {
				t.Errorf("score = %d, want %d (factors=%v)", got, tc.wantScore, factors)
			}
			if tc.wantFactors > 0 && len(factors) != tc.wantFactors {
				t.Errorf("factor count = %d, want %d (%v)", len(factors), tc.wantFactors, factors)
			}
		})
	}
}

func TestRollupSubStatuses(t *testing.T) {
	cases := []struct {
		name       string
		statuses   []string
		wantWorst  string
		wantActive int
	}{
		{name: "single active", statuses: []string{"active"}, wantWorst: "active", wantActive: 1},
		{name: "active + past_due picks past_due", statuses: []string{"active", "past_due"}, wantWorst: "past_due", wantActive: 1},
		{name: "unpaid trumps everything", statuses: []string{"active", "active", "unpaid"}, wantWorst: "unpaid", wantActive: 2},
		{name: "trialing counts as active", statuses: []string{"trialing", "trialing"}, wantWorst: "trialing", wantActive: 2},
		{name: "none", statuses: nil, wantWorst: "", wantActive: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			worst, active := rollupSubStatuses(tc.statuses)
			if worst != tc.wantWorst {
				t.Errorf("worst = %q, want %q", worst, tc.wantWorst)
			}
			if active != tc.wantActive {
				t.Errorf("active = %d, want %d", active, tc.wantActive)
			}
		})
	}
}

func TestIsReadOnlyQuery(t *testing.T) {
	cases := []struct {
		q    string
		want bool
	}{
		{q: "SELECT * FROM resources", want: true},
		{q: "  select 1", want: true},
		{q: "WITH x AS (SELECT 1) SELECT * FROM x", want: true},
		{q: "INSERT INTO resources VALUES (1,2)", want: false},
		{q: "DELETE FROM resources", want: false},
		{q: "WITH x AS (SELECT 1) DELETE FROM resources", want: false},
		{q: "DROP TABLE resources", want: false},
		{q: "PRAGMA writable_schema=1", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.q, func(t *testing.T) {
			got := isReadOnlyQuery(tc.q)
			if got != tc.want {
				t.Errorf("isReadOnlyQuery(%q) = %t, want %t", tc.q, got, tc.want)
			}
		})
	}
}
