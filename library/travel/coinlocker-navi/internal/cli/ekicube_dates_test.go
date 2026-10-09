// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"regexp"
	"testing"
	"time"
)

func TestDefaultEkicubeDates(t *testing.T) {
	// 16:30 UTC is already the next day in JST.
	now := time.Date(2026, 10, 9, 16, 30, 0, 0, time.UTC)
	cases := []struct{ from, end, wantFrom, wantEnd string }{
		{"", "", "2026-10-10", "2026-10-10"},
		{"2026-12-01", "", "2026-12-01", "2026-12-01"},
		{"2026-12-01", "2026-12-02", "2026-12-01", "2026-12-02"},
		{" ", "", "2026-10-10", "2026-10-10"},
	}
	for _, c := range cases {
		f, e := defaultEkicubeDates(c.from, c.end, now)
		if f != c.wantFrom || e != c.wantEnd {
			t.Errorf("defaultEkicubeDates(%q, %q) = %q, %q; want %q, %q", c.from, c.end, f, e, c.wantFrom, c.wantEnd)
		}
	}
}

// Examples and live-dogfood fixtures must not pin a calendar date: a fixed
// date goes stale and the source then answers for a past day.
func TestSourceEkicubeFixturesHaveNoFixedDate(t *testing.T) {
	cmd := newSourceEkicubeCmd(&rootFlags{})
	date := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	if date.MatchString(cmd.Example) {
		t.Errorf("Example pins a date: %s", cmd.Example)
	}
	if happy := cmd.Annotations["pp:happy-args"]; date.MatchString(happy) {
		t.Errorf("pp:happy-args pins a date: %s", happy)
	}
}
