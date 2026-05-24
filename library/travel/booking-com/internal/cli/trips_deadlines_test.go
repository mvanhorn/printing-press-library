// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/booking-com/internal/booking"
)

func TestTripsWithinDeadlineWindowSkipsUnknownDeadlines(t *testing.T) {
	t.Parallel()

	trips := []booking.Trip{
		{ConfirmationNumber: "KNOWN", PropertyName: "Known"},
		{ConfirmationNumber: "UNKNOWN", PropertyName: "Unknown"},
	}
	deadlines := map[string]string{"KNOWN": "2026-06-05"}
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	got := tripsWithinDeadlineWindow(trips, deadlines, now, 14*24*time.Hour)
	if len(got) != 1 {
		t.Fatalf("tripsWithinDeadlineWindow returned %d trips, want 1: %+v", len(got), got)
	}
	if got[0].ConfirmationNumber != "KNOWN" {
		t.Fatalf("tripsWithinDeadlineWindow included wrong trip: %+v", got)
	}
}

func TestTripsWithinDeadlineWindowAppliesWithinFilter(t *testing.T) {
	t.Parallel()

	trips := []booking.Trip{
		{ConfirmationNumber: "SOON", PropertyName: "Soon"},
		{ConfirmationNumber: "LATE", PropertyName: "Late"},
	}
	deadlines := map[string]string{
		"SOON": "2026-06-05",
		"LATE": "2026-07-01",
	}
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	got := tripsWithinDeadlineWindow(trips, deadlines, now, 14*24*time.Hour)
	if len(got) != 1 {
		t.Fatalf("tripsWithinDeadlineWindow returned %d trips, want 1: %+v", len(got), got)
	}
	if got[0].ConfirmationNumber != "SOON" || got[0].FreeCancellationUntil != "2026-06-05" {
		t.Fatalf("tripsWithinDeadlineWindow returned wrong deadline row: %+v", got)
	}
}
