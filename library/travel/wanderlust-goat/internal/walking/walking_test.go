package walking

import (
	"math"
	"testing"
)

func TestHaversine_KnownDistances(t *testing.T) {
	// Park Hyatt Tokyo (35.6863, 139.6906) ↔ Shibuya Station (35.6580, 139.7016).
	// Real distance ~3.2 km.
	park := LatLng{35.6863, 139.6906}
	shibuya := LatLng{35.6580, 139.7016}
	d := HaversineMeters(park, shibuya)
	if d < 2500 || d > 4000 {
		t.Errorf("Park Hyatt → Shibuya: %v m, expected 2500-4000", d)
	}
}

func TestHaversine_SamePoint(t *testing.T) {
	a := LatLng{35.0, 139.0}
	if d := HaversineMeters(a, a); d != 0 {
		t.Errorf("same-point distance = %v, want 0", d)
	}
}

func TestMinutesFromMeters(t *testing.T) {
	// 1000m crow-flies × 1.3 = 1300 walked meters; at 4.5 km/h = 17.33 min.
	m := MinutesFromMeters(1000)
	want := (1000.0 * Tortuosity) / (PaceKMH * 1000.0) * 60.0
	if math.Abs(m-want) > 0.01 {
		t.Errorf("MinutesFromMeters(1000) = %v, want %v", m, want)
	}
	if m < 17 || m > 18 {
		t.Errorf("expected ~17.33 min for 1km crow-flies, got %v", m)
	}
}

func TestMetersFromMinutes_RoundTrip(t *testing.T) {
	for _, mins := range []float64{5, 10, 15, 20, 30} {
		meters := MetersFromMinutes(mins)
		back := MinutesFromMeters(meters)
		if math.Abs(back-mins) > 0.01 {
			t.Errorf("round trip %v min → %v m → %v min", mins, meters, back)
		}
	}
}

func TestMinutesFromMeters_NonPositive(t *testing.T) {
	if MinutesFromMeters(0) != 0 {
		t.Error("0 m → 0 min")
	}
	if MinutesFromMeters(-100) != 0 {
		t.Error("negative → 0 min")
	}
}

func TestWithinMinutes(t *testing.T) {
	a := LatLng{35.6863, 139.6906}
	b := LatLng{35.6873, 139.6906} // ~111m north
	if !WithinMinutes(a, b, 5) {
		t.Error("very nearby points should be within 5 walking minutes")
	}
	c := LatLng{35.0, 139.0} // ~76km
	if WithinMinutes(a, c, 5) {
		t.Error("76km away should not be within 5 walking minutes")
	}
}
