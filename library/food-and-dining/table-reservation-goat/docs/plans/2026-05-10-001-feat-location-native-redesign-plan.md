---
title: Location-Native Redesign — Typed GeoContext, Ambiguity-Aware Tiers, Disambiguation Envelope
type: feat
status: active
date: 2026-05-10
deepened: 2026-05-10
---

# Location-Native Redesign — Typed GeoContext, Ambiguity-Aware Tiers, Disambiguation Envelope

## Summary

Make location a typed first-class concept across the CLI: a `GeoContext` flowing through every read command (`restaurants list`, `availability check`, `availability multi-day`, `earliest`, `goat`, `watch`), an **ambiguity-aware** tier decision that refuses to silently disambiguate, two plain provider-projection functions (no speculative interface), and a typed envelope so the chat agent or user becomes the final resolver. Defense in depth is radius-based (using existing `haversineKm`) — polygon containment, session pinning, and offline polygon generation are deferred to v2 once v1 ships and we have production signal. Execution is test-first per unit, with issue #406 bug-report reproductions plus added ambiguity-coverage fixtures as load-bearing acceptance tests.

---

## Problem Frame

PRs #423/#424/#425/#426 fixed the *named* failure modes in issue #406 but real-world testing on 2026-05-10 still produces embarrassing wrong-region results:

- `restaurants list --query 'sushi bellevue' --metro seattle --party 2` returns NYC sushi venues. `internal/cli/restaurants_list.go:104` has `_ = flagMetro` — the flag is parsed and discarded.
- `availability check 'I Love Sushi Bellevue'` resolves to "Ikyu Sushi II" in Manhattan. `availability_check.go` has no `--metro` flag and the underlying path hardcodes NYC coordinates.
- `inferMetroFromSlug` (in `geo_filter.go`) only handles hyphenated suffixes (`joey-bellevue`). Natural-language input like `"I Love Sushi Bellevue"` is one token after `strings.Split(slug, "-")`.
- `internal/cli/watch.go:367` has the same hardcoded NYC fallback (`c.RestaurantIDFromQuery(ctx, slug, 40.7128, -74.0060)`) the comment explicitly says "Default to NYC — same approach earliest.go uses." Any `watch <slug>` call silently anchors to Manhattan.

The root cause is architectural, not a per-command slug-suffix corner: location is *a flag bolted onto each command*, not a *typed concept flowing through the read pipeline*. Each command has its own ad-hoc handling; none of them constrain results to the user's stated region; and the slug-suffix inference assumes a CLI-shaped input that real agent calls don't provide.

A second pressure: the chat agent calling our CLI has context (prior conversation, user mentions of Seattle/PNW, calendar) that the CLI will never have. Today we ignore the asymmetry and silently pick one match. The redesign treats the agent as a first-class resolver — when input is ambiguous, the CLI returns structured candidates; the agent applies conversation context or asks the user.

---

## Requirements

- R1. Free-form `--location` flag on every read command (`restaurants list`, `availability check`, `availability multi-day`, `earliest`, `goat`, `watch`). Accepted shapes: bare city (`bellevue`), city+state (`bellevue, wa`), metro (`seattle metro`), coords (`47.62,-122.20`). Zip support deferred to v2 (no zip→Place data path in the hand-curated registry; `"98004"` parses as `LocCity{Name: "98004"}` and returns `location_unknown`).
- R2. Bare ambiguous inputs never silently resolve. Tier decision is **ambiguity-aware** — driven by candidate-count + input specificity, with the popularity prior used only to *rank within* the candidate list (not to drive tier).
- R3. Low-confidence resolution returns a typed `needs_clarification` envelope: ranked candidates with `state`, `context_hints` (`"Seattle metro"`, `"Omaha metro"`), `ot_venue_count`, `tock_business_count`, `centroid`, `score_if_picked`. The envelope omits any prose `fallback_clarification`; the agent synthesizes its own phrasing from structured fields (avoids locale/voice coupling).
- R4. Medium-confidence resolution returns results plus a `location_warning` field listing alternates.
- R5. High-confidence resolution returns results plus a `location_resolved` field showing the pick, reason, and considered alternates.
- R6. `GeoContext.ForOpenTable() opentable.LocationInput` and `GeoContext.ForTock() tock.LocationInput` are two plain projection functions defined alongside `GeoContext` in the CLI package. No interface in v1 — extract one when a third provider is added.
- R7. Defense-in-depth radius post-filter: every result returned to the user is verified within a per-place radius (typically 25 km city / 75 km metro) around the resolved `GeoContext.Centroid` using existing `haversineKm`. Hard-reject when `source = explicit_flag`; soft-demote when `source = extracted_from_query`. Numeric-ID inputs to `availability check` are exempt from hard-reject — the venue still returns with a `location_warning` if the venue is outside radius.
- R8. `location resolve <input>` exposes the resolver as a primitive agents can call independently. Single sub-command; no `set`/`unset`/`current`/`list` (deferred).
- R9. Typed `error_kind` enum across location failures: `location_unknown`, `location_ambiguous`, `venue_ambiguous`, `no_results_in_region`, `results_only_outside_region`. Each value paired with a distinct agent recovery path in SKILL.md.
- R10. SKILL.md documents three agent rules: (a) always check `location_resolved.confidence` in successful responses, (b) on `needs_clarification: true` use conversation context first then ask the user, (c) never silently accept low-confidence picks — surface to user in your reply.
- R11. `--accept-ambiguous` opt-in flag bypasses disambiguation for batch/test use; wired in the same pass as `--location` for each command. Default is disambiguate.
- R12. `--metro` flag preserved as deprecated alias. To avoid breaking existing JSON parsers, `--metro <value>` implies `--accept-ambiguous` — legacy callers always receive results-shaped responses (with a `location_warning` when ambiguity was forced), never the new envelope.
- R13. Empty `--location ""` is uniformly treated as "no location constraint" across all commands (no envelope, no pin lookup, no fallback to NYC). Same path as the `--location` flag being absent.
- R14. Bug-report reproductions and ambiguity-coverage fixtures pass as integration tests:
  - **F1**: `restaurants list --query 'sushi bellevue' --location seattle` returns Seattle/Bellevue-area venues, zero NYC venues; response includes `location_resolved.confidence > 0.85` (covers HIGH-tier wiring; replaces the originally-listed F4).
  - **F2**: `availability check 'I Love Sushi Bellevue' --location 'bellevue, wa'` resolves to a Bellevue-area I Love Sushi, OR returns `venue_ambiguous` envelope; never silently picks Manhattan.
  - **F3**: `restaurants list --query 'sushi' --location bellevue` returns a `needs_clarification` envelope with WA/NE/KY candidates (LOW tier).
  - **F4**: `restaurants list --query 'pasta' --location portland` returns the `needs_clarification` envelope with Portland OR and Portland ME as candidates ranked by popularity prior. U14 simplification: bare LocCity with 2+ candidates routes to LOW regardless of population gap, so the agent disambiguates instead of risking a wrong-city silent pick (Codex P2-F/P2-G fix).
  - **F5**: `restaurants list --query 'pizza' --location springfield` returns `needs_clarification` envelope with 4 candidates (MA, IL, MO, OR).
  - **F6**: `restaurants list --query 'tacos' --location 'columbia, sc'` returns HIGH (specific city+state, no ambiguity).
  - **F7**: `watch <slug>` honors `--location`; no silent NYC anchor when location is provided; if resolved venue is outside the location's radius, watch starts with a stderr `location_warning` line and continues (does not refuse).

---

## Scope Boundaries

- GPS or device-location detection (we don't have it).
- Changing OT/Tock client signatures beyond what each provider currently accepts (lat/lng for OT, City + lat/lng for Tock).
- The booking flow (separate plan at `docs/plans/2026-05-09-003-feat-booking-flow-free-reservations-plan.md`).
- International expansion (US-only initial scope).
- Paid external geocoding services (Google Maps, Mapbox, etc.).
- Renaming or restructuring existing commands — additive only.

### Deferred to Follow-Up Work

- **Session pin** (`location set/unset/current` sub-commands; `<UserCacheDir>/.../location-pin.json`). Originally drafted for v1; deferred because every R14 fixture passes explicit `--location`. Session-memory is a real user win once we know the resolver is working — revisit after v1 ships with production signal.
- **Polygon containment** (TIGER-derived admin boundaries, ray-casting point-in-polygon, multi-ring handling, `place_data_gen.go` offline generator). Deferred in favor of radius-based post-filter using existing `haversineKm`. Polygons are nicer for non-circular metros (LA, NYC) but radius covers R14 fixtures and the polygon stack is significant complexity (TIGER ingestion, shapefile lib, vertex counts, multi-ring logic). Revisit when radius produces a real false-positive bug.
- **Zip code resolution** (R1's accepted-shapes list originally included `98004`). Deferred because there is no zip→Place data path in the hand-curated registry, and adding a `Zips []string` field to Place plus per-city zip lists across ~80 entries is real data-collection work. v1 parses zip-shaped inputs as `LocCity` (falls through to registry by-name lookup; almost always returns `location_unknown`). Agents reading SKILL.md (U10) are advised to pass city or coords for v1.
- **`sync`-backed local geo index**. Extending `sync` to populate point-in-polygon (or even radius-based) containment in the SQLite store for offline / SQL-deterministic queries.
- **NLP extraction for `goat`**. Parsing `goat 'sushi in bellevue tonight'` into structured signals.
- **OT `MetroID` enrichment**. Hand-maintained slug→OT-MetroID mapping if lat/lng-only pre-filter proves insufficient.
- **`location list <query>` sub-command**. Duplicates the envelope's candidates list; agents can call `location resolve` to see candidates.
- **OT coverage prior**. Deferred — no reliable way to enumerate OT venues per metro from the existing client surface (Autocomplete is typeahead, not full-list; `SearchRestaurants` requires pagination that's brittle). v1 collapses to a population-driven popularity prior plus Tock's live `BusinessCount` for the Tock side. Adding OT coverage as a prior comes back when we have a real listing endpoint or a periodic scrape.
- **Periodic refresh of Tock coverage priors**. v1 reads Tock `BusinessCount` live from SSR hydration on every invocation; no refresh job needed.

---

## Context & Research

### Relevant Code and Patterns

- **Existing metro registry**: `internal/cli/metro_registry.go` carries `Metro{Slug, Name, Lat, Lng, Aliases}` — extended into a richer `Place` struct in U3.
- **Existing geo filter**: `internal/cli/geo_filter.go` has `haversineKm` (used unchanged), `applyGeoFilter` (extended), and `inferMetroFromSlug` (replaced by U2's parser).
- **OT `Autocomplete` signature**: `internal/source/opentable/client.go:401` — `Autocomplete(ctx, term, lat, lng)`. Lat/lng only; OT's `MetroID` (in `SearchRestaurants` at `internal/source/opentable/ssr.go:138`) is currently unused.
- **Tock `SearchCity`**: `internal/source/tock/search.go:100` — requires `SearchParams{City, Lat, Lng, ...}` where `City` is the display name (also drives `/search/<city-slug>` path).
- **Tock `MetroArea`**: `internal/source/tock/search.go:196` — `{Slug, Name, Lat, Lng, BusinessCount}`. `BusinessCount` is a built-in coverage prior we use directly.
- **`watch.go:367`** — `c.RestaurantIDFromQuery(ctx, slug, 40.7128, -74.0060)` with comment "Default to NYC — same approach earliest.go uses". Replaced via U8.
- **`earliest.go`** — `resolveOTSlugGeoAware` (`internal/cli/earliest.go:442`) and `resolveEarliestForVenue` carry the existing geo path; extended via U8.

### Institutional Learnings

- `docs/solutions/` is empty; no prior solutions apply. Add learnings post-implementation if novel patterns emerge.

### External References

- US Census Bureau (Wikipedia population data for the curated `Place` priors). No shapefile/TIGER ingestion in v1; radii are computed from centroid + a per-place `RadiusKm` field.

---

## Key Technical Decisions

- **`GeoContext` is a struct, not an interface**, in `internal/cli/geo_context.go`. Provider projection is two methods on the struct (`ForOpenTable() opentable.LocationInput`, `ForTock() tock.LocationInput`), defined in `internal/cli/geo_context.go` — same package, no import cycle. Provider-specific input types live in their own packages (`opentable.LocationInput`, `tock.LocationInput`) and are imported by `geo_context.go`. When a third provider is added, extract the interface at that point.
- **Tier decision is ambiguity-aware, not popularity-driven.** Per-place `popularity_prior` (log-normalized population + Tock `BusinessCount` when hydrated + tier bonus + match bonus) ranks candidates *within* a result. OT coverage is dropped from the prior in v1 — no reliable enumeration path exists. Tier (`HIGH`/`MEDIUM`/`LOW`) is decided by `(input_specificity, candidate_count, margin_between_top_and_runner_up)`. A bare ambiguous input with 3+ candidates is LOW regardless of which one dominates by popularity.
- **`--metro` implies `--accept-ambiguous`** to preserve the existing result-shape contract for legacy callers. Without this, `--metro bellevue` would suddenly start returning envelopes — a breaking JSON-shape change.
- **Defense in depth is radius-only in v1.** `applyGeoFilter` extended to accept per-place `RadiusKm` (city: ~25km, metro: ~75km, neighborhood: ~10km). Existing `haversineKm` math is reused untouched.
- **Empty `--location ""` is "no constraint"**, uniformly. `ResolveLocation("", ...)` returns `(nil, nil, nil)` — meaning "the caller has not requested a location filter." Commands skip the pre-filter and post-filter when this triple is returned.
- **Provider coverage prior is Tock-only and live.** Tock's `BusinessCount` from SSR hydration provides the coverage signal for the Tock side of the prior. The OT side is not used in v1 (no enumeration path). Population-driven popularity dominates when Tock isn't hydrated.
- **Envelope omits `fallback_clarification` prose**. The agent synthesizes phrasing from `context_hints` + structured candidates. Removes locale/voice coupling.

---

## Open Questions

### Resolved During Planning

- **Polygon vs radius for v1**: radius (deferred polygon to v2).
- **Provider adapter shape**: two functions, no interface (extract on third provider).
- **Session pin in v1**: deferred.
- **Empty `--location ""` semantics**: "no constraint requested," uniformly.
- **`--metro` breakage**: imply `--accept-ambiguous`.
- **`fallback_clarification` prose in envelope**: dropped (agent synthesizes).
- **`location` command tree shape**: just `resolve` for v1.
- **Confidence formula**: ambiguity-aware tier decision, not popularity-driven.

### Deferred to Implementation

- **Exact prior weights** (population vs coverage vs tier vs match): start with `0.3 / 0.3 / 0.2 / 0.2`. Tune against R14 fixtures during U4.
- **Per-place `RadiusKm` defaults**: start with `25` (city), `75` (metro), `10` (neighborhood). Adjust if R14 integration tests reveal misses.
- **Tier boundaries**: start with `HIGH ≥ 0.85`, `MEDIUM ∈ [0.5, 0.85)`, `LOW < 0.5`. Calibrate against R14 plus the F5-F8 added fixtures.
- **`location_warning` exact JSON shape**: finalize during U4 once the tier outputs land.

---

## High-Level Technical Design

> *This illustrates the intended approach and is directional guidance for review, not implementation specification.*

### Tier decision (the math problem the adversarial review caught)

The original formula scored each `Place` in isolation. That cannot satisfy both:
- `seattle` → HIGH (one strong candidate)
- `bellevue` → LOW (three competing candidates, popularity dominant but ambiguity high)

New formula decouples the two signals:

```
# Per-candidate prior: ranks candidates inside the result, doesn't drive tier
popularity_prior(place) =
    w_pop  * log_norm(place.population) +
    w_cov  * log_norm(place.ProviderCoverage["tock"])  /* OT coverage dropped in v1 — no enumeration path */ +
    w_tier * is_metro_centroid(place) +
    w_match* exact_match_bonus(place, input)

# Tier-input axes
input_specificity:
    coords|zip|city+state          → SpecificityHigh
    metro qualifier ("X metro")    → SpecificityMedium
    bare city or neighborhood      → SpecificityLow

candidate_count = len(candidates)
margin_ratio    = (top.prior - runner_up.prior) / max(top.prior, ε)

# Tier decision (not popularity!)
if candidate_count == 0:                         → ErrorKindLocationUnknown
elif candidate_count == 1:                       → HIGH
elif input_specificity == SpecificityHigh:       → HIGH (specific input collapses ambiguity)
elif candidate_count == 2 and margin_ratio > 0.7: → MEDIUM
elif candidate_count >= 2 and margin_ratio > 0.3: → MEDIUM
else:                                            → LOW (bare input with multiple viable matches)
```

Worked examples (illustrative — actual margin values will be derived during U4 calibration against real prior data; tier outcomes are the load-bearing claim):
- `seattle` → 1 candidate → HIGH ✓
- `bellevue` (bare) → 3 candidates, all viable → LOW ✓
- `bellevue, wa` → 1 candidate → HIGH ✓
- `47.61,-122.20` → 1 candidate via reverse-lookup → HIGH ✓
- `portland` (bare) → 2 candidates (OR ~660k, ME ~68k); under the formula margin_ratio is ~0.4–0.5 (varies with weights, lands in the `>0.3` band) → MEDIUM ✓ (popularity prior puts OR first; `location_warning.alternates` surfaces ME)
- `springfield` (bare) → 4 candidates → LOW ✓
- `columbia, sc` → 1 candidate (specific) → HIGH ✓
- `washington` (bare) → 2+ candidates (state, DC, town) → LOW

The metric name stays "confidence" *as the tier label* because that's what the user sees in the response — but inside the math it's clearly the resolution of `popularity_prior` (per-candidate ranking) and `tier_decision` (ambiguity-aware) into a single number. Documented explicitly in U4.

### Data flow

```
Agent: restaurants list --location "bellevue"
  │
  ▼
Location parser (U2) → LocationInput{kind: city, raw: "bellevue", specificity: Low}
  │
  ▼
Resolver pipeline (U5)
  ├─→ Registry lookup (U3) → candidates: [Bellevue WA, Bellevue NE, Bellevue KY]
  ├─→ Rank by popularity_prior (U4)
  ├─→ Tier decision via (specificity, count, margin) → LOW
  │
  ▼
Disambiguation envelope (U4) emitted to stdout, exit 0:
{
  "needs_clarification": true,
  "error_kind": "location_ambiguous",
  "what_was_asked": "bellevue",
  "candidates": [
    {"name": "Bellevue, WA", "state": "WA",
     "context_hints": ["Seattle metro", "Eastside"],
     "ot_venue_count": 312, "tock_business_count": 28,
     "score_if_picked": 0.78, "centroid": [47.61, -122.20]},
    {"name": "Bellevue, NE", "state": "NE",
     "context_hints": ["Omaha metro"], "ot_venue_count": 4,
     "tock_business_count": 0, "score_if_picked": 0.18,
     "centroid": [41.14, -95.91]},
    {"name": "Bellevue, KY", "state": "KY",
     "context_hints": ["Cincinnati metro"], "ot_venue_count": 11,
     "tock_business_count": 0, "score_if_picked": 0.04,
     "centroid": [39.10, -84.48]}
  ],
  "agent_guidance": {
    "preferred_recovery": "Check conversation context for geographic clues.",
    "rerun_pattern": "<command> --location '<chosen-name>'"
  }
}
```

### High-tier path

```
Agent: restaurants list --location "bellevue, wa"
  │
  ▼
Resolver → GeoContext{ResolvedTo: "Bellevue, WA", Centroid: (47.61, -122.20),
                      RadiusKm: 25, Confidence: 0.91, Source: ExplicitFlag,
                      Alternates: [Bellevue NE (0.18), Bellevue KY (0.04)]}
  │
  ▼
ForOpenTable() → opentable.LocationInput{Lat: 47.61, Lng: -122.20}
ForTock()      → tock.LocationInput{City: "Bellevue", Slug: "bellevue", Lat: 47.61, Lng: -122.20}
  │
  ▼
Pre-filter via provider clients
  │
  ▼
Post-filter: applyGeoFilter(results, centroid, radiusKm=25, mode=hardReject)
  │
  ▼
Response:
{
  "location_resolved": {
    "input": "bellevue, wa",
    "resolved_to": "Bellevue, WA",
    "confidence": 0.91,
    "reason": "151k pop + 312 OT venues, 1 candidate after city+state qualifier",
    "alternates_considered": ["Bellevue, NE", "Bellevue, KY"]
  },
  "results": [...]
}
```

---

## Output Structure

```
internal/cli/
├── geo_context.go             (NEW)  GeoContext struct, ForOpenTable/ForTock projections, Source enum
├── geo_context_test.go        (NEW)
├── location_parser.go         (NEW)  Free-form string → LocationInput (with specificity)
├── location_parser_test.go    (NEW)
├── place_registry.go          (REWRITE of metro_registry.go) Place struct, lookup, parent-metro per provider
├── place_registry_test.go     (REWRITE of metro_registry_test.go)
├── place_data.go              (NEW)  Hand-curated US place data: ~80 entries (60 metros + ~20 ambiguous-name cities)
├── confidence.go              (NEW)  popularity_prior + tier decision + disambiguation envelope shape
├── confidence_test.go         (NEW)
├── location_pipeline.go       (NEW)  ResolveLocation: parse → lookup → score → tier → envelope OR GeoContext
├── location_pipeline_test.go  (NEW)
├── location.go                (NEW)  `location resolve <input>` command (only sub-command in v1)
├── location_test.go           (NEW)
├── geo_filter.go              (MODIFY)  Extend applyGeoFilter with per-place RadiusKm; remove inferMetroFromSlug
├── geo_filter_test.go         (MODIFY)
├── restaurants_list.go        (MODIFY)  Wire --location + --accept-ambiguous, post-filter, response shape
├── availability_check.go      (MODIFY)  Add --location + --accept-ambiguous flags
├── availability_multi_day.go  (MODIFY)  Same as availability_check
├── earliest.go                (MODIFY)  Replace NYC fallback in resolveOTSlugGeoAware; add --location + --accept-ambiguous
├── goat.go                    (MODIFY)  Wire --location + --accept-ambiguous, deprecate --metro alias
├── watch.go                   (MODIFY)  Replace NYC fallback at line 367; wire --location
└── root.go                    (MODIFY)  Register `location` command

SKILL.md                       (MODIFY)  Three agent rules + examples
.printing-press-patches.json   (MODIFY)  New patch entry
```

No new files under `internal/source/opentable/` or `internal/source/tock/`. Provider projection lives in `internal/cli/geo_context.go` via plain methods on `GeoContext` that return `opentable.LocationInput` / `tock.LocationInput` types defined as small structs in their existing packages (no new files there — extend existing client files with a typed input struct each, or define inline at the call site).

---

## Implementation Units

- U1. **`GeoContext` type + provider projection functions**

**Goal:** The universal location signal and the two provider projection methods. No interface.

**Requirements:** R6

**Dependencies:** None

**Files:**
- Create: `internal/cli/geo_context.go`
- Test: `internal/cli/geo_context_test.go`
- Modify: `internal/source/opentable/client.go` (add `LocationInput{Lat, Lng float64}` struct alongside existing types)
- Modify: `internal/source/tock/search.go` (add `LocationInput{City, Slug string; Lat, Lng float64}` struct alongside `SearchParams`)

**Approach:**
- `GeoContext` struct: `Origin string`, `ResolvedTo string`, `Centroid (Lat, Lng float64)`, `RadiusKm float64`, `Confidence float64` (0.0–1.0), `Source SourceEnum`, `Alternates []Candidate`.
- `Source` typed enum: `SourceExplicitFlag`, `SourceExtractedFromQuery`, `SourceDefault`.
- Two methods:
  - `func (g *GeoContext) ForOpenTable() opentable.LocationInput { return opentable.LocationInput{Lat: g.Centroid.Lat, Lng: g.Centroid.Lng} }`
  - `func (g *GeoContext) ForTock() tock.LocationInput { return tock.LocationInput{City: cityFromResolvedTo(g.ResolvedTo), Slug: slugFromResolvedTo(g.ResolvedTo), Lat: g.Centroid.Lat, Lng: g.Centroid.Lng} }`
- Empty-input convention: a nil `*GeoContext` means "no constraint requested." Document this in the type comment.

**Execution note:** Test-first. Pure data — straightforward TDD.

**Patterns to follow:**
- Existing `Metro` struct in `internal/cli/metro_registry.go` for field-naming.
- Typed enum pattern from `internal/source/opentable/cooldown.go` (`BotDetectionKind`).

**Test scenarios:**
- Happy path: `GeoContext{Origin: "bellevue, wa", ResolvedTo: "Bellevue, WA", Centroid: {47.61, -122.20}, RadiusKm: 25, Confidence: 0.91, Source: SourceExplicitFlag}` round-trips through JSON marshal/unmarshal unchanged.
- Happy path: `gc.ForOpenTable()` returns `opentable.LocationInput{Lat: 47.61, Lng: -122.20}`.
- Happy path: `gc.ForTock()` returns `tock.LocationInput{City: "Bellevue", Slug: "bellevue", Lat: 47.61, Lng: -122.20}`.
- Happy path: nil `*GeoContext` represents "no constraint"; methods are not called on nil (caller checks).
- Edge case: `GeoContext` with `Source` zero-value defaults to `SourceDefault`, not unset.
- Edge case: `RadiusKm` zero defaults to a documented constant during marshaling.
- Error path: `Confidence < 0.0` or `> 1.0` flagged by a `Validate()` method.
- Integration: a `GeoContext` constructed from a `Place` (U3) projects correctly via both methods.

**Verification:**
- All test scenarios green.
- `go vet` clean.
- No interface; no provider package imports `internal/cli`.

---

- U2. **Location parser**

**Goal:** Parse free-form `--location` strings into a typed `LocationInput` discriminated union that carries `specificity`.

**Requirements:** R1, R13

**Dependencies:** U1

**Files:**
- Create: `internal/cli/location_parser.go`
- Test: `internal/cli/location_parser_test.go`

**Approach:**
- `LocationInput` discriminated union: `LocCity{Name}`, `LocCityState{City, State}`, `LocCoords{Lat, Lng}`, `LocZip{Code}`, `LocMetro{Slug}`. (Dropped `LocNearVenue` and `LocNeighborhood` from v1 scope — too few users, recursive resolution issues per adversarial review; revisit if asked.)
- Each variant carries a `Specificity SpecificityEnum`: `Low` (bare city), `Medium` (metro qualifier), `High` (coords / zip / city+state).
- Parsing precedence: coord pattern → zip pattern → "X metro" suffix → "city, ST" pattern → bare city.
- Whitespace trimmed; case normalized to lower for lookups; preserved for display.
- Empty / whitespace-only input → returns `(nil LocationInput, nil error)` signaling "no constraint" (R13 path).

**Execution note:** Test-first. Pin every shape with a test before writing the parser.

**Patterns to follow:**
- `parseNetworkSlug` in `internal/cli/earliest.go:298` for discriminated parsing.
- `parseDays` in `internal/cli/earliest.go:269` for return-typed error patterns.

**Test scenarios:**
- Happy: `"bellevue"` → `LocCity{Name: "bellevue", Specificity: Low}`.
- Happy: `"Bellevue, WA"` → `LocCityState{City: "bellevue", State: "WA", Specificity: High}`.
- Happy: `"47.6101,-122.2015"` → `LocCoords{Lat: 47.6101, Lng: -122.2015, Specificity: High}`.
- Happy: `"47.6101, -122.2015"` (with space) → same.
- Happy: `"98004"` → `LocZip{Code: "98004", Specificity: High}`.
- Happy: `"seattle metro"` → `LocMetro{Slug: "seattle", Specificity: Medium}`.
- Edge: `"  bellevue  "` (whitespace padding) → trimmed to `LocCity{Name: "bellevue"}`.
- Edge: `"NEW YORK"` (all caps multi-token) → `LocCity{Name: "new york"}`.
- Edge: `"bellevue, wa, usa"` → `LocCityState{City: "bellevue", State: "WA"}` (extra parts ignored).
- Edge: zip with leading zeros (`"02101"` Boston) → `LocZip{Code: "02101"}`.
- Edge: `""` → `(nil, nil)` — caller treats as "no constraint."
- Edge: `"   "` (whitespace only) → `(nil, nil)`.
- Error: `"100.5,200.3"` (coords out of range) → typed parse error citing bounds.
- Error: `"abc123def"` (no pattern match) → falls through to `LocCity{Name: "abc123def"}` (registry lookup will return unknown).

**Verification:**
- All test scenarios green.
- Specificity field set on every successful parse.
- Empty input returns the canonical "no constraint" signal.

---

- U3. **Extended `Place` registry**

**Goal:** Replace thin `Metro` struct with a richer `Place` struct carrying parent-metro-per-provider, population, provider venue counts, and per-place `RadiusKm`. Hand-curate ~80 US places.

**Requirements:** R7

**Dependencies:** U1

**Files:**
- Modify (rewrite): `internal/cli/metro_registry.go` → rename to `internal/cli/place_registry.go`
- Modify (rewrite): `internal/cli/metro_registry_test.go` → rename to `internal/cli/place_registry_test.go`
- Create: `internal/cli/place_data.go` (hand-authored curated US registry)
- Modify: `internal/cli/metro_hydration.go` — project Tock SSR data into `Place` instead of `Metro`. Tock's `BusinessCount` updates `ProviderCoverage["tock"]` live so dynamically-hydrated metros do not poison confidence scoring.

**Approach:**
- `Place` struct: `Slug`, `Name`, `State`, `Lat`, `Lng`, `RadiusKm`, `Population`, `ProviderCoverage map[string]int` (v1 holds only `"tock"` keys; populated live from Tock SSR hydration, defaults to 0), `ParentMetro map[string]string` (provider-keyed; populated by Tock hydration for Tock-keyed entries, hand-curated in `place_data.go` for OT-keyed entries), `Aliases []string`, `ContextHints []string` (hand-authored e.g., `["Seattle metro", "Eastside"]`), `Tier PlaceTierEnum` (`MetroCentroid`/`City`/`Neighborhood`). No `Country` field (US-only v1 scope, no consumer).
- Backward-compatibility: `Metro = Place` type alias for one release. Existing callers (geo_filter, earliest, goat) continue to compile. **Bump `metroCacheSchemaVersion` from 1 to 2** in `metro_hydration.go` so the on-disk cache is rebuilt cleanly on upgrade (avoids mixed-shape entries — old cache has Metro-shaped JSON, new cache has Place-shaped JSON; the version bump forces invalidation).
- `PlaceRegistry` interface: `Lookup(slug string) (Place, bool)`, `LookupByName(name string) ([]Place, bool)`, `All() []Place`, `ReverseLookup(lat, lng float64) (Place, bool)`. `LookupByZip` deferred to v2 (no zip data path).
- `ReverseLookup` tiebreak rule (explicit per adversarial review): among Places whose `haversine(point, centroid) ≤ RadiusKm`, return the one with the **smallest `RadiusKm`** (cities beat metros when both contain the point). Ties on radius are broken by smallest haversine distance, then alphabetical Slug. This means `ReverseLookup(47.61, -122.20)` returns Bellevue WA (radius 25) over Seattle (radius 75) when both contain the point.
- `place_data.go` is hand-authored — no `place_data_gen.go`, no TIGER ingestion, no Wikipedia dump pipeline. Reduce-scope decision per scope-guardian review. The data file is a Go literal with ~80 entries. Initial dataset: top ~60 US metros + ~20 ambiguous-name cities (Bellevue WA/NE/KY, Portland OR/ME, Springfield MA/IL/MO/OR, Greenville SC/NC/MS, Columbia SC/MO/MD, Washington DC/state/town).
- **Population data rule**: use **city-proper population** for all entries (Wikipedia infobox value at curation time, sourced once). Mixing city-proper with metro-area would skew the prior. Each entry comments its source year (e.g., `// 2020 Census`).
- **Coverage prior**: `ProviderCoverage["tock"]` populated live from Tock SSR `BusinessCount` (overrides any value in `place_data.go`); for sub-metros that Tock doesn't recognize as their own metro, value is 0. No OT coverage prior in v1 (deferred per Scope Boundaries).

**Execution note:** Test-first. Start with the failing test `LookupByName("bellevue") returns 3` before authoring the data file.

**Patterns to follow:**
- `staticMetroRegistry` in `internal/cli/metro_registry.go`.
- `chainedMetroRegistry` (dynamic-over-static) pattern in `metro_hydration.go`.

**Test scenarios:**
- Happy: `Lookup("seattle")` returns Place with `Name: "Seattle", State: "WA", Population > 0, ProviderCoverage["tock"] > 100` (after hydration).
- Happy: `LookupByName("bellevue")` returns 3 Places (WA, NE, KY).
- Happy: `LookupByName("Bellevue")` (capitalized) returns same 3.
- Happy: `LookupByName("portland")` returns 2 Places (OR, ME).
- Happy: `LookupByName("springfield")` returns 4 Places (MA, IL, MO, OR).
- Happy: `Lookup("nyc")` resolves via alias to "New York City".
- Happy: `ReverseLookup(47.61, -122.20)` returns Bellevue WA (radius 25) — wins over Seattle (radius 75) per the smallest-radius tiebreak rule.
- Happy: `ReverseLookup(47.6062, -122.3321)` (Seattle Space Needle) returns Seattle — Bellevue's radius (25km) doesn't reach Space Needle (~13km from Bellevue centroid but at edge of radius); Seattle's 75km radius contains it.
- Happy: Tock metro hydration overrides `ProviderCoverage["tock"]` from `BusinessCount` for hydrated metros; default 0 remains for non-hydrated.
- Edge: `Lookup("")` returns `(Place{}, false)`.
- Edge: `LookupByName("nonexistent")` returns `(nil, false)`.
- Edge: `ReverseLookup(0, 0)` returns `(Place{}, false)` (no Place contains the point).
- Edge: `ReverseLookup` on a point inside 3+ overlapping Places — returns smallest radius (tiebreak rule verified).
- Edge: Place with `RadiusKm = 0` (data error) defaults to 25 at lookup-time.
- Integration: After Tock hydration, `LookupByName("bellevue")` still returns 3; Bellevue WA's `ParentMetro["tock"] = "bellevue"` (Tock has it as its own metro, from hydration) and `ParentMetro["opentable"] = "seattle"` (hand-curated in `place_data.go`).
- Integration: `metroCacheSchemaVersion` bumped from 1 to 2; first run after upgrade invalidates the old cache and rebuilds fresh.

**Verification:**
- 80-entry registry compiles and validates at package-init.
- All ambiguous-name fixtures (Bellevue, Portland, Springfield, etc.) have ≥ 2 entries.
- The `Metro = Place` alias keeps existing callers compiling.

---

- U4. **Score, ambiguity-aware tier decision, disambiguation envelope**

**Goal:** Implement the popularity prior, the tier decision (ambiguity-aware), and the envelope shape.

**Requirements:** R2, R3, R4, R5, R9

**Dependencies:** U1, U3

**Files:**
- Create: `internal/cli/confidence.go` (popularity prior + tier decision + envelope builder). Weight constants live in the same file (not a separate `confidence_weights.go`).
- Create: `internal/cli/confidence_test.go`

**Approach:**
- `popularityPrior(place Place, input LocationInput) float64` — weighted combination: `w_pop * log_norm(population) + w_cov * log_norm(maxCoverage) + w_tier * metroCentroidBonus + w_match * exactMatchBonus`. Range `[0, 1]`. Weight constants: `w_pop = 0.3, w_cov = 0.3, w_tier = 0.2, w_match = 0.2`.
- `decideTier(input LocationInput, candidates []Place) (TierEnum, []ScoredCandidate)` — returns the tier and the candidates ranked by popularity_prior. Decision logic per the High-Level Technical Design's pseudo-code (input_specificity + candidate_count + margin_ratio).
- `DisambiguationEnvelope` struct: `NeedsClarification bool`, `ErrorKind string`, `WhatWasAsked string`, `Candidates []Candidate`, `AgentGuidance AgentGuidance{PreferredRecovery, RerunPattern}`. No `FallbackClarification` prose. JSON field names: `needs_clarification`, `error_kind`, `what_was_asked`, `candidates`, `agent_guidance`.
- `Candidate` struct: `Name, State string`, `ContextHints []string`, `TockBusinessCount int` (live; absent → 0), `ScoreIfPicked float64`, `Centroid [2]float64`. (`OTVenueCount` field removed — no enumeration path; preserved as 0 in JSON only if downstream consumers want a stable schema.)
- `BuildEnvelope(input LocationInput, ranked []ScoredCandidate) DisambiguationEnvelope` — composes the envelope from the ranked list. Same shape used for `venue_ambiguous` (R9), with candidates being venues and `WhatWasAsked` carrying the venue name + location.
- `DecorateWithLocationContext(response any, gc *GeoContext, tier TierEnum, alternates []Candidate) any` — attaches `location_resolved` (HIGH/MEDIUM tier) or `location_warning` (MEDIUM tier and forced-pick scenarios) field to a response. Operates on the response shape, NOT on the result slice — `applyGeoFilter` (U5) handles filtering only; `DecorateWithLocationContext` handles annotation. This separation matters for single-row responses (`availability check`, `watch`) where soft-demote on `MatchScore` is meaningless but a `location_warning` field is still useful.

**Execution note:** Test-first. The math is testable in isolation — table-driven tests against every R14 fixture pin the tier outcomes before writing the decision function.

**Patterns to follow:**
- Existing `goatResult.MatchScore` scoring in `internal/cli/goat.go` for log-normalization style.
- Typed enum pattern from `internal/source/opentable/cooldown.go`.

**Test scenarios:**
- Happy: `popularityPrior(BellevueWA, LocCity{"bellevue"})` > `popularityPrior(BellevueNE, ...)` > `popularityPrior(BellevueKY, ...)`.
- Happy: `popularityPrior(SeattleWA, LocCity{"seattle"}) > 0.7`.
- Happy: `decideTier(LocCity{"seattle"}, [Seattle])` returns `(HIGH, [Seattle])`.
- Happy: `decideTier(LocCity{"bellevue"}, [BellevueWA, BellevueNE, BellevueKY])` returns `(LOW, [WA, NE, KY])`.
- Happy: `decideTier(LocCityState{"bellevue", "wa"}, [BellevueWA])` returns `(HIGH, [BellevueWA])`.
- Happy: `decideTier(LocCity{"portland"}, [PortlandOR, PortlandME])` returns `(MEDIUM, [OR, ME])` (2 candidates, margin_ratio ≈ 0.6).
- Happy: `decideTier(LocCity{"springfield"}, [Springfield x4])` returns `(LOW, [...])` (4 candidates, bare input).
- Happy: `decideTier(LocCityState{"columbia", "sc"}, [ColumbiaSC])` returns `(HIGH, [ColumbiaSC])`.
- Happy: `decideTier(LocCoords{47.61, -122.20}, [BellevueWA])` (after ReverseLookup) returns `(HIGH, [BellevueWA])`.
- Edge: `decideTier(input, [])` returns `(UNKNOWN, [])` — caller emits `ErrorKindLocationUnknown` envelope.
- Edge: `decideTier(input, [single])` with the single candidate having near-zero prior returns `(HIGH, [single])` — the rule is "1 candidate → HIGH" regardless of absolute prior, since input matched something.
- Edge: 2 candidates with identical priors (margin_ratio = 0) returns `(LOW, [...])`.
- Edge: `decideTier(LocCity{"bellevue"}, [BellevueWA])` (registry hypothetically only has WA) returns `(HIGH, [BellevueWA])` — 1 candidate.
- Error: `BuildEnvelope` with empty candidates → envelope with `ErrorKind: "location_unknown"`.
- Integration (R14 fixtures pinned as table-driven test):
  - F3 `bellevue` → LOW, 3 candidates in envelope, WA first.
  - F4 `seattle` → HIGH.
  - F5 `portland` → MEDIUM, 2 candidates in `location_warning`.
  - F6 `springfield` → LOW, 4 candidates.
  - F7 `columbia, sc` → HIGH.
- Integration: envelope JSON-marshals cleanly; consumers can parse `needs_clarification`, `candidates[].name`, `candidates[].state`.

**Verification:**
- All R14 fixtures pass.
- Weight constants documented inline in `confidence.go`.
- Tier-decision function is the only place tier boundaries are defined.

---

- U5. **Shared resolver pipeline + radius post-filter**

**Goal:** `ResolveLocation` ties parser + registry + scoring + envelope together. `applyGeoFilter` extended with per-place radius.

**Requirements:** R2, R7, R9, R11, R13

**Dependencies:** U2, U3, U4

**Files:**
- Create: `internal/cli/location_pipeline.go`
- Create: `internal/cli/location_pipeline_test.go`
- Modify: `internal/cli/geo_filter.go` — `applyGeoFilter` extended to accept a `GeoContext` (instead of bare lat/lng/radius); per-place `RadiusKm` from `GeoContext.RadiusKm`. Remove `inferMetroFromSlug` (superseded by U2's parser).
- Modify: `internal/cli/geo_filter_test.go`

**Approach:**
- `ResolveLocation(input string, opts ResolveOptions) (*GeoContext, *DisambiguationEnvelope, error)` — at most one of `*GeoContext` / `*DisambiguationEnvelope` is non-nil.
- `ResolveOptions{Source SourceEnum, AcceptAmbiguous bool}`.
- Pipeline: (1) parser → if `(nil, nil)`, return `(nil, nil, nil)` meaning "no constraint" (R13). (2) Registry lookup based on `LocationInput` variant: `LocCity`/`LocCityState` → `LookupByName` + filter by state; `LocCoords` → `ReverseLookup`; `LocMetro` → `Lookup`. (Zip support deferred per Scope Boundaries — `LocZip` is not currently produced by the parser in v1.) (3) `decideTier` (U4) → `(tier, ranked)`. (4) If LOW and not `AcceptAmbiguous`: `BuildEnvelope` → return envelope. Otherwise: build `GeoContext` from `ranked[0]` with `Confidence` set per tier (HIGH: prior; MEDIUM: prior; LOW with AcceptAmbiguous: prior, marked as forced-pick so downstream `DecorateWithLocationContext` attaches a `location_warning`).
- **Signature migration for `applyGeoFilter`**: current signature is `(results []goatResult, centroid Metro, radiusKm float64, mode metroFilterMode)`. U5 replaces it with `(results []goatResult, ctx *GeoContext, mode metroFilterMode)`. This is a breaking signature change — **every existing caller must be migrated in this same unit**. The known call site is in `goat.go:188`; grep `applyGeoFilter\(` to confirm no others before landing U5. When `ctx == nil`, no-op (passes results through unchanged — supports R13). When `ctx != nil`, computes haversine distance per result against `ctx.Centroid` with radius `ctx.RadiusKm`; hard-reject or soft-demote per mode. **Annotation (`location_resolved` / `location_warning`) is attached separately by `DecorateWithLocationContext` (U4)**, not by `applyGeoFilter` — the filter is purely the include/exclude/demote decision.

**Execution note:** Test-first. R14 fixtures exercise this pipeline end-to-end.

**Patterns to follow:**
- Existing `resolveEarliestForVenue` in `internal/cli/earliest.go:308` for multi-step resolve flow.
- Existing `applyGeoFilter` (single-mode signature extended, not rewritten).

**Test scenarios:**
- Happy: `ResolveLocation("bellevue, wa", {Source: ExplicitFlag})` → `(*GeoContext{Confidence > 0.85}, nil, nil)`.
- Happy: `ResolveLocation("seattle", ...)` → high-confidence `GeoContext` with `location_resolved` payload.
- Happy: `ResolveLocation("47.61,-122.20", ...)` → high-confidence, `ResolvedTo: "Bellevue, WA"` via reverse-lookup.
- Happy: `ResolveLocation("", ...)` → `(nil, nil, nil)` (no constraint).
- Happy: `ResolveLocation("   ", ...)` → `(nil, nil, nil)` (whitespace-only).
- Edge: `ResolveLocation("seattle metro", ...)` → high-confidence GeoContext with `Tier = MetroCentroid` and a metro-sized `RadiusKm` (75km default).
- Edge: `ResolveLocation("99999", ...)` (zip not in registry) → envelope `ErrorKind: "location_unknown"`.
- Edge: `ResolveLocation("100.5, 200.3", ...)` (invalid coords) → envelope `ErrorKind: "location_unknown"`.
- Error: `ResolveLocation("bellevue", {Source: ExplicitFlag})` → envelope, 3 candidates, `WhatWasAsked: "bellevue"`.
- Error: `ResolveLocation("bellevue", {AcceptAmbiguous: true})` → `(*GeoContext{ResolvedTo: "Bellevue, WA", Confidence: <0.5}, nil, nil)` (low confidence but no envelope; downstream wraps with `location_warning` flagging the bypass).
- Integration (R14):
  - F1 `ResolveLocation("seattle", ...)` returns Seattle GeoContext; `applyGeoFilter` over a mix of Seattle + NYC venues drops the NYC venues.
  - F3 `ResolveLocation("bellevue", ...)` returns envelope.
  - F4 `ResolveLocation("seattle", ...)` returns HIGH-tier GeoContext.
  - F5 `ResolveLocation("portland", ...)` returns MEDIUM-tier GeoContext with both Portlands in `Alternates`.
- Integration: `applyGeoFilter(results, nil, ...)` is a no-op (results pass through).
- Integration: hard-reject mode drops NYC venues from a Bellevue WA `GeoContext` (radius 25km, haversine ≈ 3850km).

**Verification:**
- All R14 fixtures pass.
- Empty-input contract uniform across the pipeline.
- `applyGeoFilter` works for both `--location` and absent-flag cases.

---

- U6. **Wire `restaurants list`**

**Goal:** Fix R14 F1. Add `--location` and `--accept-ambiguous` together; deprecate `--metro` with implicit `--accept-ambiguous`.

**Requirements:** R1, R3, R5, R7, R11, R12, R14-F1, R14-F3

**Dependencies:** U5

**Files:**
- Modify: `internal/cli/restaurants_list.go`
- Modify (or create): `internal/cli/restaurants_list_test.go`

**Approach:**
- Add `--location` (string, default "") and `--accept-ambiguous` (bool, default false) flags.
- Keep `--metro` flag. When set, internally map to `--location <value>` and force `AcceptAmbiguous: true`. Emit a one-line stderr deprecation warning per process (`--metro is deprecated; use --location. legacy callers continue to receive results-shaped responses.`).
- Remove `_ = flagMetro` (the line that caused the bug).
- Call `ResolveLocation(flagLocation, ResolveOptions{Source: SourceExplicitFlag, AcceptAmbiguous: flagAcceptAmbiguous || flagMetroProvided})`.
- If `(nil, envelope, nil)` returned: marshal envelope to stdout, exit 0.
- If `(*GeoContext, nil, nil)` returned: project via `ctx.ForOpenTable()` / `ctx.ForTock()`. Pass to existing `goatQueryOpenTable` and `goatQueryTock`. Apply `applyGeoFilter(results, ctx, metroFilterHardReject)`. Wrap response with `location_resolved` (HIGH/MEDIUM with `location_warning`).
- If `(nil, nil, nil)` returned (empty input): existing no-filter behavior preserved.

**Execution note:** Test-first. Start with R14 F1 failing, then write the wiring.

**Patterns to follow:**
- Existing `restaurants_list.go` flag/registration shape.

**Test scenarios:**
- Happy (R14 F1): `restaurants list --query 'sushi bellevue' --location seattle` returns goatResponse with `location_resolved.confidence > 0.85` and zero NYC venues.
- Happy: `restaurants list --query 'sushi' --location 'bellevue, wa'` returns Bellevue-area results.
- Happy: `restaurants list --query 'sushi' --metro seattle` (legacy) returns same shape as `--location seattle` with deprecation warning on stderr.
- Happy: `restaurants list --query 'sushi' --metro bellevue` (legacy, ambiguous) returns Bellevue WA results (forced pick via implied --accept-ambiguous) with `location_warning` listing alternates. Does NOT return a `needs_clarification` envelope (preserves legacy shape).
- Happy: `restaurants list --query 'sushi' --location bellevue --accept-ambiguous` returns Bellevue WA results with `location_warning`.
- Edge: `restaurants list --query 'sushi'` (no location) returns current no-filter behavior.
- Edge: `restaurants list --query 'sushi' --location ''` (empty) same as no-location.
- Edge: `restaurants list --query 'sushi' --network tock --location seattle` only Tock branch runs; post-filter applied.
- Error (R14 F3): `restaurants list --query 'sushi' --location bellevue` returns envelope JSON, exit 0, `needs_clarification: true`, 3 candidates.

**Verification:**
- R14 F1 and F3 pass deterministically.
- `--metro` continues to work; emits deprecation warning; never returns envelope.
- Existing test cases (if any) continue to pass.

---

- U7. **Wire `availability check` and `availability multi-day`**

**Goal:** Fix R14 F2. Add `--location` + `--accept-ambiguous`. Exempt numeric-ID inputs from hard-reject post-filter (per adversarial review).

**Requirements:** R1, R7, R11, R14-F2

**Dependencies:** U5

**Files:**
- Modify: `internal/cli/availability_check.go`
- Modify: `internal/cli/availability_multi_day.go`
- Modify: `internal/cli/earliest.go` — extend `resolveEarliestForVenue` to accept an optional `*GeoContext` parameter and use its centroid in place of the hardcoded NYC fallback for the `resolveOTSlugGeoAware` call. **No `--location` flag added to `earliest` in U7** — that wiring is U8's responsibility; U7's edit is the parameter-plumbing only.
- Modify (or create): `internal/cli/availability_check_test.go`

**Approach:**
- Add `--location` and `--accept-ambiguous` flags to both commands.
- Call `ResolveLocation`. If envelope returned, emit it and exit 0.
- If `*GeoContext` available, pass to `resolveEarliestForVenue` as an optional parameter. The function uses `gc.Centroid` for OT Autocomplete in place of `40.7128, -74.0060`.
- Add a venue-disambiguation step in `resolveOTSlugGeoAware`: when Autocomplete returns multiple matches inside the radius, the function returns a `venue_ambiguous` envelope (R9). When numeric-ID input is provided, this disambiguation is skipped.
- **Numeric-ID hard-reject exemption**: post-filter on a resolved numeric-ID venue uses soft-demote mode regardless of `Source`. If the venue is outside radius, the response includes a `location_warning` ("venue is outside your stated location") but still returns the venue.

**Execution note:** Test-first. R14 F2 is the calibration test.

**Patterns to follow:**
- Existing `availability_check.go` flag/registration shape.
- Existing `resolveEarliestForVenue` flow in `earliest.go`.

**Test scenarios:**
- Happy (R14 F2 — narrow): `availability check 'I Love Sushi Bellevue' --location 'bellevue, wa'` resolves to a Bellevue-area I Love Sushi (or returns `venue_ambiguous` envelope if multiple), never silently to Manhattan.
- Happy (R14 F2 — venue ambiguous): when multiple I Love Sushi venues exist inside the Bellevue radius, return `venue_ambiguous` envelope listing them with venue ID + distance from centroid.
- Happy: `availability check 'I Love Sushi Bellevue'` (no `--location`) — falls back to current NYC-anchor behavior; response field `location_resolved.source = SourceDefault`.
- Happy: numeric ID input (`availability check 3688 --location seattle`) — numeric short-circuit applies (per PR #423); post-filter uses soft-demote; venue outside radius returns with `location_warning` rather than `no_results_in_region`.
- Happy: numeric ID input matching radius — clean response, no warning.
- Edge: `availability check 'I Love Sushi' --location seattle` (no city in venue name) — relies on `--location` alone; should still resolve to Bellevue-area when one matches.
- Edge: `availability check 'tock:canlis' --location seattle` — Tock branch uses Tock projection; post-filter applies.
- Edge: `availability multi-day 'X' --location seattle` works the same as `availability check`.
- Error (ambiguous location): `availability check 'I Love Sushi Bellevue' --location bellevue` (ambiguous) returns envelope, does NOT proceed to venue resolution.
- Error: `availability check '' --location seattle` — typed argument error for empty venue.
- Integration: post-filter rejection — Manhattan venue Autocomplete-matched despite Bellevue WA anchor → hard-reject drops it from name-input path; soft-demote applies to numeric-ID path.

**Verification:**
- R14 F2 passes.
- Numeric-ID + outside-region returns the venue with warning, not no_results_in_region.

---

- U8. **Wire `earliest`, `goat`, and `watch`**

**Goal:** Three commands that currently have hardcoded NYC fallbacks (or no location handling at all) graduate to the unified pipeline. Resolves R14 F8 (the `watch.go:367` line surfaced by adversarial review).

**Requirements:** R1, R7, R11, R14-F8

**Dependencies:** U5

**Files:**
- Modify: `internal/cli/earliest.go` — add `--location` and `--accept-ambiguous` flags; pass `*GeoContext` through `resolveEarliestForVenue`. Preserve the existing slug-suffix inference path as the lowest-precedence fallback (when no `--location` and no city-state in the slug).
- Modify: `internal/cli/goat.go` — add `--location` and `--accept-ambiguous`; deprecate `--metro` with the same legacy contract as `restaurants list` (`--metro` implies `--accept-ambiguous`).
- Modify: `internal/cli/watch.go` — add `--location` flag; replace the hardcoded `40.7128, -74.0060` at line 367 with the resolved `GeoContext.Centroid`.
- Modify: `internal/cli/earliest_test.go`, `internal/cli/goat_test.go`, and `internal/cli/watch_test.go` (create the last if absent).

**Approach:**
- All three commands follow the same flow: parse flags, call `ResolveLocation`, branch on `(GeoContext, envelope, error)`, apply post-filter to results.
- `earliest`'s existing slug-suffix inference (in `inferMetroFromSlug`/`resolveOTSlugGeoAware`) is preserved as the lowest-precedence fallback when no `--location` is given and the venue arg has a hyphenated city suffix. When a slug-suffix triggers metro inference, the synthesized `GeoContext.Source = SourceExtractedFromQuery` (per U1's enum) so the post-filter mode is soft-demote (not hard-reject) — agents who passed a city-suffixed slug get a hint, not an over-zealous filter.
- `watch <slug> --location seattle` follows the warn-and-continue pattern: at subscription start, resolve the venue, compute haversine distance from `gc.Centroid`, and if outside `gc.RadiusKm`, emit a single stderr `location_warning` line (e.g., `location_warning: <venue> is 4800km from seattle (radius 75km) — proceeding anyway`) and continue. Does not refuse to subscribe. The fire-event stream is unchanged (no per-event annotation).

**Execution note:** Test-first.

**Patterns to follow:**
- Existing `earliest.go:235` (`hydrateMetrosFromTock` call site) for where to graft the resolver call.
- U6 wiring for `goat`'s `--metro` deprecation.

**Test scenarios:**
- Happy: `earliest 'canlis,joey-bellevue' --location 'bellevue, wa'` — both venues resolve; `joey-bellevue` no longer needs city-encoded slug.
- Happy: `goat 'sushi' --location 'bellevue, wa'` returns Bellevue-area results.
- Happy: `goat 'sushi' --metro bellevue` (legacy) returns Bellevue WA results with `location_warning` (forced pick, never envelope).
- Happy (R14 F8): `watch <slug> --location seattle` honors the location; the resolved venue's centroid is checked against Seattle's radius.
- Edge: `earliest 'joey-bellevue'` (slug-suffix, no `--location`) — continues to work via existing `inferMetroFromSlug`-replacement parser. Slug-suffix is the lowest-precedence inference.
- Edge: `earliest 'joey-bellevue' --location 'bellevue, ne'` — explicit `--location` wins over slug suffix; resolves to Bellevue NE.
- Edge: `watch <slug>` (no `--location`) — falls back to current NYC anchor; emits `location_resolved.source = SourceDefault` for visibility.
- Error: `earliest 'joey-bellevue' --location bellevue` — ambiguous location wins over slug suffix; envelope returned.
- Integration: `availability multi-day` (already wired in U7) shares this code path via `resolveEarliestForVenue`.

**Verification:**
- R14 F8 passes (`watch.go:367` no longer hardcodes NYC).
- `earliest`, `goat`, `watch` all pass `--location` through; `--metro` works on `goat` with legacy contract.

---

- U9. **`location resolve` command**

**Goal:** Expose the resolver as a primitive agents can call independently. Single sub-command; no `set/unset/current/list`.

**Requirements:** R8

**Dependencies:** U5

**Files:**
- Create: `internal/cli/location.go`
- Create: `internal/cli/location_test.go`
- Modify: `internal/cli/root.go` — register the new command.

**Approach:**
- `location resolve <input> [--accept-ambiguous]` — calls `ResolveLocation(input, {Source: SourceExplicitFlag, AcceptAmbiguous: flag})`. Emits `GeoContext` JSON (HIGH/MEDIUM tier) or envelope JSON (LOW tier).
- Annotate `mcp:read-only: "true"`.

**Execution note:** Test-first.

**Patterns to follow:**
- Existing command-tree shape (cobra parent + sub-commands).

**Test scenarios:**
- Happy: `location resolve 'bellevue, wa'` returns `GeoContext` JSON with all fields populated.
- Happy: `location resolve bellevue` returns envelope JSON with 3 candidates.
- Happy: `location resolve seattle` returns `GeoContext` with `Confidence > 0.85`.
- Happy: `location resolve 'portland' --accept-ambiguous` returns `GeoContext` for Portland OR (forced pick) with `Alternates` listing Portland ME.
- Edge: `location resolve ''` returns typed error ("location resolve requires an argument").
- Edge: `location resolve '99999'` returns envelope with `ErrorKind: "location_unknown"`.

**Verification:**
- Sub-command registered and callable.
- JSON output is the same shape produced inside the resolver pipeline.

---

- U10. **SKILL.md contract**

**Goal:** Document the three agent rules and concrete recovery patterns.

**Requirements:** R10

**Dependencies:** U6, U7, U8, U9

**Files:**
- Modify: `SKILL.md`

**Approach:**
- New section "Location handling (read commands)" with three numbered rules (R10 verbatim).
- Concrete examples: paste a `needs_clarification` envelope, show agent pseudo-code for handling it; paste a `location_warning` response, show the surface-to-user move.
- Document `--accept-ambiguous` as a batch/test escape hatch with explicit warning that conversational agents should NOT use it.
- Document the `--metro` deprecation contract: legacy callers continue to get results-shaped responses (no envelope).

**Execution note:** Docs-only.

**Patterns to follow:**
- Existing SKILL.md structure.

**Test scenarios:**
- None — docs-only.

**Verification:**
- Three rules present verbatim.
- Examples cite real flag names and field shapes.
- Mirror to `cli-skills/pp-table-reservation-goat/SKILL.md` regenerated post-merge.

---

- U11. **Patch manifest entry**

**Goal:** Record the redesign in the patches index per `AGENTS.md`.

**Requirements:** Non-functional (project hygiene).

**Dependencies:** All preceding.

**Files:**
- Modify: `.printing-press-patches.json`

**Approach:**
- New patch entry `id: "location-native-redesign"`, listing every file created/modified in U1-U10.
- Summary: "Typed GeoContext with ambiguity-aware tier decision, disambiguation envelope, provider projection functions, radius-based post-filter. Fixes silent wrong-region resolution; bug-report fixtures R14 plus added ambiguity-coverage fixtures."

**Execution note:** Manifest-only.

**Test scenarios:**
- None.

**Verification:**
- JSON validates.
- Every modified/created file listed.

---

## System-Wide Impact

- **Interaction graph:** Six read commands now funnel through `ResolveLocation`. A bug in the resolver affects all six. Mitigation: U5's test scenarios exercise the full pipeline against every R14 fixture.
- **Error propagation:** Envelope returns are a second legitimate response shape (alongside existing typed responses). Consumers check `needs_clarification` before parsing `results`. SKILL.md (U10) documents the contract.
- **`--metro` legacy contract:** legacy callers receive results-shaped responses with `location_warning` for ambiguous inputs. They never see the new envelope shape. Test scenarios in U6/U8 pin this.
- **Empty `--location ""` uniform treatment:** all commands treat it as no-constraint, identical to absent flag. Test scenarios in every wiring unit pin this.
- **Numeric-ID exemption:** `availability check` and `watch` skip hard-reject for numeric IDs; venues outside radius return with `location_warning`, not `no_results_in_region`.
- **API surface parity:** MCP surface inherits all changes automatically (typed tools mirror cobra). `--location` and `--accept-ambiguous` propagate. `location resolve` becomes an MCP tool.
- **Integration coverage:** R14 fixtures F1-F8 are the load-bearing integration tests. CI must run both unit and integration layers.
- **Unchanged invariants:** Numeric-ID short-circuit (PR #423), `--metro` flag continues to work, existing JSON response shapes don't break (new fields are additive). Slot/availability fetch (PRs #424/#426) untouched. Booking flow stays orthogonal.

---

## Risks & Dependencies

| Risk | Mitigation |
|------|------------|
| Confidence formula misbehaves on non-Bellevue inputs | R14 expanded with Portland, Springfield, Columbia, Washington fixtures (F5–F7). Tier-decision function is the only place tier boundaries live; all changes are test-pinned. |
| Hand-curated `place_data.go` drifts vs reality | Tock `BusinessCount` overrides the Tock coverage prior live via SSR hydration. OT coverage drift is acceptable for v1; periodic refresh deferred. |
| `--metro` deprecation breaks legacy callers | `--metro` implies `--accept-ambiguous`, preserving the results-shape contract. Stderr deprecation warning only — no envelope ever returned through this path. Test scenarios in U6 and U8 pin this. |
| Envelope-return-shape confuses existing JSON consumers | New top-level fields are additive. Existing readers of `results` continue to work. `needs_clarification` is opt-in checking. |
| Provider adapter over-engineered for v1 | Two plain functions, no interface. Extract interface when a third provider is added (per scope-guardian review). |
| Radius-based defense-in-depth misses non-circular metros | Radius covers R14 fixtures. Polygon containment deferred to v2 once production signal reveals a real false-positive case. |
| Agent ignores envelope / uses `--accept-ambiguous` by default / re-prompts annoyingly | SKILL.md (U10) documents the contract. Detection deferred (no telemetry in v1). If user reports persist, revisit the contract or add a stderr warning when `--accept-ambiguous` would have triggered. |
| Single bug-report sample drove formula | Expanded R14 fixtures cover other ambiguous-name cities. The formula validates against 8 fixtures, not 3. |

---

## Documentation / Operational Notes

- README.md gets a "Location handling" section restating the three-rule contract for human users.
- The R14 fixtures (F1-F8) become the sentinel tests for "did we slay the location beast?" Future regressions in this area must keep them passing.
- Deprecation of `--metro` emits a one-line stderr warning the first time it's used per process. Document in `CHANGELOG.md` (if/when one exists).

---

## Sources & References

- Bug report: issue #406 follow-up (conversation 2026-05-10, real-world Mother's Day query).
- Source files inspected:
  - `internal/cli/restaurants_list.go:104` (the `_ = flagMetro` line — proximate bug).
  - `internal/cli/availability_check.go` (no `--metro` flag).
  - `internal/cli/geo_filter.go:98` (`inferMetroFromSlug` hyphenated-only).
  - `internal/cli/earliest.go:443` (hardcoded NYC fallback in `resolveOTSlugGeoAware`).
  - `internal/cli/watch.go:367` (hardcoded NYC fallback).
  - `internal/source/opentable/client.go:401` (Autocomplete signature).
  - `internal/source/tock/search.go:100` (SearchCity).
  - `internal/source/tock/search.go:196` (MetroArea with BusinessCount).
- Related plans:
  - `docs/plans/2026-05-09-003-feat-booking-flow-free-reservations-plan.md` (booking flow, out of scope).
  - PRs #423, #424, #425, #426 (predecessor work).
- Document review history: two rounds of 5-persona document review (2026-05-10). Round 1 applied two safe-auto fixes (polygon shape, ForProvider signature) and surfaced the math/scope/coverage P1s that drove the simplification round. Round 2 surfaced concrete plumbing issues that drove the inline edits captured in this revision (OT coverage prior dropped, zip support deferred, ReverseLookup tiebreak specified, `metroCacheSchemaVersion` bump, `DecorateWithLocationContext` separated from `applyGeoFilter`, slug-suffix `Source` enum value, `applyGeoFilter` signature migration scope).
