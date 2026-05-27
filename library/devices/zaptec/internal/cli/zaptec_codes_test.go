// Copyright 2026 pimmetjeoss. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestChargerOperationModeName(t *testing.T) {
	cases := map[int]string{
		0:  "Unknown",
		1:  "Disconnected",
		2:  "Connected (requesting)",
		3:  "Charging",
		5:  "Connected (finished)",
		99: "Unknown",
	}
	for mode, want := range cases {
		if got := chargerOperationModeName(mode); got != want {
			t.Errorf("chargerOperationModeName(%d) = %q, want %q", mode, got, want)
		}
	}
}

func TestObservationName(t *testing.T) {
	if got := observationName(710); got != "Operation mode" {
		t.Errorf("observationName(710) = %q, want %q", got, "Operation mode")
	}
	if got := observationName(513); got != "Total charge power" {
		t.Errorf("observationName(513) = %q, want %q", got, "Total charge power")
	}
	if got := observationName(999999); got != "" {
		t.Errorf("observationName(unknown) = %q, want empty", got)
	}
	if got := observationUnit(507); got != "A" {
		t.Errorf("observationUnit(507) = %q, want %q", got, "A")
	}
}

func TestRound2(t *testing.T) {
	cases := map[float64]float64{
		1.234:  1.23,
		1.235:  1.24,
		10.0:   10.0,
		0.001:  0.0,
		99.999: 100.0,
	}
	for in, want := range cases {
		if got := round2(in); got != want {
			t.Errorf("round2(%v) = %v, want %v", in, got, want)
		}
	}
}
