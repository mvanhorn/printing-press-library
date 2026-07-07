// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored tests for the Withings decode helpers.

package cli

import (
	"math"
	"testing"
)

func TestScaleMeasure(t *testing.T) {
	cases := []struct {
		name  string
		value int
		unit  int
		want  float64
	}{
		{"weight 81250 * 10^-3 => 81.25", 81250, -3, 81.25},
		{"fat ratio 2230 * 10^-2 => 22.3", 2230, -2, 22.3},
		{"systolic 120 * 10^0 => 120", 120, 0, 120},
		{"height 1850 * 10^-3 => 1.85", 1850, -3, 1.85},
		{"negative value -50 * 10^-1 => -5", -50, -1, -5},
		{"zero", 0, -3, 0},
		{"positive exponent 7 * 10^1 => 70", 7, 1, 70},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := scaleMeasure(tc.value, tc.unit)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Errorf("scaleMeasure(%d, %d) = %v, want %v", tc.value, tc.unit, got, tc.want)
			}
		})
	}
}

func TestWithingsMeasureTypeName(t *testing.T) {
	cases := []struct {
		code int
		want string
	}{
		{1, "weight"},
		{5, "fat_free_mass"},
		{6, "fat_ratio"},
		{8, "fat_mass"},
		{9, "diastolic_bp"},
		{10, "systolic_bp"},
		{11, "heart_pulse"},
		{54, "spo2"},
		{139, "afib_ecg"},
		{99999, "unknown_99999"},
	}
	for _, tc := range cases {
		if got := withingsMeasureTypeName(tc.code); got != tc.want {
			t.Errorf("withingsMeasureTypeName(%d) = %q, want %q", tc.code, got, tc.want)
		}
	}
}

func TestWithingsMeasureTypeUnitAndLabel(t *testing.T) {
	if got := withingsMeasureTypeUnit(1); got != "kg" {
		t.Errorf("withingsMeasureTypeUnit(1) = %q, want kg", got)
	}
	if got := withingsMeasureTypeUnit(6); got != "%" {
		t.Errorf("withingsMeasureTypeUnit(6) = %q, want %%", got)
	}
	if got := withingsMeasureTypeUnit(99999); got != "" {
		t.Errorf("withingsMeasureTypeUnit(unknown) = %q, want empty", got)
	}
	if got := withingsMeasureTypeLabel(10); got != "Systolic blood pressure" {
		t.Errorf("withingsMeasureTypeLabel(10) = %q", got)
	}
	if got := withingsMeasureTypeLabel(99999); got != "Unknown measure type 99999" {
		t.Errorf("withingsMeasureTypeLabel(unknown) = %q", got)
	}
}

func TestWithingsAfibLabel(t *testing.T) {
	cases := map[int]string{0: "negative", 1: "afib", 2: "inconclusive", 7: "unknown"}
	for code, want := range cases {
		if got := withingsAfibLabel(code); got != want {
			t.Errorf("withingsAfibLabel(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestWithingsSleepStateLabel(t *testing.T) {
	cases := map[int]string{0: "awake", 1: "light", 2: "deep", 3: "rem", 9: "unknown"}
	for code, want := range cases {
		if got := withingsSleepStateLabel(code); got != want {
			t.Errorf("withingsSleepStateLabel(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestWithingsWorkoutCategoryName(t *testing.T) {
	cases := map[int]string{
		1:      "Walk",
		2:      "Run",
		6:      "Bicycling",
		7:      "Swimming",
		16:     "Lift weights",
		18:     "Elliptical",
		28:     "Yoga",
		999999: "Other",
	}
	for code, want := range cases {
		if got := withingsWorkoutCategoryName(code); got != want {
			t.Errorf("withingsWorkoutCategoryName(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestWithingsAppliName(t *testing.T) {
	cases := map[int]string{
		1:     "Weight",
		4:     "Blood pressure / Heart rate",
		16:    "Activity",
		44:    "Sleep",
		54:    "ECG",
		88888: "Unknown appli 88888",
	}
	for code, want := range cases {
		if got := withingsAppliName(code); got != want {
			t.Errorf("withingsAppliName(%d) = %q, want %q", code, got, want)
		}
	}
}

func TestItoa(t *testing.T) {
	cases := map[int]string{0: "0", 7: "7", 42: "42", -3: "-3", 99999: "99999"}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Errorf("itoa(%d) = %q, want %q", in, got, want)
		}
	}
}
