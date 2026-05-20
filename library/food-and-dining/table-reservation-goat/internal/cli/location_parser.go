// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: location-native-redesign — free-form --location parser.
// Replaces inferMetroFromSlug (which only handled hyphenated slug
// suffixes). Accepts natural-shaped strings agents actually type:
// bare city, city+state, coords, metro qualifier. Zip support
// deferred to v2 (no zip→Place data path in the curated registry).

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// LocationKind discriminates the parsed shape of a --location input.
// LocKindNone is the zero value and is never returned by ParseLocation
// (which returns nil instead to signal "no constraint requested").
type LocationKind int

const (
	LocKindNone LocationKind = iota
	LocKindCity
	LocKindCityState
	LocKindCoords
	LocKindMetro
)

// Specificity captures how unambiguous the input is. Drives the
// tier decision in U4 — high specificity collapses multi-candidate
// ambiguity to HIGH tier even when multiple Places share the city
// name (e.g., "bellevue, wa" picks Bellevue WA decisively despite
// three Bellevues in the registry).
type Specificity int

const (
	SpecificityLow    Specificity = iota // bare city, neighborhood-shaped tokens
	SpecificityMedium                    // metro qualifier ("X metro")
	SpecificityHigh                      // city+state, coords, zip
)

// LocationInput is the parsed form of --location <free-form-string>.
// Returned as a tagged struct (not a discriminated union via
// interface) to keep call sites simple in Go — readers branch on
// Kind and reach into the field set for that variant. ParseLocation
// returns nil for empty input to signal "no constraint requested";
// downstream commands treat that as no-filter (R13).
type LocationInput struct {
	Kind        LocationKind
	Specificity Specificity
	Raw         string // original user input, preserved for echoing

	// Set when Kind ∈ {LocKindCity, LocKindCityState}. Lowercased.
	CityName string

	// Set when Kind == LocKindCityState. Two-letter state, uppercased.
	State string

	// Set when Kind == LocKindCoords.
	Lat, Lng float64

	// Set when Kind == LocKindMetro. Lowercase, hyphenated slug
	// (e.g., "new-york" from "new york metro").
	MetroSlug string
}

// coordPattern matches "lat,lng" with optional whitespace after the
// comma. Both lat and lng support a leading minus and decimal point.
// Range validation happens after capture (out-of-range produces a
// typed error rather than a fallthrough to LocCity).
var coordPattern = regexp.MustCompile(`^\s*(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)\s*$`)

// cityStatePattern matches "City Name, ST" — two-letter state
// suffix is the discriminator. Extra comma-separated trailing parts
// (e.g., "city, state, usa") are ignored.
var cityStatePattern = regexp.MustCompile(`^\s*([^,]+?)\s*,\s*([A-Za-z]{2})\b`)

// metroSuffixPattern matches "<X> metro" with X being one or more
// space-separated tokens. The trailing " metro" must be space-
// prefixed so "metropolis" doesn't accidentally match. Case-
// insensitive on "metro" so "Seattle Metro" parses the same as
// "seattle metro".
var metroSuffixPattern = regexp.MustCompile(`(?i)^\s*(.+?)\s+metro\s*$`)

// ParseLocation interprets a free-form --location string into a
// typed LocationInput. Returns (nil, nil) when input is empty or
// whitespace-only — the canonical "no constraint" signal. Returns
// a typed parse error when the input matches a structured pattern
// but fails validation (e.g., lat > 90).
func ParseLocation(input string) (*LocationInput, error) {
	raw := input
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, nil
	}

	// (1) Coords — most specific pattern, try first.
	if m := coordPattern.FindStringSubmatch(trimmed); m != nil {
		lat, errLat := strconv.ParseFloat(m[1], 64)
		lng, errLng := strconv.ParseFloat(m[2], 64)
		if errLat != nil || errLng != nil {
			// Pattern matched but parse failed — strconv shouldn't
			// fail given the regex, but defend in depth.
			return nil, fmt.Errorf("location: coord parse failed: lat=%v lng=%v", errLat, errLng)
		}
		if lat < -90 || lat > 90 {
			return nil, fmt.Errorf("location: latitude %v outside valid range [-90, 90]", lat)
		}
		if lng < -180 || lng > 180 {
			return nil, fmt.Errorf("location: longitude %v outside valid range [-180, 180]", lng)
		}
		return &LocationInput{
			Kind:        LocKindCoords,
			Specificity: SpecificityHigh,
			Raw:         raw,
			Lat:         lat,
			Lng:         lng,
		}, nil
	}

	// (2) "City, ST" — second-most specific. Two-letter state suffix
	// is the discriminator. Multi-word cities are supported
	// ("New York, NY"). Extra trailing parts (", USA") ignored.
	if m := cityStatePattern.FindStringSubmatch(trimmed); m != nil {
		city := strings.ToLower(strings.TrimSpace(m[1]))
		state := strings.ToUpper(m[2])
		return &LocationInput{
			Kind:        LocKindCityState,
			Specificity: SpecificityHigh,
			Raw:         raw,
			CityName:    city,
			State:       state,
		}, nil
	}

	// (3) "X metro" — metro qualifier. Less specific than city+state
	// because the metro slug can match multiple regions in the
	// registry (though usually one); slug is hyphenated for
	// downstream Lookup compatibility.
	if m := metroSuffixPattern.FindStringSubmatch(trimmed); m != nil {
		raw := strings.ToLower(strings.TrimSpace(m[1]))
		slug := strings.ReplaceAll(raw, " ", "-")
		return &LocationInput{
			Kind:        LocKindMetro,
			Specificity: SpecificityMedium,
			Raw:         input,
			MetroSlug:   slug,
		}, nil
	}

	// (4) Fallthrough — treat as bare city name. Registry lookup in
	// U3 will return either matching Places or unknown (envelope
	// path). LocKindCity is SpecificityLow, so the tier decision
	// will route multi-candidate inputs to LOW.
	return &LocationInput{
		Kind:        LocKindCity,
		Specificity: SpecificityLow,
		Raw:         raw,
		CityName:    strings.ToLower(trimmed),
	}, nil
}
