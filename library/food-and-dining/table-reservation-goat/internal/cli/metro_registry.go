// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: scaffold-endpoint-redirects — issue #406 failures 1 + 3.
//
// Metro registry — single source of truth for `--metro <slug>` lookups
// across `goat`, `earliest`, `availability check`, and `restaurants list`.
//
// Background: prior versions hardcoded a 20-entry switch statement (Seattle,
// Chicago, NYC, …). That static table missed every secondary metro Tock
// hydrates in `state.app.config.metroArea` (253 cities), so users asking
// for `--metro bellevue` (Bellevue WA — a real Tock metro) got `unknown
// metro` even though the data was right there in the Tock SSR they'd
// already fetched.
//
// Shape: a Metro carries a canonical slug, a display Name (Tock's
// `?city=` query param shape — preserves spaces + casing), a centroid
// (Lat/Lng for OpenTable Autocomplete + geo-filter math), and a slice
// of Aliases for human shorthand (`sf`, `nyc`, `dc`).
//
// Lookups:
//   - Lookup(slug) — exact match on canonical slug or any alias
//   - KnownSlugs() — for error-message "did you mean …" UX
//
// The registry chains a dynamic source (loaded from Tock's metroArea
// SSR, disk-cached 24h) over the static fallback. If hydration fails or
// hasn't run yet, the static fallback covers the 20 most common US
// metros so the CLI never regresses below the prior baseline.

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// Metro is a single metro-area entry: canonical slug, display name, and
// centroid coordinates. Aliases let a single canonical entry answer to
// multiple human shorthands ("sf", "san-francisco") without proliferating
// rows in the registry.
type Metro struct {
	Slug    string   `json:"slug"`              // canonical lookup key (e.g. "san-francisco")
	Name    string   `json:"name"`              // Tock `?city=` display shape (e.g. "San Francisco")
	Lat     float64  `json:"lat"`               // metro centroid latitude
	Lng     float64  `json:"lng"`               // metro centroid longitude
	Aliases []string `json:"aliases,omitempty"` // additional accepted lookup keys (e.g. ["sf"])
}

// MetroRegistry is the lookup surface every consumer uses. Concrete
// implementations: staticMetroRegistry (compile-time fallback) and
// chainedMetroRegistry (dynamic-over-static at runtime).
type MetroRegistry interface {
	// Lookup returns the canonical Metro matching the slug or any alias.
	// Normalizes input (lowercases, trims) before matching.
	Lookup(slug string) (Metro, bool)

	// All returns the full registry contents — used by `--help` error
	// messages and the "did you mean" UX.
	All() []Metro
}

// staticMetros is the 20-entry US-centric fallback registry. Replaces
// the prior metroLatLng + metroCityName switch pair with a single
// declarative table.
var staticMetros = []Metro{
	{Slug: "seattle", Name: "Seattle", Lat: 47.6062, Lng: -122.3321},
	{Slug: "chicago", Name: "Chicago", Lat: 41.8781, Lng: -87.6298},
	{Slug: "new-york-city", Name: "New York City", Lat: 40.7589, Lng: -73.9851,
		Aliases: []string{"new-york", "nyc", "manhattan"}},
	{Slug: "san-francisco", Name: "San Francisco", Lat: 37.7749, Lng: -122.4194,
		Aliases: []string{"sf"}},
	{Slug: "los-angeles", Name: "Los Angeles", Lat: 34.0522, Lng: -118.2437,
		Aliases: []string{"la"}},
	{Slug: "miami", Name: "Miami", Lat: 25.7617, Lng: -80.1918},
	{Slug: "boston", Name: "Boston", Lat: 42.3601, Lng: -71.0589},
	{Slug: "washington-dc", Name: "Washington DC", Lat: 38.9072, Lng: -77.0369,
		Aliases: []string{"dc", "washington"}},
	{Slug: "austin", Name: "Austin", Lat: 30.2672, Lng: -97.7431},
	{Slug: "portland", Name: "Portland", Lat: 45.5152, Lng: -122.6784},
	{Slug: "denver", Name: "Denver", Lat: 39.7392, Lng: -104.9903},
	{Slug: "philadelphia", Name: "Philadelphia", Lat: 39.9526, Lng: -75.1652,
		Aliases: []string{"philly"}},
	{Slug: "atlanta", Name: "Atlanta", Lat: 33.7490, Lng: -84.3880},
	{Slug: "houston", Name: "Houston", Lat: 29.7604, Lng: -95.3698},
	{Slug: "dallas", Name: "Dallas", Lat: 32.7767, Lng: -96.7970},
	{Slug: "san-diego", Name: "San Diego", Lat: 32.7157, Lng: -117.1611},
	{Slug: "minneapolis", Name: "Minneapolis", Lat: 44.9778, Lng: -93.2650},
	{Slug: "nashville", Name: "Nashville", Lat: 36.1627, Lng: -86.7816},
	{Slug: "new-orleans", Name: "New Orleans", Lat: 29.9511, Lng: -90.0715,
		Aliases: []string{"nola"}},
	{Slug: "las-vegas", Name: "Las Vegas", Lat: 36.1699, Lng: -115.1398,
		Aliases: []string{"vegas"}},
}

// staticMetroRegistry is the compile-time fallback. Always non-nil so
// even when dynamic hydration fails the CLI behaves identically to
// pre-issue-#406 versions for the 20 covered metros.
type staticMetroRegistry struct{}

func (staticMetroRegistry) All() []Metro { return staticMetros }

func (s staticMetroRegistry) Lookup(slug string) (Metro, bool) {
	return lookupIn(staticMetros, slug)
}

// lookupIn searches metros for a canonical-slug match or alias match,
// case-insensitive after trimming whitespace. Shared by static and
// chained registries.
func lookupIn(metros []Metro, slug string) (Metro, bool) {
	key := strings.ToLower(strings.TrimSpace(slug))
	if key == "" {
		return Metro{}, false
	}
	for _, m := range metros {
		if m.Slug == key {
			return m, true
		}
		for _, a := range m.Aliases {
			if a == key {
				return m, true
			}
		}
	}
	return Metro{}, false
}

// chainedMetroRegistry composes a dynamic source over the static
// fallback: Lookup tries dynamic first, returns the static match if
// dynamic doesn't have it. All() returns the union (dynamic first,
// then static entries the dynamic source didn't already cover).
type chainedMetroRegistry struct {
	dynamic []Metro // hydrated from Tock metroArea (may be empty)
}

func (c chainedMetroRegistry) Lookup(slug string) (Metro, bool) {
	if m, ok := lookupIn(c.dynamic, slug); ok {
		return m, true
	}
	return lookupIn(staticMetros, slug)
}

func (c chainedMetroRegistry) All() []Metro {
	if len(c.dynamic) == 0 {
		return staticMetros
	}
	// Dynamic-first union: static entries are appended only when their
	// canonical slug isn't already represented in the dynamic source.
	seen := make(map[string]struct{}, len(c.dynamic))
	out := make([]Metro, 0, len(c.dynamic)+len(staticMetros))
	for _, m := range c.dynamic {
		seen[m.Slug] = struct{}{}
		out = append(out, m)
	}
	for _, m := range staticMetros {
		if _, ok := seen[m.Slug]; ok {
			continue
		}
		out = append(out, m)
	}
	return out
}

// defaultRegistry is the package-wide registry singleton. It starts as
// the static-only fallback and gets upgraded to chained once dynamic
// metros are loaded. Access is guarded by registryMu — concurrent
// `goat` invocations may race to upgrade.
var (
	registryMu      sync.RWMutex
	defaultReg      MetroRegistry = staticMetroRegistry{}
	dynamicLoadedAt int64         // unix seconds; 0 until first successful load
)

// getRegistry returns the current registry (caller may hold the read
// lock for the duration of a single lookup). Always non-nil.
func getRegistry() MetroRegistry {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return defaultReg
}

// setDynamicMetros upgrades the default registry to a chained registry
// with the supplied dynamic entries. Safe to call concurrently — last
// writer wins. Pass nil/empty to revert to the static-only fallback.
func setDynamicMetros(metros []Metro, loadedAtUnix int64) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if len(metros) == 0 {
		defaultReg = staticMetroRegistry{}
		dynamicLoadedAt = 0
		return
	}
	defaultReg = chainedMetroRegistry{dynamic: metros}
	dynamicLoadedAt = loadedAtUnix
}

// metroLatLng is the legacy-shape wrapper for code that still calls the
// pre-#406 lookup signature. New code should use getRegistry().Lookup
// directly so the canonical Metro (with display name + aliases) is
// available to format error messages and inform the geo filter.
func metroLatLng(slug string) (lat, lng float64, ok bool) {
	m, found := getRegistry().Lookup(slug)
	if !found {
		return 0, 0, false
	}
	return m.Lat, m.Lng, true
}

// metroCityName mirrors the legacy display-name lookup. Returns "" on
// unknown slug so existing callers' empty-string fallbacks still work.
func metroCityName(slug string) string {
	m, ok := getRegistry().Lookup(slug)
	if !ok {
		return ""
	}
	return m.Name
}

// knownMetros returns the set of canonical slugs the registry currently
// covers, sorted alphabetically for stable error-message output.
func knownMetros() []string {
	all := getRegistry().All()
	slugs := make([]string, 0, len(all))
	for _, m := range all {
		slugs = append(slugs, m.Slug)
	}
	sort.Strings(slugs)
	return slugs
}

// titleCase uppercases the first rune of s. Replaces strings.Title
// (deprecated in Go 1.18) for the city-hint UX where we want
// "bellevue" → "Bellevue" in error messages. Doesn't try to handle
// hyphens or multi-word names — those aren't in the cityHints map
// keys today and city display names come from the registry directly.
func titleCase(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// suggestMetros returns up to maxN slugs that share a token with `query`
// (best-effort "did you mean"). Used to keep the unknown-metro error
// message readable when the registry has 200+ entries — dumping the
// full list overwhelms terminals.
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
	for _, m := range getRegistry().All() {
		score := 0
		ms := strings.ToLower(m.Slug)
		// Full-substring of query in slug → strong signal.
		if strings.Contains(ms, q) {
			score += 10
		}
		// Any shared token from query split on hyphen → weaker signal.
		for _, tok := range strings.Split(q, "-") {
			if tok == "" {
				continue
			}
			if strings.Contains(ms, tok) {
				score++
			}
		}
		// Check aliases too.
		for _, a := range m.Aliases {
			al := strings.ToLower(a)
			if al == q || strings.Contains(al, q) {
				score += 5
			}
		}
		if score > 0 {
			hits = append(hits, scored{slug: m.Slug, score: score})
		}
	}
	if len(hits) == 0 {
		return nil
	}
	// Sort by score desc, slug asc (stable).
	for i := 1; i < len(hits); i++ {
		for j := i; j > 0 && (hits[j].score > hits[j-1].score ||
			(hits[j].score == hits[j-1].score && hits[j].slug < hits[j-1].slug)); j-- {
			hits[j], hits[j-1] = hits[j-1], hits[j]
		}
	}
	out := make([]string, 0, maxN)
	for i := 0; i < len(hits) && i < maxN; i++ {
		out = append(out, hits[i].slug)
	}
	return out
}

// cityHints maps cities that aren't standalone Tock/registry metros
// onto the metro they're handled under. Issue #406's Bellevue WA case:
// Tock's `metroArea` config lumps Eastside venues into `seattle`, so
// agents typing `--metro bellevue` need to be routed to "use seattle
// with a tight radius."
//
// These are static facts about how reservation networks group
// neighborhoods — they won't change with Tock's metroArea hydration.
// Kept small and US-focused (the metros most likely to surface in
// agent prompts).
var cityHints = map[string]string{
	// Seattle / Eastern WA
	"bellevue":  "seattle",
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

// cityHintFor returns the metro slug a city is lumped under, or "" if
// the city has no hint mapping. Case-insensitive after trim.
func cityHintFor(slug string) string {
	return cityHints[strings.ToLower(strings.TrimSpace(slug))]
}

// formatUnknownMetroError builds a readable error for `--metro <slug>`
// when the lookup misses. Three layers of helpfulness:
//  1. If the slug is a known secondary city (Bellevue, Oakland, etc.),
//     point at the parent metro with a radius hint — this is the
//     "best advice" for issue #406's Bellevue WA case.
//  2. Else if there are name-similar entries in the registry, show
//     them as "did you mean: ...".
//  3. Else fall back to the count + sample.
func formatUnknownMetroError(input string) string {
	if parent := cityHintFor(input); parent != "" {
		if m, ok := getRegistry().Lookup(parent); ok {
			cityName := titleCase(input)
			return fmt.Sprintf(
				"unknown metro %q — neither OpenTable nor Tock breaks this out as its own metro. "+
					"%s is lumped under metro %q (centroid %.4f, %.4f). "+
					"Try `--metro %s --metro-radius-km 20` to constrain results to %s-area venues, "+
					"or pass `--latitude %.4f --longitude %.4f` directly with a tight `--metro-radius-km`.",
				input, cityName, m.Slug, m.Lat, m.Lng, m.Slug, cityName, m.Lat, m.Lng,
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

// hydrateMetroRegistry attempts a best-effort dynamic load. Designed to
// be called once per CLI invocation (before the first metro lookup) so
// the registry has the 253-metro Tock list available when present. On
// any failure (no Tock cookies, Akamai blocks the SSR fetch, JSON parse
// error) the static fallback stays in place — no hard error.
func hydrateMetroRegistry(ctx context.Context, load func(ctx context.Context) ([]Metro, int64, error)) {
	if load == nil {
		return
	}
	metros, loadedAt, err := load(ctx)
	if err != nil || len(metros) == 0 {
		return
	}
	setDynamicMetros(metros, loadedAt)
}
