package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestBuildAvailabilityResultUsesOnlyBookedStays(t *testing.T) {
	t.Parallel()
	location, err := time.LoadLocation("America/Los_Angeles")
	if err != nil {
		t.Fatal(err)
	}
	today := time.Date(2026, 8, 31, 0, 0, 0, 0, location)
	reservations := []reservation{
		{
			Unit: 4, Status: "BOOKED", GuestLast: "Pitts",
			CheckIn: today.AddDate(0, 0, 1), CheckOut: today.AddDate(0, 0, 3),
		},
		{
			Unit: 4, Status: "CANCELLED", GuestLast: "Cancelled",
			CheckIn: today, CheckOut: today.AddDate(0, 0, 3),
		},
	}

	got := buildAvailabilityResult(reservations, today, []int{4}, 4)
	if got.Period != "Aug 31 – Sep 3, 2026" {
		t.Fatalf("period = %q", got.Period)
	}
	wantStatuses := []string{"VACANT", "OCCUPIED", "OCCUPIED", "VACANT"}
	for i, want := range wantStatuses {
		if got.Units[0].Days[i].Status != want {
			t.Fatalf("day %d status = %q, want %q", i, got.Units[0].Days[i].Status, want)
		}
	}
}

func TestAvailabilityAlwaysUsesLiveLoaderWithoutOpeningStore(t *testing.T) {
	flags := &rootFlags{dataSource: "auto", agent: true}
	called := false
	loader := func(_ *rootFlags, units []int) ([]reservation, error) {
		called = true
		if len(units) != 1 || units[0] != 4 {
			t.Fatalf("units = %#v", units)
		}
		return []reservation{}, nil
	}
	cmd := newAvailabilityCmdWithLoader(flags, loader)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--unit", "4", "--days", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("live loader was not called")
	}
	if !strings.Contains(stdout.String(), `"unit": 4`) {
		t.Fatalf("output = %s", stdout.String())
	}
}

func TestAvailabilityRejectsLocalDataSource(t *testing.T) {
	flags := &rootFlags{dataSource: "local", agent: true}
	called := false
	cmd := newAvailabilityCmdWithLoader(flags, func(_ *rootFlags, _ []int) ([]reservation, error) {
		called = true
		return nil, nil
	})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "requires live Hostfully data") {
		t.Fatalf("error = %v", err)
	}
	if called {
		t.Fatal("loader should not run for rejected local availability request")
	}
}

func TestAvailabilityDoesNotFallBackWhenLiveReadFails(t *testing.T) {
	flags := &rootFlags{dataSource: "auto", agent: true}
	cmd := newAvailabilityCmdWithLoader(flags, func(_ *rootFlags, _ []int) ([]reservation, error) {
		return nil, errors.New("Hostfully unavailable")
	})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "loading live Hostfully availability") {
		t.Fatalf("error = %v", err)
	}
}
