// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Tests for portfolio history: wiring smoke test plus table-driven coverage of
// the pure aggregation helpers (no network, no store).

package cli

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/robinhood-agentic/internal/store"
)

// TestNovelPortfolioHistoryHelpWires smoke-tests that the portfolio history command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelPortfolioHistoryHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"portfolio", "history", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("portfolio history --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "history", "--account", "--since", "--sparkline"} {
		if !strings.Contains(help, want) {
			t.Fatalf("portfolio history --help missing %q in output:\n%s", want, help)
		}
	}
}

func histSnap(day int, total string) store.PortfolioSnapshot {
	return store.PortfolioSnapshot{
		AccountNumber: "ACC1",
		CapturedAt:    time.Date(2026, 7, day, 12, 0, 0, 0, time.UTC),
		TotalValue:    total,
	}
}

func TestSummarizePortfolioHistory(t *testing.T) {
	tests := []struct {
		name                                     string
		snaps                                    []store.PortfolioSnapshot
		wantFirst, wantLast, wantChange, wantPct float64
	}{
		{
			name: "empty series",
		},
		{
			name:      "single point has no change",
			snaps:     []store.PortfolioSnapshot{histSnap(1, "1000.00")},
			wantFirst: 1000, wantLast: 1000,
		},
		{
			name: "gain",
			snaps: []store.PortfolioSnapshot{
				histSnap(1, "1000.00"), histSnap(2, "1050.00"), histSnap(3, "1100.00"),
			},
			wantFirst: 1000, wantLast: 1100, wantChange: 100, wantPct: 10,
		},
		{
			name: "loss",
			snaps: []store.PortfolioSnapshot{
				histSnap(1, "200.00"), histSnap(2, "150.00"),
			},
			wantFirst: 200, wantLast: 150, wantChange: -50, wantPct: -25,
		},
		{
			name: "zero first value guards divide-by-zero pct",
			snaps: []store.PortfolioSnapshot{
				histSnap(1, "0"), histSnap(2, "500.00"),
			},
			wantFirst: 0, wantLast: 500, wantChange: 500, wantPct: 0,
		},
		{
			name: "malformed money parses as zero",
			snaps: []store.PortfolioSnapshot{
				histSnap(1, "not-a-number"), histSnap(2, "100"),
			},
			wantFirst: 0, wantLast: 100, wantChange: 100, wantPct: 0,
		},
	}
	const eps = 1e-9
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			first, last, change, pct := summarizePortfolioHistory(tt.snaps)
			for _, chk := range []struct {
				label     string
				got, want float64
			}{
				{"first", first, tt.wantFirst},
				{"last", last, tt.wantLast},
				{"change", change, tt.wantChange},
				{"changePct", pct, tt.wantPct},
			} {
				if math.Abs(chk.got-chk.want) > eps {
					t.Errorf("summarizePortfolioHistory() %s = %v, want %v", chk.label, chk.got, chk.want)
				}
			}
		})
	}
}

func TestSnapshotTotals(t *testing.T) {
	tests := []struct {
		name  string
		snaps []store.PortfolioSnapshot
		want  []float64
	}{
		{name: "empty", snaps: nil, want: []float64{}},
		{
			name:  "ordered series",
			snaps: []store.PortfolioSnapshot{histSnap(1, "1.50"), histSnap(2, "2.25"), histSnap(3, "3.00")},
			want:  []float64{1.5, 2.25, 3},
		},
		{
			name:  "malformed and empty values become zero",
			snaps: []store.PortfolioSnapshot{histSnap(1, ""), histSnap(2, "abc"), histSnap(3, "10")},
			want:  []float64{0, 0, 10},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := snapshotTotals(tt.snaps)
			if len(got) != len(tt.want) {
				t.Fatalf("snapshotTotals() len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if math.Abs(got[i]-tt.want[i]) > 1e-9 {
					t.Errorf("snapshotTotals()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestSnapshotAccountsDistinct pins the multi-account refusal input: history
// must detect when a series spans accounts so it never summarizes them as one.
func TestSnapshotAccountsDistinct(t *testing.T) {
	snaps := []store.PortfolioSnapshot{
		{AccountNumber: "B2"}, {AccountNumber: "A1"}, {AccountNumber: "B2"},
	}
	got := snapshotAccounts(snaps)
	if len(got) != 2 || got[0] != "A1" || got[1] != "B2" {
		t.Errorf("snapshotAccounts = %v, want [A1 B2]", got)
	}
}
