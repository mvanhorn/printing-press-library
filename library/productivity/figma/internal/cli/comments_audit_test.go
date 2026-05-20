// Copyright 2026 giuliano-giacaglia. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestParseAge(t *testing.T) {
	now := time.Now().UTC()
	cases := []struct {
		name     string
		in       string
		wantErr  bool
		wantNear time.Duration // expected delta from now (negative means in past)
	}{
		{"7 days", "7d", false, -7 * 24 * time.Hour},
		{"2 weeks", "2w", false, -14 * 24 * time.Hour},
		{"30 days", "30d", false, -30 * 24 * time.Hour},
		{"hours", "12h", false, -12 * time.Hour},
		{"minutes", "5m", false, -5 * time.Minute},
		{"empty", "", true, 0},
		{"bad suffix", "5x", true, 0},
		{"non numeric", "abcd", true, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseAge(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			diff := got.Sub(now)
			delta := diff - tc.wantNear
			if delta < 0 {
				delta = -delta
			}
			if delta > 5*time.Second {
				t.Errorf("parseAge(%q) drifted too far: got %v, want near %v (diff %v)", tc.in, got, now.Add(tc.wantNear), delta)
			}
		})
	}
}

func TestParseAge_RFC3339(t *testing.T) {
	got, err := parseAge("2026-04-01T00:00:00Z")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Year() != 2026 || got.Month() != 4 || got.Day() != 1 {
		t.Errorf("parseAge RFC3339 returned %v", got)
	}
}
