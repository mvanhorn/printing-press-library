// Copyright 2026 alex-kleis. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
)

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

func TestValidateSinceFlag(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		wantOut string
		wantErr bool
	}{
		{"empty is allowed", "", "", false},
		{"YYYY-MM-DD valid", "2025-01-01", "2025-01-01", false},
		{"YYYY-MM-DD with whitespace", "  2025-01-01  ", "2025-01-01", false},
		{"slash form rejected (regression)", "01/01/2025", "", true},
		{"M/D/YYYY rejected", "1/1/2025", "", true},
		{"YYYY/MM/DD rejected", "2025/01/01", "", true},
		{"non-date garbage rejected", "yesterday", "", true},
		{"missing day rejected", "2025-01", "", true},
		{"month out of range rejected", "2025-13-01", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateSinceFlag("since", tc.in)
			if tc.wantErr {
				if err == nil {
					t.Errorf("validateSinceFlag(%q) returned no error; want one", tc.in)
				}
				if got != "" {
					t.Errorf("validateSinceFlag(%q) returned %q on error path; want empty", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Errorf("validateSinceFlag(%q) returned unexpected error: %v", tc.in, err)
			}
			if got != tc.wantOut {
				t.Errorf("validateSinceFlag(%q) = %q, want %q", tc.in, got, tc.wantOut)
			}
			if err == nil && tc.in != "" && !strings.Contains(got, "-") {
				t.Errorf("validateSinceFlag(%q) returned %q which doesn't look like YYYY-MM-DD", tc.in, got)
			}
		})
	}
}
