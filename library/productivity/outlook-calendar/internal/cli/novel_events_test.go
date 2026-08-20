package cli

import (
	"testing"
	"time"
)

func TestParseGraphTime(t *testing.T) {
	cases := []struct {
		name string
		in   graphDateTime
		want string // RFC3339 normalized
	}{
		{"utc 7-digit fractional", graphDateTime{DateTime: "2026-05-10T14:00:00.0000000", TimeZone: "UTC"}, "2026-05-10T14:00:00Z"},
		{"plain seconds", graphDateTime{DateTime: "2026-05-10T14:00:00", TimeZone: "UTC"}, "2026-05-10T14:00:00Z"},
		{"missing tz defaults UTC", graphDateTime{DateTime: "2026-05-10T14:00:00", TimeZone: ""}, "2026-05-10T14:00:00Z"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseGraphTime(tc.in)
			if err != nil {
				t.Fatalf("parseGraphTime: %v", err)
			}
			if got.UTC().Format(time.RFC3339) != tc.want {
				t.Fatalf("parseGraphTime(%v) = %s, want %s", tc.in, got.UTC().Format(time.RFC3339), tc.want)
			}
		})
	}
}

func TestParseGraphTimeRejectsEmpty(t *testing.T) {
	if _, err := parseGraphTime(graphDateTime{}); err == nil {
		t.Fatal("expected error on empty dateTime")
	}
}

func TestParseGraphEvent(t *testing.T) {
	raw := []byte(`{
		"id": "evt-1",
		"subject": "1:1 with Alice",
		"showAs": "busy",
		"isCancelled": false,
		"isOrganizer": true,
		"type": "occurrence",
		"seriesMasterId": "master-1",
		"start": {"dateTime": "2026-05-10T14:00:00", "timeZone": "UTC"},
		"end":   {"dateTime": "2026-05-10T15:00:00", "timeZone": "UTC"},
		"location": {"displayName": "Conference Room A"},
		"attendees": [
			{"type": "required", "emailAddress": {"address": "Alice@Example.com", "name": "Alice"}, "status": {"response": "accepted", "time": "2026-05-01T12:00:00Z"}}
		],
		"organizer": {"emailAddress": {"address": "me@x.com", "name": "Me"}},
		"responseStatus": {"response": "organizer", "time": "0001-01-01T00:00:00Z"}
	}`)
	ev, err := parseGraphEvent(raw)
	if err != nil {
		t.Fatalf("parseGraphEvent: %v", err)
	}
	if ev.ID != "evt-1" || ev.Subject != "1:1 with Alice" || ev.Location != "Conference Room A" {
		t.Fatalf("basic fields wrong: %+v", ev)
	}
	if !ev.IsOrganizer {
		t.Fatal("IsOrganizer should be true")
	}
	if ev.SeriesMasterID != "master-1" || ev.Type != "occurrence" {
		t.Fatalf("recurrence fields wrong: type=%s master=%s", ev.Type, ev.SeriesMasterID)
	}
	if len(ev.Attendees) != 1 {
		t.Fatalf("expected 1 attendee, got %d", len(ev.Attendees))
	}
	if ev.Attendees[0].Email != "alice@example.com" {
		t.Fatalf("attendee email should be lowercased, got %q", ev.Attendees[0].Email)
	}
	if ev.Attendees[0].Response != "accepted" {
		t.Fatalf("attendee response should be 'accepted', got %q", ev.Attendees[0].Response)
	}
}

func TestParseHumanTime(t *testing.T) {
	anchor := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		in   string
		want time.Time
	}{
		{"now", anchor},
		{"today", time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)},
		{"tomorrow", time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)},
		{"+7d", anchor.AddDate(0, 0, 7)},
		{"-1d", anchor.AddDate(0, 0, -1)},
		{"+4h", anchor.Add(4 * time.Hour)},
		{"2026-06-01", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := parseHumanTime(tc.in, anchor)
			if err != nil {
				t.Fatalf("parseHumanTime(%q): %v", tc.in, err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("parseHumanTime(%q) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}
}

func TestParseHumanTimeBad(t *testing.T) {
	_, err := parseHumanTime("nonsense", time.Now())
	if err == nil {
		t.Fatal("expected parse error")
	}
}

func TestParseWorkingHours(t *testing.T) {
	wh, err := parseWorkingHours("Mon-Fri 9-17")
	if err != nil {
		t.Fatalf("parseWorkingHours: %v", err)
	}
	if wh.from != 9 || wh.to != 17 {
		t.Fatalf("hours: from=%d to=%d", wh.from, wh.to)
	}
	if !wh.days[time.Monday] || !wh.days[time.Friday] || wh.days[time.Sunday] {
		t.Fatalf("days wrong: %+v", wh.days)
	}
}

func TestParseWorkingHoursAny(t *testing.T) {
	wh, err := parseWorkingHours("any")
	if err != nil {
		t.Fatalf("parseWorkingHours: %v", err)
	}
	if !wh.any {
		t.Fatal("expected any=true")
	}
}

func TestMergeIntervals(t *testing.T) {
	t1 := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 5, 10, 10, 0, 0, 0, time.UTC)
	t3 := time.Date(2026, 5, 10, 9, 30, 0, 0, time.UTC) // overlaps t1..t2
	t4 := time.Date(2026, 5, 10, 11, 0, 0, 0, time.UTC) // contiguous? no, gap
	t5 := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)

	in := []freetimeInterval{
		{t1, t2}, {t3, time.Date(2026, 5, 10, 10, 30, 0, 0, time.UTC)}, {t4, t5},
	}
	out := mergeIntervals(in)
	if len(out) != 2 {
		t.Fatalf("expected 2 merged, got %d: %+v", len(out), out)
	}
	if !out[0].start.Equal(t1) || !out[0].end.Equal(time.Date(2026, 5, 10, 10, 30, 0, 0, time.UTC)) {
		t.Fatalf("first interval wrong: %+v", out[0])
	}
}

func TestResolveWindow(t *testing.T) {
	start, end, err := resolveWindow("today", "+7d", 7)
	if err != nil {
		t.Fatalf("resolveWindow: %v", err)
	}
	if !end.After(start) {
		t.Fatal("end must be after start")
	}
}

func TestResolveWindowRejectsBackwards(t *testing.T) {
	_, _, err := resolveWindow("+1d", "today", 7)
	if err == nil {
		t.Fatal("expected error when --to is before --from")
	}
}
