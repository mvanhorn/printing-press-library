// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.

package verdict

// Change kinds for the changes command.
const (
	ChangeCancelled    = "cancelled"
	ChangeNewOrUpdated = "new_or_updated"
)

// ChangeKind classifies one changed event: a cancelled status is a
// cancellation (deletions surface with status "cancelled" under
// showDeleted=true); everything else is new-or-updated. The API's updatedMin
// filter cannot distinguish create from edit, so the CLI does not pretend to.
func ChangeKind(status string) string {
	if status == "cancelled" {
		return ChangeCancelled
	}
	return ChangeNewOrUpdated
}

// Exception kinds for the events exceptions command.
const (
	ExceptionMoved             = "moved"
	ExceptionCancelledInstance = "cancelled_instance"
)

// ClassifyException reports whether a single-instance event deviates from
// its recurring series, and how:
//
//   - status "cancelled" with a recurringEventId → "cancelled_instance"
//   - originalStartTime present and != start → "moved"
//   - originalStartTime == start (instance sitting where the rule put it),
//     or no recurringEventId at all → not an exception.
func ClassifyException(e Event) (kind string, isException bool) {
	if e.RecurringEventID == "" {
		return "", false
	}
	if e.Status == "cancelled" {
		return ExceptionCancelledInstance, true
	}
	if e.OriginalStart != nil && !e.Start.IsZero() && !e.OriginalStart.Equal(e.Start) {
		return ExceptionMoved, true
	}
	return "", false
}
