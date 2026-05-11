// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: scaffold-endpoint-redirects — issue #406 failure 1.
//
// Geo-aware ranking for cross-network search results.
//
// Background: OpenTable's Autocomplete returns the closest fuzzy match
// to a query term anywhere in the world, scored by name similarity
// only. When an agent passes a city-suffixed slug like `joey-bellevue`,
// Autocomplete returns "Joey's Bold Flavors" (Tampa, FL) because the
// first-token match scores well — and our CLI surfaced that as if it
// were the Bellevue venue. Result: agents got "available" for the
// wrong venue, with no way to know.
//
// Fix shape: when a metro centroid is set (either via explicit `--metro`
// or inferred from a slug suffix), compute haversine distance per
// result. Drop or demote results beyond a radius threshold based on
// whether the metro was explicit (hard reject) or inferred (soft
// demote — caller can see the low score and decide).
//
// This file holds the pure-function pieces (haversine math, slug-suffix
// inference, score adjustment). The wiring into goat/earliest lives in
// those command files so each can tune the policy independently.

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/opentable"
)

// defaultMetroRadiusKm is the cutoff for "this venue is in the metro."
// 50km covers most metro areas including their suburbs (e.g., a
// Bellevue, WA venue is ~13km from Seattle's centroid; LA spans ~50km
// on its longest axis). Tunable per-invocation via --metro-radius-km.
const defaultMetroRadiusKm = 50.0

// metroFilterMode controls how the filter handles results outside the
// radius. Issue #406 brainstorm conclusion: hard-reject when the user
// EXPLICITLY set `--metro`, soft-demote when we INFERRED it from a
// slug suffix. Hard-reject is honest about confident intent; soft-
// demote preserves "show me everything" when the geo hint was a
// best-effort inference we might have gotten wrong.
type metroFilterMode int

const (
	// metroFilterOff disables geo filtering. Used when no centroid is
	// set (no --metro, no slug suffix, no lat/lng).
	metroFilterOff metroFilterMode = iota

	// metroFilterHardReject drops results outside the radius. Used when
	// the user explicitly passed `--metro <slug>`.
	metroFilterHardReject

	// metroFilterSoftDemote keeps results but multiplies their match
	// score by a small constant so they sort to the bottom. Used when
	// the metro was inferred from a slug suffix.
	metroFilterSoftDemote
)

// softDemoteFactor is multiplied into match_score for results outside
// the radius under soft-demote mode. 0.1 means a baseline 0.95-match
// venue ends up at 0.095 — clearly below any in-radius venue's score,
// but still visible to the agent.
const softDemoteFactor = 0.1

// haversineKm returns the great-circle distance in kilometers between
// two lat/lng points using the standard haversine formula. Radius
// 6371 km is mean Earth radius — accurate to ~0.5% which is plenty
// for metro filtering (a 50km threshold tolerates ~250m of slop).
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusKm = 6371.0
	rad := math.Pi / 180.0
	dLat := (lat2 - lat1) * rad
	dLng := (lng2 - lng1) * rad
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*rad)*math.Cos(lat2*rad)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusKm * c
}

// inferMetroFromSlug_DEPRECATED tries to detect a city-suffix in a
// user-supplied slug. Issue #406 failure 1: agents compose slugs like
// `joey-bellevue`, `13-coins-bellevue` to disambiguate by city —
// without this inference the suffix is dropped on the floor and the
// resolver returns wrong-city matches.
//
// Strategy: walk the slug from end to start, peeling off hyphen-
// separated tokens. For each suffix of 1-3 tokens, try the registry.
// Returns the matched Metro + the prefix (slug-minus-suffix) so
// callers can use the prefix as the actual search term. Returns
// ok=false when no suffix maps to a known metro.
//
// Renamed in U5 (location-native-redesign): the free-form
// `--location` parser (`ParseLocation` + `ResolveLocation`) supersedes
// the hyphenated-slug-suffix pathway for human/agent-typed input. The
// slug-suffix logic is still used by `resolveOTSlugGeoAware` to peel
// city hints off venue slugs like `joey-bellevue` — that's a distinct
// concern (slug-suffix parsing) from the new free-form parser.
// Migration to a unified `ResolveLocation`-based path lives in U7/U8;
// until then, this helper keeps the slug-suffix venue resolution
// behavior intact. The `_DEPRECATED` suffix flags it for that future
// migration.
func inferMetroFromSlug_DEPRECATED(slug string, reg MetroRegistry) (m Metro, prefix string, ok bool) {
	tokens := strings.Split(slug, "-")
	// Try longer suffixes first so "new-york-city" wins over the
	// substring "city" / "york".
	for suffixLen := 3; suffixLen >= 1; suffixLen-- {
		if suffixLen > len(tokens) {
			continue
		}
		suffix := strings.Join(tokens[len(tokens)-suffixLen:], "-")
		if metro, found := reg.Lookup(suffix); found {
			prefixTokens := tokens[:len(tokens)-suffixLen]
			return metro, strings.Join(prefixTokens, "-"), true
		}
	}
	return Metro{}, slug, false
}

// resolveOTSlugGeoAware is the geo-aware replacement for the bare
// opentable.Client.RestaurantIDFromQuery call site in
// resolveEarliestForVenue. Issue #406 failure 1 root cause:
// RestaurantIDFromQuery picks the FIRST Autocomplete result whose name
// matches the query — so `joey-bellevue` resolves to "Joey's Bold
// Flavors" (Tampa) because it scores well on first-token match. The
// `-bellevue` city suffix is dropped on the floor.
//
// This function:
//  1. Detects a city suffix in the slug. If present, peels it off and
//     uses the metro's centroid as the Autocomplete coordinate hint.
//  2. Calls Autocomplete directly (bypassing RestaurantIDFromQuery's
//     first-match-wins logic).
//  3. Among candidates whose name actually matches the prefix, picks
//     the one CLOSEST to the metro centroid — not the highest-scoring
//     by name alone.
//  4. Hard-rejects candidates beyond the radius.
//
// When no city suffix is detected, falls back to the existing
// RestaurantIDFromQuery behavior via opentable.Client.
func resolveOTSlugGeoAware(
	ctx context.Context,
	c *opentable.Client,
	slug string,
	defaultLat, defaultLng float64,
	radiusKm float64,
) (restID int, restName, restSlug string, metroUsed Metro, err error) {
	if radiusKm <= 0 {
		radiusKm = defaultMetroRadiusKm
	}
	reg := getRegistry()
	metro, prefix, found := inferMetroFromSlug_DEPRECATED(slug, reg)
	if !found {
		// No city suffix — fall back to the existing resolver path.
		id, name, urlSlug, fallbackErr := c.RestaurantIDFromQuery(ctx, slug, defaultLat, defaultLng)
		return id, name, urlSlug, Metro{}, fallbackErr
	}

	// City suffix detected. Use the metro centroid as the Autocomplete
	// coordinate hint and search by the prefix.
	q := strings.ReplaceAll(strings.ToLower(prefix), "-", " ")
	q = strings.TrimSpace(q)
	if q == "" {
		return 0, "", "", metro,
			fmt.Errorf("slug %q is just a city suffix with no venue prefix", slug)
	}

	results, autoErr := c.Autocomplete(ctx, q, metro.Lat, metro.Lng)
	if autoErr != nil {
		return 0, "", "", metro, autoErr
	}

	// Walk candidates. Keep only Restaurant entries whose name matches
	// the prefix on full-substring OR first-token. Among those, pick
	// the one closest to the metro centroid. Hard-reject everything
	// beyond radius.
	type candidate struct {
		id     int
		name   string
		slug   string
		distKm float64
	}
	var inRadius []candidate
	for _, r := range results {
		if r.Type != "Restaurant" {
			continue
		}
		nameLower := strings.ToLower(r.Name)
		match := strings.Contains(nameLower, q) || strings.Contains(q, nameLower)
		if !match {
			tokens := strings.Fields(q)
			if len(tokens) == 0 || !strings.Contains(nameLower, tokens[0]) {
				continue
			}
		}
		var idInt int
		fmt.Sscanf(r.ID, "%d", &idInt)
		if idInt == 0 {
			continue
		}
		// If venue lat/lng is missing, skip — we can't make a geo
		// judgement, and including it risks the wrong-city symptom.
		if r.Latitude == 0 && r.Longitude == 0 {
			continue
		}
		dist := haversineKm(metro.Lat, metro.Lng, r.Latitude, r.Longitude)
		if dist > radiusKm {
			continue
		}
		inRadius = append(inRadius, candidate{
			id:     idInt,
			name:   r.Name,
			slug:   r.URLSlug,
			distKm: dist,
		})
	}

	if len(inRadius) == 0 {
		return 0, "", "", metro, fmt.Errorf(
			"no %q venue found in metro %q within %.0fkm of (%.4f, %.4f) — "+
				"the slug suffix appears to be a city hint, but no matching "+
				"restaurant resolved there; try `restaurants list --query %q --metro %s` "+
				"to discover the correct slug or numeric ID",
			q, metro.Slug, radiusKm, metro.Lat, metro.Lng, q, metro.Slug)
	}

	// Pick the closest. Stable: name tiebreak.
	// PR #425 round-2 Greptile P2: rename loop variable from `c` to
	// `cand` so it doesn't shadow the outer `c *opentable.Client`
	// parameter. The shadow was non-functional (different types) but
	// reading the code is easier with distinct names.
	best := inRadius[0]
	for _, cand := range inRadius[1:] {
		if cand.distKm < best.distKm {
			best = cand
		} else if cand.distKm == best.distKm && cand.name < best.name {
			best = cand
		}
	}
	return best.id, best.name, best.slug, metro, nil
}

// applyGeoFilter mutates a slice of goatResult according to the
// configured mode and the typed GeoContext. Returns the post-filter
// slice (which may be shorter under hard-reject, same length under
// soft-demote). Also annotates each row with the computed distance
// in km so agents can see why a row was kept/dropped.
//
// Signature migrated in U5 from `(centroid Metro, radiusKm float64,
// mode metroFilterMode)` to `(ctx *GeoContext, mode metroFilterMode)`
// so the typed location pipeline (ResolveLocation -> GeoContext) flows
// through the post-filter without an ad-hoc Metro reconstruction at
// each call site. A nil ctx means "no location constraint requested";
// the function returns results unchanged to support R13 (no-filter
// when the caller didn't pass --location).
func applyGeoFilter(results []goatResult, ctx *GeoContext, mode metroFilterMode) []goatResult {
	if ctx == nil || mode == metroFilterOff {
		return results
	}
	lat := ctx.Centroid[0]
	lng := ctx.Centroid[1]
	if lat == 0 && lng == 0 {
		return results
	}
	radiusKm := ctx.RadiusKm
	if radiusKm <= 0 {
		radiusKm = defaultMetroRadiusKm
	}
	out := results[:0]
	for _, r := range results {
		// Results with no lat/lng (Tock SSR sometimes omits Location
		// for newly-listed venues) bypass the filter — we can't make a
		// geo judgement on missing data.
		if r.Latitude == 0 && r.Longitude == 0 {
			out = append(out, r)
			continue
		}
		dist := haversineKm(lat, lng, r.Latitude, r.Longitude)
		r.MetroCentroidDistanceKm = dist
		if dist <= radiusKm {
			out = append(out, r)
			continue
		}
		switch mode {
		case metroFilterHardReject:
			// Drop entirely.
			continue
		case metroFilterSoftDemote:
			r.MatchScore *= softDemoteFactor
			out = append(out, r)
		default:
			out = append(out, r)
		}
	}
	return out
}
