// Copyright 2026 Bob Benjy and contributors. Licensed under Apache-2.0. See LICENSE.

package courtdays

import (
	"strings"
	"testing"
	"time"
)

// Real 2026/2027 LASC holidays (subset relevant to the tests).
var fixture = map[int]string{
	2026: `"[{\"date\":\"11/11/2026\",\"day\":\"Wednesday\",\"description\":\"Veterans Day\"},{\"date\":\"11/26/2026\",\"day\":\"Thursday\",\"description\":\"Thanksgiving Day\"},{\"date\":\"11/27/2026\",\"day\":\"Friday\",\"description\":\"Day after Thanksgiving\"},{\"date\":\"12/25/2026\",\"day\":\"Friday\",\"description\":\"Christmas Day\"}]"`,
	2027: `[{"date":"01/01/2027","day":"Friday","description":"New Year's Day (2027)"},{"date":"01/18/2027","day":"Monday","description":"Martin Luther King, Jr. Birthday"}]`,
}

func newTestCal(t *testing.T) *Calendar {
	t.Helper()
	return NewCalendar(func(year int) ([]Holiday, error) {
		raw, ok := fixture[year]
		if !ok {
			return []Holiday{}, nil
		}
		return ParseHolidays([]byte(raw))
	})
}

func d(s string) time.Time {
	t, err := ParseDate(s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestParseHolidays(t *testing.T) {
	hs, err := ParseHolidays([]byte(fixture[2026]))
	if err != nil {
		t.Fatal(err)
	}
	if len(hs) != 4 || hs[0].Date != "2026-11-11" || hs[3].Description != "Christmas Day" {
		t.Fatalf("unexpected parse: %+v", hs)
	}
	if _, err := ParseHolidays([]byte(`"not json"`)); err == nil {
		t.Fatal("expected error for malformed payload")
	}
	empty, err := ParseHolidays([]byte(`"[]"`))
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty payload: %v %v", empty, err)
	}
}

func TestParseDate(t *testing.T) {
	for _, s := range []string{"2026-11-27", "11/27/2026", "11/27/2026"} {
		if got := d(s).Format(isoLayout); got != "2026-11-27" {
			t.Errorf("%s -> %s", s, got)
		}
	}
	if _, err := ParseDate("Nov 27"); err == nil {
		t.Error("expected error")
	}
}

func TestCheck(t *testing.T) {
	c := newTestCal(t)
	cases := []struct {
		date   string
		court  bool
		reason string
	}{
		{"2026-11-27", false, "court holiday: Day after Thanksgiving"},
		{"2026-11-28", false, "weekend"},
		{"2026-11-30", true, "court is open"},
	}
	for _, tc := range cases {
		st, err := c.Check(d(tc.date))
		if err != nil {
			t.Fatal(err)
		}
		if st.CourtDay != tc.court || st.Reason != tc.reason {
			t.Errorf("%s: got %+v", tc.date, st)
		}
	}
	if _, err := c.Check(d("2031-03-03")); !IsUnpublished(err) {
		t.Errorf("expected unpublished error, got %v", err)
	}
}

func TestAddCourtDays(t *testing.T) {
	c := newTestCal(t)
	// Wed 2026-11-25 + 1 court day skips Thu/Fri holidays and the weekend -> Mon 11-30.
	r, err := c.AddCourtDays(d("2026-11-25"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if r.Result != "2026-11-30" || len(r.SkippedHolidays) != 2 {
		t.Fatalf("forward: %+v", r)
	}
	// Backward: Mon 11-30 - 1 court day -> Wed 11-25.
	r, err = c.AddCourtDays(d("2026-11-30"), -1)
	if err != nil || r.Result != "2026-11-25" {
		t.Fatalf("backward: %+v %v", r, err)
	}
	// Crossing into 2027: Thu 2026-12-31 + 1 skips New Year's Day and the weekend -> Mon 2027-01-04.
	r, err = c.AddCourtDays(d("2026-12-31"), 1)
	if err != nil || r.Result != "2027-01-04" {
		t.Fatalf("year cross: %+v %v", r, err)
	}
	// Zero is the start date itself.
	r, err = c.AddCourtDays(d("2026-11-27"), 0)
	if err != nil || r.Result != "2026-11-27" || len(r.SkippedHolidays) != 0 || r.StartIsCourtDay {
		t.Fatalf("zero: %+v %v", r, err)
	}
}

func TestCalendarDeadline(t *testing.T) {
	c := newTestCal(t)
	// 2026-11-16 + 10 calendar days = Thu 11-26 (Thanksgiving) -> rolls past Fri holiday + weekend to Mon 11-30.
	r, err := c.CalendarDeadline(d("2026-11-16"), 10, false)
	if err != nil {
		t.Fatal(err)
	}
	if r.RawEnd != "2026-11-26" || r.Deadline != "2026-11-30" || !r.Rolled || len(r.RolledPast) != 4 {
		t.Fatalf("rolled: %+v", r)
	}
	// --back with n=0 on a holiday rolls backward: Fri 11-27 -> Wed 11-25.
	r, err = c.CalendarDeadline(d("2026-11-27"), 0, true)
	if err != nil || r.Deadline != "2026-11-25" {
		t.Fatalf("back roll: %+v %v", r, err)
	}
	// Landing on a court day does not roll.
	r, err = c.CalendarDeadline(d("2026-11-16"), 1, false)
	if err != nil || r.Rolled || r.Deadline != "2026-11-17" {
		t.Fatalf("no roll: %+v %v", r, err)
	}
}

func TestCourtDaysBetween(t *testing.T) {
	c := newTestCal(t)
	r, err := c.CourtDaysBetween(d("2026-11-25"), d("2026-11-30"))
	if err != nil {
		t.Fatal(err)
	}
	if r.CourtDays != 1 || len(r.Holidays) != 2 {
		t.Fatalf("between: %+v", r)
	}
	// Consistency with AddCourtDays.
	add, _ := c.AddCourtDays(d("2026-11-02"), 15)
	b, _ := c.CourtDaysBetween(d("2026-11-02"), d(add.Result))
	if b.CourtDays != 15 {
		t.Fatalf("round trip: add=%s between=%d", add.Result, b.CourtDays)
	}
	neg, _ := c.CourtDaysBetween(d("2026-11-30"), d("2026-11-25"))
	if neg.CourtDays != -1 {
		t.Fatalf("negative: %+v", neg)
	}
	same, _ := c.CourtDaysBetween(d("2026-11-30"), d("2026-11-30"))
	if same.CourtDays != 0 {
		t.Fatalf("same day: %+v", same)
	}
}

func TestNextHolidays(t *testing.T) {
	c := newTestCal(t)
	hs, err := c.NextHolidays(d("2026-12-26"), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(hs) != 2 || hs[0].Date != "2027-01-01" || hs[1].Date != "2027-01-18" {
		t.Fatalf("next: %+v", hs)
	}
	// On-or-after includes the from date.
	hs, _ = c.NextHolidays(d("2026-11-27"), 1)
	if hs[0].Date != "2026-11-27" {
		t.Fatalf("inclusive: %+v", hs)
	}
	// Running off the end of published years returns what exists, no error.
	hs, err = c.NextHolidays(d("2027-01-02"), 5)
	if err != nil || len(hs) != 1 {
		t.Fatalf("end of data: %+v %v", hs, err)
	}
}

func TestICS(t *testing.T) {
	hs, _ := ParseHolidays([]byte(fixture[2027]))
	out := ICS(hs, time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	for _, want := range []string{"BEGIN:VCALENDAR", "DTSTART;VALUE=DATE:20270101", "DTEND;VALUE=DATE:20270102", `SUMMARY:LASC closed: Martin Luther King\, Jr. Birthday`, "END:VCALENDAR"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Count(out, "BEGIN:VEVENT") != 2 {
		t.Error("expected 2 events")
	}
}

func TestICSFolding(t *testing.T) {
	hs := []Holiday{{Date: "2027-02-15", Day: "Monday", Description: "Presidents’ Day with a long enough description to force folding past seventy-five octets"}}
	out := ICS(hs, time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC))
	for _, line := range strings.Split(out, "\r\n") {
		if len(line) > 75 {
			t.Errorf("line over 75 octets (%d): %q", len(line), line)
		}
	}
	if !strings.Contains(out, "\r\n ") {
		t.Error("expected folded continuation lines")
	}
	if got := icsEscape("a\r\nb\rc"); got != `a\nb\nc` {
		t.Errorf("escape CR: %q", got)
	}
}

func TestYearKeepsDuplicateDates(t *testing.T) {
	c := NewCalendar(func(int) ([]Holiday, error) {
		return []Holiday{{Date: "2027-01-01", Description: "A"}, {Date: "2027-01-01", Description: "B"}}, nil
	})
	hs, err := c.Year(2027)
	if err != nil || len(hs) != 2 {
		t.Fatalf("got %+v %v", hs, err)
	}
}
