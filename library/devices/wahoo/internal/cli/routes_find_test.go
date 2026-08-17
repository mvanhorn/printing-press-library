// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored tests for the offline route finder.

package cli

import (
	"encoding/json"
	"math"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/devices/wahoo/internal/store"
)

func TestParseKmRange(t *testing.T) {
	cases := []struct {
		in       string
		min, max float64
		wantErr  bool
	}{
		{"80-120", 80, 120, false},
		{"80-", 80, 0, false},
		{"-120", 0, 120, false},
		{"", 0, 0, false},
		{"100", 100, 0, false},
		{"120-80", 0, 0, true},
		{"a-b", 0, 0, true},
	}
	for _, c := range cases {
		mn, mx, err := parseKmRange(c.in)
		if (err != nil) != c.wantErr {
			t.Errorf("%q: err=%v wantErr=%v", c.in, err, c.wantErr)
			continue
		}
		if !c.wantErr && (mn != c.min || mx != c.max) {
			t.Errorf("%q: got (%v,%v) want (%v,%v)", c.in, mn, mx, c.min, c.max)
		}
	}
}

func TestParseLatLng(t *testing.T) {
	lat, lng, err := parseLatLng("40.7,-74.0")
	if err != nil || lat != 40.7 || lng != -74.0 {
		t.Errorf("got (%v,%v,%v)", lat, lng, err)
	}
	if _, _, err := parseLatLng("nope"); err == nil {
		t.Error("expected error for malformed --near")
	}
}

func TestHaversineKm(t *testing.T) {
	// NYC -> Philadelphia is roughly 130 km.
	d := haversineKm(40.7128, -74.0060, 39.9526, -75.1652)
	if math.Abs(d-130) > 10 {
		t.Errorf("haversine=%v want ~130", d)
	}
	if haversineKm(10, 10, 10, 10) != 0 {
		t.Error("expected zero distance for identical points")
	}
}

func TestFindRoutes(t *testing.T) {
	db, err := store.Open(filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	routes := []string{
		`{"id":1,"name":"Short Loop","distance":40000,"ascent":200,"starting_lat":40.70,"starting_lng":-74.00}`,
		`{"id":2,"name":"Century","distance":160000,"ascent":1500,"starting_lat":40.75,"starting_lng":-74.02}`,
		`{"id":3,"name":"Hilly Metric","distance":100000,"ascent":1800,"starting_lat":41.50,"starting_lng":-75.50}`,
	}
	for _, r := range routes {
		if err := db.UpsertRoutes(json.RawMessage(r)); err != nil {
			t.Fatal(err)
		}
	}
	// 90-120 km band -> only the 100km route.
	got, err := findRoutes(db, routeFilter{minDistKm: 90, maxDistKm: 120})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Hilly Metric" {
		t.Errorf("distance filter got %+v", got)
	}
	if got[0].DistanceKm != 100 {
		t.Errorf("distance_km=%v want 100 (meters/1000)", got[0].DistanceKm)
	}
	// max-ascent 500 -> only the flat short loop.
	got, _ = findRoutes(db, routeFilter{maxAscent: 500})
	if len(got) != 1 || got[0].Name != "Short Loop" {
		t.Errorf("ascent filter got %+v", got)
	}
	// near NYC within 25km -> the two NYC-area routes, not the 140km-away one.
	got, _ = findRoutes(db, routeFilter{haveNear: true, nearLat: 40.71, nearLng: -74.00, radiusKm: 25})
	if len(got) != 2 {
		t.Fatalf("near filter got %d want 2", len(got))
	}
	for _, m := range got {
		if m.FromKm <= 0 || m.FromKm > 25 {
			t.Errorf("FromKm out of expected range: %v", m.FromKm)
		}
	}
}
