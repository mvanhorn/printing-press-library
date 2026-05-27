// Copyright 2026 rahul-bansal. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
	"time"
)

func TestExtractCreditAccountAttrs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want creditBalanceRow
	}{
		{
			name: "attributes envelope",
			in:   `{"attributes":{"balance":250,"total_credits":1000,"used_credits":750}}`,
			want: creditBalanceRow{Balance: 250, TotalCredits: 1000, UsedCredits: 750},
		},
		{
			name: "flat shape",
			in:   `{"balance":42,"total_credits":100,"used_credits":58}`,
			want: creditBalanceRow{Balance: 42, TotalCredits: 100, UsedCredits: 58},
		},
		{
			name: "remaining_credits fallback",
			in:   `{"attributes":{"remaining_credits":99}}`,
			want: creditBalanceRow{Balance: 99},
		},
		{
			name: "malformed JSON falls back to zeroes",
			in:   `not json`,
			want: creditBalanceRow{},
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var row creditBalanceRow
			extractCreditAccountAttrs(tc.in, &row)
			if row != tc.want {
				t.Fatalf("got %+v, want %+v", row, tc.want)
			}
		})
	}
}

func TestBuildCreditsForecast(t *testing.T) {
	cases := []struct {
		name        string
		balances    []creditBalanceRow
		logRows     []creditLogRow
		wantOnTrack bool
		wantWindow  int
	}{
		{
			name:        "no deductions, no balance, trivially on track",
			balances:    nil,
			logRows:     nil,
			wantOnTrack: true,
			wantWindow:  14,
		},
		{
			name:        "low burn fits in balance",
			balances:    []creditBalanceRow{{Balance: 1000}},
			logRows:     []creditLogRow{{CreditsUsed: 28}}, // 2/day
			wantOnTrack: true,
			wantWindow:  14,
		},
		{
			name:        "heavy burn exceeds balance",
			balances:    []creditBalanceRow{{Balance: 10}},
			logRows:     []creditLogRow{{CreditsUsed: 700}}, // 50/day
			wantOnTrack: false,
			wantWindow:  14,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := buildCreditsForecast(tc.balances, tc.logRows)
			if got.OnTrack != tc.wantOnTrack {
				t.Errorf("OnTrack = %v, want %v (forecast %+v)", got.OnTrack, tc.wantOnTrack, got)
			}
			if got.WindowDaysAnalyzed != tc.wantWindow {
				t.Errorf("WindowDaysAnalyzed = %d, want %d", got.WindowDaysAnalyzed, tc.wantWindow)
			}
		})
	}
}

func TestDaysUntilMonthEnd(t *testing.T) {
	cases := []struct {
		name string
		when time.Time
		min  int
		max  int
	}{
		{"first day of January", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), 30, 31},
		{"mid March", time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC), 15, 17},
		{"last day of leap February", time.Date(2024, 2, 29, 23, 0, 0, 0, time.UTC), 1, 1},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := daysUntilMonthEnd(tc.when)
			if got < tc.min || got > tc.max {
				t.Fatalf("daysUntilMonthEnd(%s) = %d, want %d..%d", tc.when, got, tc.min, tc.max)
			}
		})
	}
}

func TestNewCreditsCmdHelp(t *testing.T) {
	flags := &rootFlags{}
	cmd := newCreditsCmd(flags)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("credits --help failed: %v", err)
	}
}

func TestNewCreditsForecastDryRun(t *testing.T) {
	// --dry-run is a persistent root flag the leaf only sees through the
	// rootCmd wiring. Flip it directly on rootFlags so the leaf's RunE
	// reaches the dryRunOK short-circuit and exits cleanly.
	flags := &rootFlags{dryRun: true}
	cmd := newCreditsForecastCmd(flags)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil && !strings.Contains(err.Error(), "subcommand") {
		t.Fatalf("credits forecast dry-run failed: %v", err)
	}
}
