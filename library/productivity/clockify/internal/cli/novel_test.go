// Copyright 2026 melanson633. Licensed under Apache-2.0. See LICENSE.
// Tests for the hand-built transcendence-feature support logic.

package cli

import (
	"testing"
	"time"
)

func TestParseISO8601Duration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"PT8H", 8 * time.Hour},
		{"PT8H30M", 8*time.Hour + 30*time.Minute},
		{"PT90M", 90 * time.Minute},
		{"PT1H30M15S", time.Hour + 30*time.Minute + 15*time.Second},
		{"PT45S", 45 * time.Second},
		{"P", 0},
		{"garbage", 0},
		{"8h", 0}, // Go-style is not ISO-8601
	}
	for _, c := range cases {
		if got := parseISO8601Duration(c.in); got != c.want {
			t.Errorf("parseISO8601Duration(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseHoursDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"", 0},
		{"PT2H", 2 * time.Hour},
		{"1h30m", time.Hour + 30*time.Minute},
		{"1.5", 90 * time.Minute},
		{"0.25", 15 * time.Minute},
		{"nonsense", 0},
	}
	for _, c := range cases {
		if got := parseHoursDuration(c.in); got != c.want {
			t.Errorf("parseHoursDuration(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestWeekStart(t *testing.T) {
	// 2026-05-20 is a Wednesday; its week starts Monday 2026-05-18.
	wed := time.Date(2026, 5, 20, 14, 30, 0, 0, time.Local)
	got := weekStart(wed)
	want := time.Date(2026, 5, 18, 0, 0, 0, 0, time.Local)
	if !got.Equal(want) {
		t.Errorf("weekStart(Wed) = %v, want %v", got, want)
	}
	// A Monday is its own week start.
	mon := time.Date(2026, 5, 18, 9, 0, 0, 0, time.Local)
	if got := weekStart(mon); !got.Equal(want) {
		t.Errorf("weekStart(Mon) = %v, want %v", got, want)
	}
	// A Sunday belongs to the week that started the prior Monday.
	sun := time.Date(2026, 5, 24, 23, 0, 0, 0, time.Local)
	if got := weekStart(sun); !got.Equal(want) {
		t.Errorf("weekStart(Sun) = %v, want %v", got, want)
	}
}

func TestResolveRange(t *testing.T) {
	now := time.Date(2026, 5, 20, 12, 0, 0, 0, time.Local) // Wednesday

	start, end, label, err := resolveRange("today", now)
	if err != nil || label != "today" {
		t.Fatalf("today: err=%v label=%q", err, label)
	}
	if !start.Equal(time.Date(2026, 5, 20, 0, 0, 0, 0, time.Local)) || end.Sub(start) != 24*time.Hour {
		t.Errorf("today window wrong: %v..%v", start, end)
	}

	start, end, _, err = resolveRange("7d", now)
	if err != nil {
		t.Fatalf("7d: %v", err)
	}
	if d := end.Sub(start); d != 7*24*time.Hour {
		t.Errorf("7d window = %v, want 168h", d)
	}

	start, end, _, err = resolveRange("this-week", now)
	if err != nil {
		t.Fatalf("this-week: %v", err)
	}
	if !start.Equal(time.Date(2026, 5, 18, 0, 0, 0, 0, time.Local)) || end.Sub(start) != 7*24*time.Hour {
		t.Errorf("this-week window wrong: %v..%v", start, end)
	}

	if _, _, _, err := resolveRange("bogus", now); err == nil {
		t.Error("resolveRange(bogus) should error")
	}

	// Empty defaults to this-month.
	_, _, label, err = resolveRange("", now)
	if err != nil || label != "this month" {
		t.Errorf(`resolveRange("") label=%q err=%v, want "this month"`, label, err)
	}
}

func TestParseEntry(t *testing.T) {
	raw := []byte(`{"id":"e1","description":"work","billable":true,"projectId":"p1",
		"timeInterval":{"start":"2026-05-20T09:00:00Z","end":"2026-05-20T11:30:00Z"}}`)
	te, ok := parseEntry(raw)
	if !ok {
		t.Fatal("parseEntry returned ok=false for a valid entry")
	}
	if te.Duration != 2*time.Hour+30*time.Minute {
		t.Errorf("duration = %v, want 2h30m", te.Duration)
	}
	if te.Running {
		t.Error("completed entry flagged as running")
	}

	// Running timer: end is absent.
	running := []byte(`{"id":"e2","timeInterval":{"start":"2026-05-20T09:00:00Z"}}`)
	te, ok = parseEntry(running)
	if !ok || !te.Running {
		t.Errorf("running entry: ok=%v running=%v", ok, te.Running)
	}

	// Invalid: no id.
	if _, ok := parseEntry([]byte(`{"description":"x"}`)); ok {
		t.Error("parseEntry accepted an entry with no id")
	}
}

func TestWindowEvents(t *testing.T) {
	base := time.Date(2026, 5, 20, 9, 0, 0, 0, time.Local)
	events := []timedEvent{
		{t: base, label: "git"},
		{t: base.Add(5 * time.Minute), label: "npm"},
		{t: base.Add(3 * time.Hour), label: "vim"}, // gap > 15m -> new window
	}
	drafts := windowEvents(events, 15*time.Minute, "shell", false)
	if len(drafts) != 2 {
		t.Fatalf("windowEvents produced %d drafts, want 2", len(drafts))
	}
	// First window spans the two close events.
	if drafts[0].End.Sub(drafts[0].Start) != 5*time.Minute {
		t.Errorf("first window = %v, want 5m", drafts[0].End.Sub(drafts[0].Start))
	}
	// Single-event window gets the 5-minute floor.
	if drafts[1].End.Sub(drafts[1].Start) != 5*time.Minute {
		t.Errorf("single-event window = %v, want 5m floor", drafts[1].End.Sub(drafts[1].Start))
	}
}

func TestSubmissionLabel(t *testing.T) {
	cases := []struct {
		state     string
		submitted bool
	}{
		{"", false},
		{"PENDING", true},
		{"APPROVED", true},
		{"REJECTED", false},
		{"WITHDRAWN_SUBMISSION", false},
	}
	for _, c := range cases {
		if _, got := submissionLabel(c.state); got != c.submitted {
			t.Errorf("submissionLabel(%q) submitted=%v, want %v", c.state, got, c.submitted)
		}
	}
}
