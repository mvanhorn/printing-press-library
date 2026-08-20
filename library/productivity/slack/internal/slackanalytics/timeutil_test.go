// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package slackanalytics

import (
	"testing"
	"time"
)

func TestParseSlackTS(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		ts     string
		wantOK bool
		want   time.Time
	}{
		{"microseconds", "1712345678.000200", true, time.Unix(1712345678, 200000).UTC()},
		{"bare seconds", "1712345678", true, time.Unix(1712345678, 0).UTC()},
		{"trailing dot", "1712345678.", true, time.Unix(1712345678, 0).UTC()},
		{"whitespace", "  1712345678.500000 ", true, time.Unix(1712345678, 500000000).UTC()},
		{"empty", "", false, time.Time{}},
		{"garbage", "not-a-ts", false, time.Time{}},
		{"zero", "0", false, time.Time{}},
		{"negative", "-5.000000", false, time.Time{}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := ParseSlackTS(tc.ts)
			if ok != tc.wantOK {
				t.Fatalf("ParseSlackTS(%q) ok = %v, want %v", tc.ts, ok, tc.wantOK)
			}
			if ok && !got.Equal(tc.want) {
				t.Fatalf("ParseSlackTS(%q) = %v, want %v", tc.ts, got, tc.want)
			}
		})
	}
}

func TestFormatSlackTSRoundTrips(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"1712345678.000200", "1600000000.123456", "1500000000.000000"} {
		parsed, ok := ParseSlackTS(raw)
		if !ok {
			t.Fatalf("ParseSlackTS(%q) failed", raw)
		}
		if got := FormatSlackTS(parsed); got != raw {
			t.Fatalf("FormatSlackTS(ParseSlackTS(%q)) = %q, want %q", raw, got, raw)
		}
	}
}

func TestBeyondRetention(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		msg  time.Time
		wall time.Duration
		want bool
	}{
		{"recent", now.Add(-24 * time.Hour), RetentionWall, false},
		{"just inside wall", now.Add(-89 * 24 * time.Hour), RetentionWall, false},
		{"just outside wall", now.Add(-91 * 24 * time.Hour), RetentionWall, true},
		{"ancient", now.Add(-400 * 24 * time.Hour), RetentionWall, true},
		{"zero time", time.Time{}, RetentionWall, false},
		{"disabled wall", now.Add(-400 * 24 * time.Hour), 0, false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := BeyondRetention(tc.msg, now, tc.wall); got != tc.want {
				t.Fatalf("BeyondRetention(%v, %v, %v) = %v, want %v", tc.msg, now, tc.wall, got, tc.want)
			}
		})
	}
}

func TestAgeDaysAndRoundDays(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	ageCases := []struct {
		name string
		t    time.Time
		want int
	}{
		{"same instant", now, 0},
		{"twelve hours", now.Add(-12 * time.Hour), 0},
		{"one day", now.Add(-24 * time.Hour), 1},
		{"ten and a half days", now.Add(-252 * time.Hour), 10},
		{"future", now.Add(48 * time.Hour), 0},
		{"zero value", time.Time{}, 0},
	}
	for _, tc := range ageCases {
		tc := tc
		t.Run("age/"+tc.name, func(t *testing.T) {
			t.Parallel()
			if got := AgeDays(tc.t, now); got != tc.want {
				t.Fatalf("AgeDays(%v) = %d, want %d", tc.t, got, tc.want)
			}
		})
	}

	roundCases := []struct {
		name string
		d    time.Duration
		want float64
	}{
		{"zero", 0, 0},
		{"negative", -time.Hour, 0},
		{"one day", 24 * time.Hour, 1},
		{"half day", 12 * time.Hour, 0.5},
		{"awkward", 30 * time.Hour, 1.25},
	}
	for _, tc := range roundCases {
		tc := tc
		t.Run("round/"+tc.name, func(t *testing.T) {
			t.Parallel()
			if got := RoundDays(tc.d); got != tc.want {
				t.Fatalf("RoundDays(%v) = %v, want %v", tc.d, got, tc.want)
			}
		})
	}
}
