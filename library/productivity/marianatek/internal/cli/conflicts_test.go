// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseICSDateHonorsTZID(t *testing.T) {
	got := parseICSDate("DTSTART;TZID=America/New_York:20260515T090000")
	if got.IsZero() {
		t.Fatal("parseICSDate returned zero time")
	}
	if got.Location().String() != "America/New_York" {
		t.Fatalf("location = %q, want America/New_York", got.Location())
	}
	if utc := got.UTC().Format(time.RFC3339); utc != "2026-05-15T13:00:00Z" {
		t.Fatalf("UTC instant = %s, want 2026-05-15T13:00:00Z", utc)
	}
}

func TestParseICSDateFallsBackToUTCForUnknownTZID(t *testing.T) {
	got := parseICSDate("DTSTART;TZID=Eastern Standard Time:20260515T090000")
	if got.IsZero() {
		t.Fatal("parseICSDate returned zero time")
	}
	if got.Location() != time.UTC {
		t.Fatalf("location = %q, want UTC", got.Location())
	}
	if utc := got.UTC().Format(time.RFC3339); utc != "2026-05-15T09:00:00Z" {
		t.Fatalf("UTC instant = %s, want 2026-05-15T09:00:00Z", utc)
	}
}

func TestParseICSEventsMatchesTZIDLocalDate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "calendar.ics")
	ics := `BEGIN:VCALENDAR
BEGIN:VEVENT
SUMMARY:Late class
DTSTART;TZID=America/Los_Angeles:20260515T233000
DTEND;TZID=America/Los_Angeles:20260516T003000
END:VEVENT
END:VCALENDAR
`
	if err := os.WriteFile(path, []byte(ics), 0o600); err != nil {
		t.Fatalf("write ics: %v", err)
	}
	date, err := time.Parse("2006-01-02", "2026-05-15")
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	events, err := parseICSEvents(path, date)
	if err != nil {
		t.Fatalf("parseICSEvents returned error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].Title != "Late class" {
		t.Fatalf("title = %q, want Late class", events[0].Title)
	}
}
