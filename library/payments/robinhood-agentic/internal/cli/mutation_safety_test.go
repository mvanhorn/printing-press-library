// Copyright 2026 Kevin Magnan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestOrderNotional(t *testing.T) {
	cases := []struct {
		name string
		tool string
		args map[string]any
		want float64
	}{
		{"equity limit qty×price", "place_equity_order", map[string]any{"quantity": "2", "limit_price": "180"}, 360},
		{"equity dollar_amount wins", "place_equity_order", map[string]any{"quantity": "2", "limit_price": "180", "dollar_amount": "500"}, 500},
		{"equity stop price fallback", "place_equity_order", map[string]any{"quantity": "1", "stop_price": "50"}, 50},
		{"equity market qty only -> 0", "place_equity_order", map[string]any{"quantity": "3"}, 0},
		{"option qty×price×100", "place_option_order", map[string]any{"quantity": "2", "price": "1.50"}, 300},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := orderNotional(c.tool, c.args); got != c.want {
				t.Errorf("orderNotional(%s) = %v, want %v", c.tool, got, c.want)
			}
		})
	}
}

func TestMutationAction(t *testing.T) {
	cases := map[string]string{
		"place_equity_order":  "place",
		"cancel_option_order": "cancel",
		"add_to_watchlist":    "watchlist",
		"create_scan":         "scan",
	}
	for tool, want := range cases {
		if got := mutationAction(tool); got != want {
			t.Errorf("mutationAction(%s) = %q, want %q", tool, got, want)
		}
	}
}

func TestOrderSymbolResolvesOptionLeg(t *testing.T) {
	if got := orderSymbol(map[string]any{"symbol": "AAPL"}); got != "AAPL" {
		t.Errorf("equity symbol = %q", got)
	}
	legArgs := map[string]any{"legs": []any{map[string]any{"option_id": "opt-9"}}}
	if got := orderSymbol(legArgs); got != "option:opt-9" {
		t.Errorf("option leg symbol = %q, want option:opt-9", got)
	}
}
