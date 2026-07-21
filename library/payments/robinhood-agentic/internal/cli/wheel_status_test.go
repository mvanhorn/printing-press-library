// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelWheelStatusHelpWires smoke-tests that the wheel status command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelWheelStatusHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"wheel", "status", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("wheel status --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "status"} {
		if !strings.Contains(help, want) {
			t.Fatalf("wheel status --help missing %q in output:\n%s", want, help)
		}
	}
}

// TestInferWheelStage covers every branch of the pure stage classifier.
func TestInferWheelStage(t *testing.T) {
	tests := []struct {
		name       string
		hasEquity  bool
		equityQty  float64
		shortPuts  int
		shortCalls int
		want       string
	}{
		{"shares plus short call is covered_call", true, 100, 0, 1, "covered_call"},
		{"shares plus short call and short put still covered_call", true, 200, 1, 2, "covered_call"},
		{"shares without short call is assigned_holding", true, 100, 0, 0, "assigned_holding"},
		{"shares with short put but no short call is assigned_holding", true, 100, 1, 0, "assigned_holding"},
		{"fractional shares count as holding", true, 0.5, 0, 0, "assigned_holding"},
		{"short put and no equity record is cash_secured_put", false, 0, 1, 0, "cash_secured_put"},
		{"short put with zero-quantity equity record is cash_secured_put", true, 0, 2, 0, "cash_secured_put"},
		{"short call with no shares is called_away_or_idle", false, 0, 0, 1, "called_away_or_idle"},
		{"zero-share equity record with no shorts is called_away_or_idle", true, 0, 0, 0, "called_away_or_idle"},
		{"negative equity quantity with no shorts is called_away_or_idle", true, -5, 0, 0, "called_away_or_idle"},
		{"no equity record and no shorts is long_option", false, 0, 0, 0, "long_option"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inferWheelStage(tt.hasEquity, tt.equityQty, tt.shortPuts, tt.shortCalls)
			if got != tt.want {
				t.Fatalf("inferWheelStage(%v, %v, %d, %d) = %q, want %q",
					tt.hasEquity, tt.equityQty, tt.shortPuts, tt.shortCalls, got, tt.want)
			}
		})
	}
}

// TestBuildWheelRows covers the pure join: symbol aggregation, unknown-leg
// resolution, earliest-expiration selection, symbol filtering, and sorting.
func TestBuildWheelRows(t *testing.T) {
	equity := []wheelEquityPosition{
		{Symbol: "msft", Quantity: 100},
		{Symbol: "TSLA", Quantity: 0},
	}
	options := []wheelOptionPosition{
		{Symbol: "MSFT", Direction: "short", OptionType: "call", Contracts: 1, Expiration: "2026-09-18"},
		{Symbol: "MSFT", Direction: "short", OptionType: "call", Contracts: 2, Expiration: "2026-08-21"},
		{Symbol: "AAPL", Direction: "short", OptionType: "put", Contracts: 1, Expiration: "2026-08-07"},
		{Symbol: "NVDA", Direction: "long", OptionType: "call", Contracts: 3, Expiration: "2027-01-15"},
		{Symbol: "AMD", Direction: "short", OptionType: "", Contracts: 2, Expiration: "2026-08-14"},
	}
	openOrders := map[string]int{"MSFT": 1}

	rows := buildWheelRows(equity, options, openOrders, "")
	wantOrder := []string{"AAPL", "AMD", "MSFT", "NVDA", "TSLA"}
	if len(rows) != len(wantOrder) {
		t.Fatalf("buildWheelRows returned %d rows, want %d: %+v", len(rows), len(wantOrder), rows)
	}
	bySymbol := map[string]wheelRow{}
	for i, r := range rows {
		if r.Symbol != wantOrder[i] {
			t.Fatalf("row %d symbol = %q, want %q (rows must sort by symbol)", i, r.Symbol, wantOrder[i])
		}
		bySymbol[r.Symbol] = r
	}

	msft := bySymbol["MSFT"]
	if msft.Stage != "covered_call" || msft.Shares != 100 || msft.ShortCalls != 3 || msft.ShortPuts != 0 {
		t.Fatalf("MSFT row = %+v, want covered_call with 100 shares and 3 short calls", msft)
	}
	if msft.NextExpiration != "2026-08-21" {
		t.Fatalf("MSFT next expiration = %q, want earliest %q", msft.NextExpiration, "2026-08-21")
	}
	if msft.OpenOptionOrders != 1 {
		t.Fatalf("MSFT open orders = %d, want 1", msft.OpenOptionOrders)
	}

	if aapl := bySymbol["AAPL"]; aapl.Stage != "cash_secured_put" || aapl.ShortPuts != 1 {
		t.Fatalf("AAPL row = %+v, want cash_secured_put with 1 short put", aapl)
	}
	// Unknown call/put on a short with no shares resolves to puts.
	if amd := bySymbol["AMD"]; amd.Stage != "cash_secured_put" || amd.ShortPuts != 2 || amd.ShortCalls != 0 {
		t.Fatalf("AMD row = %+v, want unknown shorts counted as 2 puts", amd)
	}
	if nvda := bySymbol["NVDA"]; nvda.Stage != "long_option" || nvda.ShortPuts != 0 || nvda.ShortCalls != 0 {
		t.Fatalf("NVDA row = %+v, want long_option with no shorts", nvda)
	}
	if tsla := bySymbol["TSLA"]; tsla.Stage != "called_away_or_idle" || tsla.Shares != 0 {
		t.Fatalf("TSLA row = %+v, want called_away_or_idle with 0 shares", tsla)
	}

	// Unknown short legs against held shares resolve to calls instead.
	held := buildWheelRows(
		[]wheelEquityPosition{{Symbol: "AMD", Quantity: 200}},
		[]wheelOptionPosition{{Symbol: "AMD", Direction: "short", OptionType: "", Contracts: 2}},
		nil, "")
	if len(held) != 1 || held[0].Stage != "covered_call" || held[0].ShortCalls != 2 || held[0].ShortPuts != 0 {
		t.Fatalf("held-shares unknown-leg rows = %+v, want covered_call with 2 short calls", held)
	}

	// A symbol filter restricts the join, case-insensitively.
	filtered := buildWheelRows(equity, options, openOrders, "msft")
	if len(filtered) != 1 || filtered[0].Symbol != "MSFT" {
		t.Fatalf("filtered rows = %+v, want only MSFT", filtered)
	}
}
