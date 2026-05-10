// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestMetroCityName(t *testing.T) {
	cases := map[string]string{
		"seattle":       "Seattle",
		"chicago":       "Chicago",
		"new-york":      "New York City",
		"nyc":           "New York City",
		"manhattan":     "New York City",
		"san-francisco": "San Francisco",
		"sf":            "San Francisco",
		"los-angeles":   "Los Angeles",
		"la":            "Los Angeles",
		"washington-dc": "Washington DC",
		"dc":            "Washington DC",
		"new-orleans":   "New Orleans",
		"nola":          "New Orleans",
		"  Seattle  ":   "Seattle", // whitespace + case-insensitive
		"unknown-metro": "",        // unknown returns empty for caller fallback
		"":              "",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			if got := metroCityName(in); got != want {
				t.Errorf("metroCityName(%q) = %q; want %q", in, got, want)
			}
		})
	}
}

func TestMetroCityName_AllKnownSlugsResolve(t *testing.T) {
	// Every slug in knownMetros() must resolve to a non-empty display name —
	// otherwise the city-search URL would carry an empty `?city=` value.
	for _, slug := range knownMetros() {
		t.Run(slug, func(t *testing.T) {
			if got := metroCityName(slug); got == "" {
				t.Errorf("metroCityName(%q) returned empty; every knownMetros slug must map", slug)
			}
		})
	}
}

func TestFirstToken(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		"canlis":           "canlis",
		"tasting menu":     "tasting",
		"tasting\tmenu":    "tasting",
		"  leading spaces": "",
		"single":           "single",
	}
	for in, want := range cases {
		t.Run(in, func(t *testing.T) {
			if got := firstToken(in); got != want {
				t.Errorf("firstToken(%q) = %q; want %q", in, got, want)
			}
		})
	}
}
