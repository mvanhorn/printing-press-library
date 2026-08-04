// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"
)

// ageEligibilityNote flags an offer only when a specified driver age is below the
// offer's stated minimum — never for standard/unspecified ages or offers with no
// stated minimum. Suppliers still show (flag-but-keep), so this drives the label.
func TestAgeEligibilityNote(t *testing.T) {
	cases := []struct {
		name   string
		minAge int
		age    int
		want   string
	}{
		{"under Goldcar minimum", 21, 20, "min age 21"},
		{"under Clickrent premium gate", 30, 24, "min age 30"},
		{"meets the minimum exactly", 21, 21, ""},
		{"above the minimum", 21, 35, ""},
		{"age unspecified", 21, 0, ""},
		{"offer states no minimum", 0, 20, ""},
	}
	for _, c := range cases {
		o := carsource.Offer{Car: "Kia Picanto", MinAge: c.minAge}
		if got := ageEligibilityNote(o, c.age); got != c.want {
			t.Errorf("%s: ageEligibilityNote(minAge=%d, age=%d) = %q, want %q", c.name, c.minAge, c.age, got, c.want)
		}
	}
}

// carCellWithAge appends the flag to the car name only when the driver is too
// young, and truncates the base name to the given width.
func TestCarCellWithAge(t *testing.T) {
	o := carsource.Offer{Car: "Kia Picanto", MinAge: 21}
	if got := carCellWithAge(o, 20, 28); got != "Kia Picanto [min age 21]" {
		t.Errorf("young driver cell = %q, want the flag appended", got)
	}
	if got := carCellWithAge(o, 35, 28); got != "Kia Picanto" {
		t.Errorf("eligible driver cell = %q, want no flag", got)
	}
}
