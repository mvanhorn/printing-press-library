// Copyright 2026 Chris Rodriguez and contributors. Licensed under Apache-2.0. See LICENSE.

// PATCH: tests for the digest-section helpers added by the amend
// (digestCurrencyLabel, normalizeCurrencyCode). These cover the
// single/multi/empty currency-bucket cases that distinguish the
// "USD" / "Total (mixed currency)" / "—" labels in the markdown
// renderer.

package cli

import "testing"

func TestNormalizeCurrencyCode(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "usd", want: "USD"},
		{in: "USD", want: "USD"},
		{in: " eur ", want: "EUR"},
		{in: "Gbp", want: "GBP"},
		{in: "", want: "unknown"},
		{in: "   ", want: "unknown"},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got := normalizeCurrencyCode(c.in)
			if got != c.want {
				t.Errorf("normalizeCurrencyCode(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestDigestCurrencyLabel(t *testing.T) {
	cases := []struct {
		name string
		in   map[string]int64
		want string
	}{
		{
			name: "single currency renders that code",
			in:   map[string]int64{"USD": 1500},
			want: "USD",
		},
		{
			name: "multi-currency renders mixed label",
			in:   map[string]int64{"USD": 1500, "EUR": 800},
			want: "Total (mixed currency)",
		},
		{
			name: "empty bucket renders dash",
			in:   map[string]int64{},
			want: "—",
		},
		{
			name: "nil bucket renders dash",
			in:   nil,
			want: "—",
		},
		{
			name: "single non-USD currency",
			in:   map[string]int64{"EUR": 12000},
			want: "EUR",
		},
		{
			name: "three currencies still renders mixed label",
			in:   map[string]int64{"USD": 1000, "EUR": 500, "GBP": 200},
			want: "Total (mixed currency)",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := digestCurrencyLabel(c.in)
			if got != c.want {
				t.Errorf("digestCurrencyLabel(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
