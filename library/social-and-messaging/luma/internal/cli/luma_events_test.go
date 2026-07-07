// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests (NOT generated) for the novel Luma command helpers.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestHaversineKm(t *testing.T) {
	tests := []struct {
		name                   string
		lat1, lng1, lat2, lng2 float64
		wantKm                 float64
		tolKm                  float64
	}{
		{"same point", 37.77, -122.42, 37.77, -122.42, 0, 0.01},
		{"sf to nyc", 37.7749, -122.4194, 40.7128, -74.0060, 4129, 50},
		{"short hop ~1km", 37.7749, -122.4194, 37.7839, -122.4194, 1.0, 0.2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := haversineKm(tc.lat1, tc.lng1, tc.lat2, tc.lng2)
			if got < tc.wantKm-tc.tolKm || got > tc.wantKm+tc.tolKm {
				t.Fatalf("haversineKm = %.2f, want %.2f ± %.2f", got, tc.wantKm, tc.tolKm)
			}
		})
	}
}

func TestWithinWindow(t *testing.T) {
	now := time.Date(2026, 6, 16, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name   string
		t      time.Time
		window time.Duration
		want   bool
	}{
		{"future inside 7d", now.Add(48 * time.Hour), 7 * 24 * time.Hour, true},
		{"future outside 7d", now.Add(10 * 24 * time.Hour), 7 * 24 * time.Hour, false},
		{"past event", now.Add(-48 * time.Hour), 7 * 24 * time.Hour, false},
		{"zero window keeps future", now.Add(100 * 24 * time.Hour), 0, true},
		{"zero window drops past", now.Add(-48 * time.Hour), 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := withinWindow(tc.t, now, tc.window); got != tc.want {
				t.Fatalf("withinWindow = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDedupeByID(t *testing.T) {
	mk := func(id string) lumaEntry { return lumaEntry{Event: lumaEventInner{APIID: id}} }
	in := []lumaEntry{mk("evt-a"), mk("evt-b"), mk("evt-a"), mk(""), mk("evt-c"), mk("evt-b")}
	out := dedupeByID(in)
	if len(out) != 3 {
		t.Fatalf("dedupeByID kept %d, want 3 (evt-a, evt-b, evt-c)", len(out))
	}
	if out[0].id() != "evt-a" || out[1].id() != "evt-b" || out[2].id() != "evt-c" {
		t.Fatalf("order not preserved: %v", []string{out[0].id(), out[1].id(), out[2].id()})
	}
}

func TestParseWindow(t *testing.T) {
	cases := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{"", 0, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"24h", 24 * time.Hour, false},
		{"nonsense", 0, true},
	}
	for _, tc := range cases {
		got, err := parseWindow(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("parseWindow(%q) expected error", tc.in)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Fatalf("parseWindow(%q) = %v, %v; want %v", tc.in, got, err, tc.want)
		}
	}
}

func TestBuildICS(t *testing.T) {
	raw := `{"api_id":"evt-1","event":{"api_id":"evt-1","name":"AI, Agents & Cafe","start_at":"2026-06-20T18:00:00.000Z","end_at":"2026-06-20T21:00:00.000Z","url":"ai-cafe","geo_address_info":{"full_address":"123 Market St, SF"}}}`
	var e lumaEntry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 6, 16, 0, 0, 0, 0, time.UTC)
	ics := buildICS([]lumaEntry{e}, stamp)
	for _, want := range []string{
		"BEGIN:VCALENDAR", "BEGIN:VEVENT", "UID:evt-1@luma.com",
		"DTSTART:20260620T180000Z", "DTEND:20260620T210000Z",
		"SUMMARY:AI\\, Agents & Cafe", // comma escaped per RFC5545
		"URL:https://luma.com/ai-cafe", "END:VEVENT", "END:VCALENDAR",
	} {
		if !strings.Contains(ics, want) {
			t.Fatalf("ICS missing %q\n---\n%s", want, ics)
		}
	}
}

func TestStartTimeAndView(t *testing.T) {
	raw := `{"api_id":"evt-9","event":{"api_id":"evt-9","name":"Summit","start_at":"2026-06-20T18:00:00.000Z","timezone":"America/Los_Angeles","url":"summit","coordinate":{"latitude":37.78,"longitude":-122.40},"geo_address_info":{"city":"San Francisco"}},"guest_count":42,"ticket_count":10}`
	var e lumaEntry
	if err := json.Unmarshal([]byte(raw), &e); err != nil {
		t.Fatal(err)
	}
	if _, ok := e.startTime(); !ok {
		t.Fatal("startTime should parse RFC3339 with millis")
	}
	v := e.view()
	if v.APIID != "evt-9" || v.GuestCount != 42 || v.Lat == 0 || v.City == "" {
		t.Fatalf("view incomplete: %+v", v)
	}
}

func TestRound1AndAbs(t *testing.T) {
	if round1(1.2345) != 1.2 {
		t.Fatalf("round1(1.2345)=%v want 1.2", round1(1.2345))
	}
	if round1(1.25) != 1.3 {
		t.Fatalf("round1(1.25)=%v want 1.3", round1(1.25))
	}
	if abs(-5) != 5 || abs(5) != 5 {
		t.Fatal("abs broken")
	}
}
