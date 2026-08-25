// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/rentalcarspain/internal/carsource"
)

// classFilterMatch (used by best/direct) matches brand and multi-word names by
// substring, body/size via ACRISS decoding, treats comma-separated terms as OR,
// and matches everything when the filter is empty.
func TestClassFilterMatch(t *testing.T) {
	cases := []struct {
		name   string
		offer  carsource.Offer
		filter string
		want   bool
	}{
		{"empty filter matches all", carsource.Offer{Car: "Fiat 500"}, "", true},
		{"brand by name", carsource.Offer{Car: "BMW 1 Series"}, "bmw", true},
		{"multi-word brand", carsource.Offer{Car: "Alfa Romeo Stelvio"}, "alfa romeo", true},
		{"type by name substring", carsource.Offer{Car: "Fiat 500 Cabrio"}, "cabrio", true},
		{"type via ACRISS code only", carsource.Offer{CarClass: "IFAR", Car: "Nissan Qashqai"}, "suv", true},
		{"OR list matches second term", carsource.Offer{Car: "Mercedes A Class"}, "bmw,mercedes", true},
		{"non-match", carsource.Offer{Car: "Fiat 500", CarClass: "MBMR"}, "bmw", false},
	}
	for _, c := range cases {
		if got := classFilterMatch(c.offer, c.filter); got != c.want {
			t.Errorf("%s: classFilterMatch(%q, %q) = %v, want %v", c.name, c.offer.Car, c.filter, got, c.want)
		}
	}
}

func TestLooksACRISS(t *testing.T) {
	yes := []string{"IFAR", "CFMR", "MBMR", "EDMR", "CDARFF", "MBMRZE", "SFAH"}
	for _, s := range yes {
		if !looksACRISS(s) {
			t.Errorf("looksACRISS(%q) = false, want true", s)
		}
	}
	// Human labels must never be mistaken for codes — these are the traps that
	// would cause silent mis-filtering.
	no := []string{
		"SUVs",        // 3rd char 'V' is not a transmission code
		"Automatic",   // 3rd char 'T'
		"Large Cars",  // contains a space
		"Small Cars",  // contains a space
		"Medium Cars", // contains a space
		"Premium",     // 3rd char 'E'
		"Estate Cars", // contains a space
		"Group A",     // too short / space
		"", "AB",      // too short
	}
	for _, s := range no {
		if looksACRISS(s) {
			t.Errorf("looksACRISS(%q) = true, want false", s)
		}
	}
}

func TestAnyClassACRISSAndText(t *testing.T) {
	cases := []struct {
		name       string
		class, car string
		key        string
		want       bool
	}{
		// Rentalcars ACRISS: 2nd char F/G/J = SUV family.
		{"acriss suv F", "CFMR", "Volkswagen T-Cross", "suv", true},
		{"acriss suv F auto", "IFAR", "Nissan Qashqai", "suv", true},
		{"acriss crossover G", "CGAR", "Some Crossover", "suv", true},
		{"acriss not suv (2-4 door)", "EDMR", "Seat Ibiza", "suv", false},
		{"acriss special X not suv", "EXMR", "Audi A1", "suv", false},
		{"acriss wagon is not suv", "CWAR", "Skoda Octavia Estate", "suv", false},
		{"acriss wagon is estate", "CWAR", "Skoda Octavia Estate", "estate", true},
		// ACRISS size decoding.
		{"acriss mini is small", "MBMR", "Kia Picanto", "small", true},
		{"acriss fullsize is large", "FFAR", "Big Car", "large", true},
		{"acriss mini is not large", "MBMR", "Kia Picanto", "large", false},
		// DoYouSpain human labels still match by substring.
		{"text SUVs", "SUVs", "Peugeot 2008", "suv", true},
		{"text Small Cars", "Small Cars", "Fiat 500", "small", true},
		{"text Large Cars is not luxury", "Large Cars", "Opel Insignia", "luxury", false},
		{"text Medium is not mini", "Medium Cars", "Seat Leon", "mini", false},
		// Model-name substring still works.
		{"model name match", "Small Cars", "Fiat 500", "fiat", true},
	}
	for _, c := range cases {
		if got := anyClass(c.class, c.car, []string{c.key}); got != c.want {
			t.Errorf("%s: anyClass(%q,%q,[%q]) = %v, want %v", c.name, c.class, c.car, c.key, got, c.want)
		}
	}
	// Multiple keys: any match wins.
	if !anyClass("CFMR", "VW T-Cross", []string{"estate", "suv"}) {
		t.Error("multi-key: expected suv to match CFMR")
	}
}
