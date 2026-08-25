// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.

package verdict

import (
	"testing"
	"time"
)

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return parsed
}

func TestOverlaps(t *testing.T) {
	t.Parallel()
	d := func(h, m int) time.Time { return time.Date(2026, 8, 18, h, m, 0, 0, time.UTC) }

	cases := []struct {
		name                       string
		aStart, aEnd, bStart, bEnd time.Time
		want                       bool
	}{
		{"touching intervals (end==start) do NOT conflict", d(10, 0), d(11, 0), d(11, 0), d(12, 0), false},
		{"touching intervals reversed order do NOT conflict", d(11, 0), d(12, 0), d(10, 0), d(11, 0), false},
		{"containment conflicts", d(10, 0), d(12, 0), d(10, 30), d(11, 0), true},
		{"partial overlap conflicts", d(10, 0), d(11, 0), d(10, 30), d(11, 30), true},
		{"disjoint does not conflict", d(9, 0), d(10, 0), d(13, 0), d(14, 0), false},
		{"identical intervals conflict", d(10, 0), d(11, 0), d(10, 0), d(11, 0), true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Overlaps(tc.aStart, tc.aEnd, tc.bStart, tc.bEnd); got != tc.want {
				t.Errorf("Overlaps(%v,%v,%v,%v) = %v, want %v", tc.aStart, tc.aEnd, tc.bStart, tc.bEnd, got, tc.want)
			}
		})
	}
}

// TestOverlapsCrossTimezoneSameInstant asserts that offset-differing but
// instant-equal RFC3339 stamps conflict: 10:00-06:00 IS 16:00Z.
func TestOverlapsCrossTimezoneSameInstant(t *testing.T) {
	t.Parallel()
	aStart := mustParse(t, "2026-08-18T10:00:00-06:00")
	aEnd := mustParse(t, "2026-08-18T11:00:00-06:00")
	bStart := mustParse(t, "2026-08-18T16:00:00Z")
	bEnd := mustParse(t, "2026-08-18T17:00:00Z")
	if !aStart.Equal(bStart) {
		t.Fatalf("test premise broken: %v and %v should be the same instant", aStart, bStart)
	}
	if !Overlaps(aStart.UTC(), aEnd.UTC(), bStart, bEnd) {
		t.Errorf("cross-timezone same-instant events must conflict")
	}
}

func TestFindConflictsMirrorsAndPairs(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 8, 18, 16, 0, 0, 0, time.UTC)
	end := time.Date(2026, 8, 18, 17, 0, 0, 0, time.UTC)

	t.Run("same start+end+summary across accounts is a mirror, excluded from conflicts", func(t *testing.T) {
		t.Parallel()
		a := timedEvent("personal", "a1", start, end)
		a.Summary = "Dentist"
		b := timedEvent("work", "b1", start, end)
		b.Summary = "dentist" // case-insensitive match
		rep := FindConflicts([]Event{a, b})
		if len(rep.Conflicts) != 0 {
			t.Errorf("mirror pair must not appear in conflicts, got %+v", rep.Conflicts)
		}
		if len(rep.Mirrors) != 1 {
			t.Fatalf("want 1 mirror pair, got %d", len(rep.Mirrors))
		}
		if rep.Mirrors[0].A.Account == rep.Mirrors[0].B.Account {
			t.Errorf("mirror pair must span accounts, got %+v", rep.Mirrors[0])
		}
	})

	t.Run("same times different summaries is a real conflict", func(t *testing.T) {
		t.Parallel()
		a := timedEvent("personal", "a1", start, end)
		a.Summary = "Dentist"
		b := timedEvent("work", "b1", start, end)
		b.Summary = "Sprint review"
		rep := FindConflicts([]Event{a, b})
		if len(rep.Mirrors) != 0 {
			t.Errorf("different summaries must not be a mirror, got %+v", rep.Mirrors)
		}
		if len(rep.Conflicts) != 1 {
			t.Fatalf("want 1 conflict, got %d", len(rep.Conflicts))
		}
		c := rep.Conflicts[0]
		if c.OverlapStart != "2026-08-18T16:00:00Z" || c.OverlapEnd != "2026-08-18T17:00:00Z" {
			t.Errorf("overlap window = [%s, %s], want [2026-08-18T16:00:00Z, 2026-08-18T17:00:00Z]", c.OverlapStart, c.OverlapEnd)
		}
	})

	t.Run("same summary+times SAME account is a duplicate, still a conflict", func(t *testing.T) {
		t.Parallel()
		a := timedEvent("personal", "a1", start, end)
		a.Summary = "Dentist"
		b := timedEvent("personal", "a2", start, end)
		b.Summary = "Dentist"
		rep := FindConflicts([]Event{a, b})
		if len(rep.Mirrors) != 0 {
			t.Errorf("same-account identical pair must not be a mirror, got %+v", rep.Mirrors)
		}
		if len(rep.Conflicts) != 1 {
			t.Errorf("same-account identical pair must be conflict-eligible, got %d conflicts", len(rep.Conflicts))
		}
	})

	t.Run("overlapping pair with one transparent is NOT in conflicts", func(t *testing.T) {
		t.Parallel()
		a := timedEvent("personal", "a1", start, end)
		b := timedEvent("work", "b1", start.Add(15*time.Minute), end.Add(15*time.Minute))
		b.Transparency = "transparent"
		rep := FindConflicts([]Event{a, b})
		if len(rep.Conflicts) != 0 {
			t.Errorf("transparent participant must suppress the conflict, got %+v", rep.Conflicts)
		}
	})

	t.Run("zero events yields empty non-nil arrays", func(t *testing.T) {
		t.Parallel()
		rep := FindConflicts(nil)
		if rep.Conflicts == nil || rep.Mirrors == nil || rep.AllDayNotes == nil {
			t.Fatalf("report slices must be non-nil for JSON []: %+v", rep)
		}
		if len(rep.Conflicts) != 0 || len(rep.Mirrors) != 0 || len(rep.AllDayNotes) != 0 {
			t.Errorf("zero events must yield zero findings, got %+v", rep)
		}
	})

	t.Run("cross-timezone same-instant events from different calendars conflict", func(t *testing.T) {
		t.Parallel()
		// Parsed the way the fetch layer does: normalized to UTC.
		aStart := mustParse(t, "2026-08-18T10:00:00-06:00").UTC()
		aEnd := mustParse(t, "2026-08-18T11:00:00-06:00").UTC()
		bStart := mustParse(t, "2026-08-18T16:00:00Z").UTC()
		bEnd := mustParse(t, "2026-08-18T17:00:00Z").UTC()
		a := timedEvent("personal", "a1", aStart, aEnd)
		a.Summary = "Standup"
		b := timedEvent("work", "b1", bStart, bEnd)
		b.Summary = "Client call"
		rep := FindConflicts([]Event{a, b})
		if len(rep.Conflicts) != 1 {
			t.Fatalf("want 1 conflict for equal instants across offsets, got %d", len(rep.Conflicts))
		}
	})
}

func TestFindConflictsAllDay(t *testing.T) {
	t.Parallel()
	allDay := Event{
		Account:  "personal",
		Calendar: "personal-cal",
		ID:       "ad1",
		Summary:  "Conference travel",
		Status:   "confirmed",
		AllDay:   true,
		Start:    time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
		End:      time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC),
	}
	timed := timedEvent("work", "t1",
		time.Date(2026, 8, 18, 15, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 18, 16, 0, 0, 0, time.UTC))

	t.Run("all-day vs timed produces a note, never a conflict", func(t *testing.T) {
		t.Parallel()
		rep := FindConflicts([]Event{allDay, timed})
		if len(rep.Conflicts) != 0 {
			t.Errorf("all-day vs timed must not be a conflict, got %+v", rep.Conflicts)
		}
		if len(rep.AllDayNotes) != 1 {
			t.Fatalf("want 1 all_day_note, got %d", len(rep.AllDayNotes))
		}
		note := rep.AllDayNotes[0]
		if note.Kind != NoteAllDayVsTimed {
			t.Errorf("note kind = %q, want %q", note.Kind, NoteAllDayVsTimed)
		}
		if note.Date != "2026-08-18" {
			t.Errorf("note date = %q, want 2026-08-18", note.Date)
		}
		if note.AllDay.ID != "ad1" || note.Other.ID != "t1" {
			t.Errorf("note refs wrong: %+v", note)
		}
	})

	t.Run("timed event on a different date produces no note", func(t *testing.T) {
		t.Parallel()
		other := timedEvent("work", "t2",
			time.Date(2026, 8, 20, 15, 0, 0, 0, time.UTC),
			time.Date(2026, 8, 20, 16, 0, 0, 0, time.UTC))
		rep := FindConflicts([]Event{allDay, other})
		if len(rep.AllDayNotes) != 0 {
			t.Errorf("no note expected for non-overlapping date, got %+v", rep.AllDayNotes)
		}
	})

	t.Run("overlapping all-day events report separately as notes", func(t *testing.T) {
		t.Parallel()
		other := Event{
			Account:  "work",
			Calendar: "work-cal",
			ID:       "ad2",
			Summary:  "Offsite",
			Status:   "confirmed",
			AllDay:   true,
			Start:    time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC),
			End:      time.Date(2026, 8, 20, 0, 0, 0, 0, time.UTC),
		}
		rep := FindConflicts([]Event{allDay, other})
		if len(rep.Conflicts) != 0 {
			t.Errorf("all-day overlap must not be a conflict, got %+v", rep.Conflicts)
		}
		found := false
		for _, n := range rep.AllDayNotes {
			if n.Kind == NoteAllDayOverlap {
				found = true
			}
		}
		if !found {
			t.Errorf("want an %s note, got %+v", NoteAllDayOverlap, rep.AllDayNotes)
		}
	})
}
