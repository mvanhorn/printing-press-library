// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for `portfolio winrate`: wiring smoke test plus table-driven
// coverage of the pure computeWinrate aggregation. Hermetic: no network,
// no store.

package cli

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

// TestNovelPortfolioWinrateHelpWires smoke-tests that the portfolio winrate command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review.
func TestNovelPortfolioWinrateHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"portfolio", "winrate", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("portfolio winrate --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "winrate", "--account", "--span", "--by-symbol"} {
		if !strings.Contains(help, want) {
			t.Fatalf("portfolio winrate --help missing %q in output:\n%s", want, help)
		}
	}
}

func winrateAlmostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func TestComputeWinrate(t *testing.T) {
	tests := []struct {
		name        string
		trades      []winTrade
		wantOverall winrateBucket
		wantSymbols map[string]winrateBucket
	}{
		{
			name:        "empty input yields zero stats and empty by-symbol map",
			trades:      nil,
			wantOverall: winrateBucket{},
			wantSymbols: map[string]winrateBucket{},
		},
		{
			name: "all wins",
			trades: []winTrade{
				{Symbol: "AAPL", RealizedPnl: 100},
				{Symbol: "AAPL", RealizedPnl: 50},
			},
			wantOverall: winrateBucket{
				TotalTrades: 2, Wins: 2, Losses: 0,
				WinRate: 1.0, AvgWin: 75, AvgLoss: 0, TotalRealized: 150,
			},
			wantSymbols: map[string]winrateBucket{
				"AAPL": {TotalTrades: 2, Wins: 2, WinRate: 1.0, AvgWin: 75, TotalRealized: 150},
			},
		},
		{
			name: "mixed wins and losses across symbols",
			trades: []winTrade{
				{Symbol: "AAPL", RealizedPnl: 100},
				{Symbol: "AAPL", RealizedPnl: -40},
				{Symbol: "MSFT", RealizedPnl: -60},
				{Symbol: "MSFT", RealizedPnl: 20},
				{Symbol: "MSFT", RealizedPnl: 40},
				{Symbol: "TSLA", RealizedPnl: -10},
			},
			wantOverall: winrateBucket{
				TotalTrades: 6, Wins: 3, Losses: 3,
				WinRate:       0.5,
				AvgWin:        (100.0 + 20 + 40) / 3,
				AvgLoss:       (-40.0 - 60 - 10) / 3,
				TotalRealized: 50,
			},
			wantSymbols: map[string]winrateBucket{
				"AAPL": {TotalTrades: 2, Wins: 1, Losses: 1, WinRate: 0.5, AvgWin: 100, AvgLoss: -40, TotalRealized: 60},
				"MSFT": {TotalTrades: 3, Wins: 2, Losses: 1, WinRate: 2.0 / 3.0, AvgWin: 30, AvgLoss: -60, TotalRealized: 0},
				"TSLA": {TotalTrades: 1, Wins: 0, Losses: 1, WinRate: 0, AvgWin: 0, AvgLoss: -10, TotalRealized: -10},
			},
		},
		{
			name: "zero pnl counts as a round trip but is skipped from wins losses and win rate",
			trades: []winTrade{
				{Symbol: "NVDA", RealizedPnl: 30},
				{Symbol: "NVDA", RealizedPnl: 0},
				{Symbol: "NVDA", RealizedPnl: -10},
			},
			wantOverall: winrateBucket{
				TotalTrades: 3, Wins: 1, Losses: 1,
				WinRate: 0.5, AvgWin: 30, AvgLoss: -10, TotalRealized: 20,
			},
			wantSymbols: map[string]winrateBucket{
				"NVDA": {TotalTrades: 3, Wins: 1, Losses: 1, WinRate: 0.5, AvgWin: 30, AvgLoss: -10, TotalRealized: 20},
			},
		},
		{
			name: "only breakevens leaves win rate at zero not NaN",
			trades: []winTrade{
				{Symbol: "AMD", RealizedPnl: 0},
				{Symbol: "AMD", RealizedPnl: 0},
			},
			wantOverall: winrateBucket{TotalTrades: 2},
			wantSymbols: map[string]winrateBucket{
				"AMD": {TotalTrades: 2},
			},
		},
	}

	checkBucket := func(t *testing.T, label string, got, want winrateBucket) {
		t.Helper()
		if got.TotalTrades != want.TotalTrades || got.Wins != want.Wins || got.Losses != want.Losses {
			t.Errorf("%s counts = trades %d / wins %d / losses %d, want %d / %d / %d",
				label, got.TotalTrades, got.Wins, got.Losses, want.TotalTrades, want.Wins, want.Losses)
		}
		if !winrateAlmostEqual(got.WinRate, want.WinRate) {
			t.Errorf("%s WinRate = %v, want %v", label, got.WinRate, want.WinRate)
		}
		if !winrateAlmostEqual(got.AvgWin, want.AvgWin) {
			t.Errorf("%s AvgWin = %v, want %v", label, got.AvgWin, want.AvgWin)
		}
		if !winrateAlmostEqual(got.AvgLoss, want.AvgLoss) {
			t.Errorf("%s AvgLoss = %v, want %v", label, got.AvgLoss, want.AvgLoss)
		}
		if !winrateAlmostEqual(got.TotalRealized, want.TotalRealized) {
			t.Errorf("%s TotalRealized = %v, want %v", label, got.TotalRealized, want.TotalRealized)
		}
		if math.IsNaN(got.WinRate) || math.IsNaN(got.AvgWin) || math.IsNaN(got.AvgLoss) {
			t.Errorf("%s produced NaN: %+v", label, got)
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeWinrate(tt.trades)
			checkBucket(t, "overall", got.Overall, tt.wantOverall)
			if len(got.BySymbol) != len(tt.wantSymbols) {
				t.Fatalf("BySymbol has %d symbols (%v), want %d", len(got.BySymbol), got.BySymbol, len(tt.wantSymbols))
			}
			for sym, want := range tt.wantSymbols {
				gotSym, ok := got.BySymbol[sym]
				if !ok {
					t.Fatalf("BySymbol missing %q", sym)
				}
				checkBucket(t, "symbol "+sym, gotSym, want)
			}
		})
	}
}
