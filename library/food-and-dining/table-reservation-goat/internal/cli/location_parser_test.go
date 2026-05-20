// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"
)

// TestParseLocation_BareCity — bare city tokens land as LocKindCity
// with SpecificityLow. CityName is lowercased; original input
// preserved in Raw.
func TestParseLocation_BareCity(t *testing.T) {
	cases := []struct {
		input string
		want  string // expected CityName
	}{
		{"bellevue", "bellevue"},
		{"Bellevue", "bellevue"},
		{"NEW YORK", "new york"},
		{"san francisco", "san francisco"},
		{"  bellevue  ", "bellevue"}, // whitespace trimmed
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			li, err := ParseLocation(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if li == nil {
				t.Fatalf("got nil LocationInput; want LocCity")
			}
			if li.Kind != LocKindCity {
				t.Errorf("kind: got %v, want LocKindCity", li.Kind)
			}
			if li.Specificity != SpecificityLow {
				t.Errorf("specificity: got %v, want SpecificityLow", li.Specificity)
			}
			if li.CityName != tc.want {
				t.Errorf("city name: got %q, want %q", li.CityName, tc.want)
			}
		})
	}
}

// TestParseLocation_CityState — "City, ST" pattern lands as
// LocKindCityState with SpecificityHigh (city+state is unambiguous
// enough to collapse multi-candidate ambiguity).
func TestParseLocation_CityState(t *testing.T) {
	cases := []struct {
		input     string
		wantCity  string
		wantState string
	}{
		{"Bellevue, WA", "bellevue", "WA"},
		{"bellevue, wa", "bellevue", "WA"}, // state uppercased
		{"Portland, OR", "portland", "OR"},
		{"New York, NY", "new york", "NY"}, // multi-word city
		{"Springfield, MA", "springfield", "MA"},
		{"  Portland , OR  ", "portland", "OR"}, // loose whitespace
		{"bellevue, wa, usa", "bellevue", "WA"}, // extra parts ignored
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			li, err := ParseLocation(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if li == nil {
				t.Fatalf("got nil LocationInput")
			}
			if li.Kind != LocKindCityState {
				t.Errorf("kind: got %v, want LocKindCityState", li.Kind)
			}
			if li.Specificity != SpecificityHigh {
				t.Errorf("specificity: got %v, want SpecificityHigh", li.Specificity)
			}
			if li.CityName != tc.wantCity {
				t.Errorf("city: got %q, want %q", li.CityName, tc.wantCity)
			}
			if li.State != tc.wantState {
				t.Errorf("state: got %q, want %q", li.State, tc.wantState)
			}
		})
	}
}

// TestParseLocation_Coords — coordinate pattern lands as LocKindCoords
// with SpecificityHigh.
func TestParseLocation_Coords(t *testing.T) {
	cases := []struct {
		input            string
		wantLat, wantLng float64
	}{
		{"47.6101,-122.2015", 47.6101, -122.2015},
		{"47.6101, -122.2015", 47.6101, -122.2015}, // space after comma
		{"47.6,-122.2", 47.6, -122.2},              // low precision
		{"40.7128,-74.0060", 40.7128, -74.0060},    // NYC
		{"-33.8688,151.2093", -33.8688, 151.2093},  // southern hemisphere
		{"0,0", 0, 0}, // origin (valid input even if uncovered)
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			li, err := ParseLocation(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if li == nil {
				t.Fatalf("got nil LocationInput")
			}
			if li.Kind != LocKindCoords {
				t.Errorf("kind: got %v, want LocKindCoords", li.Kind)
			}
			if li.Specificity != SpecificityHigh {
				t.Errorf("specificity: got %v, want SpecificityHigh", li.Specificity)
			}
			if li.Lat != tc.wantLat || li.Lng != tc.wantLng {
				t.Errorf("lat/lng: got (%v, %v), want (%v, %v)", li.Lat, li.Lng, tc.wantLat, tc.wantLng)
			}
		})
	}
}

// TestParseLocation_Metro — "X metro" suffix lands as LocKindMetro
// with SpecificityMedium (more specific than bare city, less than
// city+state).
func TestParseLocation_Metro(t *testing.T) {
	cases := []struct {
		input    string
		wantSlug string
	}{
		{"seattle metro", "seattle"},
		{"Seattle Metro", "seattle"},
		{"new york metro", "new-york"},
		{"san francisco metro", "san-francisco"},
		{"  seattle metro  ", "seattle"}, // whitespace
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			li, err := ParseLocation(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if li == nil {
				t.Fatalf("got nil LocationInput")
			}
			if li.Kind != LocKindMetro {
				t.Errorf("kind: got %v, want LocKindMetro", li.Kind)
			}
			if li.Specificity != SpecificityMedium {
				t.Errorf("specificity: got %v, want SpecificityMedium", li.Specificity)
			}
			if li.MetroSlug != tc.wantSlug {
				t.Errorf("metro slug: got %q, want %q", li.MetroSlug, tc.wantSlug)
			}
		})
	}
}

// TestParseLocation_Empty — empty or whitespace-only input returns
// (nil, nil) signaling "no constraint." This is the R13 path that
// every command treats identically to --location being absent.
func TestParseLocation_Empty(t *testing.T) {
	cases := []string{"", " ", "   ", "\t", "\n", "  \t  "}
	for _, tc := range cases {
		t.Run("input=["+tc+"]", func(t *testing.T) {
			li, err := ParseLocation(tc)
			if err != nil {
				t.Errorf("empty input should not error; got %v", err)
			}
			if li != nil {
				t.Errorf("empty input should return nil; got %+v", li)
			}
		})
	}
}

// TestParseLocation_InvalidCoords — coords syntactically valid but
// out of geographic range produce a typed parse error citing the
// bound that was violated.
func TestParseLocation_InvalidCoords(t *testing.T) {
	cases := []string{
		"91,0",        // lat > 90
		"-91,0",       // lat < -90
		"0,181",       // lng > 180
		"0,-181",      // lng < -180
		"100.5,200.3", // both out of range
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			li, err := ParseLocation(tc)
			if err == nil {
				t.Errorf("expected error for out-of-range coords %q; got %+v", tc, li)
			}
		})
	}
}

// TestParseLocation_FallthroughToCity — strings that don't match any
// specific pattern (coord, city+state, metro) fall through to
// LocKindCity. Registry lookup in U3 will handle the "is this a real
// place?" question.
func TestParseLocation_FallthroughToCity(t *testing.T) {
	cases := []string{
		"abc123def",        // garbage falls through to LocCity
		"downtown seattle", // neighborhood-ish lands as bare city
		"the eastside",     // colloquial location
	}
	for _, tc := range cases {
		t.Run(tc, func(t *testing.T) {
			li, err := ParseLocation(tc)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if li == nil {
				t.Fatalf("got nil; want LocCity fallthrough")
			}
			if li.Kind != LocKindCity {
				t.Errorf("kind: got %v, want LocKindCity (fallthrough)", li.Kind)
			}
		})
	}
}

// TestParseLocation_RawPreserved — original input is preserved in
// Raw field across all kinds. Useful for echoing back in error
// messages and the envelope's WhatWasAsked field.
func TestParseLocation_RawPreserved(t *testing.T) {
	input := "  Bellevue, WA  "
	li, err := ParseLocation(input)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if li == nil {
		t.Fatalf("nil")
	}
	if li.Raw != input {
		t.Errorf("raw: got %q, want %q", li.Raw, input)
	}
}

// TestParseLocation_AmbiguousPriority — verify pattern precedence.
// Edge cases at the boundary between patterns:
//   - "0,0" is valid coords (not "0" comma "0" interpreted as
//     city+state)
//   - "seattle" (one token) is bare city, not metro
//   - "X metro" with no space-prefix doesn't match (e.g., "metropolis"
//     stays a city)
func TestParseLocation_AmbiguousPriority(t *testing.T) {
	cases := []struct {
		input    string
		wantKind LocationKind
	}{
		{"0,0", LocKindCoords},
		{"seattle", LocKindCity},
		{"metropolis", LocKindCity},    // doesn't have " metro" suffix
		{"metro detroit", LocKindCity}, // "metro" prefix isn't the suffix pattern
		{"seattlemetro", LocKindCity},  // no space before "metro"
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			li, err := ParseLocation(tc.input)
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if li == nil {
				t.Fatalf("nil")
			}
			if li.Kind != tc.wantKind {
				t.Errorf("input %q: got kind %v, want %v", tc.input, li.Kind, tc.wantKind)
			}
		})
	}
}
