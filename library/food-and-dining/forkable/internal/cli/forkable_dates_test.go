// Copyright 2026 Allen Lew and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Tests for the shared Forkable date helpers. Hand-authored; preserved
// across generate --force, matching forkable_dates.go.

package cli

import (
	"testing"
	"time"
)

// TestDaysSinceUTC pins down the venue-rotation recency calculation across
// timezones, including the DST edge case a naive local-zone truncation
// would get wrong. now is constructed at a fixed instant observed through
// different *time.Location values rather than mutating the process TZ, so
// the test is hermetic and can run in parallel with other tests.
func TestDaysSinceUTC(t *testing.T) {
	mustLoc := func(name string) *time.Location {
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Fatalf("time.LoadLocation(%q) failed: %v", name, err)
		}
		return loc
	}

	tests := []struct {
		name     string
		lastSeen string
		now      time.Time
		want     int
	}{
		{
			name:     "same day is zero",
			lastSeen: "2026-08-05",
			now:      time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC),
			want:     0,
		},
		{
			name:     "three days ago in UTC",
			lastSeen: "2026-08-02",
			now:      time.Date(2026, 8, 5, 9, 0, 0, 0, time.UTC),
			want:     3,
		},
		{
			name:     "three days ago observed from Los Angeles (UTC-7 in August)",
			lastSeen: "2026-08-02",
			now:      time.Date(2026, 8, 5, 22, 0, 0, 0, mustLoc("America/Los_Angeles")),
			want:     3,
		},
		{
			name: "late-evening local time does not roll lastSeen into tomorrow",
			// 2026-08-05 23:30 in Los Angeles is 2026-08-06 06:30 UTC. The
			// pre-fix bug (time.Now() in local time minus a UTC-midnight
			// time.Parse) would compute this against the wrong UTC day;
			// the fix must use LA's own calendar day (Aug 5) as "today".
			lastSeen: "2026-08-05",
			now:      time.Date(2026, 8, 5, 23, 30, 0, 0, mustLoc("America/Los_Angeles")),
			want:     0,
		},
		{
			name:     "three days ago observed from Kiritimati (UTC+14, the worst-case offset)",
			lastSeen: "2026-08-02",
			now:      time.Date(2026, 8, 5, 3, 0, 0, 0, mustLoc("Pacific/Kiritimati")),
			want:     3,
		},
		{
			name: "stable across the US spring-forward DST boundary",
			// 2026-03-08 is the US DST spring-forward date; local midnight
			// to local midnight across it is only 23 real hours. A
			// local-zone Sub().Hours()/24 truncation could report 0 here
			// instead of 1. The UTC-anchored calculation is immune since
			// it never computes a local-zone elapsed-hours span.
			lastSeen: "2026-03-08",
			now:      time.Date(2026, 3, 9, 10, 0, 0, 0, mustLoc("America/Los_Angeles")),
			want:     1,
		},
		{
			name:     "unparseable date returns -1",
			lastSeen: "not-a-date",
			now:      time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC),
			want:     -1,
		},
		{
			name:     "empty date returns -1",
			lastSeen: "",
			now:      time.Date(2026, 8, 5, 12, 0, 0, 0, time.UTC),
			want:     -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := daysSinceUTC(tt.lastSeen, tt.now)
			if got != tt.want {
				t.Fatalf("daysSinceUTC(%q, %v) = %d, want %d", tt.lastSeen, tt.now, got, tt.want)
			}
		})
	}
}
