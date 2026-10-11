package cli

import (
	"testing"
	"time"
)

func TestSummarizeReservationsUsesRequestedDateNotHostClock(t *testing.T) {
	t.Parallel()
	day := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	stays := []reservation{
		{Unit: 2, Status: "BOOKED", CheckIn: time.Date(2026, 8, 30, 15, 0, 0, 0, time.UTC), CheckOut: time.Date(2026, 8, 31, 11, 0, 0, 0, time.UTC)},
		{Unit: 3, Status: "BOOKED", CheckIn: time.Date(2026, 8, 29, 15, 0, 0, 0, time.UTC), CheckOut: time.Date(2026, 8, 30, 11, 0, 0, 0, time.UTC)},
		{Unit: 4, Status: "CANCELLED", CheckIn: time.Date(2026, 8, 30, 15, 0, 0, 0, time.UTC), CheckOut: time.Date(2026, 8, 31, 11, 0, 0, 0, time.UTC)},
		{Unit: 10, Status: "BLOCKED", CheckIn: time.Date(2026, 8, 29, 15, 0, 0, 0, time.UTC), CheckOut: time.Date(2026, 8, 30, 11, 0, 0, 0, time.UTC)},
	}
	summary := summarizeReservations(stays, day)
	if summary.Date != "2026-08-30" {
		t.Fatalf("date = %q", summary.Date)
	}
	if len(summary.CheckIns) != 1 || summary.CheckIns[0].Unit != 2 {
		t.Fatalf("check-ins = %#v", summary.CheckIns)
	}
	if len(summary.CheckOuts) != 1 || summary.CheckOuts[0].Unit != 3 {
		t.Fatalf("check-outs = %#v", summary.CheckOuts)
	}
	if len(summary.Occupied) != 1 || summary.Occupied[0].Unit != 2 {
		t.Fatalf("occupied = %#v", summary.Occupied)
	}
}

func TestOperationalReservationAcceptsOnlyBookedGuestStays(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		status string
		want   bool
	}{
		{status: "BOOKED", want: true},
		{status: "booked", want: true},
		{status: "BLOCKED", want: false},
		{status: "CANCELLED", want: false},
		{status: "NEW", want: false},
		{status: "", want: false},
	} {
		if got := operationalReservation(reservation{Status: tc.status}); got != tc.want {
			t.Errorf("operationalReservation(status=%q) = %v, want %v", tc.status, got, tc.want)
		}
	}
}
