// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: location-native-redesign — typed GeoContext flowing through
// every read command's resolver pipeline. Issue #406 follow-up: prior
// PRs (#423-#426) fixed named symptoms but real-world testing on
// 2026-05-10 showed --metro was silently discarded in restaurants_list
// and there was no --location surface on availability_check. The
// redesign makes location a first-class typed concept.

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/opentable"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
)

// Source enumerates how a GeoContext was obtained, so post-filter
// behavior (hard-reject vs soft-demote) can branch on the strength
// of intent rather than guessing.
//
//	SourceExplicitFlag — caller passed --location explicitly. The
//	  constraint is authoritative; post-filter hard-rejects results
//	  outside the radius.
//
//	SourceExtractedFromQuery — location inferred from the input
//	  shape (e.g., hyphenated slug suffix "joey-bellevue"). Best-
//	  effort hint; post-filter soft-demotes (keeps but flags) rather
//	  than removing.
//
//	SourceDefault — no explicit location and no inference; the CLI
//	  applied a back-compat fallback (e.g., NYC for the legacy
//	  resolveOTSlugGeoAware path). No post-filter applied; the field
//	  carries the marker so consumers can see the fallback fired.
type Source string

const (
	// SourceExplicitFlag — --location <value> from the user/agent.
	SourceExplicitFlag Source = "explicit_flag"

	// SourceExtractedFromQuery — derived from the input itself
	// (hyphenated slug suffix today; NLP extraction in a future v2).
	SourceExtractedFromQuery Source = "extracted_from_query"

	// SourceDefault — CLI fallback path (back-compat). Signals
	// "no constraint was requested but we needed something."
	SourceDefault Source = "default"
)

// Candidate carries a Place projection used in DisambiguationEnvelope
// candidates and in GeoContext.Alternates. The JSON shape is stable
// across both uses so agents can parse uniformly. TockBusinessCount
// is always emitted (not omitempty) — its presence is part of the
// envelope contract documented in SKILL.md.
type Candidate struct {
	Name              string     `json:"name"`
	State             string     `json:"state,omitempty"`
	ContextHints      []string   `json:"context_hints,omitempty"`
	TockBusinessCount int        `json:"tock_business_count"`
	ScoreIfPicked     float64    `json:"score_if_picked"`
	Centroid          [2]float64 `json:"centroid"`
}

// GeoContext is the typed location signal flowing through every read
// command's resolver pipeline. A nil *GeoContext means "no location
// constraint requested" — caller skips pre-filter and post-filter,
// preserving the no-filter behavior callers had before --location
// was added.
//
// Two methods project this typed shape into the provider-specific
// input the source clients accept: ForOpenTable() returns the
// opentable.LocationInput (lat/lng only — OT's Autocomplete and
// SearchRestaurants accept those), ForTock() returns the
// tock.LocationInput (City + Slug + lat/lng — Tock's SearchCity
// requires the display name as both a query param and a path slug).
//
// When a third provider is added (Resy, SevenRooms, …), add a new
// ForX() method here and a corresponding LocationInput type in that
// provider's package. The two-method shape is the deliberate
// not-yet-an-interface choice (per the plan's Key Technical
// Decisions): one implementation behind an interface is speculative
// generality; extract the interface when a third provider lands.
type GeoContext struct {
	Origin     string      `json:"origin"`
	ResolvedTo string      `json:"resolved_to"`
	Centroid   [2]float64  `json:"centroid"` // [lat, lng]
	RadiusKm   float64     `json:"radius_km"`
	Confidence float64     `json:"confidence"`
	Source     Source      `json:"source"`
	Alternates []Candidate `json:"alternates,omitempty"`
}

// ForOpenTable projects the GeoContext into the input shape OT's
// client accepts. v1 carries lat/lng only — OT exposes MetroID on
// SearchRestaurants but we have no slug→ID mapping to maintain
// today, so MetroID stays zero.
//
// Nil-safe: a nil *GeoContext returns a zero-value LocationInput.
// Callers should check for nil before calling and skip the pre-
// filter entirely when nil, but the nil-safety is defense in depth.
func (g *GeoContext) ForOpenTable() opentable.LocationInput {
	if g == nil {
		return opentable.LocationInput{}
	}
	return opentable.LocationInput{
		Lat: g.Centroid[0],
		Lng: g.Centroid[1],
	}
}

// ForTock projects the GeoContext into Tock's required shape. Tock
// SearchCity needs the City display name (e.g., "Bellevue") to drive
// the ?city= query param AND the Slug (e.g., "bellevue") to drive the
// /search/<slug> path segment. Both are derived from ResolvedTo.
//
// Nil-safe (see ForOpenTable).
func (g *GeoContext) ForTock() tock.LocationInput {
	if g == nil {
		return tock.LocationInput{}
	}
	city, slug := cityAndSlugFromResolvedTo(g.ResolvedTo)
	return tock.LocationInput{
		City: city,
		Slug: slug,
		Lat:  g.Centroid[0],
		Lng:  g.Centroid[1],
	}
}

// Validate enforces invariants on a constructed GeoContext. Returns
// nil for nil receivers ("no constraint" is a valid state). The
// Confidence range check is the load-bearing invariant; downstream
// tier decisions assume [0,1].
func (g *GeoContext) Validate() error {
	if g == nil {
		return nil
	}
	if g.Confidence < 0 || g.Confidence > 1 {
		return fmt.Errorf("geo_context: confidence must be in [0,1], got %v", g.Confidence)
	}
	return nil
}

// cityAndSlugFromResolvedTo splits "Bellevue, WA" into ("Bellevue",
// "bellevue"). Tock's SearchCity needs both shapes — the display
// name goes into ?city= and a slug version (lowercased, hyphenated)
// goes into the /city/<slug>/search path segment.
//
// Handles a few realistic shapes the parser might produce:
//   - "Bellevue, WA" → ("Bellevue", "bellevue")
//   - "New York City, NY" → ("New York City", "new-york-city")
//   - "Seattle" (no comma) → ("Seattle", "seattle")
//   - "  Portland , OR  " (loose whitespace) → ("Portland", "portland")
func cityAndSlugFromResolvedTo(resolvedTo string) (city, slug string) {
	if i := strings.Index(resolvedTo, ","); i > 0 {
		city = strings.TrimSpace(resolvedTo[:i])
	} else {
		city = strings.TrimSpace(resolvedTo)
	}
	slug = strings.ToLower(strings.ReplaceAll(city, " ", "-"))
	return city, slug
}
