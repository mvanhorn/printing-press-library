// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.

package verdict

import (
	"testing"
	"time"
)

func TestChangeKind(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		status string
		want   string
	}{
		{"status cancelled classifies as cancelled", "cancelled", ChangeCancelled},
		{"confirmed classifies as new_or_updated", "confirmed", ChangeNewOrUpdated},
		{"tentative classifies as new_or_updated", "tentative", ChangeNewOrUpdated},
		{"empty status classifies as new_or_updated", "", ChangeNewOrUpdated},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := ChangeKind(tc.status); got != tc.want {
				t.Errorf("ChangeKind(%q) = %q, want %q", tc.status, got, tc.want)
			}
		})
	}
}

func TestClassifyException(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 8, 18, 16, 0, 0, 0, time.UTC)
	moved := time.Date(2026, 8, 18, 18, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		event    Event
		wantKind string
		wantIs   bool
	}{
		{
			"originalStartTime == start with status confirmed is NOT an exception",
			Event{RecurringEventID: "r1", Status: "confirmed", Start: start, End: start.Add(time.Hour), OriginalStart: &start},
			"", false,
		},
		{
			"originalStartTime != start is a moved exception",
			Event{RecurringEventID: "r1", Status: "confirmed", Start: moved, End: moved.Add(time.Hour), OriginalStart: &start},
			ExceptionMoved, true,
		},
		{
			"status cancelled with recurringEventId is cancelled_instance",
			Event{RecurringEventID: "r1", Status: "cancelled", OriginalStart: &start},
			ExceptionCancelledInstance, true,
		},
		{
			"no recurringEventId is never an exception even if cancelled",
			Event{Status: "cancelled", Start: start, End: start.Add(time.Hour)},
			"", false,
		},
		{
			"recurring instance without originalStartTime is not an exception",
			Event{RecurringEventID: "r1", Status: "confirmed", Start: start, End: start.Add(time.Hour)},
			"", false,
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			kind, is := ClassifyException(tc.event)
			if kind != tc.wantKind || is != tc.wantIs {
				t.Errorf("ClassifyException() = (%q, %v), want (%q, %v)", kind, is, tc.wantKind, tc.wantIs)
			}
		})
	}
}
