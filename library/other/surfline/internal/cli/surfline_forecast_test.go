// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
)

func TestTopSwell(t *testing.T) {
	tests := []struct {
		name    string
		swells  []swellComponent
		wantOK  bool
		wantPer float64
	}{
		{"empty", nil, false, 0},
		{"all zero", []swellComponent{{}}, false, 0},
		{"picks highest energy", []swellComponent{
			{Height: 1, Period: 8},
			{Height: 2, Period: 14}, // energy 2*14*14 = 392, wins
			{Height: 3, Period: 6},  // energy 3*36 = 108
		}, true, 14},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := wavePoint{Swells: tt.swells}
			got, ok := w.topSwell()
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got.Period != tt.wantPer {
				t.Fatalf("period = %v, want %v", got.Period, tt.wantPer)
			}
		})
	}
}

func TestRatingValue(t *testing.T) {
	tests := []struct {
		name string
		in   ratingPoint
		want float64
	}{
		{"numeric value preferred", mkRating("GOOD", 5.5), 5.5},
		{"key fallback EPIC", mkRating("EPIC", 0), 7},
		{"key fallback POOR", mkRating("POOR", 0), 2},
		{"unknown key", mkRating("WAT", 0), 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ratingValue(tt.in); got != tt.want {
				t.Fatalf("ratingValue = %v, want %v", got, tt.want)
			}
		})
	}
}

func mkRating(key string, val float64) ratingPoint {
	r := ratingPoint{}
	r.Rating.Key = key
	r.Rating.Value = val
	return r
}

func TestLocalTime(t *testing.T) {
	// 2021-01-01 00:00:00 UTC = 1609459200; offset -8 → prior day 16:00.
	got := localTime(1609459200, -8, "2006-01-02 15:04")
	if got != "2020-12-31 16:00" {
		t.Fatalf("localTime = %q, want 2020-12-31 16:00", got)
	}
	if z := localTime(0, -8, "15:04"); z != "-" {
		t.Fatalf("zero timestamp = %q, want -", z)
	}
}

func TestNextTides(t *testing.T) {
	tides := []tidePoint{
		{Timestamp: 100, Type: "LOW"},
		{Timestamp: 200, Type: "NORMAL"}, // skipped
		{Timestamp: 300, Type: "HIGH"},
		{Timestamp: 400, Type: "LOW"},
	}
	got := nextTides(tides, 150, 2)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Type != "HIGH" || got[1].Type != "LOW" {
		t.Fatalf("got %v", got)
	}
}

func TestIsDaylight(t *testing.T) {
	sun := []sunlightPoint{{Sunrise: 100, Sunset: 200}}
	if !isDaylight(150, sun) {
		t.Fatal("150 should be daylight")
	}
	if isDaylight(250, sun) {
		t.Fatal("250 should be night")
	}
	if !isDaylight(150, nil) {
		t.Fatal("no sunlight data should default to daylight")
	}
	// Fail open: sunlight present but all-zero timestamps must not hide every point.
	if !isDaylight(150, []sunlightPoint{{}, {Sunrise: 0, Sunset: 0}}) {
		t.Fatal("all-zero sunlight entries should fail open (daylight)")
	}
	// A mix with one valid window still tests ts against the valid window.
	mixed := []sunlightPoint{{}, {Sunrise: 100, Sunset: 200}}
	if isDaylight(250, mixed) {
		t.Fatal("250 outside the one valid window should be night")
	}
}

func TestParseBuoys(t *testing.T) {
	// Nested {data:{buoys:[...]}} with NDBC-style fields.
	body := []byte(`{"data":{"buoys":[
		{"stationId":"46026","name":"SF","distance":12.3,"significantWaveHeight":5.5,"dominantWavePeriod":13,"meanWaveDirection":270},
		{"_id":"x","latest":{"height":3.1,"period":9}}
	]}}`)
	got := parseBuoys(json.RawMessage(body))
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != "46026" || got[0].SwellHt != 5.5 || got[0].SwellPer != 13 {
		t.Fatalf("buoy[0] parsed wrong: %+v", got[0])
	}
	if !got[0].parsedAny {
		t.Fatal("buoy[0] should be marked parsed")
	}
	if got[1].SwellHt != 3.1 || got[1].SwellPer != 9 {
		t.Fatalf("buoy[1] nested parse wrong: %+v", got[1])
	}
}

func TestParseBuoysBareArray(t *testing.T) {
	got := parseBuoys(json.RawMessage(`[{"name":"B","waveHeight":2}]`))
	if len(got) != 1 || got[0].SwellHt != 2 {
		t.Fatalf("bare array parse wrong: %+v", got)
	}
}

func TestParseBuoysLiveShape(t *testing.T) {
	// The real /kbyg/buoys/nearby shape: {data:[{...latestData}]}.
	body := []byte(`{"associated":{},"data":[
		{"sourceId":"46284","name":"Soquel Cove","latitude":36.93,"longitude":-121.934,"status":"OFFLINE",
		 "latestData":{"height":2.3,"period":14,"direction":212}}
	]}`)
	got := parseBuoys(json.RawMessage(body))
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	b := got[0]
	if b.ID != "46284" || b.Name != "Soquel Cove" || b.SwellHt != 2.3 || b.SwellPer != 14 || b.SwellDir != 212 {
		t.Fatalf("live-shape parse wrong: %+v", b)
	}
	if b.Lat != 36.93 || !b.parsedAny {
		t.Fatalf("lat/parsed wrong: %+v", b)
	}
}

func TestHaversineKm(t *testing.T) {
	// Pleasure Point → Soquel Cove buoy, roughly 4 km.
	d := haversineKm(36.95468, -121.97234, 36.93, -121.934)
	if d < 2 || d > 7 {
		t.Fatalf("haversine = %.1f km, want ~4", d)
	}
}
