// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// Place registry — single source of truth for location lookups across
// `goat`, `earliest`, `availability check`, `restaurants list`, and
// `location resolve`.
//
// A Place is the unit of geographic resolution: it carries a canonical
// slug, a display name, a centroid, a radius, a population, optional
// per-provider parent-metro routing (so an OpenTable agent asking about
// "Bellevue WA" can be told that OpenTable lumps it under "seattle"),
// per-provider coverage hints (live business counts where the provider
// surfaces them), aliases for human shorthand, prose hints for the
// disambiguation UX, and a tier so the location resolver can prefer
// city-level matches over metro centroids when both qualify.
//
// Shape choices:
//   - US-only in v1, so no Country field.
//   - Zip support is deferred to v2; no LookupByZip on the interface.
//   - ProviderCoverage is reserved for live values (Tock BusinessCount);
//     curated entries leave it nil and let hydration fill it.
//   - ParentMetro is per-provider because the same physical city can
//     be its own metro in one provider's catalog and a neighborhood in
//     another's. Example: Bellevue WA is its own Tock metro but rolls
//     under "seattle" in OpenTable's hierarchy.
//
// Lookups:
//   - Lookup(slug)       — exact match on canonical slug or any alias.
//   - LookupByName(name) — every Place sharing this display name
//                          (case-insensitive). Returns ALL matches so
//                          the resolver can disambiguate ambiguous
//                          city names ("Bellevue", "Portland",
//                          "Springfield", "Columbia") with explicit
//                          alternates rather than silently picking one.
//   - ReverseLookup(...) — smallest Place whose radius contains the
//                          point. Cities beat metros when both qualify
//                          because their radius is smaller; ties go to
//                          shortest haversine distance, then alpha
//                          slug for stability.
//   - All()              — full registry for "did you mean" / list UX.
//
// The registry chains a dynamic source (Tock SSR, disk-cached 24h)
// over the curated static fallback. If hydration fails or hasn't run
// yet, the curated table covers the major US metros plus the
// disambiguation-fixture cities (Bellevue/Portland/Springfield/Columbia)
// so the resolver always has alternates to surface.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// PlaceTier ranks the geographic granularity of a Place. Smaller-tier
// numbers don't mean "better" — the resolver uses tier to break ties
// and to phrase the disambiguation UX, not to filter. The
// ReverseLookup tiebreak prefers smaller RadiusKm regardless of tier
// since a city's 25 km radius mechanically beats a metro's 75 km when
// both contain the query point.
type PlaceTier int

const (
	PlaceTierUnknown PlaceTier = iota
	PlaceTierMetroCentroid
	PlaceTierCity
	PlaceTierNeighborhood
)

// Place is a single location entry. Aliases let one canonical Place
// answer to multiple human shorthands ("sf", "nyc") without
// proliferating rows. ParentMetro routes per-provider lookups:
// `ParentMetro["opentable"] = "seattle"` means "for OpenTable, this
// place's metro is seattle." ProviderCoverage carries live business
// counts where the provider surfaces them (Tock's BusinessCount); a
// nil map means "no live coverage data" and is the default for
// curated entries.
type Place struct {
	Slug             string            `json:"slug"`
	Name             string            `json:"name"`
	State            string            `json:"state,omitempty"`
	Lat              float64           `json:"lat"`
	Lng              float64           `json:"lng"`
	RadiusKm         float64           `json:"radius_km"`
	Population       int               `json:"population"`
	ProviderCoverage map[string]int    `json:"provider_coverage,omitempty"`
	ParentMetro      map[string]string `json:"parent_metro,omitempty"`
	Aliases          []string          `json:"aliases,omitempty"`
	ContextHints     []string          `json:"context_hints,omitempty"`
	Tier             PlaceTier         `json:"tier"`
}

// Metro is the legacy name for Place. Existing call sites in
// geo_filter.go, earliest.go, goat.go, and metro_hydration.go use
// `Metro{Slug: ..., Name: ..., Lat: ..., Lng: ..., Aliases: ...}` and
// `var x Metro` — Place is a strict superset so the alias keeps every
// call site compiling without per-file edits.
type Metro = Place

// PlaceRegistry is the lookup surface every consumer uses.
type PlaceRegistry interface {
	// Lookup returns the canonical Place matching slug or any alias.
	// Normalizes input (lowercase, trim) before matching. Empty input
	// returns (zero, false).
	Lookup(slug string) (Place, bool)

	// LookupByName returns every Place whose display Name matches
	// (case-insensitive) — used to enumerate ambiguous names like
	// "Bellevue", "Portland", "Springfield" so the resolver can offer
	// alternates instead of silently picking the most populous match.
	LookupByName(name string) ([]Place, bool)

	// All returns the full registry contents.
	All() []Place

	// ReverseLookup returns the smallest Place whose RadiusKm circle
	// contains (lat, lng). Tiebreak: smallest RadiusKm wins (cities
	// beat metros when both contain the point), then shortest
	// haversine distance, then alphabetical Slug.
	ReverseLookup(lat, lng float64) (Place, bool)
}

// MetroRegistry is the legacy name for PlaceRegistry. Existing code
// in geo_filter.go (`inferMetroFromSlug(... reg MetroRegistry)`) uses
// this name; the alias keeps it compiling.
type MetroRegistry = PlaceRegistry

// staticPlaceRegistry is the curated fallback. Always non-nil so the
// CLI never regresses on lookups when dynamic hydration hasn't run.
type staticPlaceRegistry struct{}

func (staticPlaceRegistry) All() []Place { return curatedPlaces }

func (staticPlaceRegistry) Lookup(slug string) (Place, bool) {
	return lookupIn(curatedPlaces, slug)
}

func (staticPlaceRegistry) LookupByName(name string) ([]Place, bool) {
	return lookupByNameIn(curatedPlaces, name)
}

func (staticPlaceRegistry) ReverseLookup(lat, lng float64) (Place, bool) {
	return reverseLookupIn(curatedPlaces, lat, lng)
}

// lookupIn searches places for a canonical-slug or alias match,
// case-insensitive after trimming. Shared by static and chained
// registries.
func lookupIn(places []Place, slug string) (Place, bool) {
	key := strings.ToLower(strings.TrimSpace(slug))
	if key == "" {
		return Place{}, false
	}
	for _, p := range places {
		if p.Slug == key {
			return p, true
		}
		for _, a := range p.Aliases {
			if a == key {
				return p, true
			}
		}
	}
	return Place{}, false
}

// PATCH: lookupbyname-alias-aware — `LookupByName` originally did strict
// exact-equal on display Name only; short forms already curated as
// `Aliases` ("nyc", "sf", "la", "dc", "weho", "bk") never resolved
// through the by-name path. U22 layered an alias-check on top of the
// exact-Name match so the by-name path honors the same curated alias
// surface `Lookup(slug)` does. Hyphen↔space normalization on both
// sides lets slug-style aliases ("new-york") interchange with natural-
// language input ("new york"). The dedup-by-slug guard is defensive —
// curated data avoids redundant Name+alias double-matches but a future
// entry that carries both still returns once. See
// .printing-press-patches.json entry `lookupbyname-alias-aware`.
//
// lookupByNameIn returns every Place that matches the query by display
// Name OR by curated alias, case-insensitive after trim. Order
// preserved from the input slice for determinism. Empty input or zero
// hits returns (nil, false).
func lookupByNameIn(places []Place, name string) ([]Place, bool) {
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return nil, false
	}
	keyNormalized := strings.ReplaceAll(key, "-", " ")

	var hits []Place
	seenSlug := make(map[string]bool)
	for _, p := range places {
		if seenSlug[p.Slug] {
			continue
		}
		// Strategy 1: exact display-Name match.
		if strings.ToLower(p.Name) == key {
			hits = append(hits, p)
			seenSlug[p.Slug] = true
			continue
		}
		// Strategy 2: curated-alias match, with hyphen↔space
		// normalization on both sides so slug-style and
		// natural-language forms interchange.
		for _, a := range p.Aliases {
			aKey := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(a)), "-", " ")
			if aKey == keyNormalized {
				hits = append(hits, p)
				seenSlug[p.Slug] = true
				break
			}
		}
	}
	if len(hits) == 0 {
		return nil, false
	}
	return hits, true
}

// reverseLookupIn picks the smallest Place whose radius covers
// (lat, lng). Tiebreak order: smallest RadiusKm, then smallest
// haversine distance, then alphabetical Slug.
func reverseLookupIn(places []Place, lat, lng float64) (Place, bool) {
	type candidate struct {
		place Place
		dist  float64
	}
	var hits []candidate
	for _, p := range places {
		if p.RadiusKm <= 0 {
			continue
		}
		d := haversineKm(lat, lng, p.Lat, p.Lng)
		if d <= p.RadiusKm {
			hits = append(hits, candidate{place: p, dist: d})
		}
	}
	if len(hits) == 0 {
		return Place{}, false
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].place.RadiusKm != hits[j].place.RadiusKm {
			return hits[i].place.RadiusKm < hits[j].place.RadiusKm
		}
		if hits[i].dist != hits[j].dist {
			return hits[i].dist < hits[j].dist
		}
		return hits[i].place.Slug < hits[j].place.Slug
	})
	return hits[0].place, true
}

// chainedPlaceRegistry composes a dynamic source (Tock SSR) over the
// curated static fallback using MERGE semantics — not chain-override.
//
// U13 Codex P1-C fix: the old chain shadowed curated entries by slug,
// so a dynamic Tock "Bellevue" (Slug="bellevue", State="", RadiusKm=75,
// no ContextHints) shadowed curated "Bellevue, WA"
// (State="WA", RadiusKm=25, ContextHints=["Seattle metro","Eastside",
// "tech hub"]). The agent-facing disambiguation UX lost the curated
// fields that made it useful. The merged registry preserves curated
// fields and only ENRICHES the provider-routing pair from dynamic:
//
//  1. Slug match → enrich curated entry's ProviderCoverage and
//     ParentMetro from dynamic. All other curated fields preserved.
//  2. Name (case-insensitive) AND haversine ≤ 5 km → same enrichment.
//  3. Neither matches → dynamic added as a separate row.
//
// `merged` is the precomputed slice; every lookup walks it directly.
type chainedPlaceRegistry struct {
	merged []Place
}

func (c chainedPlaceRegistry) Lookup(slug string) (Place, bool) {
	return lookupIn(c.merged, slug)
}

func (c chainedPlaceRegistry) LookupByName(name string) ([]Place, bool) {
	return lookupByNameIn(c.merged, name)
}

func (c chainedPlaceRegistry) All() []Place {
	return c.merged
}

func (c chainedPlaceRegistry) ReverseLookup(lat, lng float64) (Place, bool) {
	return reverseLookupIn(c.merged, lat, lng)
}

// mergeMatchKind discriminates the two enrichment paths so the caller
// knows whether to also preserve the dynamic slug as an alias. Slug-
// exact matches never need an alias (it would be a self-reference);
// name+coords matches do, so suffix-peel callers like
// inferMetroFromSlug_DEPRECATED can still resolve the dynamic slug to
// the merged canonical row.
type mergeMatchKind int

const (
	mergeMatchNone mergeMatchKind = iota
	mergeMatchSlug
	mergeMatchNameCoords
)

// mergeRegistry produces a NEW slice combining curated static entries
// with dynamic ones. The static slice is treated as read-only — copies
// are made before any field assignment so callers' source slices stay
// intact (the curated `place_data.go` var must never be mutated). The
// shallow `copy(out, static)` carries map and slice references through,
// so before any write the destination's ProviderCoverage, ParentMetro,
// and Aliases are deep-copied via cloneForEnrich (Codex U18 P2 fix).
//
// Merge rules per dynamic entry D:
//
//  1. If D.Slug matches a curated entry C.Slug, enrich C's
//     ProviderCoverage and ParentMetro from D. All other curated
//     fields (Name, State, Population, Lat, Lng, RadiusKm, Tier,
//     Aliases, ContextHints) are preserved.
//
//  2. Else if a curated entry has the same Name (case-insensitive) AND
//     haversineKm ≤ 5.0 from D's centroid, enrich as in rule 1 AND
//     append D.Slug to the merged entry's Aliases (Codex U18 P1 fix)
//     so `Lookup(dynamicSlug)` keeps returning the merged place. Without
//     this, slug-suffix peelers like inferMetroFromSlug_DEPRECATED stop
//     resolving venue slugs like `joey-bellevue` once Tock hydration
//     absorbs the bare `bellevue` row into curated `bellevue-wa`.
//
//  3. Else D is truly new — append it as-is.
//
// Curated entries that aren't matched by any dynamic entry pass
// through unchanged. The output preserves curated order for stability
// (the curated table's order is meaningful for ReverseLookup ties).
func mergeRegistry(static []Place, dynamic []Place) []Place {
	out := make([]Place, len(static))
	copy(out, static)
	for _, d := range dynamic {
		idx, kind := findMergeMatch(out, d)
		if kind == mergeMatchNone {
			out = append(out, d)
			continue
		}
		cloneForEnrich(&out[idx])
		enrichInPlace(&out[idx], d)
		if kind == mergeMatchNameCoords {
			addAliasIfMissing(&out[idx], d.Slug)
		}
	}
	return out
}

// findMergeMatch implements the slug-first, name+coords-second match
// rules. Returns the index in `places` of the matched entry plus the
// match kind, or (-1, mergeMatchNone). Slug match short-circuits — a
// dynamic entry whose slug already matches a curated row does NOT fall
// through to name+coords matching against a different row.
func findMergeMatch(places []Place, d Place) (int, mergeMatchKind) {
	if d.Slug != "" {
		for i, p := range places {
			if p.Slug == d.Slug {
				return i, mergeMatchSlug
			}
		}
	}
	if d.Name == "" {
		return -1, mergeMatchNone
	}
	for i, p := range places {
		if !strings.EqualFold(p.Name, d.Name) {
			continue
		}
		if haversineKm(p.Lat, p.Lng, d.Lat, d.Lng) <= 5.0 {
			return i, mergeMatchNameCoords
		}
	}
	return -1, mergeMatchNone
}

// cloneForEnrich replaces dst's ProviderCoverage, ParentMetro, and
// Aliases with fresh copies of the originals so subsequent writes don't
// leak back into the static source slice. The shallow `copy(out, static)`
// in mergeRegistry copies the Place values but their map/slice fields
// remain shared with the static source until this clone runs. Nil
// source maps stay nil here — enrichInPlace's lazy init still owns the
// "do we need an empty map" decision, so curated-only rows keep their
// nil-valued ProviderCoverage and the JSON shape is unchanged.
func cloneForEnrich(dst *Place) {
	if dst.ProviderCoverage != nil {
		fresh := make(map[string]int, len(dst.ProviderCoverage))
		for k, v := range dst.ProviderCoverage {
			fresh[k] = v
		}
		dst.ProviderCoverage = fresh
	}
	if dst.ParentMetro != nil {
		fresh := make(map[string]string, len(dst.ParentMetro))
		for k, v := range dst.ParentMetro {
			fresh[k] = v
		}
		dst.ParentMetro = fresh
	}
	if dst.Aliases != nil {
		fresh := make([]string, len(dst.Aliases))
		copy(fresh, dst.Aliases)
		dst.Aliases = fresh
	}
}

// addAliasIfMissing appends alias to dst.Aliases iff it's non-empty,
// not equal to dst.Slug (no self-reference), and not already present.
// Caller must have already invoked cloneForEnrich so the append targets
// the merged copy, not the static source slice.
func addAliasIfMissing(dst *Place, alias string) {
	if alias == "" || alias == dst.Slug {
		return
	}
	for _, a := range dst.Aliases {
		if a == alias {
			return
		}
	}
	dst.Aliases = append(dst.Aliases, alias)
}

// enrichInPlace copies the live provider-routing fields from dynamic
// into curated. Initializes nil destination maps lazily so the first
// "tock" entry doesn't panic. Source nil maps short-circuit — there's
// nothing to copy, and we don't create an empty destination map (a
// nil-valued ProviderCoverage stays nil so JSON marshaling stays
// stable for curated-only rows).
func enrichInPlace(dst *Place, src Place) {
	for k, v := range src.ProviderCoverage {
		if dst.ProviderCoverage == nil {
			dst.ProviderCoverage = make(map[string]int)
		}
		dst.ProviderCoverage[k] = v
	}
	for k, v := range src.ParentMetro {
		if dst.ParentMetro == nil {
			dst.ParentMetro = make(map[string]string)
		}
		dst.ParentMetro[k] = v
	}
}

// staticMetroRegistry is the legacy name for staticPlaceRegistry,
// kept for the metro_hydration_test.go fallback assertions and any
// other in-package callers that haven't migrated yet.
type staticMetroRegistry = staticPlaceRegistry

// chainedMetroRegistry is the legacy name for chainedPlaceRegistry.
type chainedMetroRegistry = chainedPlaceRegistry

// defaultRegistry singleton. Starts as the curated fallback; upgrades
// to the chained registry when dynamic places are loaded. Access is
// guarded by registryMu so concurrent CLI invocations don't race.
var (
	registryMu      sync.RWMutex
	defaultReg      PlaceRegistry = staticPlaceRegistry{}
	dynamicLoadedAt int64         // unix seconds; 0 until first successful load
)

// getRegistry returns the current registry. Always non-nil.
func getRegistry() PlaceRegistry {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return defaultReg
}

// setDynamicMetros upgrades the registry to merged mode with the
// supplied dynamic entries. Safe to call concurrently — last writer
// wins. Pass nil/empty to revert to the curated-only fallback.
//
// The merge runs here (at hydration time), not at query time, so the
// merged snapshot is computed once and every lookup walks a single
// slice. See mergeRegistry for the slug-then-name+coords rules.
//
// Named `setDynamicMetros` (not `setDynamicPlaces`) for backward
// compat with existing test fixtures; the function now operates on
// Place values via the type alias.
func setDynamicMetros(places []Place, loadedAtUnix int64) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if len(places) == 0 {
		defaultReg = staticPlaceRegistry{}
		dynamicLoadedAt = 0
		return
	}
	defaultReg = chainedPlaceRegistry{merged: mergeRegistry(curatedPlaces, places)}
	dynamicLoadedAt = loadedAtUnix
}

// metroLatLng is the legacy lookup shape some pre-redesign callers
// still use. Returns (0, 0, false) on unknown slug.
func metroLatLng(slug string) (lat, lng float64, ok bool) {
	p, found := getRegistry().Lookup(slug)
	if !found {
		return 0, 0, false
	}
	return p.Lat, p.Lng, true
}

// metroCityName mirrors the legacy display-name lookup. Returns "" on
// unknown slug so existing empty-string fallbacks keep working.
func metroCityName(slug string) string {
	p, ok := getRegistry().Lookup(slug)
	if !ok {
		return ""
	}
	return p.Name
}

// knownMetros returns the registry's canonical slugs, sorted, for
// stable error-message output.
func knownMetros() []string {
	all := getRegistry().All()
	slugs := make([]string, 0, len(all))
	for _, p := range all {
		slugs = append(slugs, p.Slug)
	}
	sort.Strings(slugs)
	return slugs
}

// titleCase uppercases the first rune of s. Replaces strings.Title
// (deprecated). Used by formatUnknownMetroError to render "bellevue"
// → "Bellevue" in error messages.
func titleCase(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// suggestMetros returns up to maxN slugs that share a token with the
// query. Best-effort "did you mean" UX for the unknown-metro error.
func suggestMetros(query string, maxN int) []string {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	type scored struct {
		slug  string
		score int
	}
	var hits []scored
	for _, p := range getRegistry().All() {
		score := 0
		ps := strings.ToLower(p.Slug)
		if strings.Contains(ps, q) {
			score += 10
		}
		for _, tok := range strings.Split(q, "-") {
			if tok == "" {
				continue
			}
			if strings.Contains(ps, tok) {
				score++
			}
		}
		for _, a := range p.Aliases {
			al := strings.ToLower(a)
			if al == q || strings.Contains(al, q) {
				score += 5
			}
		}
		if score > 0 {
			hits = append(hits, scored{slug: p.Slug, score: score})
		}
	}
	if len(hits) == 0 {
		return nil
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].slug < hits[j].slug
	})
	out := make([]string, 0, maxN)
	for i := 0; i < len(hits) && i < maxN; i++ {
		out = append(out, hits[i].slug)
	}
	return out
}

// cityHints maps neighborhoods / secondary cities onto the metro
// slug their venues actually appear under in OpenTable's catalog.
// This is the pre-redesign disambiguation map; the new Place
// registry supersedes it for any city with its own curated entry,
// but the map stays for places that aren't worth a full Place row
// (Redmond, Kirkland, etc.).
var cityHints = map[string]string{
	// Seattle / Eastside
	"redmond":   "seattle",
	"kirkland":  "seattle",
	"issaquah":  "seattle",
	"renton":    "seattle",
	"sammamish": "seattle",
	// Bay Area
	"oakland":   "san-francisco",
	"berkeley":  "san-francisco",
	"alameda":   "san-francisco",
	"san-mateo": "san-francisco",
	"daly-city": "san-francisco",
	// NYC outer boroughs / NJ commuter
	"brooklyn":         "new-york-city",
	"queens":           "new-york-city",
	"bronx":            "new-york-city",
	"staten-island":    "new-york-city",
	"long-island-city": "new-york-city",
	"hoboken":          "new-york-city",
	"jersey-city":      "new-york-city",
	// LA area
	"santa-monica":  "los-angeles",
	"pasadena":      "los-angeles",
	"beverly-hills": "los-angeles",
	"venice":        "los-angeles",
	"culver-city":   "los-angeles",
	// Boston area
	"cambridge":  "boston",
	"somerville": "boston",
	"newton":     "boston",
	"brookline":  "boston",
	// DC area
	"arlington":     "washington-dc",
	"alexandria":    "washington-dc",
	"bethesda":      "washington-dc",
	"silver-spring": "washington-dc",
	// Chicago area
	"evanston": "chicago",
	"oak-park": "chicago",
}

// cityHintFor returns the metro slug a neighborhood is lumped under,
// or "" if there's no hint. Case-insensitive after trim. If the
// input has its own curated Place entry, prefer that — the registry
// is the canonical source. cityHints is only consulted by the
// unknown-metro error fallback.
func cityHintFor(slug string) string {
	return cityHints[strings.ToLower(strings.TrimSpace(slug))]
}

// formatUnknownMetroError builds a readable error for `--metro <slug>`
// when the lookup misses. Three layers:
//  1. If the slug is a known secondary city (cityHints), point at the
//     parent metro with a radius hint.
//  2. Else if registry entries share tokens with the input, show
//     them as "did you mean".
//  3. Else fall back to the count + sample.
func formatUnknownMetroError(input string) string {
	if parent := cityHintFor(input); parent != "" {
		if p, ok := getRegistry().Lookup(parent); ok {
			cityName := titleCase(input)
			return fmt.Sprintf(
				"unknown metro %q — neither OpenTable nor Tock breaks this out as its own metro. "+
					"%s is lumped under metro %q (centroid %.4f, %.4f). "+
					"Try `--metro %s --metro-radius-km 20` to constrain results to %s-area venues, "+
					"or pass `--latitude %.4f --longitude %.4f` directly with a tight `--metro-radius-km`.",
				input, cityName, p.Slug, p.Lat, p.Lng, p.Slug, cityName, p.Lat, p.Lng,
			)
		}
	}
	suggestions := suggestMetros(input, 5)
	if len(suggestions) > 0 {
		return fmt.Sprintf("unknown metro %q (did you mean: %s? — %d metros known total; pass `--list-metros` to see them all)",
			input, strings.Join(suggestions, ", "), len(getRegistry().All()))
	}
	all := getRegistry().All()
	sample := make([]string, 0, 10)
	for i := 0; i < len(all) && i < 10; i++ {
		sample = append(sample, all[i].Slug)
	}
	return fmt.Sprintf("unknown metro %q (no similar entries in registry; %d metros known — pass `--list-metros` to see them all, sample: %s, ...)",
		input, len(all), strings.Join(sample, ", "))
}

// hydrateMetroRegistry is a generic best-effort dynamic loader hook
// used by tests and any future non-Tock provider. On any failure the
// curated fallback stays in place — no hard error.
func hydrateMetroRegistry(ctx context.Context, load func(ctx context.Context) ([]Place, int64, error)) {
	if load == nil {
		return
	}
	places, loadedAt, err := load(ctx)
	if err != nil || len(places) == 0 {
		return
	}
	setDynamicMetros(places, loadedAt)
}
