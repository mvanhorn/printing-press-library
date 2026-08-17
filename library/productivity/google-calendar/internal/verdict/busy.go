// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.

package verdict

// IsBusy reports whether an event blocks time under the verdict contract:
// an event counts busy UNLESS
//
//   - transparency == "transparent" (the event is marked Free), or
//   - the account's own attendee entry declined it (self:true +
//     responseStatus "declined"), or
//   - status == "cancelled".
//
// TENTATIVE COUNTS AS BUSY — both status "tentative" and an undetermined
// self response ("tentative"/"needsAction") leave the event busy. An honest
// double-booking check must not silently un-book a slot the human has not
// declined.
func IsBusy(e Event) bool {
	if e.Status == "cancelled" {
		return false
	}
	if e.Transparency == "transparent" {
		return false
	}
	if e.SelfDeclined {
		return false
	}
	return true
}
