// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Tests for the hand-authored Ride with GPS novel-feature helpers.
package cli

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestMetersConversions(t *testing.T) {
	if got := metersToKM(1000); got != 1 {
		t.Errorf("metersToKM(1000) = %v, want 1", got)
	}
	if got := metersToMiles(1609.344); math.Abs(got-1) > 1e-9 {
		t.Errorf("metersToMiles(1609.344) = %v, want ~1", got)
	}
}

func TestHaversineMeters(t *testing.T) {
	// ~1 degree of latitude is ~111 km.
	d := haversineMeters(0, 0, 1, 0)
	if d < 110000 || d > 112000 {
		t.Errorf("haversineMeters(0,0,1,0) = %v, want ~111000", d)
	}
	// Same point is zero.
	if d := haversineMeters(40.7, -74.0, 40.7, -74.0); d != 0 {
		t.Errorf("haversineMeters(same) = %v, want 0", d)
	}
}

func TestSecondsToHMS(t *testing.T) {
	cases := map[float64]string{0: "0:00:00", 3661: "1:01:01", 90: "0:01:30"}
	for in, want := range cases {
		if got := secondsToHMS(in); got != want {
			t.Errorf("secondsToHMS(%v) = %q, want %q", in, got, want)
		}
	}
}

func f64p(v float64) *float64 { return &v }

func TestBuildGPX(t *testing.T) {
	tps := []trackPoint{
		{X: f64p(-81.29262), Y: f64p(29.0363), E: f64p(26.1)},
		{X: f64p(-81.29231), Y: f64p(29.03737), E: f64p(27.0)},
	}
	cps := []coursePoint{{X: f64p(-81.29231), Y: f64p(29.03737), T: "Right", N: "Turn right onto E University Ave"}}
	gpx := buildGPX("Test & Ride", tps, cps)
	if !strings.Contains(gpx, `<gpx`) || !strings.Contains(gpx, `lat="29.0363"`) {
		t.Errorf("GPX missing track point lat: %s", gpx[:120])
	}
	if !strings.Contains(gpx, "<ele>26.1</ele>") {
		t.Error("GPX missing elevation")
	}
	if !strings.Contains(gpx, "Turn right onto E University Ave") {
		t.Error("GPX missing cue waypoint")
	}
	// Ampersand in name must be escaped.
	if strings.Contains(gpx, "Test & Ride") {
		t.Error("GPX did not escape ampersand in name")
	}
}

func TestBuildCSV(t *testing.T) {
	tps := []trackPoint{{X: f64p(-81.29), Y: f64p(29.03), E: f64p(26.1), D: f64p(0)}}
	csv := buildCSV(tps)
	if !strings.HasPrefix(csv, "lat,lng,elevation_m") {
		t.Errorf("CSV header wrong: %q", csv[:30])
	}
	if !strings.Contains(csv, "29.03,-81.29,26.1,0") {
		t.Errorf("CSV row wrong: %q", csv)
	}
}

func TestUnwrapAssetDetail(t *testing.T) {
	// v1 envelope: {"route": {...}}
	wrapped := json.RawMessage(`{"route":{"id":42,"name":"Loop","distance":1000,"track_points":[{"x":-81,"y":29,"e":10}]}}`)
	d, err := unwrapAssetDetail(wrapped, "route")
	if err != nil {
		t.Fatalf("unwrap wrapped: %v", err)
	}
	if d.Name != "Loop" || len(d.TrackPoints) != 1 {
		t.Errorf("wrapped detail = %+v, want Loop with 1 track point", d)
	}
	// legacy unwrapped: top-level object
	legacy := json.RawMessage(`{"id":42,"name":"Legacy","track_points":[{"x":-81,"y":29}]}`)
	d2, err := unwrapAssetDetail(legacy, "route")
	if err != nil {
		t.Fatalf("unwrap legacy: %v", err)
	}
	if d2.Name != "Legacy" || len(d2.TrackPoints) != 1 {
		t.Errorf("legacy detail = %+v, want Legacy with 1 track point", d2)
	}
}

func TestExtractEventRoutes(t *testing.T) {
	cases := []json.RawMessage{
		json.RawMessage(`{"routes":[{"id":1,"name":"A"},{"id":2,"name":"B"}]}`),
		json.RawMessage(`{"event":{"routes":[{"id":1,"name":"A"},{"id":2,"name":"B"}]}}`),
		json.RawMessage(`{"eventDetails":{"routes":[{"id":1,"name":"A"},{"id":2,"name":"B"}]}}`),
	}
	for i, raw := range cases {
		got := extractEventRoutes(raw)
		if len(got) != 2 || got[0].ID != "1" || got[1].Name != "B" {
			t.Errorf("case %d: extractEventRoutes = %+v, want 2 routes [1,A][2,B]", i, got)
		}
	}
	if got := extractEventRoutes(json.RawMessage(`{"other":true}`)); len(got) != 0 {
		t.Errorf("no-routes case = %+v, want empty", got)
	}
}
