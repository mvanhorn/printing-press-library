// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.

package verdict

import (
	"encoding/json"
	"testing"
	"time"
)

func TestParseEventTimedNormalizesToUTC(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{
		"id": "evt1",
		"etag": "\"3390000000000000\"",
		"status": "confirmed",
		"summary": "Standup",
		"eventType": "default",
		"updated": "2026-08-16T12:30:00.123Z",
		"start": {"dateTime": "2026-08-18T10:00:00-06:00"},
		"end":   {"dateTime": "2026-08-18T10:30:00-06:00"}
	}`)
	ev, err := ParseEvent("personal", "cal1", raw)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	wantStart := time.Date(2026, 8, 18, 16, 0, 0, 0, time.UTC)
	if !ev.Start.Equal(wantStart) {
		t.Errorf("start = %v, want %v (UTC-normalized)", ev.Start, wantStart)
	}
	if ev.Start.Location() != time.UTC {
		t.Errorf("start location = %v, want UTC", ev.Start.Location())
	}
	if ev.AllDay {
		t.Errorf("dateTime event must not be all-day")
	}
	if ev.Etag == "" || ev.EventType != "default" || ev.Account != "personal" || ev.Calendar != "cal1" {
		t.Errorf("metadata not carried: %+v", ev)
	}
	if ev.Updated.IsZero() {
		t.Errorf("updated must parse RFC3339 with fractional seconds")
	}
}

func TestParseEventAllDay(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{
		"id": "evt2",
		"status": "confirmed",
		"summary": "Travel day",
		"start": {"date": "2026-08-18"},
		"end":   {"date": "2026-08-19"}
	}`)
	ev, err := ParseEvent("personal", "cal1", raw)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if !ev.AllDay {
		t.Fatalf("date-only start must mark all-day")
	}
	if !ev.Start.Equal(time.Date(2026, 8, 18, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("all-day start = %v, want 2026-08-18T00:00:00Z", ev.Start)
	}
	if !ev.End.Equal(time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("all-day end = %v, want exclusive 2026-08-19T00:00:00Z", ev.End)
	}
	if ref := ev.Ref(); ref.Start != "2026-08-18" || ref.End != "2026-08-19" {
		t.Errorf("all-day ref should be date-only, got %+v", ref)
	}
}

func TestParseEventSelfDeclined(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{
		"id": "evt3",
		"status": "confirmed",
		"summary": "Optional sync",
		"start": {"dateTime": "2026-08-18T10:00:00Z"},
		"end":   {"dateTime": "2026-08-18T11:00:00Z"},
		"attendees": [
			{"email": "other@example.com", "responseStatus": "accepted"},
			{"email": "me@example.com", "self": true, "responseStatus": "declined"}
		]
	}`)
	ev, err := ParseEvent("personal", "cal1", raw)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if !ev.SelfDeclined {
		t.Fatalf("self:true + responseStatus declined must set SelfDeclined")
	}
	if IsBusy(ev) {
		t.Errorf("self-declined event must not be busy")
	}
}

func TestParseEventSelfTentativeStaysBusy(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{
		"id": "evt4",
		"status": "tentative",
		"summary": "Maybe",
		"start": {"dateTime": "2026-08-18T10:00:00Z"},
		"end":   {"dateTime": "2026-08-18T11:00:00Z"},
		"attendees": [{"email": "me@example.com", "self": true, "responseStatus": "tentative"}]
	}`)
	ev, err := ParseEvent("personal", "cal1", raw)
	if err != nil {
		t.Fatalf("ParseEvent: %v", err)
	}
	if ev.SelfDeclined {
		t.Errorf("tentative self response is not a decline")
	}
	if !IsBusy(ev) {
		t.Errorf("tentative must count as busy")
	}
}

// TestParseEventCancelledInstanceStub covers the stripped-down shape Google
// returns for cancelled recurring-event instances under showDeleted=true:
// no summary, no start/end, but recurringEventId + originalStartTime.
func TestParseEventCancelledInstanceStub(t *testing.T) {
	t.Parallel()
	raw := json.RawMessage(`{
		"id": "r1_20260818T160000Z",
		"status": "cancelled",
		"recurringEventId": "r1",
		"originalStartTime": {"dateTime": "2026-08-18T16:00:00Z"}
	}`)
	ev, err := ParseEvent("work", "cal2", raw)
	if err != nil {
		t.Fatalf("cancelled stub must parse without error: %v", err)
	}
	if ev.OriginalStart == nil || !ev.OriginalStart.Equal(time.Date(2026, 8, 18, 16, 0, 0, 0, time.UTC)) {
		t.Errorf("originalStartTime not parsed: %+v", ev.OriginalStart)
	}
	kind, is := ClassifyException(ev)
	if !is || kind != ExceptionCancelledInstance {
		t.Errorf("ClassifyException = (%q, %v), want (%q, true)", kind, is, ExceptionCancelledInstance)
	}
	if IsBusy(ev) {
		t.Errorf("cancelled stub must not be busy")
	}
}

func TestParseEventInvalidJSONErrors(t *testing.T) {
	t.Parallel()
	if _, err := ParseEvent("a", "c", json.RawMessage(`{"start": {"dateTime": "not-a-time"}}`)); err == nil {
		t.Errorf("unparseable dateTime must error, not silently zero")
	}
	if _, err := ParseEvent("a", "c", json.RawMessage(`not json`)); err == nil {
		t.Errorf("malformed JSON must error")
	}
}
