// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestFormatMoneyMinor(t *testing.T) {
	cases := []struct {
		name     string
		amount   float64
		currency string
		want     string
	}{
		{"mxn basic", 150000, "MXN", "MXN 1500.00"},
		{"zero", 0, "MXN", "MXN 0.00"},
		{"cents", 150005, "MXN", "MXN 1500.05"},
		{"usd", 999, "USD", "USD 9.99"},
		{"defaults to mxn", 100, "", "MXN 1.00"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatMoneyMinor(tc.amount, tc.currency); got != tc.want {
				t.Fatalf("formatMoneyMinor(%v,%q) = %q, want %q", tc.amount, tc.currency, got, tc.want)
			}
		})
	}
}

func TestNormalizeCurrency(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", "MXN"},
		{"MXN", "MXN"},
		{"USD", "USD"},
	}
	for _, tc := range cases {
		if got := normalizeCurrency(tc.in); got != tc.want {
			t.Fatalf("normalizeCurrency(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	// The unspecified and explicit forms must be the same map key, otherwise
	// aggregations split into two buckets that render under the same label.
	if normalizeCurrency("") != normalizeCurrency("MXN") {
		t.Fatal("unspecified and explicit MXN must normalize to the same key")
	}
}

func TestAbsVal(t *testing.T) {
	if got := absVal(-123.5); got != 123.5 {
		t.Fatalf("absVal(-123.5) = %v, want 123.5", got)
	}
	if got := absVal(42); got != 42 {
		t.Fatalf("absVal(42) = %v, want 42", got)
	}
}
