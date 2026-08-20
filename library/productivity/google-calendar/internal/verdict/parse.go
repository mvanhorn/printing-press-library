// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.

package verdict

import (
	"encoding/json"
	"fmt"
	"time"
)

// apiEventTime mirrors the Google Calendar API's start/end/originalStartTime
// shape: dateTime (RFC3339 with offset) for timed events, date (YYYY-MM-DD)
// for all-day events.
type apiEventTime struct {
	Date     string `json:"date"`
	DateTime string `json:"dateTime"`
	TimeZone string `json:"timeZone"`
}

type apiAttendee struct {
	Self           bool   `json:"self"`
	ResponseStatus string `json:"responseStatus"`
}

type apiEvent struct {
	ID                string        `json:"id"`
	Etag              string        `json:"etag"`
	Status            string        `json:"status"`
	Summary           string        `json:"summary"`
	Transparency      string        `json:"transparency"`
	EventType         string        `json:"eventType"`
	RecurringEventID  string        `json:"recurringEventId"`
	Updated           string        `json:"updated"`
	Start             *apiEventTime `json:"start"`
	End               *apiEventTime `json:"end"`
	OriginalStartTime *apiEventTime `json:"originalStartTime"`
	Attendees         []apiAttendee `json:"attendees"`
}

// parseAPITime converts an apiEventTime into a UTC instant. All-day values
// (date-only) anchor at UTC midnight; timed values keep their instant and are
// re-expressed in UTC. Returns ok=false for an absent/empty value — cancelled
// recurring-event stubs legitimately omit start/end.
func parseAPITime(t *apiEventTime) (instant time.Time, allDay bool, ok bool, err error) {
	if t == nil {
		return time.Time{}, false, false, nil
	}
	switch {
	case t.DateTime != "":
		parsed, perr := time.Parse(time.RFC3339, t.DateTime)
		if perr != nil {
			return time.Time{}, false, false, fmt.Errorf("parsing dateTime %q: %w", t.DateTime, perr)
		}
		return parsed.UTC(), false, true, nil
	case t.Date != "":
		parsed, perr := time.Parse("2006-01-02", t.Date)
		if perr != nil {
			return time.Time{}, false, false, fmt.Errorf("parsing date %q: %w", t.Date, perr)
		}
		return parsed.UTC(), true, true, nil
	default:
		return time.Time{}, false, false, nil
	}
}

// ParseEvent converts one Google Calendar API event JSON object into a typed,
// UTC-normalized Event tagged with its (account, calendar) origin. Missing
// start/end are tolerated (cancelled instance stubs); malformed JSON or an
// unparseable timestamp is an error.
func ParseEvent(account, calendar string, raw json.RawMessage) (Event, error) {
	var ae apiEvent
	if err := json.Unmarshal(raw, &ae); err != nil {
		return Event{}, fmt.Errorf("parsing event JSON (account %s, calendar %s): %w", account, calendar, err)
	}
	ev := Event{
		Account:          account,
		Calendar:         calendar,
		ID:               ae.ID,
		Etag:             ae.Etag,
		Status:           ae.Status,
		Summary:          ae.Summary,
		Transparency:     ae.Transparency,
		EventType:        ae.EventType,
		RecurringEventID: ae.RecurringEventID,
	}
	start, allDay, ok, err := parseAPITime(ae.Start)
	if err != nil {
		return Event{}, fmt.Errorf("event %s: %w", ae.ID, err)
	}
	if ok {
		ev.Start = start
		ev.AllDay = allDay
	}
	end, _, ok, err := parseAPITime(ae.End)
	if err != nil {
		return Event{}, fmt.Errorf("event %s: %w", ae.ID, err)
	}
	if ok {
		ev.End = end
	}
	orig, _, ok, err := parseAPITime(ae.OriginalStartTime)
	if err != nil {
		return Event{}, fmt.Errorf("event %s: %w", ae.ID, err)
	}
	if ok {
		ev.OriginalStart = &orig
	}
	if ae.Updated != "" {
		if u, uerr := time.Parse(time.RFC3339, ae.Updated); uerr == nil {
			ev.Updated = u.UTC()
		}
	}
	for _, a := range ae.Attendees {
		if a.Self && a.ResponseStatus == "declined" {
			ev.SelfDeclined = true
			break
		}
	}
	return ev, nil
}
