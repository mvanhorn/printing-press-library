// Copyright 2026 Michael Schreiber and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"
)

func TestComputeStreak(t *testing.T) {
	t.Parallel()

	// Friday 2026-07-03 is the reference "today".
	through := time.Date(2026, 7, 3, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		dates []string
		want  int
	}{
		{"no dates", nil, 0},
		{"single day match through today", []string{"2026-07-03"}, 1},
		{
			"five consecutive business days including a weekend gap",
			[]string{"2026-07-03", "2026-07-02", "2026-07-01", "2026-06-30", "2026-06-29"},
			5,
		},
		{
			"streak broken by a missing weekday",
			[]string{"2026-07-03", "2026-07-01"},
			1,
		},
		{
			"most recent match is not today",
			[]string{"2026-07-01", "2026-06-30"},
			2,
		},
		{
			// 2026-06-27 is a Saturday. Today (Friday 2026-07-03) has no
			// record, so the fallback path starts the walk at the most
			// recent matched date, which itself lands on a weekend. That
			// matched day must still be counted, not skipped by the
			// weekend-bridging logic.
			"most recent match falls on a Saturday",
			[]string{"2026-06-27"},
			1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var dates []time.Time
			for _, s := range tc.dates {
				d, err := time.Parse("2006-01-02", s)
				if err != nil {
					t.Fatalf("parsing fixture date %q: %v", s, err)
				}
				dates = append(dates, d)
			}
			got := computeStreak(dates, through)
			if got != tc.want {
				t.Fatalf("computeStreak(%v, %s) = %d, want %d", tc.dates, through.Format("2006-01-02"), got, tc.want)
			}
		})
	}
}

func TestFindRecordDate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		rec     map[string]any
		wantOK  bool
		wantVal string
	}{
		{"no date field", map[string]any{"symbolCode": "GME"}, false, ""},
		{"tradeReportDate field", map[string]any{"tradeReportDate": "2026-06-30"}, true, "2026-06-30"},
		{"non-string date value", map[string]any{"tradeReportDate": 20260630}, false, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := findRecordDate(tc.rec)
			if ok != tc.wantOK {
				t.Fatalf("findRecordDate(%v) ok = %v, want %v", tc.rec, ok, tc.wantOK)
			}
			if ok && got.Format("2006-01-02") != tc.wantVal {
				t.Fatalf("findRecordDate(%v) = %s, want %s", tc.rec, got.Format("2006-01-02"), tc.wantVal)
			}
		})
	}
}
