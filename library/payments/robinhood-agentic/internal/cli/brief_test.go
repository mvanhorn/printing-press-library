// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"strings"
	"testing"
)

// TestNovelBriefHelpWires smoke-tests that the brief command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelBriefHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"brief", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("brief --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "brief"} {
		if !strings.Contains(help, want) {
			t.Fatalf("brief --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestPickDefaultAccount(t *testing.T) {
	cases := []struct {
		name string
		in   []accountRef
		want string
	}{
		{"empty", nil, ""},
		{"first when none default", []accountRef{{AccountNumber: "A"}, {AccountNumber: "B"}}, "A"},
		{"flagged default wins", []accountRef{{AccountNumber: "A"}, {AccountNumber: "B", IsDefault: true}}, "B"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := pickDefaultAccount(c.in); got != c.want {
				t.Errorf("pickDefaultAccount() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestComputeDelta(t *testing.T) {
	d := computeDelta(1000, 1100)
	if d["change"] != 100 || d["change_pct"] != 10 {
		t.Errorf("computeDelta(1000,1100) = %#v", d)
	}
	d = computeDelta(0, 500) // avoid divide-by-zero
	if d["change"] != 500 || d["change_pct"] != 0 {
		t.Errorf("computeDelta(0,500) = %#v", d)
	}
}

func TestTopMovers(t *testing.T) {
	quotes := []map[string]any{
		{"symbol": "AAPL", "last_trade_price": "110", "previous_close": "100"}, // +10%
		{"symbol": "MSFT", "last_trade_price": "95", "previous_close": "100"},  // -5%
		{"symbol": "NVDA", "last_trade_price": "100", "previous_close": "100"}, // 0%
		{"symbol": "BAD", "last_trade_price": "5", "previous_close": "0"},      // skipped (prev=0)
	}
	got := topMovers(quotes, 2)
	if len(got) != 2 {
		t.Fatalf("want top 2, got %d", len(got))
	}
	if got[0].Symbol != "AAPL" || got[0].ChangePct != 10 {
		t.Errorf("top mover = %+v, want AAPL +10%%", got[0])
	}
	if got[1].Symbol != "MSFT" {
		t.Errorf("second mover = %s, want MSFT (abs 5%% > 0%%)", got[1].Symbol)
	}
}

func TestTopMoversNestedQuoteShape(t *testing.T) {
	// get_equity_quotes nests price fields under a "quote" object — the shape
	// the live API actually returns. Reading them at the top level silently
	// yielded zero movers (caught by live smoke); this locks the fix in.
	quotes := []map[string]any{
		{"quote": map[string]any{"symbol": "AMD", "last_trade_price": "220", "previous_close": "200"}}, // +10%
		{"quote": map[string]any{"symbol": "AAPL", "last_trade_price": "326.7", "previous_close": "326.59"}},
	}
	got := topMovers(quotes, 5)
	if len(got) != 2 {
		t.Fatalf("nested-quote movers: got %d, want 2", len(got))
	}
	if got[0].Symbol != "AMD" || got[0].ChangePct < 9.9 || got[0].ChangePct > 10.1 {
		t.Errorf("top nested mover = %+v, want AMD ~+10%%", got[0])
	}
}

func TestPositionSymbols(t *testing.T) {
	got := positionSymbols([]map[string]any{
		{"symbol": "AAPL"}, {"symbol": "MSFT"}, {"symbol": "AAPL"}, {"symbol": ""},
	})
	if len(got) != 2 || got[0] != "AAPL" || got[1] != "MSFT" {
		t.Errorf("positionSymbols dedup = %v, want [AAPL MSFT]", got)
	}
}

func TestIsOpenOrderState(t *testing.T) {
	open := []string{"new", "queued", "confirmed", "unconfirmed", "partially_filled"}
	closed := []string{"filled", "cancelled", "rejected", "failed", "voided"}
	for _, s := range open {
		if !isOpenOrderState(s) {
			t.Errorf("%s should be open", s)
		}
	}
	for _, s := range closed {
		if isOpenOrderState(s) {
			t.Errorf("%s should be closed", s)
		}
	}
}
