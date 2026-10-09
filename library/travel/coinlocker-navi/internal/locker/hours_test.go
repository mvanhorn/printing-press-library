// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package locker

import "testing"

func TestParseHours(t *testing.T) {
	cases := []struct {
		raw, kind, open, close string
		overnight              bool
	}{
		{"09時00分～21時00分", "clock_range", "09:00", "21:00", false},
		{"05時00分～02時00分", "clock_range", "05:00", "02:00", true},
		{"始発から終電", "first_to_last_train", "", "", false},
		{"初電～終電", "first_to_last_train", "", "", false},
		{"24時間", "24h", "", "", false},
		{"情報なし", "unknown", "", "", false},
		{"", "unknown", "", "", false},
		{"施設の営業時間に準ずる", "unparsed", "", "", false},
	}
	for _, c := range cases {
		h := ParseHours(c.raw)
		if h.Kind != c.kind {
			t.Errorf("%q: kind = %s, want %s", c.raw, h.Kind, c.kind)
			continue
		}
		if c.open != "" && (h.Open == nil || *h.Open != c.open || *h.Close != c.close || h.Overnight != c.overnight) {
			t.Errorf("%q: got %v-%v overnight=%v", c.raw, h.Open, h.Close, h.Overnight)
		}
		if c.kind == "unknown" && h.Raw != nil {
			t.Errorf("%q: unknown must have raw=nil", c.raw)
		}
	}
}

func TestParseWindow(t *testing.T) {
	w, err := ParseWindow("10:00-21:30")
	if err != nil || w.Start != 600 || w.End != 1290 {
		t.Fatalf("got %+v %v", w, err)
	}
	w, err = ParseWindow("22:00-01:00")
	if err != nil || w.End != 1500 {
		t.Fatalf("overnight window: %+v %v", w, err)
	}
	for _, bad := range []string{"10-21", "25:00-26:00", "10:00", "aa:bb-cc:dd"} {
		if _, err := ParseWindow(bad); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}

func TestOpenDuring(t *testing.T) {
	day, _ := ParseWindow("10:00-20:30")
	late, _ := ParseWindow("10:00-22:00")
	night, _ := ParseWindow("23:00-01:30")
	cases := []struct {
		hours string
		w     Window
		want  string
	}{
		{"09時00分～21時00分", day, OpenYes},
		{"09時00分～21時00分", late, OpenNo},
		{"05時00分～02時00分", night, OpenYes},
		{"05時00分～02時00分", late, OpenYes},
		{"10時30分～20時00分", day, OpenNo},
		{"24時間", night, OpenYes},
		{"始発から終電", day, OpenTrainHours},
		{"情報なし", day, OpenUnknown},
		{"施設の営業時間に準ずる", day, OpenUnknown},
	}
	for _, c := range cases {
		if got := OpenDuring(ParseHours(c.hours), c.w); got != c.want {
			t.Errorf("%q window %s: got %s, want %s", c.hours, c.w.Raw, got, c.want)
		}
	}
}

func TestOpenDuringFullDay(t *testing.T) {
	cross, _ := ParseWindow("23:00-01:00")
	for _, raw := range []string{"00時00分～24時00分", "00時00分～00時00分"} {
		h := ParseHours(raw)
		if h.Kind != "clock_range" {
			t.Logf("%q parses as %s; skipping", raw, h.Kind)
			continue
		}
		if got := OpenDuring(h, cross); got != OpenYes {
			t.Errorf("%q over midnight: got %s, want yes", raw, got)
		}
	}
}

func TestOpenDuringPastMidnightClock(t *testing.T) {
	h := ParseHours("07時00分～25時00分")
	if h.Kind != HoursClockRange || !h.Overnight {
		t.Fatalf("hours = %+v", h)
	}
	late, _ := ParseWindow("20:00-00:30")
	tooLate, _ := ParseWindow("20:00-02:00")
	if got := OpenDuring(h, late); got != OpenYes {
		t.Errorf("until 00:30: got %s", got)
	}
	if got := OpenDuring(h, tooLate); got != OpenNo {
		t.Errorf("until 02:00: got %s", got)
	}
}

func TestOvernightFlag(t *testing.T) {
	cases := map[string]bool{"10時00分～24時00分": false, "00時00分～24時00分": false, "06時00分～25時00分": true, "05時00分～02時00分": true, "09時00分～21時00分": false}
	for raw, want := range cases {
		if got := ParseHours(raw).Overnight; got != want {
			t.Errorf("%q: overnight = %v, want %v", raw, got, want)
		}
	}
}

func TestWindowInNoTrainHours(t *testing.T) {
	cases := map[string]bool{
		"10:00-21:30": false,
		"01:00-02:00": true,
		"00:00-00:59": false,
		"22:00-08:00": true,
		"04:30-06:00": false,
		"04:00-06:00": true,
		"23:30-00:50": false,
	}
	for raw, want := range cases {
		w, err := ParseWindow(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := WindowInNoTrainHours(w); got != want {
			t.Errorf("%s: got %v, want %v", raw, got, want)
		}
	}
}
