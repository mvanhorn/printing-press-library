package profile

import (
	"math"
	"path/filepath"
	"testing"
)

func TestMETForSpeed(t *testing.T) {
	cases := []struct {
		kmh  float64
		want float64
	}{
		{0, 0},
		{-1, 0},
		{3.2, 2.8},  // table low end
		{6.4, 5.0},  // table high end
		{10, 5.0},   // above clamps to high end
		{2.0, 2.8},  // below clamps to low end
		{4.4, 3.25}, // midpoint between (4.0,3.0) and (4.8,3.5)
	}
	for _, c := range cases {
		got := METForSpeed(c.kmh)
		if math.Abs(got-c.want) > 0.001 {
			t.Errorf("METForSpeed(%v) = %v, want %v", c.kmh, got, c.want)
		}
	}
}

func TestCalories(t *testing.T) {
	// MET 3.5 (4.8 km/h) * 80 kg * 1 h = 280 kcal.
	got := Calories(80, 4.8, 3600)
	if math.Abs(got-280) > 0.01 {
		t.Errorf("Calories = %v, want 280", got)
	}
	if Calories(0, 4.8, 3600) != 0 {
		t.Error("zero weight should yield 0 kcal")
	}
	if Calories(80, 4.8, 0) != 0 {
		t.Error("zero duration should yield 0 kcal")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := Save(path, Profile{WeightKg: 72.5}); err != nil {
		t.Fatal(err)
	}
	p, ok, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || p.WeightKg != 72.5 {
		t.Fatalf("Load = %+v ok=%v", p, ok)
	}
}

func TestLoadMissingIsNotError(t *testing.T) {
	p, ok, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("missing profile should not error: %v", err)
	}
	if ok || p.WeightKg != 0 {
		t.Fatalf("missing profile should be zero/!ok, got %+v ok=%v", p, ok)
	}
}

func TestSaveRejectsNonPositiveWeight(t *testing.T) {
	if err := Save(filepath.Join(t.TempDir(), "p.json"), Profile{WeightKg: 0}); err == nil {
		t.Error("Save should reject zero weight")
	}
}
