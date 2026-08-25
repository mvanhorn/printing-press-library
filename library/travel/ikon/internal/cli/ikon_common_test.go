// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"reflect"
	"strings"
	"testing"
)

func TestAggregateMostVisited(t *testing.T) {
	view := aggregateMostVisited([][]seasonUsage{
		{
			{
				DisplayName: "Jackson Hole, WY, Winter 2023-2024",
				UsedDays:    6,
				Redemptions: []redemption{{ResortName: "Jackson Hole, WY"}},
			},
			{
				DisplayName: "Winter Park, CO, Winter 2024-2025",
				UsedDays:    3,
				Redemptions: []redemption{{ResortName: "Winter Park, CO"}},
			},
		},
		{
			{
				DisplayName: "Jackson Hole, WY, Winter 2024-2025",
				UsedDays:    4,
				Redemptions: []redemption{{ResortName: "Jackson Hole, WY"}},
			},
		},
	}, 2, nil)

	if view.TotalDays != 13 {
		t.Fatalf("TotalDays = %d, want 13", view.TotalDays)
	}
	if view.SeasonsTracked != 2 {
		t.Fatalf("SeasonsTracked = %d, want 2", view.SeasonsTracked)
	}
	if got, want := view.Resorts[0].Resort, "Jackson Hole, WY"; got != want {
		t.Fatalf("top resort = %q, want %q", got, want)
	}
	if got, want := view.Resorts[0].TotalDays, 10; got != want {
		t.Fatalf("top resort days = %d, want %d", got, want)
	}
}

func TestIkonDataSurfacesStructuredErrors(t *testing.T) {
	_, err := ikonData([]byte(`{"data":null,"errors":[{"message":"Unauthorized","code":401}]}`))
	if err == nil {
		t.Fatal("ikonData returned nil error for structured API errors")
	}
	if got := err.Error(); !strings.Contains(got, `"Unauthorized"`) || !strings.Contains(got, `"code":401`) {
		t.Fatalf("error = %q, want compact structured payload", got)
	}
}

func TestBookableAndClassifyAvailabilityDate(t *testing.T) {
	passes := []passAvailability{
		{
			HasAccess:             true,
			ReservationsAvailable: 1,
			UnavailableDates:      []string{"2026-01-17"},
			BlackoutDates:         []string{"2026-01-18"},
			ClosedDates:           []string{"2026-01-19"},
			MaxReservationDate:    "2026-01-31",
		},
	}
	windows := accessiblePasses(passes)
	if !bookable(windows, "2026-01-16") {
		t.Fatal("2026-01-16 should be bookable")
	}
	if bookable(windows, "2026-01-17") {
		t.Fatal("2026-01-17 should not be bookable")
	}
	cases := map[string]string{
		"2026-01-16": "open",
		"2026-01-17": "full",
		"2026-01-18": "blackout",
		"2026-01-19": "closed",
		"2026-02-01": "unavailable",
	}
	for date, want := range cases {
		if got := classifyAvailabilityDate(passes, date); got != want {
			t.Fatalf("classifyAvailabilityDate(%s) = %q, want %q", date, got, want)
		}
	}
}

func TestClassifyAvailabilityDateIgnoresInaccessibleFallbackDates(t *testing.T) {
	passes := []passAvailability{
		{
			HasAccess:        false,
			BlackoutDates:    []string{"2026-01-17"},
			ClosedDates:      []string{"2026-01-18"},
			UnavailableDates: []string{"2026-01-19"},
		},
		{
			HasAccess:             true,
			ReservationsAvailable: 0,
			UnavailableDates:      []string{"2026-01-17"},
		},
	}
	if got, want := classifyAvailabilityDate(passes, "2026-01-17"), "full"; got != want {
		t.Fatalf("classifyAvailabilityDate = %q, want %q", got, want)
	}
}

func TestValidateCDPWebSocketURLRequiresLoopbackSamePort(t *testing.T) {
	cases := []struct {
		name string
		url  string
		ok   bool
	}{
		{"localhost", "ws://localhost:9222/devtools/page/1", true},
		{"ipv4 loopback", "ws://127.0.0.1:9222/devtools/page/1", true},
		{"ipv6 loopback", "ws://[::1]:9222/devtools/page/1", true},
		{"wrong port", "ws://localhost:9223/devtools/page/1", false},
		{"remote host", "ws://example.com:9222/devtools/page/1", false},
		{"bad scheme", "http://localhost:9222/devtools/page/1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCDPWebSocketURL(tc.url, "9222")
			if tc.ok && err != nil {
				t.Fatalf("validateCDPWebSocketURL returned error: %v", err)
			}
			if !tc.ok && err == nil {
				t.Fatal("validateCDPWebSocketURL returned nil error for unsafe URL")
			}
		})
	}
}

func TestDiffFullDates(t *testing.T) {
	opened, closed := diffFullDates(
		[]string{"2026-01-17", "2026-01-18"},
		[]string{"2026-01-18", "2026-01-19"},
	)
	if !reflect.DeepEqual(opened, []string{"2026-01-17"}) {
		t.Fatalf("opened = %#v", opened)
	}
	if !reflect.DeepEqual(closed, []string{"2026-01-19"}) {
		t.Fatalf("closed = %#v", closed)
	}
}
