// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.

package verdict

import (
	"sort"
	"strings"
	"time"
)

// Overlaps reports whether the half-open intervals [aStart, aEnd) and
// [bStart, bEnd) intersect. Touching intervals (aEnd == bStart) do NOT
// overlap: back-to-back meetings are not a conflict.
func Overlaps(aStart, aEnd, bStart, bEnd time.Time) bool {
	return aStart.Before(bEnd) && bStart.Before(aEnd)
}

// IsMirrorPair reports whether two events look like the same real-world
// commitment mirrored across accounts: DIFFERENT accounts, equal start,
// equal end, case-insensitively equal summary. Two identical events on the
// SAME account are a duplicate, not a mirror — still conflict-eligible.
func IsMirrorPair(a, b Event) bool {
	return a.Account != b.Account &&
		a.Start.Equal(b.Start) &&
		a.End.Equal(b.End) &&
		strings.EqualFold(a.Summary, b.Summary)
}

// ConflictReport is the engine output for a conflicts run over one window.
type ConflictReport struct {
	Conflicts   []Conflict   `json:"conflicts"`
	Mirrors     []MirrorPair `json:"mirrors"`
	AllDayNotes []AllDayNote `json:"all_day_notes"`
}

// FindConflicts computes pairwise overlaps of busy events across every
// calendar handed in. Timed-vs-timed overlaps become Conflicts unless the
// pair is a suspected mirror (reported under Mirrors instead). All-day
// events are never overlap-checked against timed events; their interactions
// (all-day covering a timed event's instant, or two all-day events
// overlapping) are reported under AllDayNotes.
//
// Output slices are always non-nil (JSON emits [] rather than null) and
// deterministically ordered: events are sorted by (start, account, calendar,
// id) before pairing.
func FindConflicts(events []Event) ConflictReport {
	report := ConflictReport{
		Conflicts:   []Conflict{},
		Mirrors:     []MirrorPair{},
		AllDayNotes: []AllDayNote{},
	}

	var timed, allDay []Event
	for _, e := range events {
		if !IsBusy(e) {
			continue
		}
		if e.Start.IsZero() || e.End.IsZero() || !e.End.After(e.Start) {
			continue // no usable interval (cancelled stubs are already filtered by IsBusy)
		}
		if e.AllDay {
			allDay = append(allDay, e)
		} else {
			timed = append(timed, e)
		}
	}
	sortEvents(timed)
	sortEvents(allDay)

	for i := 0; i < len(timed); i++ {
		for j := i + 1; j < len(timed); j++ {
			a, b := timed[i], timed[j]
			if !Overlaps(a.Start, a.End, b.Start, b.End) {
				continue
			}
			if IsMirrorPair(a, b) {
				report.Mirrors = append(report.Mirrors, MirrorPair{A: a.Ref(), B: b.Ref()})
				continue
			}
			os, oe := overlapWindow(a, b)
			report.Conflicts = append(report.Conflicts, Conflict{
				A:            a.Ref(),
				B:            b.Ref(),
				OverlapStart: os.Format(time.RFC3339),
				OverlapEnd:   oe.Format(time.RFC3339),
			})
		}
	}

	// All-day vs timed: informational note when a busy timed event falls
	// inside a busy all-day event's UTC date range.
	for _, d := range allDay {
		for _, t := range timed {
			if !Overlaps(d.Start, d.End, t.Start, t.End) {
				continue
			}
			noteDate := maxTime(d.Start, t.Start).UTC().Format("2006-01-02")
			report.AllDayNotes = append(report.AllDayNotes, AllDayNote{
				Kind:   NoteAllDayVsTimed,
				AllDay: d.Ref(),
				Other:  t.Ref(),
				Date:   noteDate,
			})
		}
	}
	// All-day vs all-day: overlapping date ranges reported separately.
	for i := 0; i < len(allDay); i++ {
		for j := i + 1; j < len(allDay); j++ {
			a, b := allDay[i], allDay[j]
			if !Overlaps(a.Start, a.End, b.Start, b.End) {
				continue
			}
			report.AllDayNotes = append(report.AllDayNotes, AllDayNote{
				Kind:   NoteAllDayOverlap,
				AllDay: a.Ref(),
				Other:  b.Ref(),
				Date:   maxTime(a.Start, b.Start).UTC().Format("2006-01-02"),
			})
		}
	}
	return report
}

func overlapWindow(a, b Event) (time.Time, time.Time) {
	return maxTime(a.Start, b.Start), minTime(a.End, b.End)
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func sortEvents(evs []Event) {
	sort.SliceStable(evs, func(i, j int) bool {
		if !evs[i].Start.Equal(evs[j].Start) {
			return evs[i].Start.Before(evs[j].Start)
		}
		if evs[i].Account != evs[j].Account {
			return evs[i].Account < evs[j].Account
		}
		if evs[i].Calendar != evs[j].Calendar {
			return evs[i].Calendar < evs[j].Calendar
		}
		return evs[i].ID < evs[j].ID
	})
}
