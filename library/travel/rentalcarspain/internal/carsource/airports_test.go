// Copyright 2026 serranoX and contributors. Licensed under Apache-2.0. See LICENSE.

package carsource

import "testing"

func TestSpainAirportsTableIntegrity(t *testing.T) {
	seenIATA := map[string]bool{}
	seenCode := map[string]bool{}
	for _, a := range spainAirports {
		if len(a.IATA) != 3 {
			t.Errorf("%q: IATA must be 3 chars", a.IATA)
		}
		if a.Name == "" || a.DoYouSpainCode == "" {
			t.Errorf("%s: name/code must not be empty", a.IATA)
		}
		if seenIATA[a.IATA] {
			t.Errorf("duplicate IATA %s", a.IATA)
		}
		if seenCode[a.DoYouSpainCode] {
			t.Errorf("duplicate DoYouSpain code %s (%s)", a.DoYouSpainCode, a.IATA)
		}
		seenIATA[a.IATA] = true
		seenCode[a.DoYouSpainCode] = true
	}
	if spainAirports[0].IATA != "AGP" {
		t.Errorf("Málaga must stay at index 0 (empty-query default), got %s", spainAirports[0].IATA)
	}
}

func TestResolveAirport(t *testing.T) {
	cases := []struct{ query, want string }{
		// Empty defaults to Málaga.
		{"", "AGP"},
		// Exact IATA (case-insensitive) and DoYouSpain code.
		{"BCN", "BCN"}, {"tfn", "TFN"}, {"MAL02", "AGP"}, {"PMP02", "PNA"},
		// Accent-insensitive name search — users type without accents.
		{"malaga", "AGP"}, {"Málaga", "AGP"},
		{"almeria", "LEI"}, {"Almería", "LEI"},
		{"coruna", "LCG"}, {"Coruña", "LCG"},
		{"san sebastian", "EAS"},
		// Same-name pairs must resolve to the busier airport.
		{"palma", "PMI"},    // not SPC (La Palma)
		{"tenerife", "TFS"}, // not TFN
		// Newly added airports resolve by name.
		{"santiago", "SCQ"}, {"jerez", "XRY"}, {"melilla", "MLN"},
	}
	for _, c := range cases {
		got, ok := ResolveAirport(c.query)
		if !ok {
			t.Errorf("ResolveAirport(%q) not found, want %s", c.query, c.want)
			continue
		}
		if got.IATA != c.want {
			t.Errorf("ResolveAirport(%q) = %s, want %s", c.query, got.IATA, c.want)
		}
	}
	if _, ok := ResolveAirport("Lisbon"); ok {
		t.Error("a non-Spanish airport should not resolve")
	}
}
