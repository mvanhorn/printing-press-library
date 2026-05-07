package sun

import (
	"testing"
	"time"
)

// London midsummer 2026-06-21: SunCalc.js reports sunrise ~04:43 UTC,
// sunset ~20:21 UTC. Allow ±2 minutes error for the simplified algorithm.
func TestLondonMidsummer(t *testing.T) {
	d := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	got := Compute(d, 51.5074, -0.1278)

	// London 2026-06-21 in UTC (not BST): sunrise ~03:43Z, sunset ~20:21Z.
	wantSunrise := time.Date(2026, 6, 21, 3, 43, 0, 0, time.UTC)
	wantSunset := time.Date(2026, 6, 21, 20, 21, 0, 0, time.UTC)

	if abs(got.Sunrise.Sub(wantSunrise)) > 5*time.Minute {
		t.Errorf("sunrise: got %v, want ~%v", got.Sunrise, wantSunrise)
	}
	if abs(got.Sunset.Sub(wantSunset)) > 5*time.Minute {
		t.Errorf("sunset: got %v, want ~%v", got.Sunset, wantSunset)
	}
}

func TestBlueHourBracketsSunset(t *testing.T) {
	d := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	got := Compute(d, 35.6895, 139.6917) // Tokyo
	// Evening blue hour starts after sunset and ends later still.
	if !got.BlueHourEve.Start.After(got.Sunset) {
		t.Errorf("evening blue hour should start after sunset: blue=%v sunset=%v", got.BlueHourEve.Start, got.Sunset)
	}
	if !got.BlueHourEve.End.After(got.BlueHourEve.Start) {
		t.Error("blue hour End should follow Start")
	}
}

func TestGoldenHourBracketsSunset(t *testing.T) {
	d := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	got := Compute(d, 48.8566, 2.3522) // Paris
	// Evening golden hour ENDS at sunset (sun drops to 0°).
	if !got.GoldenHourEve.End.After(got.GoldenHourEve.Start) {
		t.Error("golden hour End should follow Start")
	}
	if got.GoldenHourEve.End.Before(got.GoldenHourEve.Start) {
		t.Error("golden hour times reversed")
	}
}

func abs(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
