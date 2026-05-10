---
title: feat: Tock SearchCity — broader cross-network queries
type: feat
status: completed
date: 2026-05-09
deepened: 2026-05-09
completed: 2026-05-09
---

# feat: Tock SearchCity — broader cross-network queries

## Summary

Wire Tock's `GET /city/<city>/search?...` SSR endpoint into the `goat` cross-network dispatcher so broader queries (`metro + date + time + party`) return Tock results alongside OpenTable Autocomplete. The page returns HTML with `window.$REDUX_STATE = {...}` inline; `state.availability.result.offeringAvailability[]` carries 60+ venues with full metadata AND slot times in a single GET. No protobuf, no encoder, no auth — pure HTML+JSON parsing. The existing slug-direct Tock path stays as the cheap canonical-resolution fallback for single-venue queries.

---

## Problem Frame

Today, `goat 'canlis'` works because the resolver hits Tock's `/<slug>` SSR directly. But broader queries — `goat 'tasting menu chicago'`, `goat 'omakase ny'` — return zero Tock results because the slug-direct path is the only Tock entrypoint we've wired. Tock's `/city/<city>/search?city=...&date=...&latlng=...&size=...&time=...&type=DINE_IN_EXPERIENCES` URL is the consumer-facing geo-search surface — it returns a fully server-rendered page with venues, neighborhoods, cuisines, lat/lng, and live slot tokens in `window.$REDUX_STATE`. We never wired it. The result: cross-network search has a real coverage gap on the Tock side that hides legitimate availability from agents and CLI users.

We confirmed the SSR shape live via chrome-MCP capture (2026-05-09): a Seattle query returned 63 entries at `state.availability.result.offeringAvailability[]`, each with `business: {id, name, domainName, cuisines, neighborhood, location: {lat, lng, address, city, state}, ...}`, `offering[].ticketGroup[].availableDateTime`, and `ranking: {distanceMeters, relevanceScore}`. No XHR fired during page load — the `/api/v2/consumer/search/business` proto endpoint we'd seen in earlier session probing is a different surface (likely the autocomplete textbox) and isn't needed for city-page search.

---

## Requirements

- R1. The CLI exposes a Tock city-search path that returns multiple venue matches for a metro+date+time+party query.
- R2. `goat` dispatches the new Tock search call alongside the existing OT Autocomplete and Tock slug-direct lookups; results merge into the existing cross-network result list with the same `goatResult` shape. v1 chains slug-direct → SearchCity sequentially within `goatQueryTock`; concurrent dispatch is a v0.2 follow-up if measured latency justifies.
- R3. Existing slug-direct Tock queries (`goat canlis`) keep working — the new path adds coverage rather than replacing the existing resolver.
- R4. The HTTP+JSON extractor lives in `internal/source/tock/search.go` mirroring the structure of `internal/source/tock/calendar.go` (method + thin response struct + tests).
- R5. JSON-path discipline matches the calendar module: a `// SSR shape (captured YYYY-MM-DD): ...` comment block at the top of `search.go` documents the exact `$REDUX_STATE` paths read so a future Tock SPA refactor surfaces as a clear diff target.
- R6. The `--metro` flag drives the search city + coords. Without `--metro`, default to NYC `(40.7589, -73.9851)` matching `goatQueryOpenTable`'s existing fallback.
- R7. Search results respect the existing `goatResult` ranking and dedupe rules — a venue surfaced by both slug-direct and search returns once, not twice.

---

## Scope Boundaries

- Tock's autocomplete textbox XHR (the `/api/v2/consumer/search/business` proto endpoint earlier-session probing showed). Different surface, different use case (per-keystroke search-as-you-type). v0.2 if needed.
- Caching SearchCity responses. The OT availability cache exists for a different reason (slot-token freshness against an aggressive WAF); search results are catalog data and queries are infrequent enough that caching is YAGNI for v1.
- OpenTable booking flow. Separate brainstorm.
- Tock cuisine taxonomy normalization. SearchCity returns whatever cuisine strings Tock attaches to venues; don't transform.
- Multi-page pagination. The page returns ~60 venues in one fetch; "load more" client-side is a separate XHR we don't need for v1's "top results" use case.
- Wiring the slot-token data (`offering[].ticketGroup[].availableDateTime`) into `goat`'s `earliest`/`watch` flows. Available for free in the same response and worth a follow-up unit, but expanding `goatResult` to carry slot times changes the JSON contract — defer to its own change.

### Deferred to Follow-Up Work

- Dynamic metro discovery. The same SSR fetch hydrates `state.app.config.metroArea` (253 metros worldwide with name/slug/lat/lng/businessCount). U1 captures the raw data; a small `LoadMetros()` helper that replaces the static `metroLatLng` map is a clean follow-up but reaches into multiple call sites in `goat.go` — keep U1 narrow, file the metro replacement separately.
- Tock-side `--type` flag (DINE_IN_EXPERIENCES is one of several enum values; `EVENT`, `TAKEOUT`, etc. exist). v1 hardcodes DINE_IN_EXPERIENCES to match the typical reservation use case. Expose `--type` once we have a real demand signal.

---

## Context & Research

### Relevant Code and Patterns

- **`internal/source/tock/calendar.go`** — direct parallel pattern. Provides `CalendarBootstrap` and `Calendar`, which read inline state from a Tock SSR HTML response. The new `search.go` follows the same shape: GET an SSR URL, regex-extract the inline state JSON, walk the relevant subtree.
- **`internal/source/tock/client.go`** — provides `Client`, `do429Aware`, and the cookie/header wiring shared with calendar. SearchCity reuses `do429Aware` for the GET.
- **`internal/cli/goat.go`** — the dispatcher. `goatQueryTock` (the slug-direct path) is what U2 extends with a SearchCity call. `goatQueryOpenTable` is the structural parallel for parallel-resolver shape. `metroLatLng(metro)` and `knownMetros()` are reused; `--metro` flag handling is unchanged.
- **Live capture (chrome-MCP, 2026-05-09):** GET `https://www.exploretock.com/city/seattle/search?city=Seattle&date=2026-05-10&latlng=47.6062095%2C-122.3320708&size=4&time=19%3A00&type=DINE_IN_EXPERIENCES` returns HTML containing `window.$REDUX_STATE = {...}` (~630KB). 63 venues hydrated at `state.availability.result.offeringAvailability[]` for that query. Returns 200 cleanly with no auth headers, no warmed cookies, no JWT.
- **`internal/source/opentable/client.go`** — the OT-side parallel. `Autocomplete(ctx, query, lat, lng)` is the API-level shape SearchCity mirrors at the goat-integration level (one call → many venues).

### Institutional Learnings

No matching `docs/solutions/` entries. Tock-side reverse-engineering precedent is documented inline in `calendar.go` and the `cross-network-source-clients` patch in `.printing-press-patches.json` (v0.1.9). The new search work extends that body of work rather than introducing a new pattern.

### External References

None used. The technique stack is fully repo-local.

---

## Key Technical Decisions

- **GET the SSR HTML and parse `window.$REDUX_STATE` — do not call any XHR.** Earlier-session probing surfaced a `POST /api/v2/consumer/search/business` proto endpoint, which seemed like the natural target. Live chrome-MCP capture (2026-05-09) showed that endpoint is NOT what the city-page calls — the page is fully server-rendered, results live inline in `$REDUX_STATE`, and no client-side search XHR fires. SSR-extraction is also what `calendar.go` does, which makes this a known-stable pattern in the codebase.
- **No protobuf, no encoder, no auth bootstrap.** The earlier plan budgeted for proto schema reverse-engineering, encoder hand-rolling, and a chrome-MCP capture-and-replay gate to verify auth/cookie warmth. All three drop out: the SSR response is JSON, the JSON shape is observable directly, and the page returns 200 anonymously.
- **Reuse the existing `metroLatLng` static map for v1; expose `metroArea` discovery as a follow-up.** Each SearchCity response carries 253 metros worldwide in `state.app.config.metroArea`, which means dynamic metro discovery is essentially free. But replacing the static `metroLatLng` callers throughout `goat.go` is its own refactor; keep U1 narrow.
- **Default to NYC `(40.7589, -73.9851)` when `--metro` is unset, matching `goatQueryOpenTable`'s existing fallback.** No mismatched constants between OT and Tock fallbacks.
- **Single-fetch result set for v1.** The page returns ~60 venues without pagination; "load more" would be a follow-up XHR we don't need. CLI use cases want top results, not full enumeration.
- **Dedupe-by-slug across the two Tock paths.** Slug-direct returns when the user types a venue's exact slug; SearchCity also returns it when matching by city/date. Merge by `domainName` (slug). Slug-direct runs first and wins on dup.
- **Surface raw cuisine strings and ranking fields.** SearchCity attaches `cuisines` (string), `ranking.distanceMeters`, `ranking.relevanceScore`. Pass through to `TockBusiness`; let `goat`'s match-score layer decide what matters.

---

## Open Questions

### Resolved During Planning

- **Where does the search method live?** Resolved: `internal/source/tock/search.go`, mirroring `calendar.go`'s placement and shape.
- **Does search need a per-venue bootstrap?** Resolved by live capture (2026-05-09): no. The SSR page returns 200 anonymously without JWT, cookies, or scope headers.
- **What's the actual endpoint?** Resolved: `GET https://www.exploretock.com/city/<city-slug>/search?city=<CityName>&date=YYYY-MM-DD&latlng=<lat>%2C<lng>&size=<n>&time=HH%3AMM&type=DINE_IN_EXPERIENCES` (note the URL-encoded comma and colon). Returns HTML with `window.$REDUX_STATE = {...}` inline.
- **Where in the response do venues live?** Resolved: `state.availability.result.offeringAvailability[]`. Each entry has `{business, offering, ranking}`.
- **Does the response include slots?** Resolved: yes — `offering[].ticketGroup[].availableDateTime`, `price`, `ticketPriceInformation`. Available for goat's earliest/watch flows in a follow-up.
- **How does goat reconcile two Tock paths returning the same venue?** Resolved: dedupe by `domainName` (the slug field). Slug-direct runs first and wins on duplicates.

### Deferred to Implementation

- **Exact regex anchor for `$REDUX_STATE` extraction.** The inline declaration is `<script>...window.$REDUX_STATE = {...}</script>` in script #5 of the page. A reasonable extractor: find `window.$REDUX_STATE = ` then balance braces to capture the object literal, then `json.Unmarshal`. Alternative: capture between `window.$REDUX_STATE = ` and `</script>` then trim. Implementer picks the most robust shape.
- **Whether to use `domainName` as-is or normalize.** Live capture shows values like `kricketclub`, `fusion-india-bothell`, `chutneys-bellevue` — Tock-canonical slugs. These should be passed through unchanged.
- **`cuisines` field type.** Live sample showed it as a single string (`"Indian"`); the JSON schema may use `string` or `[]string` depending on venue. Implementer handles both via `json.RawMessage` if needed.

---

## Implementation Units

- U1. **Tock SearchCity client (HTTP GET + `$REDUX_STATE` extractor)**

**Goal:** Implement `SearchCity(ctx, params) ([]TockBusiness, error)` on the existing `tock.Client`. GET the city-search SSR URL, regex-extract `window.$REDUX_STATE`, json-unmarshal into a thin Go struct mirroring `state.availability.result.offeringAvailability`, return a flat `[]TockBusiness` ready for goat to merge.

**Requirements:** R1, R4, R5

**Dependencies:** None

**Files:**
- Create: `internal/source/tock/search.go`
- Create: `internal/source/tock/search_test.go`
- Create: `internal/source/tock/testdata/seattle-search.html` (saved real-world capture; shrunk if practical, but keep enough venues to exercise dedupe/ranking)

**Approach:**
- **`SearchParams` shape:** `{City string; Date string; Time string; PartySize int; Lat, Lng float64}`. URL-encodes city as `?city=<CityName>` (display name, not slug — the Tock URL uses both). Type defaults to `DINE_IN_EXPERIENCES` (constant; not in struct unless flag exposure is added later).
- **GET the search URL:** `https://www.exploretock.com/city/<city-slug>/search?city=<CityName>&date=<date>&latlng=<lat>%2C<lng>&size=<n>&time=<HH%3AMM>&type=DINE_IN_EXPERIENCES`. The URL `<city-slug>` path segment is the lower-cased, dash-joined city name (`Seattle` → `seattle`, `New York` → `new-york`). Headers minimal: `Accept: text/html`, `User-Agent` matching existing Tock client. Run through `do429Aware`.
- **`$REDUX_STATE` extraction:** Regex `window\.\$REDUX_STATE\s*=\s*(\{` followed by a brace-balanced scan to find the matching closing `}`, then `json.Unmarshal` into `searchSSRState` (a thin Go struct mirroring just the path we read: `{Availability: {Result: {OfferingAvailability: []offeringAvailEntry}}}`). Comment block at top of file documents the exact JSON path so a Tock SPA refactor surfaces as a clear diff target.
- **`offeringAvailEntry` shape (just what we need):**
  ```go
  type offeringAvailEntry struct {
      Business struct {
          ID           int    `json:"id"`
          Name         string `json:"name"`
          DomainName   string `json:"domainName"`
          BusinessType string `json:"businessType"`
          Cuisines     json.RawMessage `json:"cuisines"`  // string or []string
          Neighborhood string `json:"neighborhood"`
          Location     struct {
              Address string  `json:"address"`
              City    string  `json:"city"`
              State   string  `json:"state"`
              Country string  `json:"country"`
              Lat     float64 `json:"lat"`
              Lng     float64 `json:"lng"`
          } `json:"location"`
      } `json:"business"`
      Offering []struct {
          TicketGroup []struct {
              AvailableDateTime string `json:"availableDateTime"`
          } `json:"ticketGroup"`
      } `json:"offering"`  // captured for v0.2 follow-up; v1 ignores
      Ranking struct {
          DistanceMeters float64 `json:"distanceMeters"`
          RelevanceScore float64 `json:"relevanceScore"`
      } `json:"ranking"`
  }
  ```
- **`TockBusiness` shape:** `{ID int; Name string; Slug string; BusinessType string; Cuisine string; Neighborhood string; City, State string; Latitude, Longitude float64; URL string; DistanceMeters, RelevanceScore float64}`. URL composed as `https://www.exploretock.com/<DomainName>`. `Cuisine` flattens cuisines (string passthrough; if `[]string`, join with `", "`).

**Patterns to follow:**
- `internal/source/tock/calendar.go` — SSR fetch + inline-state extraction. Same pattern, simpler payload (JSON not proto).
- `internal/source/tock/client.go` `do429Aware` for the actual GET + retry/cooldown handling.

**Test scenarios:**
- Happy path: `SearchCity(ctx, {City:"Seattle", Date:"2026-05-10", Time:"19:00", PartySize:4, Lat:47.6062, Lng:-122.3321})` parses the saved fixture and returns `>5` venues with non-zero IDs, names, slugs, lat/lng.
- Happy path: each returned `TockBusiness` carries a non-empty `URL`, `City: "Seattle"` or nearby Bellevue/Bothell (cross-metro spillover is normal — the search radius isn't strictly the city), `BusinessType: "Restaurant"` (or whatever the fixture has), and a non-zero `RelevanceScore`.
- Edge case: `cuisines` field is a single string (per live sample); decoder produces `Cuisine` as that string.
- Edge case: `cuisines` field is an array (defensive — venue with multiple cuisines); decoder joins with `", "`.
- Edge case: response with no `offeringAvailability` (zero-result page) → returns empty slice, no error.
- Edge case: HTML missing `window.$REDUX_STATE` (Tock SPA refactor scenario) → returns a clear sentinel error; not a panic, not zero results pretending to be a successful empty query.
- Edge case: `$REDUX_STATE` present but truncated/invalid JSON → returns parse error wrapped with context.
- Error path: HTTP non-200 from Tock returns the error untouched (caller decides retry; `do429Aware` handles 429).
- Integration: `httptest.Server` serves the saved fixture HTML; assert the parsed slice matches the venues observed live.

**Verification:**
- A unit test running `client.SearchCity(...)` against the fixture server returns ≥5 venues including at least one venue whose `Slug` matches a real Tock domain we can corroborate (`kricketclub`, `canlis`, etc.).
- SSR-shape comment block at the top of `search.go` documents the exact JSON path (`state.availability.result.offeringAvailability[]`) so a future Tock SPA refactor surfaces as a clear diff target.

---

- U2. **Wire SearchCity into goat dispatcher**

**Goal:** Add a SearchCity call to `goat`'s Tock resolver after the existing slug-direct lookup. Merge results into the cross-network response with the existing `goatResult` shape; dedupe by slug.

**Requirements:** R2, R3, R6, R7

**Dependencies:** U1

**Files:**
- Modify: `internal/cli/goat.go` (extend `goatQueryTock` with a SearchCity call)
- Modify: `internal/cli/goat_test.go` (or create if absent) — covers the merged-result shape and dedupe

**Approach:**
- Extend `goatQueryTock`'s signature from `(ctx, session, query)` to `(ctx, session, query, city, date, time, partySize, lat, lng)`; update the single call site at `internal/cli/goat.go:120` to pass the metro-resolved city + coordinates already computed for `goatQueryOpenTable`. (Existing date/time/party already in scope; thread them through.)
- In `goatQueryTock`, after the existing slug-direct path runs, ALSO fire `c.SearchCity(ctx, params)`. The two run sequentially in v1 (slug-direct is fast — usually a single SSR fetch — and search runs after).
- Convert each `TockBusiness` into a `goatResult` row matching the existing shape produced by the slug-direct path: `Network: "tock"`, `Slug` (from DomainName), `Name`, `URL`, `Metro`, `Latitude`/`Longitude`, `Cuisine`. Match-score is set by the calling goat layer's existing logic.
- Dedupe by slug across the slug-direct and search results. Slug-direct runs first — its result wins on duplicate slugs (it's the canonical/cheap source).
- City/coords come from `metroLatLng(metro)` lookup. When `--metro` is unset, default to NYC `(40.7589, -73.9851)` and `City: "New York"` — same fallback `goatQueryOpenTable` already uses (`internal/cli/goat.go:224`).
- Existing `goat` ranking, output formatting, and `--agent` behavior are unchanged.

**Patterns to follow:**
- `goatQueryOpenTable` in `internal/cli/goat.go` — structural parallel for the multi-result shape (parses `Autocomplete` results into `goatResult` rows the same way SearchCity will).
- The existing slug-direct branch in `goatQueryTock` — preserve as the first call; SearchCity runs after.

**Test scenarios:**
- Happy path: `goat 'tasting menu chicago' --metro chicago` returns at least one Tock result with a Chicago-metro venue. (Note: the query string itself doesn't filter — Tock's geo search returns metro venues; goat's match-score layer scores them against the query.)
- Happy path: `goat canlis` (the existing slug-direct case) still returns the canonical Canlis row from Tock — search may also return it; dedupe shows it once.
- Edge case: slug-direct hits AND search returns the same slug — output contains exactly one row.
- Edge case: slug-direct misses AND search returns N venues — output contains N rows, all `Network: "tock"`.
- Edge case: both miss → Tock side returns zero rows; OT side independent.
- Error path: SearchCity errors mid-call — slug-direct results still surface; the error is logged via the goat layer's existing error channel; the call as a whole doesn't fail.
- Integration: `--metro chicago` plumbs through to SearchCity's city/lat/lng (verified against an httptest fixture asserting the request URL matches `/city/chicago/search?city=Chicago&...&latlng=...`).
- Edge case: `--metro` unset → NYC default coords + `City: "New York"` are used; matching test asserts the request URL carries `latlng=40.7589%2C-73.9851`.
- Edge case: `slugify(query) ≠ canonical Tock slug`. slug-direct misses (404), SearchCity returns the canonical-slug venue. Output contains exactly one row keyed by SearchCity's `domainName`.

**Verification:**
- Live dogfood (deferred to U3): `goat 'tasting menu chicago'` returns >0 Tock rows alongside whatever OT returns.

---

- U3. **PATCH manifest + dogfood + ship**

**Goal:** Update `.printing-press-patches.json` to record the new Tock search surface; live-dogfood the dispatcher against real OT/Tock; confirm the cross-network coverage gap is closed.

**Requirements:** R1-R7 (verification gate)

**Dependencies:** U1, U2

**Files:**
- Modify: `.printing-press-patches.json`

**Approach:**
- Append a v0.1.15 entry to the `cross-network-source-clients` patch's `validated_outcome` describing the new `internal/source/tock/search.go` file (HTML+JSON SSR extractor; no protobuf), the `$REDUX_STATE` JSON path captured, and the goat dispatcher integration.
- Add `internal/source/tock/search.go` to the patch's `files` list.
- **Live dogfood matrix (run with a fresh Chrome / curl, no persistent filter state from prior browsing):**
  1. `goat 'canlis'` — slug-direct still works; canonical Canlis row appears.
  2. `goat 'tasting menu chicago' --metro chicago` — Tock returns multiple Chicago-metro matches; OT returns its parallel set.
  3. `goat 'omakase' --metro nyc` — both networks return omakase venues; cross-network ranking surfaces the strongest matches.
  4. `goat 'goldfinch tavern' --metro seattle` — slug-direct hits Tock OR misses, OT autocomplete hits; the result row is consistent.
  5. `goat 'goldfinch tavern'` (no --metro) — defaults to NYC coords; Tock returns NYC venues (irrelevant to the query) but the call doesn't error; goat's match-score should rank them low.

**Test scenarios:**
- Test expectation: none — manual dogfood. Unit tests in U1 + U2 cover correctness.

**Verification:**
- Patches manifest mentions v0.1.15 with the new behavior described.
- Dogfood transcript pasted into the PR description shows: a broader query returning Tock+OT results in parallel, AND existing slug-direct queries unchanged.

---

## System-Wide Impact

- **Interaction graph:** The new SearchCity call only fires from `goatQueryTock`. `earliest`, `watch tick`, and `drift` are unaffected — they operate on resolved venue slugs, not search queries. The OT side of `goat` is unchanged.
- **Error propagation:** SearchCity errors don't fail the goat call — they surface via the goat layer's existing error channel alongside any OT errors. The slug-direct Tock path keeps producing results regardless of search outcome. (Note: slug-direct currently swallows all errors silently into "zero rows"; SearchCity will surface its errors, creating an asymmetry — see Risks.)
- **State lifecycle risks:** None. SearchCity is read-only; no cache, no persisted state.
- **API surface parity:** OT autocomplete and Tock SearchCity both produce `goatResult` rows. The shape is preserved; consumers see the same JSON contract with strictly more results.
- **Integration coverage:** The new flow exercises Tock's SSR HTML+JSON parse path for the first time (`calendar.go` parses Redux state but for a single business detail, not a metro list). U1's fixture-based integration test is the anchor.
- **Unchanged invariants:** `internal/source/tock/calendar.go`, `client.go`, the existing `goatQueryTock` slug-direct path, and the OT side of goat are all unchanged in behavior.

---

## Risks & Dependencies

| Risk | Mitigation |
|---|---|
| Tock SPA-refactors the city-search page (moves from SSR-with-`$REDUX_STATE` to client-side rendering) — extractor breaks. | U1's "extractor returns sentinel error when `$REDUX_STATE` is missing" test scenario is the canary. CI doesn't catch this (we use a saved fixture); it surfaces in U3 dogfood and in production via the error channel. Recovery: switch to whatever XHR the new SPA fires — likely the proto endpoint after all. |
| Tock changes the JSON shape of `offeringAvailability` (rename `domainName` → `slug`, drop `location`, etc.). | The thin Go struct only reads the fields we use; extra fields are ignored. Renamed/removed fields surface as zero-valued `TockBusiness` rows in U3 dogfood. The SSR-shape comment block at top of `search.go` is the diff target. |
| `cuisines` field type drift (string today, `[]string` tomorrow). | Decoder uses `json.RawMessage` and handles both shapes. Test scenarios cover both. |
| Tock applies bot detection to anonymous `/city/.../search` GETs we haven't seen yet. | `do429Aware` already handles 429. If we see 403/Cloudflare-mitigation, treat the same as the slug-direct path (zero rows, log error). |
| Dedupe by slug misses a case (slug differs by one path segment between slug-direct and search). | Test scenarios cover this; if a venue surfaces twice the user sees a clear duplicate, not a silent failure — fixable in a follow-up by also keying dedupe on `business.id`. |
| Persistent search filters in a real user's browser session leak into the SSR response (e.g., my Seattle test session showed all-Indian results because of prior browsing). | CLI requests don't carry browser cookies, so this is a chrome-MCP-only artifact. U3 dogfood verifies via fresh-curl. Document as known: a logged-in Tock user with persistent filter cookies might see filtered results from `goat`; we don't authenticate, so we get the unfiltered metro view. |
| Slug-direct currently swallows all errors (Cloudflare blocks, 5xx, network) into "zero rows"; SearchCity will surface its errors loudly. Asymmetric UX. | Document the asymmetry in U3 dogfood. Acceptable for v1 — fixing slug-direct's swallow-all is a separate concern. |
| Tock client's `AdaptiveLimiter` floor is 1 req/s; chained slug-direct → SearchCity adds ~1s minimum to every Tock-side query. | Accept for v1. v0.2 may parallelize the two calls if measured latency justifies (limiter still serializes them, so worker-pool gains require also raising the floor). |
| ~600KB HTML response per query (vs ~50KB for a hypothetical proto response). | Acceptable for CLI-frequency queries (1–10/day). Surfaces only if someone scripts thousands of queries. |

---

## Documentation / Operational Notes

- README mention: under the existing "OpenTable / Tock" section, note that `goat` now searches Tock by metro+date+time+party, not just slug. One sentence; the headline `goat` example already shows the cross-network shape.
- Patches manifest v0.1.15 entry under `cross-network-source-clients`.
- No monitoring or rollout coordination needed — printed CLI; users update by re-installing.

---

## Sources & References

- Direct parallel module: `internal/source/tock/calendar.go` (SSR-fetch + inline-state extraction technique)
- Tock client base: `internal/source/tock/client.go` (`Client`, `do429Aware`, headers)
- Goat dispatcher: `internal/cli/goat.go` (`goatQueryTock`, `goatQueryOpenTable`, `metroLatLng`, `knownMetros`)
- OT structural parallel: `internal/source/opentable/client.go` `Autocomplete` method (the API-level shape SearchCity mirrors at the goat-integration layer)
- Live capture (chrome-MCP, 2026-05-09): `https://www.exploretock.com/city/seattle/search?city=Seattle&date=2026-05-10&latlng=47.6062095%2C-122.3320708&size=4&time=19%3A00&type=DINE_IN_EXPERIENCES` returned HTML with 63 venues at `state.availability.result.offeringAvailability[]`. No XHR fired; no auth required.
- Patches manifest: `.printing-press-patches.json` (v0.1.9 documents the calendar-side Tock SSR work)
