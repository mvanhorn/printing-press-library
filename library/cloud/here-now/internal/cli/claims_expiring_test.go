// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored test for the `claims expiring` --within duration parsing.
// Plain header so regen-merge preserves this file.
package cli

import (
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/here-now/internal/cliutil"
)

func TestClaimsExpiringWithinDurationParse(t *testing.T) {
	tests := []struct {
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"6h", 6 * time.Hour, false},
		{"2h", 2 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"90m", 90 * time.Minute, false},
		{"garbage", 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := cliutil.ParseDurationLoose(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseDurationLoose(%q): expected error", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseDurationLoose(%q): %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseDurationLoose(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}
