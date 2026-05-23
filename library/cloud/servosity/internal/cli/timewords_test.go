// Copyright 2026 dstevens. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestParseHumanTime(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	now := time.Date(2026, 5, 11, 14, 30, 0, 0, loc) // Mon 2026-05-11 14:30 EDT

	cases := []struct {
		in   string
		want time.Time
	}{
		{"now", now},
		{"today", time.Date(2026, 5, 11, 0, 0, 0, 0, loc)},
		{"yesterday", time.Date(2026, 5, 10, 0, 0, 0, 0, loc)},
		{"tomorrow", time.Date(2026, 5, 12, 0, 0, 0, 0, loc)},
		{"6am", time.Date(2026, 5, 11, 6, 0, 0, 0, loc)},
		{"6am tomorrow", time.Date(2026, 5, 12, 6, 0, 0, 0, loc)},
		{"11pm yesterday", time.Date(2026, 5, 10, 23, 0, 0, 0, loc)},
		{"06:00", time.Date(2026, 5, 11, 6, 0, 0, 0, loc)},
		{"23:30", time.Date(2026, 5, 11, 23, 30, 0, 0, loc)},
		{"30m", now.Add(30 * time.Minute)},
		{"2h", now.Add(2 * time.Hour)},
		{"3d", now.Add(72 * time.Hour)},
		{"1w", now.Add(7 * 24 * time.Hour)},
		{"-2h", now.Add(-2 * time.Hour)},
		{"+30m", now.Add(30 * time.Minute)},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := parseHumanTime(c.in, now)
			if err != nil {
				t.Fatalf("parseHumanTime(%q): %v", c.in, err)
			}
			if !got.Equal(c.want) {
				t.Errorf("parseHumanTime(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

func TestParseHumanTimeErrors(t *testing.T) {
	now := time.Now()
	bad := []string{"", "next tuesday", "the day after tomorrow", "27pm", "15am"}
	for _, b := range bad {
		t.Run(b, func(t *testing.T) {
			_, err := parseHumanTime(b, now)
			if err == nil {
				t.Errorf("parseHumanTime(%q) should have errored", b)
			}
		})
	}
}

func TestParseExtendedDurationKnownUnits(t *testing.T) {
	cases := map[string]time.Duration{
		"30s": 30 * time.Second,
		"30m": 30 * time.Minute,
		"2h":  2 * time.Hour,
		"3d":  3 * 24 * time.Hour,
		"1w":  7 * 24 * time.Hour,
	}
	for in, want := range cases {
		got, ok := parseExtendedDuration(in)
		if !ok {
			t.Errorf("parseExtendedDuration(%q): not ok", in)
			continue
		}
		if got != want {
			t.Errorf("parseExtendedDuration(%q) = %v, want %v", in, got, want)
		}
	}
}
