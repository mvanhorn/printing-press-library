// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import "testing"

func TestCanonicalSupplier(t *testing.T) {
	cases := map[string]string{
		"ALAMO":             "Alamo",
		"Alamo":             "Alamo",
		"Niza Cars":         "Niza",
		"Nizacars":          "Niza",
		"OK MOBILITY NR":    "OK Mobility",
		"OK Mobility":       "OK Mobility",
		"Click NR":          "Clickrent",
		"Clickrent":         "Clickrent",
		"KEDDY by Europcar": "Keddy",
		"RENTBYCAR":         "Rent By Car",
		"GOBYCAR":           "Goby Car",
		"RECORD":            "Record Go",
		"Record Go":         "Record Go",
		"CENTAURO":          "Centauro",
		"":                  "",
	}
	for in, want := range cases {
		if got := CanonicalSupplier(in); got != want {
			t.Errorf("CanonicalSupplier(%q) = %q, want %q", in, got, want)
		}
	}
	// Cross-source merge: the two spellings must produce the same identity.
	if CanonicalSupplier("Niza Cars") != CanonicalSupplier("Nizacars") {
		t.Error("Niza Cars and Nizacars must canonicalize to the same value")
	}
}
