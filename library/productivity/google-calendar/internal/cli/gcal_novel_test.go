package cli

import (
	"testing"
	"time"
)

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return tm
}

func TestMergeIntervals(t *testing.T) {
	a := mustParse(t, "2026-05-25T09:00:00Z")
	b := mustParse(t, "2026-05-25T10:00:00Z")
	c := mustParse(t, "2026-05-25T09:30:00Z")
	d := mustParse(t, "2026-05-25T11:00:00Z")
	e := mustParse(t, "2026-05-25T12:00:00Z")
	f := mustParse(t, "2026-05-25T13:00:00Z")

	// [9-10] overlaps [9:30-11] -> [9-11]; [12-13] separate.
	got := mergeIntervals([]interval{{a, b}, {c, d}, {e, f}})
	if len(got) != 2 {
		t.Fatalf("expected 2 merged intervals, got %d: %+v", len(got), got)
	}
	if !got[0].start.Equal(a) || !got[0].end.Equal(d) {
		t.Errorf("first merged = %v..%v, want %v..%v", got[0].start, got[0].end, a, d)
	}
}

func TestSubtractBusy_FindsGaps(t *testing.T) {
	open := interval{mustParse(t, "2026-05-25T09:00:00Z"), mustParse(t, "2026-05-25T17:00:00Z")}
	busy := []interval{
		{mustParse(t, "2026-05-25T10:00:00Z"), mustParse(t, "2026-05-25T11:00:00Z")},
		{mustParse(t, "2026-05-25T14:00:00Z"), mustParse(t, "2026-05-25T15:00:00Z")},
	}
	gaps := subtractBusy(open, mergeIntervals(busy))
	// Expect 3 gaps: 9-10, 11-14, 15-17.
	if len(gaps) != 3 {
		t.Fatalf("expected 3 gaps, got %d: %+v", len(gaps), gaps)
	}
	if gaps[1].start.Sub(gaps[1].end) == 0 {
		t.Error("middle gap should be non-empty")
	}
	want := mustParse(t, "2026-05-25T11:00:00Z")
	if !gaps[1].start.Equal(want) {
		t.Errorf("middle gap starts %v, want %v", gaps[1].start, want)
	}
}

func TestSubtractBusy_FullyBooked(t *testing.T) {
	open := interval{mustParse(t, "2026-05-25T09:00:00Z"), mustParse(t, "2026-05-25T10:00:00Z")}
	busy := []interval{{mustParse(t, "2026-05-25T08:00:00Z"), mustParse(t, "2026-05-25T11:00:00Z")}}
	gaps := subtractBusy(open, mergeIntervals(busy))
	if len(gaps) != 0 {
		t.Errorf("expected no free gaps when fully booked, got %+v", gaps)
	}
}

func TestResolveCalendars(t *testing.T) {
	if got := resolveCalendars(""); len(got) != 1 || got[0] != "primary" {
		t.Errorf("empty -> %v, want [primary]", got)
	}
	got := resolveCalendars("work, team ,")
	if len(got) != 2 || got[0] != "work" || got[1] != "team" {
		t.Errorf("csv -> %v, want [work team]", got)
	}
}

func TestParseWindow_Today(t *testing.T) {
	start, end, err := parseWindow("today")
	if err != nil {
		t.Fatal(err)
	}
	if d := end.Sub(start); d != 24*time.Hour {
		t.Errorf("today window = %v, want 24h", d)
	}
}

func TestParseWindow_Range(t *testing.T) {
	start, end, err := parseWindow("2026-05-01..2026-05-08")
	if err != nil {
		t.Fatal(err)
	}
	if d := end.Sub(start); d != 7*24*time.Hour {
		t.Errorf("range window = %v, want 168h", d)
	}
}

func TestParseWindow_Invalid(t *testing.T) {
	if _, _, err := parseWindow("bldkfj"); err == nil {
		t.Error("expected error for unparseable window")
	}
}

func TestParseBookBound(t *testing.T) {
	tm, allDay, err := parseBookBound("2026-05-25T15:00:00Z")
	if err != nil || allDay {
		t.Fatalf("timed parse: allDay=%v err=%v", allDay, err)
	}
	if tm.Hour() != 15 {
		t.Errorf("hour = %d, want 15", tm.Hour())
	}
	_, allDay, err = parseBookBound("2026-05-25")
	if err != nil || !allDay {
		t.Fatalf("date parse: allDay=%v err=%v", allDay, err)
	}
	if _, _, err := parseBookBound("not-a-time"); err == nil {
		t.Error("expected error for bad bound")
	}
}

func TestParseStoredEvent(t *testing.T) {
	raw := `{"id":"e1","summary":"Standup","status":"confirmed","start":{"dateTime":"2026-05-25T09:00:00Z"},"end":{"dateTime":"2026-05-25T09:15:00Z"},"attendees":[{"displayName":"Attendee One","responseStatus":"accepted","self":true},{"displayName":"Attendee Two","responseStatus":"declined"}]}`
	ev, ok := parseStoredEvent("primary", []byte(raw))
	if !ok {
		t.Fatal("expected ok")
	}
	if ev.AllDay {
		t.Error("timed event flagged all-day")
	}
	if len(ev.Attendees) != 2 || ev.Attendees[0].ResponseStatus != "accepted" {
		t.Errorf("attendees not parsed: %+v", ev.Attendees)
	}
	if ev.End.Sub(ev.Start) != 15*time.Minute {
		t.Errorf("duration = %v, want 15m", ev.End.Sub(ev.Start))
	}
}
