// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestSlotMatchesWindowSpec_Empty(t *testing.T) {
	if !slotMatchesWindowSpec("2026-05-13", "19:00", "") {
		t.Error("expected empty spec to match any slot")
	}
}

func TestSlotMatchesWindowSpec_DayOfWeek(t *testing.T) {
	// 2026-05-09 is a Saturday
	cases := map[string]bool{
		"sat":            true,
		"saturday":       true,
		"sun":            false,
		"mon":            false,
		"tue":            false,
		"sat 7-9pm":      true,
		"sun 6pm":        false,
		"this saturday":  true,
		"NEXT Saturday!": true, // case-insensitive
	}
	for spec, want := range cases {
		t.Run(spec, func(t *testing.T) {
			got := slotMatchesWindowSpec("2026-05-09", "19:00", spec)
			if got != want {
				t.Errorf("spec=%q for Saturday slot: got %v, want %v", spec, got, want)
			}
		})
	}
}

func TestSlotMatchesWindowSpec_HourRange12h(t *testing.T) {
	// Saturday 19:00 should match "7-9pm"; 18:00 should not match "7-9pm".
	cases := []struct {
		date, hhmm, spec string
		want             bool
	}{
		{"2026-05-09", "19:00", "7-9pm", true},
		{"2026-05-09", "21:00", "7-9pm", true},  // inclusive end
		{"2026-05-09", "21:30", "7-9pm", false}, // past end
		{"2026-05-09", "18:30", "7-9pm", false}, // before start
		{"2026-05-09", "19:00", "7pm-9pm", true},
		{"2026-05-09", "12:00", "11am-2pm", true},
		{"2026-05-09", "14:30", "11am-2pm", false},
	}
	for _, tc := range cases {
		t.Run(tc.spec+"@"+tc.hhmm, func(t *testing.T) {
			got := slotMatchesWindowSpec(tc.date, tc.hhmm, tc.spec)
			if got != tc.want {
				t.Errorf("date=%s time=%s spec=%q: got %v, want %v", tc.date, tc.hhmm, tc.spec, got, tc.want)
			}
		})
	}
}

func TestSlotMatchesWindowSpec_HourRange24h(t *testing.T) {
	cases := []struct {
		hhmm, spec string
		want       bool
	}{
		{"19:00", "19:00-21:00", true},
		{"21:00", "19:00-21:00", true},
		{"18:59", "19:00-21:00", false},
		{"21:01", "19:00-21:00", false},
	}
	for _, tc := range cases {
		t.Run(tc.spec+"@"+tc.hhmm, func(t *testing.T) {
			got := slotMatchesWindowSpec("2026-05-09", tc.hhmm, tc.spec)
			if got != tc.want {
				t.Errorf("time=%s spec=%q: got %v, want %v", tc.hhmm, tc.spec, got, tc.want)
			}
		})
	}
}

func TestSlotMatchesWindowSpec_DayAndHourCombined(t *testing.T) {
	// Saturday 19:00 should match "sat 7-9pm"; Sunday 19:00 should not.
	if !slotMatchesWindowSpec("2026-05-09", "19:00", "sat 7-9pm") {
		t.Error("Saturday 19:00 should match 'sat 7-9pm'")
	}
	if slotMatchesWindowSpec("2026-05-10", "19:00", "sat 7-9pm") {
		t.Error("Sunday 19:00 should NOT match 'sat 7-9pm'")
	}
	if slotMatchesWindowSpec("2026-05-09", "18:00", "sat 7-9pm") {
		t.Error("Saturday 18:00 should NOT match 'sat 7-9pm'")
	}
}

func TestSlotMatchesWindowSpec_UnparseableReturnsTrue(t *testing.T) {
	// Free-form specs we can't parse should over-fire (true) rather than
	// silently drop watches the user explicitly wanted.
	cases := []string{
		"around dinner",
		"whenever there's room",
		"prime time",
	}
	for _, spec := range cases {
		t.Run(spec, func(t *testing.T) {
			if !slotMatchesWindowSpec("2026-05-09", "19:00", spec) {
				t.Errorf("expected unparseable spec %q to default to true", spec)
			}
		})
	}
}

func TestSlotMatchesWindowSpec_MalformedDate(t *testing.T) {
	// Defensive: bad date input → match anyway (don't drop the watch).
	if !slotMatchesWindowSpec("not-a-date", "19:00", "sat 7-9pm") {
		t.Error("expected malformed date to default to true")
	}
}

func TestSlotMatchesWindowSpec_MalformedTime(t *testing.T) {
	if !slotMatchesWindowSpec("2026-05-09", "bad", "7-9pm") {
		t.Error("expected malformed time to default to true")
	}
}
