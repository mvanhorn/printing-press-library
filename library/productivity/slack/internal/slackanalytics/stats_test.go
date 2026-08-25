// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package slackanalytics

import (
	"reflect"
	"testing"
	"time"
)

func TestMedianFloat(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []float64
		want float64
	}{
		{"empty", nil, 0},
		{"single", []float64{4}, 4},
		{"odd", []float64{5, 1, 3}, 3},
		{"even", []float64{4, 1, 3, 2}, 2.5},
		{"duplicates", []float64{2, 2, 2}, 2},
		{"negatives", []float64{-4, -1, -3}, -3},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := MedianFloat(tc.in); got != tc.want {
				t.Fatalf("MedianFloat(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMedianFloatDoesNotMutateInput(t *testing.T) {
	t.Parallel()
	in := []float64{9, 1, 5}
	original := append([]float64(nil), in...)
	_ = MedianFloat(in)
	if !reflect.DeepEqual(in, original) {
		t.Fatalf("MedianFloat mutated its input: %v, want %v", in, original)
	}
}

func TestMedianDuration(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   []time.Duration
		want time.Duration
	}{
		{"empty", nil, 0},
		{"single", []time.Duration{90 * time.Second}, 90 * time.Second},
		{"odd", []time.Duration{time.Minute, 10 * time.Minute, 5 * time.Minute}, 5 * time.Minute},
		{"even rounds to seconds", []time.Duration{time.Second, 2 * time.Second}, 2 * time.Second},
		{"hours", []time.Duration{time.Hour, 3 * time.Hour}, 2 * time.Hour},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := MedianDuration(tc.in); got != tc.want {
				t.Fatalf("MedianDuration(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestPerDay(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		count  int
		window time.Duration
		want   float64
	}{
		{"zero count", 0, 24 * time.Hour, 0},
		{"zero window", 10, 0, 0},
		{"negative window", 10, -time.Hour, 0},
		{"one per day", 7, 7 * 24 * time.Hour, 1},
		{"rounds to two decimals", 10, 3 * 24 * time.Hour, 3.33},
		{"sub day window", 2, 12 * time.Hour, 4},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := PerDay(tc.count, tc.window); got != tc.want {
				t.Fatalf("PerDay(%d, %v) = %v, want %v", tc.count, tc.window, got, tc.want)
			}
		})
	}
}

func TestLargestGap(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name string
		in   []time.Time
		want time.Duration
	}{
		{"empty", nil, 0},
		{"single", []time.Time{base}, 0},
		{"even spacing", []time.Time{base, base.Add(time.Hour), base.Add(2 * time.Hour)}, time.Hour},
		{"unsorted input", []time.Time{base.Add(72 * time.Hour), base, base.Add(time.Hour)}, 71 * time.Hour},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := LargestGap(tc.in); got != tc.want {
				t.Fatalf("LargestGap(%v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}
