// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: location-native-redesign — U5 pipeline entry point.
// ResolveLocation ties the U2 free-form parser, the U3 Place registry,
// the U4 popularity-prior + tier decision, and the U1 GeoContext +
// DisambiguationEnvelope shapes together into a single function the
// read-command wiring (U6-U8) can call uniformly.
//
// The return-tuple shape is the load-bearing contract:
//   - (*GeoContext, nil, nil)      — caller has a constraint; apply
//                                    pre-filter (provider input) and
//                                    post-filter (applyGeoFilter).
//   - (nil, *Envelope, nil)        — caller emits the envelope in place
//                                    of results (location_unknown or
//                                    location_ambiguous), no filtering.
//   - (nil, nil, nil)              — no constraint requested; caller
//                                    skips pre- and post-filter (R13).
//   - (nil, nil, error)            — hard parse error; propagate to
//                                    the user.

import (
	"fmt"
	"strings"
)

// ResolveOptions controls how ResolveLocation handles ambiguity and
// where the caller wants the post-filter to land for soft-demote vs
// hard-reject semantics.
type ResolveOptions struct {
	// Source is propagated to the returned GeoContext.Source. Callers
	// pass SourceExplicitFlag for --location, SourceExtractedFromQuery
	// for slug-suffix inference, and SourceDefault for fallback paths.
	Source Source

	// AcceptAmbiguous flips the LOW-tier behavior: when true, LOW
	// returns a forced-pick GeoContext (top candidate) instead of the
	// disambiguation envelope. Used by callers who explicitly told us
	// to pick the best match anyway (--batch-accept-ambiguous flag on
	// commands that surface it).
	AcceptAmbiguous bool
}

// defaultSyntheticRadiusKm is the radius assigned to a coords-only
// LocationInput when ReverseLookup misses (the query point falls
// outside every curated/dynamic Place). Set to 50 km so the synthetic
// context behaves like a metro-scale window — wide enough to surface
// neighbors without false-matching across regions.
const defaultSyntheticRadiusKm = 50.0

// ResolveLocation parses + looks up + scores + decides tier for a
// free-form location string. Exactly one of (*GeoContext,
// *DisambiguationEnvelope) is non-nil when err == nil; both nil
// signals "no constraint requested" (empty input).
//
// Pipeline:
//
//  1. ParseLocation(input). Empty -> (nil, nil, nil). Parse error ->
//     (nil, nil, err).
//  2. Lookup candidates by LocationKind:
//     - LocKindCity       — reg.LookupByName(CityName)
//     - LocKindCityState  — reg.LookupByName(CityName), filtered by State
//     - LocKindCoords     — reg.ReverseLookup(Lat, Lng); on miss,
//     synthesize a single-candidate Place at the query point
//     - LocKindMetro      — reg.Lookup(MetroSlug) (alias chain)
//  3. Zero candidates -> envelope with ErrorKindLocationUnknown.
//  4. decideTier(li, candidates) -> (tier, ranked).
//  5. LOW tier without AcceptAmbiguous -> envelope with
//     ErrorKindLocationAmbiguous.
//  6. Otherwise build a GeoContext from ranked[0] with Alternates
//     projected from ranked[1:].
func ResolveLocation(input string, opts ResolveOptions) (*GeoContext, *DisambiguationEnvelope, error) {
	li, err := ParseLocation(input)
	if err != nil {
		return nil, nil, err
	}
	if li == nil {
		// Empty input — no constraint requested. Caller skips both the
		// pre-filter and post-filter; R13 no-filter behavior.
		return nil, nil, nil
	}

	reg := getRegistry()
	candidates, lookupErr := lookupCandidates(li, reg)
	if lookupErr != nil {
		// Lookup-stage hard error (e.g., a future provider error path).
		// Today the lookups are pure-function over the in-process
		// registry so this branch is defensive only.
		return nil, nil, lookupErr
	}

	if len(candidates) == 0 {
		env := BuildEnvelope(li, nil, ErrorKindLocationUnknown)
		return nil, &env, nil
	}

	tier, ranked := decideTier(li, candidates)

	if tier == TierLow && !opts.AcceptAmbiguous {
		env := BuildEnvelope(li, ranked, ErrorKindLocationAmbiguous)
		return nil, &env, nil
	}

	gc := buildGeoContext(li, ranked, opts.Source, tier)
	return gc, nil, nil
}

// lookupCandidates dispatches on LocationKind to the right registry
// call. Returns an empty slice when nothing matches; the zero-
// candidates state is the caller's signal to emit
// ErrorKindLocationUnknown.
func lookupCandidates(li *LocationInput, reg PlaceRegistry) ([]Place, error) {
	switch li.Kind {
	case LocKindCity:
		hits, _ := reg.LookupByName(li.CityName)
		return hits, nil

	case LocKindCityState:
		// LookupByName returns every place sharing the display name;
		// the state qualifier then narrows. If the by-name lookup hit
		// but the state filter eliminates every match (e.g.,
		// "bellevue, zz"), that's a location_unknown — the state
		// signal contradicted the city.
		hits, _ := reg.LookupByName(li.CityName)
		if len(hits) == 0 {
			return nil, nil
		}
		filtered := make([]Place, 0, len(hits))
		for _, p := range hits {
			if p.State == li.State {
				filtered = append(filtered, p)
			}
		}
		return filtered, nil

	case LocKindCoords:
		// ReverseLookup returns at most one Place (the smallest-radius
		// containing region). On a miss, synthesize a single-
		// candidate Place anchored at the query point so the caller
		// always gets a usable GeoContext rather than an envelope —
		// coords are an unambiguous constraint by definition.
		if p, ok := reg.ReverseLookup(li.Lat, li.Lng); ok {
			return []Place{p}, nil
		}
		return []Place{syntheticCoordsPlace(li.Lat, li.Lng)}, nil

	case LocKindMetro:
		if p, ok := reg.Lookup(li.MetroSlug); ok {
			return []Place{p}, nil
		}
		return nil, nil

	default:
		// LocKindNone shouldn't reach here (ParseLocation returns nil
		// for empty input), but be defensive.
		return nil, nil
	}
}

// syntheticCoordsPlace builds a one-off Place at the requested
// lat/lng, used when ReverseLookup finds no covering region. The
// slug "(coords)" is a sentinel — never a real registry slug — so
// callers can recognize the synthetic case if they care to (today
// they don't: the GeoContext drives the post-filter and that's it).
func syntheticCoordsPlace(lat, lng float64) Place {
	return Place{
		Slug:     "(coords)",
		Name:     fmt.Sprintf("(%.4f, %.4f)", lat, lng),
		Lat:      lat,
		Lng:      lng,
		RadiusKm: defaultSyntheticRadiusKm,
		Tier:     PlaceTierUnknown,
	}
}

// buildGeoContext projects the top-ranked candidate into a
// GeoContext, with Alternates carrying the remaining candidates.
// Score is the top candidate's popularity prior (a mechanical [0,1]
// number, not a confidence — see ResolutionTier). Tier carries the
// categorical decision from decideTier so agents can branch on the
// tier string rather than re-inferring from Score and Alternates.
func buildGeoContext(li *LocationInput, ranked []ScoredCandidate, source Source, tier TierEnum) *GeoContext {
	top := ranked[0]
	radius := top.Place.RadiusKm
	if radius <= 0 {
		radius = defaultSyntheticRadiusKm
	}
	gc := &GeoContext{
		Origin:     li.Raw,
		ResolvedTo: formatPlaceName(top.Place),
		Centroid:   [2]float64{top.Place.Lat, top.Place.Lng},
		RadiusKm:   radius,
		Score:      top.Prior,
		Tier:       tier.ResolutionTier(),
		Source:     source,
		Alternates: candidatesFromRanked(ranked[1:]),
	}
	return gc
}

// formatPlaceName renders a Place's display name with its state
// suffix when present. Matches BuildEnvelope's candidate naming
// ("Bellevue, WA") so the agent-facing string stays consistent
// between resolved GeoContexts and disambiguation envelopes.
func formatPlaceName(p Place) string {
	if p.State == "" {
		return p.Name
	}
	return fmt.Sprintf("%s, %s", p.Name, p.State)
}

// resolveLocationFlags routes --location / --metro through ResolveLocation
// and emits the legacy --metro deprecation warning (once per process)
// when callers come through the legacy entry point. Returns the resolved
// GeoContext, the envelope (when disambiguation is required), an error
// (when the input parse fails — e.g., out-of-range coords), and the
// acceptAmbiguousBypass value that flowed into ResolveLocation.
//
// Resolution precedence:
//  1. --location <value> — new typed entry point; uses
//     --batch-accept-ambiguous verbatim.
//  2. --metro <slug>     — legacy fallback. U14 narrows the implicit
//     --batch-accept-ambiguous to the CANONICAL-only case: when the
//     value resolves to a single known metro via registry Lookup (slug
//     or alias) or LookupByName (single hit), preserve back-compat by
//     silent-picking. When the value is ambiguous (e.g., --metro
//     bellevue matches WA/NE/KY by name), suppress AcceptAmbiguous so
//     the envelope path fires — agent disambiguates instead of
//     silently picking the wrong city (Codex P1-D fix). Deprecation
//     warning fires once-per-process regardless of canonical-vs-ambiguous.
//  3. neither            — returns (nil, nil, nil, false) so the caller
//     skips the pre/post filter entirely (R13 no-filter behavior).
//
// Lives in location_pipeline.go (not in the per-command files) so every
// read command (restaurants list, availability check, multi-day, future
// earliest/goat/watch) shares one helper rather than reimplementing the
// precedence + deprecation-warning logic per command.
func resolveLocationFlags(
	stderr interface{ Write(p []byte) (int, error) },
	flagLocation string,
	flagMetro string,
	flagAcceptAmbiguous bool,
) (*GeoContext, *DisambiguationEnvelope, error, bool) {
	input := strings.TrimSpace(flagLocation)
	acceptAmbiguous := flagAcceptAmbiguous
	if input == "" {
		if metro := strings.TrimSpace(flagMetro); metro != "" {
			// Legacy path. The once-gate ensures the warning fires only
			// on the first --metro invocation per process; subsequent
			// calls stay quiet so scripted callers don't see one warning
			// per loop iteration.
			metroDeprecationOnce.Do(func() {
				fmt.Fprintln(stderr,
					"warning: --metro is deprecated; use --location <city>.")
			})
			input = metro
			// U14: canonical-only forced-pick. Only set AcceptAmbiguous
			// when the value resolves to a single known metro. Ambiguous
			// values fall through to the standard envelope path so the
			// agent can disambiguate.
			acceptAmbiguous = isCanonicalMetro(metro)
		}
	}
	if input == "" {
		// No location requested — caller skips both pre- and post-filter.
		return nil, nil, nil, false
	}
	gc, env, err := ResolveLocation(input, ResolveOptions{
		Source:          SourceExplicitFlag,
		AcceptAmbiguous: acceptAmbiguous,
	})
	if err != nil {
		return nil, nil, err, acceptAmbiguous
	}
	return gc, env, nil, acceptAmbiguous
}

// isCanonicalMetro reports whether value names a single, unambiguous
// metro in the registry. Two acceptance paths:
//  1. reg.Lookup(value) succeeds — value is a known canonical slug or
//     an alias chain (e.g., "sf" -> "san-francisco"). The slug-lookup
//     path is the only one a typical --metro caller hits.
//  2. reg.Lookup misses BUT reg.LookupByName returns exactly 1 hit —
//     the user passed a display-name-shaped value (e.g., "Seattle")
//     that happens to uniquely match one Place by Name. Treated as
//     canonical so legacy --metro callers passing display names still
//     get the back-compat shape.
//
// Zero or multi-hit on LookupByName (e.g., "bellevue" -> WA/NE/KY) is
// the ambiguous case: not canonical, so the envelope path fires.
func isCanonicalMetro(value string) bool {
	reg := getRegistry()
	if _, ok := reg.Lookup(value); ok {
		return true
	}
	if hits, ok := reg.LookupByName(value); ok && len(hits) == 1 {
		return true
	}
	return false
}

// candidatesFromRanked projects ScoredCandidate values into the
// Candidate shape used in GeoContext.Alternates. Mirrors
// BuildEnvelope's projection logic so the two surfaces emit
// identical alternate entries (agents pasting between envelope
// candidates and GeoContext alternates can rely on field-by-field
// equivalence).
func candidatesFromRanked(ranked []ScoredCandidate) []Candidate {
	if len(ranked) == 0 {
		return nil
	}
	out := make([]Candidate, len(ranked))
	for i, sc := range ranked {
		p := sc.Place
		name := p.Name
		if p.State != "" {
			name = fmt.Sprintf("%s, %s", p.Name, p.State)
		}
		tockCov := 0
		if p.ProviderCoverage != nil {
			tockCov = p.ProviderCoverage["tock"]
		}
		out[i] = Candidate{
			Name:              name,
			State:             p.State,
			ContextHints:      p.ContextHints,
			TockBusinessCount: tockCov,
			ScoreIfPicked:     sc.Prior,
			Centroid:          [2]float64{p.Lat, p.Lng},
		}
	}
	return out
}
