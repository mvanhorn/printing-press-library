// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package gflights

import (
	"encoding/json"
	"net/url"
	"testing"
	"time"
)

func TestBuildDatesPayloadRoundTripRequiresDuration(t *testing.T) {
	opts := DatesOptions{Origin: "LHR", Destination: "BCN", RoundTrip: true}
	from, _ := time.Parse("2006-01-02", "2027-03-01")
	to, _ := time.Parse("2006-01-02", "2027-03-27")
	if _, err := buildDatesPayload(opts, from, to); err == nil {
		t.Fatal("expected error for --round without --duration")
	}
}

// TestBuildDatesPayloadRoundTripShape locks in the payload shape against a
// live captured GetCalendarGraph round-trip request (2026-08-31): two
// segments (second origin/dest swapped, travel_date = anchor + duration),
// trip_type=1, and a trailing [min,max]-nights duration filter.
func TestBuildDatesPayloadRoundTripShape(t *testing.T) {
	opts := DatesOptions{Origin: "LHR", Destination: "BCN", RoundTrip: true, Duration: 2}
	from, _ := time.Parse("2006-01-02", "2027-03-01")
	to, _ := time.Parse("2006-01-02", "2027-03-27")

	payload, err := buildDatesPayload(opts, from, to)
	if err != nil {
		t.Fatalf("buildDatesPayload: %v", err)
	}
	decoded, err := url.QueryUnescape(payload)
	if err != nil {
		t.Fatalf("QueryUnescape: %v", err)
	}

	var wrapped []any
	if err := json.Unmarshal([]byte(decoded), &wrapped); err != nil {
		t.Fatalf("unmarshal wrapper: %v", err)
	}
	if len(wrapped) != 2 {
		t.Fatalf("wrapper: want 2 elements, got %d", len(wrapped))
	}
	var filters []any
	if err := json.Unmarshal([]byte(wrapped[1].(string)), &filters); err != nil {
		t.Fatalf("unmarshal filters: %v", err)
	}
	if len(filters) != 5 {
		t.Fatalf("filters: want 5 elements (outer shape), got %d: %v", len(filters), filters)
	}
	if filters[3] != nil {
		t.Errorf("filters[3]: want nil, got %v", filters[3])
	}
	duration, ok := filters[4].([]any)
	if !ok || len(duration) != 2 || duration[0] != float64(2) || duration[1] != float64(2) {
		t.Errorf("filters[4] duration: want [2,2], got %v", filters[4])
	}

	inner, ok := filters[1].([]any)
	if !ok || len(inner) != 18 {
		t.Fatalf("filters[1]: want 18-element inner filter array, got %v", filters[1])
	}
	if tripType := inner[2]; tripType != float64(tripTypeRoundTrip) {
		t.Errorf("trip type: want %d, got %v", tripTypeRoundTrip, tripType)
	}
	segments, ok := inner[13].([]any)
	if !ok || len(segments) != 2 {
		t.Fatalf("segments: want 2 segments for round trip, got %v", inner[13])
	}
	outbound := segments[0].([]any)
	inbound := segments[1].([]any)
	if outbound[6] != "2027-03-01" {
		t.Errorf("outbound travel_date: want 2027-03-01, got %v", outbound[6])
	}
	if inbound[6] != "2027-03-03" {
		t.Errorf("inbound travel_date: want anchor+duration 2027-03-03, got %v", inbound[6])
	}
}

func TestBuildDatesPayloadOneWayUnaffected(t *testing.T) {
	opts := DatesOptions{Origin: "LHR", Destination: "BCN"}
	from, _ := time.Parse("2006-01-02", "2027-03-01")
	to, _ := time.Parse("2006-01-02", "2027-03-27")

	payload, err := buildDatesPayload(opts, from, to)
	if err != nil {
		t.Fatalf("buildDatesPayload: %v", err)
	}
	decoded, _ := url.QueryUnescape(payload)
	var wrapped []any
	if err := json.Unmarshal([]byte(decoded), &wrapped); err != nil {
		t.Fatalf("unmarshal wrapper: %v", err)
	}
	var filters []any
	if err := json.Unmarshal([]byte(wrapped[1].(string)), &filters); err != nil {
		t.Fatalf("unmarshal filters: %v", err)
	}
	if filters[4] != nil {
		t.Errorf("one-way filters[4]: want nil, got %v", filters[4])
	}
	inner := filters[1].([]any)
	if tripType := inner[2]; tripType != float64(tripTypeOneWay) {
		t.Errorf("trip type: want %d, got %v", tripTypeOneWay, tripType)
	}
	segments := inner[13].([]any)
	if len(segments) != 1 {
		t.Errorf("segments: want 1 for one-way, got %d", len(segments))
	}
}
