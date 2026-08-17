// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"
)

func TestCarSize(t *testing.T) {
	bigger := []carsource.Offer{
		{CarClass: "Small Cars", Car: "Nissan Juke SUV"},
		{CarClass: "SUVs", Car: "Kia Sportage"},
		{CarClass: "IFMR", Car: "Ford Puma"},
		{Car: "VW Caddy", Seats: 7},
		{CarClass: "Estate", Car: "Skoda Octavia Estate"},
	}
	for _, o := range bigger {
		if carSize(o) != "bigger" {
			t.Errorf("carSize(%q/%q) = small, want bigger", o.CarClass, o.Car)
		}
	}
	small := []carsource.Offer{
		{CarClass: "Small Cars", Car: "Fiat 500"},
		{CarClass: "EDMR", Car: "Seat Ibiza"},
		{CarClass: "MBMR", Car: "Toyota Aygo"},
		{CarClass: "Economy", Car: "Citroen C3"},
	}
	for _, o := range small {
		if carSize(o) != "small" {
			t.Errorf("carSize(%q/%q) = bigger, want small", o.CarClass, o.Car)
		}
	}
}

func TestFiPrice(t *testing.T) {
	// base 20 + 2.65/day over 7 days = 20 + 18.55 = 38.55 standalone cover.
	const base, perDay = 20.0, 2.65
	// Direct offer is already zero-excess — no cover added.
	d, zero := fiPrice(carsource.Offer{Source: "centauro", Total: 120}, 7, base, perDay)
	if d != 120 || !zero {
		t.Errorf("direct fiPrice = %v,%v want 120,true", d, zero)
	}
	// Aggregator with excess adds base+per-day cover.
	a, zero := fiPrice(carsource.Offer{Source: "doyouspain", Total: 60, Excess: 1000, ExcessKnown: true}, 7, base, perDay)
	if a != 60+38.55 || zero {
		t.Errorf("aggregator fiPrice = %v,%v want %v,false", a, zero, 60+38.55)
	}
	// Aggregator already zero-excess — no cover added.
	z, zero := fiPrice(carsource.Offer{Source: "rentalcars", Total: 150, Excess: 0, ExcessKnown: true}, 7, base, perDay)
	if z != 150 || !zero {
		t.Errorf("zero-excess aggregator fiPrice = %v,%v want 150,true", z, zero)
	}
}

// excessCoverEstimate reproduces the iCarHire single-trip quotes it's calibrated
// on (base €20 + €2.65/night): ≈€27.95 for 3 nights, ≈€38.55 for 7 nights — the
// fixed base is what a flat per-day model misses on short rentals.
func TestExcessCoverEstimate(t *testing.T) {
	if got := excessCoverEstimate(7, 20, 2.65); got != 38.55 {
		t.Errorf("7-night cover = %v, want 38.55", got)
	}
	if got := excessCoverEstimate(3, 20, 2.65); got != 27.95 {
		t.Errorf("3-night cover = %v, want 27.95", got)
	}
	// The fixed base applies even to a zero-day edge case.
	if got := excessCoverEstimate(0, 20, 2.65); got != 20 {
		t.Errorf("0-day cover = %v, want the base 20", got)
	}
	if got := excessCoverEstimate(-5, 20, 2.65); got != 20 {
		t.Errorf("negative days clamps to base, got %v", got)
	}
}
