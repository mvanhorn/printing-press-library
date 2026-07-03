// Copyright 2026 jbriaux and contributors. Licensed under Apache-2.0. See LICENSE.
// Unit tests for pure TradingView market helpers.

package cli

import "testing"

func TestStripEm(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"<em>AAPL</em>", "AAPL"},
		{"Apple Inc.", "Apple Inc."},
		{"  <em>BTC</em>USD  ", "BTCUSD"},
		{"Tom &amp; Jerry", "Tom & Jerry"},
		{"", ""},
	}
	for _, c := range cases {
		if got := stripEm(c.in); got != c.want {
			t.Errorf("stripEm(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsUSDLike(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"USD", true},
		{"usd", true},
		{"USDT", true},
		{"USDC", true},
		{"EUR", false},
		{"ARS", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isUSDLike(c.in); got != c.want {
			t.Errorf("isUSDLike(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestFmtNum(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{308.63, "308.63"},
		{1.14365, "1.14365"},
		{100, "100"},
	}
	for _, c := range cases {
		if got := fmtNum(c.in); got != c.want {
			t.Errorf("fmtNum(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}
