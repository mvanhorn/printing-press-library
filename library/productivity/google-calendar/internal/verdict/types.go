// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Package verdict is the pure engine behind the cross-account verdict
// commands (conflicts, slots, changes, events exceptions). It owns busy
// classification, overlap pairing, mirror detection, interval algebra, and
// the coverage contract. No HTTP: commands feed it parsed API payloads, and
// the unit tests feed it fixtures directly.
//
// Everything is normalized to UTC internally. All-day events are represented
// as [date 00:00Z, endDate 00:00Z) half-open ranges (Google's all-day end
// date is exclusive) and are never overlap-checked against timed events.
package verdict

import "time"

// Event is one calendar event, UTC-normalized, tagged with its origin.
type Event struct {
	Account          string
	Calendar         string
	ID               string
	Summary          string
	Start            time.Time
	End              time.Time
	AllDay           bool
	Transparency     string
	Status           string
	EventType        string
	SelfDeclined     bool
	RecurringEventID string
	OriginalStart    *time.Time
	Updated          time.Time
	Etag             string
}

// EventRef is the compact JSON projection of an Event carried in verdict
// outputs: enough to identify the event and re-fetch it, never the payload.
type EventRef struct {
	Account   string `json:"account"`
	Calendar  string `json:"calendar"`
	ID        string `json:"id"`
	Summary   string `json:"summary"`
	Start     string `json:"start"`
	End       string `json:"end"`
	EventType string `json:"event_type,omitempty"`
}

// Ref projects an Event into its output reference. Timed events format as
// RFC3339 UTC; all-day events format as date-only (their natural shape).
func (e Event) Ref() EventRef {
	layout := time.RFC3339
	if e.AllDay {
		layout = "2006-01-02"
	}
	ref := EventRef{
		Account:   e.Account,
		Calendar:  e.Calendar,
		ID:        e.ID,
		Summary:   e.Summary,
		EventType: e.EventType,
	}
	if !e.Start.IsZero() {
		ref.Start = e.Start.UTC().Format(layout)
	}
	if !e.End.IsZero() {
		ref.End = e.End.UTC().Format(layout)
	}
	return ref
}

// Conflict is one pairwise double-booking between busy timed events.
type Conflict struct {
	A            EventRef `json:"a"`
	B            EventRef `json:"b"`
	OverlapStart string   `json:"overlap_start"`
	OverlapEnd   string   `json:"overlap_end"`
}

// MirrorPair is two events from different accounts with equal start, equal
// end, and case-insensitively equal summary — almost certainly the same
// real-world commitment mirrored across calendars, so it is reported here
// instead of being double-counted as a conflict.
type MirrorPair struct {
	A EventRef `json:"a"`
	B EventRef `json:"b"`
}

// AllDayNote kinds.
const (
	// NoteAllDayVsTimed marks a busy all-day event with a busy timed event
	// falling inside its date range (UTC).
	NoteAllDayVsTimed = "all_day_vs_timed"
	// NoteAllDayOverlap marks two busy all-day events whose date ranges
	// overlap.
	NoteAllDayOverlap = "all_day_overlap"
)

// AllDayNote reports all-day interactions, which are informational and never
// counted as conflicts.
type AllDayNote struct {
	Kind   string   `json:"kind"`
	AllDay EventRef `json:"all_day"`
	Other  EventRef `json:"other"`
	Date   string   `json:"date"`
}

// Interval is a half-open [Start, End) time range in UTC.
type Interval struct {
	Start time.Time
	End   time.Time
}

// Duration returns End - Start.
func (iv Interval) Duration() time.Duration { return iv.End.Sub(iv.Start) }
