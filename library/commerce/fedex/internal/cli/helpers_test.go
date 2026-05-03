// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

// parseWeightArg accepts bare numbers and unit-suffixed values; the
// suffix wins over the caller's --units flag when present.
func TestParseWeightArg(t *testing.T) {
	cases := []struct {
		raw       string
		defUnits  string
		wantValue float64
		wantUnits string
		wantErr   bool
	}{
		{"", "LB", 0, "", false},
		{"5", "LB", 5, "", false},
		{"5lb", "KG", 5, "LB", false},
		{"5 lb", "KG", 5, "LB", false},
		{"2.5kg", "LB", 2.5, "KG", false},
		{"2.5KG", "LB", 2.5, "KG", false},
		{"10pounds", "KG", 10, "LB", false},
		{"oops", "LB", 0, "", true},
		{"5stone", "LB", 0, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			val, units, err := parseWeightArg(tc.raw, tc.defUnits)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got value=%v units=%q", tc.raw, val, units)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.raw, err)
			}
			if val != tc.wantValue {
				t.Errorf("value: want %v got %v", tc.wantValue, val)
			}
			if units != tc.wantUnits {
				t.Errorf("units: want %q got %q", tc.wantUnits, units)
			}
		})
	}
}

// ftsQuoteQuery wraps every term in FTS5 double quotes so quoted
// phrases, ASCII apostrophes, and parentheses don't trip the FTS5
// tokenizer. Empty input round-trips to empty.
func TestFTSQuoteQuery(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"   ", ""},
		{"acme", `"acme"`},
		{"acme corp", `"acme" AND "corp"`},
		{`'warehouse 47'`, `"warehouse 47"`},
		{`"acme corp"`, `"acme corp"`},
		{`he said "hi"`, `"he" AND "said" AND "hi"`},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := ftsQuoteQuery(tc.in)
			if got != tc.want {
				t.Errorf("ftsQuoteQuery(%q): want %q got %q", tc.in, tc.want, got)
			}
		})
	}
}
