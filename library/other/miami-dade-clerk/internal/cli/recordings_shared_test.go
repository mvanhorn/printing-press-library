// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNormalizeFolio(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want int64
	}{
		{"dashed canonical", "30-2232-066-1610", 3022320661610},
		{"clean numeric", "3022320661610", 3022320661610},
		{"leading/trailing whitespace", "  30-2232-066-1610  ", 3022320661610},
		{"12-digit clerk shape", "302232066161", 302232066161},
		{"empty", "", 0},
		{"all dashes", "----", 0},
		{"embedded letter (the bug case)", "30-2232-066A-1610", 0},
		{"alpha suffix", "3022320661610A", 0},
		{"alpha prefix", "A3022320661610", 0},
		{"interior space", "30-2232 066-1610", 0},
		{"non-digit only", "ABCDEFG", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeFolio(tc.in)
			if got != tc.want {
				t.Errorf("NormalizeFolio(%q) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}
