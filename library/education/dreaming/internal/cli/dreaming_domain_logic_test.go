// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"regexp"
	"testing"
	"time"
)

func TestLevelForHours(t *testing.T) {
	cases := []struct {
		hours float64
		want  int
	}{
		{0, 1}, {49, 1}, {50, 2}, {149, 2}, {150, 3}, {300, 4}, {600, 5}, {1000, 6}, {1500, 7}, {2000, 7},
	}
	for _, c := range cases {
		if got := levelForHours(c.hours); got != c.want {
			t.Errorf("levelForHours(%v) = %d, want %d", c.hours, got, c.want)
		}
	}
}

func TestVideoLevelBands(t *testing.T) {
	if got := videoLevelBands(3); len(got) != 1 || got[0] != "beginner" {
		t.Errorf("videoLevelBands(3) = %v, want [beginner]", got)
	}
	if got := videoLevelBands(7); len(got) == 0 {
		t.Errorf("videoLevelBands(7) returned no bands")
	}
}

func TestParseVTT(t *testing.T) {
	raw := "WEBVTT\n\n1\n00:00:01.000 --> 00:00:04.000\nHola mundo\n\n2\n00:00:04.000 --> 00:00:08.000\n<i>Segunda</i> línea"
	cues := parseVTT(raw)
	if len(cues) != 2 {
		t.Fatalf("parseVTT returned %d cues, want 2", len(cues))
	}
	if cues[0].StartMS != 1000 || cues[0].EndMS != 4000 {
		t.Errorf("cue0 timing = %d-%d, want 1000-4000", cues[0].StartMS, cues[0].EndMS)
	}
	if cues[0].Text != "Hola mundo" {
		t.Errorf("cue0 text = %q", cues[0].Text)
	}
	if cues[1].Text != "Segunda línea" { // VTT tags stripped
		t.Errorf("cue1 text = %q, want tags stripped", cues[1].Text)
	}
}

func TestVTTToPlainText(t *testing.T) {
	raw := "WEBVTT\n\n00:00:01.000 --> 00:00:02.000\nuno\n\n00:00:02.000 --> 00:00:03.000\ndos"
	if got := vttToPlainText(raw); got != "uno dos" {
		t.Errorf("vttToPlainText = %q, want %q", got, "uno dos")
	}
}

func TestTensePresetsCompileAndMatch(t *testing.T) {
	cases := map[string]struct {
		match   string
		nomatch string
	}{
		"imperfect":             {"compraba", "compro"},
		"future":                {"iré", "voy"},
		"conditional":           {"compraría", "compra"},
		"subjunctive-imperfect": {"tuviera", "tengo"},
		"gerund":                {"comprando", "compra"},
	}
	for preset, c := range cases {
		pat, ok := tensePresets[preset]
		if !ok {
			t.Errorf("missing preset %q", preset)
			continue
		}
		re, err := regexp.Compile(pat)
		if err != nil {
			t.Errorf("preset %q does not compile: %v", preset, err)
			continue
		}
		if !re.MatchString(c.match + " ") {
			t.Errorf("preset %q should match %q", preset, c.match)
		}
		if re.MatchString(c.nomatch + " ") {
			t.Errorf("preset %q should NOT match %q", preset, c.nomatch)
		}
	}
}

func TestParseWindowDays(t *testing.T) {
	cases := []struct {
		in   string
		want int
		err  bool
	}{
		{"90d", 90, false}, {"12w", 84, false}, {"3m", 90, false}, {"0", 0, false}, {"all", 0, false}, {"", 0, false}, {"xyz", 0, true},
	}
	for _, c := range cases {
		got, err := parseWindowDays(c.in)
		if c.err && err == nil {
			t.Errorf("parseWindowDays(%q) expected error", c.in)
		}
		if !c.err && got != c.want {
			t.Errorf("parseWindowDays(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestParseTargetHours(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		err  bool
	}{
		{"600h", 600, false}, {"5", 600, false}, {"L5", 600, false}, {"level5", 600, false}, {"1500h", 1500, false}, {"", 0, true}, {"abc", 0, true},
	}
	for _, c := range cases {
		got, err := parseTargetHours(c.in)
		if c.err {
			if err == nil {
				t.Errorf("parseTargetHours(%q) expected error", c.in)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("parseTargetHours(%q) = %v, %v; want %v", c.in, got, err, c.want)
		}
	}
}

func TestStreaks(t *testing.T) {
	// Build a 5-day consecutive run ending today via relative formatting is
	// awkward; instead test the gap-breaking logic with fixed dates.
	series := []daySeconds{
		{Date: "2026-01-01", Seconds: 600},
		{Date: "2026-01-02", Seconds: 600},
		{Date: "2026-01-03", Seconds: 600},
		{Date: "2026-01-10", Seconds: 600}, // gap resets
	}
	_, longest := streaks(series)
	if longest != 3 {
		t.Errorf("longest streak = %d, want 3", longest)
	}
}

func TestRecentAverageSeconds(t *testing.T) {
	day := func(offset int, secs int64) daySeconds {
		return daySeconds{Date: time.Now().AddDate(0, 0, offset).Format("2006-01-02"), Seconds: secs}
	}
	// Three active days inside a 10-day window (every other day) plus one active
	// day well outside it. The window is a calendar-day window, so the old entry
	// must be excluded and the denominator must be 10 (not the active-day count,
	// and not inflated by the out-of-window entry).
	series := []daySeconds{
		day(-30, 9999), // outside the 10-day window — must not count
		day(-4, 3600),
		day(-2, 3600),
		day(0, 3600),
	}
	got := recentAverageSeconds(series, 10)
	want := float64(3*3600) / 10.0 // 1080: only in-window seconds, divided by calendar days
	if got != want {
		t.Errorf("recentAverageSeconds = %v, want %v (out-of-window entry leaked or denominator wrong)", got, want)
	}
	// Empty / non-positive guards.
	if recentAverageSeconds(nil, 10) != 0 {
		t.Error("empty series should average 0")
	}
	if recentAverageSeconds(series, 0) != 0 {
		t.Error("zero days should average 0")
	}
}

// daysToDate must count local calendar days. The target arrives as UTC
// midnight (date-only time.Parse, as plan --by does), so a wall-clock
// subtraction is off by the timezone offset: in UTC-8 at 10 PM, "tomorrow"
// computed 0 days and plan reported a future date as not feasible.
func TestDaysToDateLocalCalendarDays(t *testing.T) {
	parse := func(offsetDays int) time.Time {
		d := time.Now().AddDate(0, 0, offsetDays).Format("2006-01-02")
		tt, err := time.Parse("2006-01-02", d)
		if err != nil {
			t.Fatalf("parse %s: %v", d, err)
		}
		return tt
	}
	cases := []struct {
		offset int
		want   int
	}{
		{0, 0},
		{1, 1},
		{-1, -1},
		{30, 30},
		{365, 365},
	}
	for _, tc := range cases {
		if got := daysToDate(parse(tc.offset)); got != tc.want {
			t.Errorf("daysToDate(today%+dd as UTC midnight) = %d, want %d", tc.offset, got, tc.want)
		}
	}
}

// ftsQuery must phrase-quote every input so FTS5 operators and special
// characters match literally instead of raising "fts5: syntax error".
func TestFTSQueryQuoting(t *testing.T) {
	cases := []struct{ in, want string }{
		{"entonces", `"entonces"`},
		{"me gusta", `"me gusta"`},
		{"*", `"*"`},
		{"(foo", `"(foo"`},
		{"-bar", `"-bar"`},
		{`say "hola"`, `"say ""hola"""`},
		{"  padded  ", `"padded"`},
		{"", ""},
	}
	for _, tc := range cases {
		if got := ftsQuery(tc.in); got != tc.want {
			t.Errorf("ftsQuery(%q) = %s, want %s", tc.in, got, tc.want)
		}
	}
}
