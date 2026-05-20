---
title: "feat: Add flights value-compare with TPG cents-per-point valuation"
date: 2026-05-20
status: active
owner: mvanhorn
depth: Standard
target_repo: printing-press-library
target_module: library/travel/alaska-airlines
related: 2026-05-19-amend-alaska-airlines-plan.md (PR #713 — award-search + award-cheapest)
---

## Summary

Add a new `flights value-compare` core Go command to the alaska-airlines CLI. Given an itinerary (origin, destination, depart, optional return, cabin, passengers), the command runs paired cash + award (miles) searches against the same alaskaair.com endpoint, extracts the lowest cash price and lowest miles+taxes price, looks up a points valuation in cents-per-point (default: Alaska/Atmos from The Points Guy, scraped on first use and cached on disk for 30 days), and emits a structured comparison: effective cpp on the redemption, multiple over the baseline, and the TPG-valued dollar cost of paying with points.

A new `internal/valuation/` subpackage holds the pluggable seam — a flat program registry, the TPG scraper, and the file-cache wrapper — so a future amend can add `united`, `aadvantage`, etc. by appending one row.

---

## Problem Frame

The user repeatedly runs cash and award searches by hand, then types the cpp math into a calculator and grep-scrapes a TPG page in a browser to know whether a redemption is a good deal. Today's `flights search --award` returns clean award data and `flights search` returns clean cash data, but nothing in the CLI joins them or attaches a points valuation. The result is a manual five-step ritual every time the user wants to answer "is this redemption worth burning miles for vs. paying cash?".

Build the comparator into the CLI so the agent (or the user) can answer the question with one command:

```
alaska-airlines-pp-cli flights value-compare --origin FCO --destination SEA --depart 2026-08-30 --cabin economy --json
```

returning `cash_usd`, `miles + taxes_usd`, `effective_cpp_cents`, `tpg_cpp_cents`, `multiple`, `tpg_valued_usd`, with cache age and source visible in the meta envelope.

---

## Requirements

R1. Command runs both cash and award searches against alaskaair.com for the same itinerary and extracts the lowest-priced result on each side.

R2. Default points valuation is sourced from The Points Guy's `loyalty-programs/monthly-valuations/` page (Atmos row → cents value), scraped on first use and cached for 30 days at `~/.cache/alaska-airlines-pp-cli/valuations/`.

R3. `--cpp <float>` flag overrides the scraped value. `--no-cache` (already a root flag) forces re-scrape. A `--no-valuation-cache` flag forces re-scrape of the valuation only (not the HTTP cache).

R4. Output envelope mirrors the `flights award-cheapest` pattern (custom `meta` + `results` map), not `wrapWithProvenance`. Meta carries `cpp_baseline_source` (`tpg-live`, `tpg-cached`, `override`, `fallback-stale`, `fallback-constant`) plus `cpp_baseline_fetched_at` so a downstream agent knows the freshness of the baseline.

R5. Cabin is locked to a single value across both searches (default `economy`, `--cabin` override). Cross-cabin comparison is explicitly out of scope for v1 — comparing economy cash to business award is misleading.

R6. Soft validation: if the TPG scrape fails (network error, Cloudflare challenge, selector miss) and a cached value exists (even expired), fall back to it with `fallback-stale` source and a stderr warning. If no cache exists and no `--cpp` was given, fall back to a constant baseline (Atmos: 1.4¢ as of 2026-05) with `fallback-constant` source and a stderr warning. Never hard-fail the command on valuation issues — the comparison numbers are still useful with a slightly stale or constant baseline.

R7. The new command is auto-registered as an MCP tool by the existing `cobratree` walker (no hand-registration). Annotation must include `mcp:read-only: "true"` so the walker emits the correct read-only hint.

R8. The `internal/valuation/` subpackage is structured so adding a second airline (United/Delta/AA) later is a single struct-literal append, not a refactor. Pluggable seam = flat registry of `Program` rows, not an interface or plugin loader.

---

## Scope Boundaries

In scope:
- One-way and round-trip support (round-trip composes one search call; the SvelteKit endpoint handles both via the `RT` flag).
- Atmos (Alaska) only for v1 — the only TPG row this PR will exercise.
- Single cabin per invocation (economy default).
- TPG as the sole valuation source for v1.
- The cash extractor that doesn't exist yet (the existing `walkAwardJSON` only emits when miles is present — see U2).

### Deferred to Follow-Up Work

- Multi-airline registry expansion (United, Delta, American, JetBlue, etc.) — design supports it, content not shipped.
- NerdWallet / AwardWallet as alternate valuation sources — `--source nerdwallet` flag deferred until at least one user asks.
- `--with-value` flag on existing `flights award-cheapest` to annotate top-N results with cpp + multiple — useful follow-up, defer until value-compare ships and gets used.
- `flights award-watch` (the F5 deferred from the prior amend) is unrelated to this work and stays deferred.
- Cross-cabin "what cabin gives me the best cpp on this route?" mode — useful but explicitly different UX surface.

### Outside this command's identity

- Real-time TPG API consumption — no such API exists; scrape-and-cache is the only available path.
- Multi-program redemption optimization (transfer partners, e.g. "should I move my AmEx points to Delta or pay cash?"). That's a different product.
- Booking flow integration — value-compare reads only; it does not initiate booking.

---

## Key Technical Decisions

D1. **TPG fetch path: plain `net/http` + goquery, not chromedp / firecrawl runtime dep.** The TPG monthly valuations page returns the Atmos row in static HTML; firecrawl-scraped markdown shows a clean table with `| Alaska Airlines Atmos Rewards | 1.4* |`. A goquery selector on the program-name cell plus `regexp.MustCompile(\`(\d+\.\d+)\`)` on the value cell extracts the float without a browser. Chrome User-Agent header to dodge default-Go-UA 403s.

D2. **Reuse existing `internal/cache` package (TTL via mtime, sha256 file keys).** Already shipped with every PP CLI but currently unimported in alaska-airlines. One key per program (e.g. `tpg:atmos`). 30-day TTL. Wrap `Get` to also return the file mod-time so the caller can detect "expired but readable" and use it as the stale fallback.

D3. **New `internal/valuation/` subpackage, not `pkg/valuation/`.** The repo convention is `internal/` only — no public `pkg/` exists. Per the printing-press-library AGENTS.md, CLIs stay internal.

D4. **Add `extractLowestCashPrice` as a sibling of `extractLowestAwardPrice` in `flights_award_cheapest.go`, not a refactor of `walkAwardJSON`.** The award walker currently emits only when `hasMiles && miles > 0`. Refactoring it to also handle cash-only risks breaking the working award-cheapest extractor. A parallel walker (or a parallel emit branch) is lower-risk. Pure function, unit-testable on JSON fixtures.

D5. **Envelope shape mirrors `flights award-cheapest`, not `wrapWithProvenance`.** Custom `map[string]any{"meta": ..., "results": ...}` envelope with the comparison's domain meta (cpp, source, multiple). Use `printOutput(out, json.RawMessage, true)` for JSON output and a small `printValueCompareTable` for the TTY branch.

D6. **Soft-validation on every failure surface — stderr warning + fallback, never hard error.** The existing `library/travel/alaska-airlines/internal/cache` package + the recipe-goat `ErrBlocked` pattern feed this. Distinct typed errors (`ErrTPGFetch`, `ErrTPGParse`, `ErrTPGBlocked`) inside the valuation package, swallowed at the command boundary.

D7. **No hand-registered MCP tool.** Add the command to the cobra tree with `Annotations: {"pp:endpoint": "flights.value_compare", "pp:method": "GET", "pp:path": "/search/results/__data.json (paired)", "mcp:read-only": "true"}` and the `cobratree` walker emits the MCP tool automatically at server startup.

D8. **`--cpp` override means literal cents-per-point as a float** (e.g. `--cpp 1.4`). Not `--cpp 1.4c` or `--cpp 0.014`. Matches how TPG publishes the number and how the user mentioned it in the chat ("TPG point value 1.4¢"). Document in `--help`.

---

## High-Level Technical Design

> Directional guidance for review, not implementation specification. The implementer should treat this as context, not code to reproduce.

```
flights value-compare command
        │
        ├──► buildCashSearchParams(...)  ─┐
        │                                 ├──► resolveRead (cash)   ──► extractLowestCashPrice ──► {miles=nil, cashUSD, carrier, cabin, stops}
        ├──► buildAwardSearchParams(...) ─┘                                                              │
        │                                       resolveRead (award) ──► extractLowestAwardPrice ──► {miles, taxesUSD, carrier, cabin, stops}
        │                                                                                                │
        ├──► valuation.Lookup(ctx, ProgramAtmos)                                                          │
        │       ├─ cache hit (<30d) → return (cpp, fetchedAt, source="tpg-cached")                       │
        │       ├─ cache miss → tpg.FetchAtmos(ctx)                                                      │
        │       │     ├─ success → cache.Set → return (cpp, now, source="tpg-live")                      │
        │       │     ├─ Cloudflare / 4xx / 5xx → return cached-if-any + source="fallback-stale"         │
        │       │     └─ no cache → return constantAtmos (1.4) + source="fallback-constant" + stderr warn│
        │       └─ override (--cpp) short-circuits all of above → source="override"                      │
        │                                                                                                │
        └──► comparator.Compare(cash, award, cpp) ────► {effectiveCPP, multiple, tpgValuedUSD}           │
                                                                                                          │
                            ───────────────────────────────────────────────────────────────────────────────
                                                                                                          ▼
                                                          envelope {meta: {...,cpp_baseline_source,...}, results: {cash, award, comparison}}
                                                                          │
                                                              ┌───────────┴────────────┐
                                                       JSON (printOutput)         TTY (printValueCompareTable)
```

Comparison math (lives in `valuation.Compare` or inlined in the command — pure function either way):

```
cashSavedUSD     = cashUSD - awardTaxesUSD
effectiveCPP     = (cashSavedUSD * 100) / miles            # cents per point
multiple         = effectiveCPP / tpgCPP
tpgValuedUSD     = (miles * tpgCPP / 100) + awardTaxesUSD  # apples-to-apples cost of points option
```

---

## Output Structure (new package only)

```
library/travel/alaska-airlines/
├── internal/
│   ├── valuation/                              [NEW]
│   │   ├── program.go                          # Program type, registry slice, BySlug lookup
│   │   ├── tpg.go                              # goquery scraper, typed errors (ErrTPGFetch/Parse/Blocked)
│   │   ├── cache.go                            # wraps internal/cache.Store, 30d TTL, mtime-aware Get
│   │   ├── lookup.go                           # public Lookup(ctx, program) → (cpp, fetchedAt, source, error)
│   │   ├── compare.go                          # pure cpp math (effective, multiple, tpgValuedUSD)
│   │   ├── program_test.go
│   │   ├── tpg_test.go                         # synthetic HTML fixture
│   │   ├── lookup_test.go                      # tempdir + stale + fresh + override paths
│   │   └── compare_test.go
│   └── cli/
│       ├── flights_value_compare.go            [NEW] command definition + envelope
│       ├── flights_value_compare_test.go       [NEW] table-driven on the comparator and envelope shape
│       ├── flights_award_cheapest.go           [MOD] add extractLowestCashPrice next to extractLowestAwardPrice
│       └── flights_award_cheapest_test.go      [MOD] add TestExtractLowestCashPrice_*
├── flights.go                                  [MOD] one AddCommand line under the existing PATCH block
├── SKILL.md                                    [MOD] Unique Capabilities + Command Reference + Recipes
├── .printing-press.json                        [MOD] novel_features append
└── .printing-press-patches.json                [MOD] new patch entry id="value-compare"
```

---

## Implementation Units

### U1. Add `internal/valuation/` subpackage with TPG scraper, file cache, and program registry

**Goal.** Stand up the pluggable valuation seam. Build the lookup function (`Lookup(ctx, Program) → (cpp, fetchedAt, source, err)`) with TPG scrape, 30-day file cache, soft-fallback chain (live → stale → constant), and the pure comparison math.

**Requirements.** R2, R3, R6, R8, D1, D2, D3, D6, D8.

**Dependencies.** None (foundation unit).

**Files:**
- `library/travel/alaska-airlines/internal/valuation/program.go` (NEW)
- `library/travel/alaska-airlines/internal/valuation/tpg.go` (NEW)
- `library/travel/alaska-airlines/internal/valuation/cache.go` (NEW)
- `library/travel/alaska-airlines/internal/valuation/lookup.go` (NEW)
- `library/travel/alaska-airlines/internal/valuation/compare.go` (NEW)
- `library/travel/alaska-airlines/internal/valuation/program_test.go` (NEW)
- `library/travel/alaska-airlines/internal/valuation/tpg_test.go` (NEW)
- `library/travel/alaska-airlines/internal/valuation/lookup_test.go` (NEW)
- `library/travel/alaska-airlines/internal/valuation/compare_test.go` (NEW)
- `library/travel/alaska-airlines/go.mod` (MOD — add `github.com/PuerkitoBio/goquery`)

**Approach.**

- `program.go`: `type Program string` with `const ProgramAtmos Program = "atmos"`. Flat registry slice `var programs = []ProgramDef{{Slug: "atmos", Display: "Alaska Airlines Atmos Rewards", TPGRowMatch: "Alaska Airlines Atmos", FallbackCPP: 1.4}}` and a `BySlug(p Program) (ProgramDef, bool)` lookup. No interface.
- `tpg.go`: package-level `defaultClient = &http.Client{Timeout: 15s}`. `FetchValuation(ctx, def ProgramDef) (cpp float64, err error)` issues `GET https://thepointsguy.com/loyalty-programs/monthly-valuations/` with `User-Agent: Mozilla/5.0 ... Chrome/130 ...`. Parses with goquery: find the table row whose first cell contains `def.TPGRowMatch`, read the second cell, strip `*†` markers, parse float. Typed errors: `ErrTPGFetch` (network), `ErrTPGParse` (selector miss / float parse fail), `ErrTPGBlocked` (HTTP 403/503 + body looks like Cloudflare).
- `cache.go`: wraps the existing `internal/cache.Store` with the alaska-airlines `~/.cache/alaska-airlines-pp-cli/valuations/` directory and 30-day TTL. Records are `{cpp: float64, fetched_at: RFC3339 string, source_url: string, program_slug: string}`. Expose `Get(p Program) (rec ValuationRecord, modTime time.Time, fresh bool, ok bool)` — `fresh` is true when within TTL, `ok` true if any file exists (used for stale fallback).
- `lookup.go`: `Lookup(ctx, p, opts LookupOptions) (cpp float64, fetchedAt time.Time, source string, err error)`. `opts.Override` (set to non-zero by CLI when `--cpp` was passed) returns immediately with `source="override"`. Otherwise: try cache fresh → if hit, return `tpg-cached`. Cache miss/stale → call `FetchValuation`; on success → `cache.Set` and return `tpg-live`. On `ErrTPGBlocked`/`ErrTPGFetch`/`ErrTPGParse` → if a cached record exists at any age, return it with `source="fallback-stale"` plus the wrapped error in a returned `err` (so caller can stderr-warn but proceed); if no cache exists, return `def.FallbackCPP` with `source="fallback-constant"` + wrapped error.
- `compare.go`: pure functions, no I/O: `EffectiveCPP(cashUSD, taxesUSD float64, miles int) float64`, `Multiple(effectiveCPP, baselineCPP float64) float64`, `TPGValuedUSD(miles int, cpp, taxesUSD float64) float64`. Trivial — separate file for unit-testability.

**Patterns to follow.**
- `library/travel/airbnb/internal/source/airbnb/client.go` — package-level `defaultClient` with timeout and a polite rate-limiter; package-level convenience functions for the top-level operation.
- `library/travel/airbnb/internal/source/vrbo/extract.go` — `isBotChallenge(doc *goquery.Document)` helper next to the parser, typed error sentinel.
- `library/travel/alaska-airlines/internal/cache/cache.go` — existing store template; reuse via composition.

**Test scenarios.**

`program_test.go`:
- `TestBySlug_FindsAtmos`: lookup of `ProgramAtmos` returns the registered definition with `FallbackCPP=1.4`.
- `TestBySlug_UnknownProgram`: lookup of an unregistered slug returns `(ProgramDef{}, false)`.

`tpg_test.go`:
- `TestParseTPGTable_ExtractsAtmosCPP`: given a synthetic `<html><table>...<tr><td>Alaska Airlines Atmos Rewards</td><td>1.4*</td></tr>...</table></html>` fixture, parser returns `1.4`.
- `TestParseTPGTable_HandlesObelisk`: cell value `1.5†` parses to `1.5`.
- `TestParseTPGTable_RowMissing`: HTML without the Atmos row returns `ErrTPGParse`.
- `TestParseTPGTable_CloudflareChallenge`: HTML with `<title>Just a moment...</title>` body returns `ErrTPGBlocked`.

`lookup_test.go` (uses `t.TempDir()` as the cache dir, plugs in a stub fetcher):
- `TestLookup_OverrideShortCircuits`: `opts.Override=2.0` returns `(2.0, _, "override", nil)` without calling fetcher.
- `TestLookup_FreshCacheHit`: cache file <30 days old → returns its value with `source="tpg-cached"`, fetcher NOT called.
- `TestLookup_CacheMissTriggersFetch`: no cache file → calls fetcher, persists result, returns `source="tpg-live"`.
- `TestLookup_StaleCacheRefreshed`: cache file >30 days old + fetcher succeeds → returns fresh value with `source="tpg-live"`, cache overwritten.
- `TestLookup_StaleCacheFallbackOnFetchError`: cache file >30 days old + fetcher returns `ErrTPGBlocked` → returns stale cached value with `source="fallback-stale"` + non-nil wrapped err.
- `TestLookup_NoCacheNoNetwork`: no cache + fetcher fails → returns `FallbackCPP` with `source="fallback-constant"` + non-nil wrapped err.

`compare_test.go`:
- `TestEffectiveCPP_Math`: `EffectiveCPP(1766.23, 64.53, 30000)` ≈ `5.6723`.
- `TestEffectiveCPP_ZeroMiles`: returns `0` (avoid divide-by-zero).
- `TestMultiple_AboveBaseline`: `Multiple(5.67, 1.4)` ≈ `4.05`.
- `TestTPGValuedUSD`: `TPGValuedUSD(30000, 1.4, 64.53)` = `484.53`.

**Verification.** `go test ./internal/valuation/...` passes. `go vet ./internal/valuation/...` clean. Manually delete cache dir and invoke `Lookup(ctx, ProgramAtmos, LookupOptions{})` against live TPG — confirm it returns `~1.4`, persists the JSON record, and re-invocation reads from cache without a network call.

---

### U2. Add `extractLowestCashPrice` to the existing flights extractor

**Goal.** Add a cash-fare extractor that walks the SvelteKit `__data.json` response and returns the lowest cash-priced itinerary (mirroring `extractLowestAwardPrice`'s shape and tolerance to JSON shape drift). The existing `walkAwardJSON` only emits when miles is present, so a parallel walker is needed for cash-only extraction.

**Requirements.** R1, R5, D4.

**Dependencies.** None. Independent of U1.

**Files:**
- `library/travel/alaska-airlines/internal/cli/flights_award_cheapest.go` (MOD — add `extractLowestCashPrice` and supporting walker next to `extractLowestAwardPrice`)
- `library/travel/alaska-airlines/internal/cli/flights_award_cheapest_test.go` (MOD — add `TestExtractLowestCashPrice_*` cases)

**Approach.**

- Define `type lowestCashPrice struct { CashUSD float64; Currency string; Carrier string; Cabin string; Stops int }` next to `lowestAwardPrice`.
- Define `extractLowestCashPrice(data json.RawMessage, maxStops int, cabin string) lowestCashPrice` mirroring `extractLowestAwardPrice` signature. The optional `cabin` filter ("economy", "business", etc.) limits the walker to matching fare classes — needed because the comparator must lock cabin.
- New `walkCashJSON(v any, emit func(...))` recursive walker. Identical traversal to `walkAwardJSON`, different emit condition: emit when the node carries a cash-amount field. Candidate cash keys (probe via the existing `/tmp/sea-nrt-hydrated.json` fixture the user already has on disk before implementing — the keys are observable from that file): likely `cashAmount`, `totalCashAmount`, `price`, `fareTotal`, `fareAmount`, `displayPrice`, `cashAmountPerPax`. Read currency from `currency`, `currencyCode`, or fallback to "USD". Read cabin from `cabin`, `fareClass`, `cabinClass`. Use the existing `readFloatField`, `readIntField`, `readStringField` helpers.
- Cabin-filter behavior: if `cabin != ""`, the walker skips nodes whose cabin field doesn't case-insensitive-prefix-match the filter ("economy" matches "ECONOMY", "MAIN", "REFUNDABLE_MAIN" all map to economy — use the same cabin normalization the existing `awardSearchInput.Cabin` flag uses).
- The walker should be tolerant of the same shape drift the award walker handles — partner award rows, multiple leg objects, nested itinerary arrays.

**Patterns to follow.**
- `internal/cli/flights_award_cheapest.go` `extractLowestAwardPrice`, `walkAwardJSON`, `readIntField`, `readFloatField`, `readStringField`.

**Test scenarios.**

- `TestExtractLowestCashPrice_NoData`: empty JSON or empty array returns zero value (`lowestCashPrice{}`).
- `TestExtractLowestCashPrice_FindsLowest`: synthetic JSON with three nodes carrying cash amounts of 1900.00, 1766.23, 2400.50 → extractor returns 1766.23.
- `TestExtractLowestCashPrice_MaxStopsFilter`: synthetic JSON where the cheapest node has `stops=2` but `maxStops=0` is requested → extractor returns the cheapest nonstop, not the global cheapest.
- `TestExtractLowestCashPrice_CabinFilter_Economy`: synthetic JSON where the cheapest is `BUSINESS` at 1200 and next is `MAIN` at 1766 → with `cabin="economy"`, extractor returns 1766.
- `TestExtractLowestCashPrice_CurrencyExtracted`: when currency field is present on the matched node, the result carries the right currency string.
- `TestExtractLowestCashPrice_DefaultsUSD`: when no currency field is anywhere in the doc, result carries `"USD"`.

**Verification.** `go test ./internal/cli/ -run TestExtractLowestCashPrice` passes. Run the extractor manually against the user's existing `/tmp/sea-nrt-hydrated.json` (cash-mode search response) and confirm it returns a sensible price floor matching what the human-readable browser flow shows.

---

### U3. Add `flights value-compare` cobra command

**Goal.** Wire the new command. Compose: parse flags → call cash search → call award search → call `valuation.Lookup` → call `valuation.Compare` (math) → build envelope → render. Honor all root flags (`--json`, `--agent`, `--dry-run`, `--no-cache`, `--select`).

**Requirements.** R1, R3, R4, R5, R7.

**Dependencies.** U1 (valuation package), U2 (cash extractor).

**Files:**
- `library/travel/alaska-airlines/internal/cli/flights_value_compare.go` (NEW)
- `library/travel/alaska-airlines/internal/cli/flights_value_compare_test.go` (NEW)
- `library/travel/alaska-airlines/internal/cli/flights.go` (MOD — single `cmd.AddCommand(newFlightsValueCompareCmd(flags))` line under the existing PATCH(amend-2026-05-19) block)

**Approach.**

- `newFlightsValueCompareCmd(flags *rootFlags) *cobra.Command` mirrors the shape of `newFlightsAwardCheapestCmd`. Cobra annotations: `{"pp:endpoint": "flights.value_compare", "pp:method": "GET", "pp:path": "/search/results/__data.json (paired)", "mcp:read-only": "true"}`.
- Flags:
  - `--origin`, `--destination` (required, IATA codes)
  - `--depart` (required, YYYY-MM-DD)
  - `--return` (optional, YYYY-MM-DD; presence triggers round-trip)
  - `--cabin` (default `economy`)
  - `--adults` (default `1`), `--children` (default `0`)
  - `--max-stops` (default `-1` = unset, matching award-cheapest)
  - `--program` (default `atmos`; v1 accepts only `atmos` and returns an error otherwise — surfaces the seam without shipping non-existent programs)
  - `--cpp` (float, optional; overrides TPG lookup)
  - `--no-valuation-cache` (bool, forces TPG re-scrape even if cache is fresh)
- `RunE`:
  1. Build cash params (extract a small `buildCashSearchParams(in cashSearchInput) map[string]string` helper into `flights_search.go` so the cash side has the same testable seam as `buildAwardSearchParams`; this is a small lift but cleaner than copy-paste). Build award params via the existing `buildAwardSearchParams`.
  2. `c := flags.newClient(); defer c.Close()`. Two `resolveRead` calls — cash then award. Surface both provenance lines via `printProvenance` to stderr.
  3. `cashRes := extractLowestCashPrice(cashData, maxStops, cabin)`; `awardRes := extractLowestAwardPrice(awardData, maxStops)`. If either returns zero value → emit envelope with `"comparison": null` and a `meta.note` explaining ("no cash itinerary found", "no award inventory at this cabin", etc.) — still soft-fail-with-data, not hard error.
  4. `cpp, fetchedAt, source, vErr := valuation.Lookup(ctx, valuation.ProgramAtmos, valuation.LookupOptions{Override: flagCPP, ForceRefresh: noValCache})`. If `vErr != nil`, stderr-warn but proceed (lookup already returned a fallback). If both cash and award were extracted, call `valuation.Compare(cashRes, awardRes, cpp)` to get the comparison block.
  5. Build envelope:
     ```
     {
       "meta": {
         "source": "live",
         "origin": ..., "destination": ..., "depart": ..., "return": ..., "cabin": ...,
         "adults": ..., "children": ...,
         "program": "atmos",
         "cpp_baseline_cents": cpp,
         "cpp_baseline_source": source,
         "cpp_baseline_fetched_at": fetchedAt.UTC().Format(time.RFC3339),
         "tpg_url": "https://thepointsguy.com/loyalty-programs/monthly-valuations/",
         "valuation_warning": vErr-message-if-any,
       },
       "results": {
         "cash":  {"price_usd": ..., "currency": ..., "carrier": ..., "cabin": ..., "stops": ...},
         "award": {"miles": ..., "taxes_usd": ..., "carrier": ..., "cabin": ..., "stops": ...},
         "comparison": {
           "effective_cpp_cents": ...,
           "multiple": ...,
           "tpg_valued_usd": ...,
           "cash_saved_usd": ...,
         }
       }
     }
     ```
  6. Render: JSON path → `printOutput(cmd.OutOrStdout(), envelope-as-json, true)`. TTY path → `printValueCompareTable(out, envelope)` — small two-row + comparison-row table renderer next to `printAutoTable`. Use the existing `--select` flag plumbing in `printOutputWithFlags` for JSON-mode field selection.

**Patterns to follow.**
- `internal/cli/flights_award_cheapest.go` lines 100–250 — envelope construction, `meta` map shape, custom `Annotations`, mcp:read-only hint, `printOutput` with manually-built JSON.
- `internal/cli/flights_award_search.go` `awardSearchInput` + `buildAwardSearchParams` — the exact factoring pattern to apply when extracting `buildCashSearchParams` from `flights_search.go`.
- `internal/cli/data_source.go:resolveRead` — single shared chokepoint for reads; the same call used by every other read-side command.

**Test scenarios.**

- `TestBuildCashSearchParams_AlwaysSetsCorePieces`: given a `cashSearchInput`, the param map carries `O`, `D`, `OD` (depart date), `A`, `C`, `L`, `RT`, `locale`. Mirrors `TestBuildAwardSearchParams_AlwaysSets`.
- `TestBuildCashSearchParams_OmitsAwardKeys`: confirms the cash builder does NOT set `ShoppingMethod`, `UPG`, `OT`, `DT` (the award-only keys). Guards against accidental award-mode leakage.
- `TestBuildCashSearchParams_RoundTripFlag`: when `Return != ""`, `RT="true"` and `DD=<return>`. When `Return == ""`, `RT="false"` and `DD` is absent.
- `TestValueCompareEnvelope_HappyPath`: given fixed cash + award + cpp inputs, the envelope's `comparison.effective_cpp_cents` and `comparison.tpg_valued_usd` and `comparison.multiple` match the expected numbers (the FCO-SEA example: cash $1766.23, award 30000+$64.53, cpp 1.4 → effective ~5.67¢, multiple ~4.05, tpg-valued ~$484.53).
- `TestValueCompareEnvelope_NoAwardInventory`: when the award extractor returns zero, envelope's `results.award` is null-like and `comparison` is null-like with a `meta.note`. Command exits 0, not non-zero.
- `TestValueCompareEnvelope_NoCash`: symmetric — no cash itinerary → `results.cash` null, `comparison` null, note set.
- `TestValueCompareEnvelope_ValuationFallback`: stub `valuation.Lookup` returning `("fallback-stale", non-nil-err)` → envelope's `meta.cpp_baseline_source = "fallback-stale"`, `meta.valuation_warning` non-empty, exit 0.
- `TestValueCompareEnvelope_OverrideCPP`: `--cpp 2.0` → envelope's `cpp_baseline_source="override"`, lookup not called.

**Verification.** `go test ./internal/cli/ -run TestValueCompare` passes. `go test ./internal/cli/ -run TestBuildCashSearchParams` passes. End-to-end smoke: `alaska-airlines-pp-cli flights value-compare --origin FCO --destination SEA --depart 2026-08-30 --json --agent` returns an envelope with all expected fields populated. `flights --help` shows `value-compare` in the subcommand list. `alaska-airlines-pp-mcp` (MCP server) at startup logs `flights_value_compare` as a registered tool (no hand-registration needed).

---

### U4. Catalog the patch, update SKILL.md, register the novel feature

**Goal.** Land the discoverability layer so the agent and the SKILL.md mirror know the new command exists. The repo's bot-driven `cli-skills/pp-alaska-airlines/SKILL.md` regenerates from the library-side SKILL.md post-merge; do NOT touch the mirror.

**Requirements.** All from the discoverability layer of R7 (MCP) plus standard PP-library hygiene.

**Dependencies.** U3 must be wired (the patch entry references files that exist).

**Files:**
- `library/travel/alaska-airlines/.printing-press-patches.json` (MOD — new entry, id `value-compare`)
- `library/travel/alaska-airlines/SKILL.md` (MOD — Unique Capabilities subsection, Command Reference row, Recipes example)
- `library/travel/alaska-airlines/.printing-press.json` (MOD — `novel_features` array append)
- `library/travel/alaska-airlines/AGENTS.md` (MOD only if a new operational caveat applies — e.g. "value-compare scrapes thepointsguy.com on cache miss; first invocation requires network egress to that host")

**Approach.**

- `.printing-press-patches.json`: append an entry mirroring the existing `award-cheapest-planner` shape:
  ```
  {
    "id": "value-compare",
    "summary": "New flights value-compare command runs paired cash + award searches and applies a TPG cents-per-point valuation, with a 30-day on-disk cache.",
    "reason": "Manually composing cash + award + TPG lookup is a five-step ritual that recurs constantly; agents need a single read for the comparison.",
    "files": ["internal/cli/flights_value_compare.go", "internal/cli/flights_value_compare_test.go", "internal/cli/flights.go", "internal/cli/flights_award_cheapest.go", "internal/valuation/..."]
  }
  ```
- `SKILL.md` Unique Capabilities: new bullet next to "Award (miles+cash) planner" — "Cash vs. award (points) comparator with TPG valuation: `flights value-compare` runs paired cash + award searches and emits an apples-to-apples comparison using The Points Guy's published cents-per-point (cached locally for 30 days)."
- `SKILL.md` Command Reference: new line under the `flights` block — `flights value-compare`  with one-line description and a `--cpp` flag reference.
- `SKILL.md` Recipes: new recipe block — the FCO-SEA example as documented in this plan's Summary section, plus a `--cpp 1.2` override example showing how to swap NerdWallet's number into the comparison.
- `.printing-press.json` `novel_features` array: append `{"name": "Cash vs award comparison with TPG valuation", "command": "flights value-compare", "description": "Runs paired cash + award searches for the same itinerary and applies a cents-per-point valuation (default: Alaska/Atmos from thepointsguy.com, cached 30 days). Returns effective cpp, TPG multiple, and TPG-valued dollar cost of the points option."}`
- `AGENTS.md`: only add a one-line note under "Operational notes" if needed: "First `flights value-compare` invocation makes one HTTPS GET to thepointsguy.com to fetch the published Alaska cpp; subsequent runs read the 30-day on-disk cache."

**Patterns to follow.**
- Existing `.printing-press-patches.json` `award-cheapest-planner` entry — copy field shape exactly.
- Existing `SKILL.md` Unique Capabilities formatting — same bullet density, same description tone.

**Test scenarios.** Test expectation: none — this unit is documentation/catalog only.

**Verification.** `python3 .github/scripts/verify-skill/verify_skill.py --dir library/travel/alaska-airlines/` passes (every flag mentioned in SKILL.md must exist on a cobra command in `internal/cli/*.go`). The mirror at `cli-skills/pp-alaska-airlines/SKILL.md` is NOT touched (it regenerates post-merge by the bot). `agent-context --pretty` from the freshly-built CLI shows the `value-compare` command and its annotation.

---

## System-Wide Impact

- **HTTP egress surface expands by one host.** alaska-airlines-pp-cli previously called only alaskaair.com; now also thepointsguy.com once per 30 days per cache directory. Worth a single line in AGENTS.md (see U4) so air-gapped or strict-egress users know what to expect.
- **New optional dependency** `github.com/PuerkitoBio/goquery`. Adds ~250KB to the binary. Already widely used in sibling PP CLIs (airbnb, vrbo). Accepted.
- **MCP tool surface gains one read-only tool** (`flights_value_compare`) auto-generated by cobratree. No hand-registration. Downstream MCP consumers (Claude Code, agent harnesses) discover it automatically at startup.
- **No schema migration** — local SQLite store is untouched. The valuation cache is a flat JSON file in `~/.cache/alaska-airlines-pp-cli/valuations/`, isolated from the existing HTTP cache and the data.db.
- **No effect on existing commands.** `flights search`, `flights search --award`, `flights award-search`, `flights award-cheapest` are untouched (U2 adds new code in `flights_award_cheapest.go` but does not modify existing functions).
- **Downstream users of the printing-press-library catalog** (`agent-context --pretty`, registry.json bot-regen) automatically pick up the new command via the manifest and SKILL.md update.

---

## Risks

R-1. **TPG selector breaks.** Thepointsguy.com may restructure the monthly valuations page, breaking the goquery selector. Mitigation: typed `ErrTPGParse` surfaces the failure cleanly; the soft-fallback chain (`fallback-stale` → `fallback-constant`) keeps the command working with a slightly stale or constant baseline. CI smoke probe (run monthly via GitHub Action — out of scope for this plan but worth filing as a follow-up issue) would catch the break before users do.

R-2. **TPG anti-bot / Cloudflare challenge.** The page is a public marketing page so Cloudflare's bot-fight is unlikely to fire, but it's possible. Mitigation: `ErrTPGBlocked` detection via title-tag inspection; soft-fallback to cached/constant; Chrome User-Agent header reduces challenge probability. Headless browser escalation is explicitly out of scope (D1) — if Cloudflare becomes an issue, the right next step is a separate amend with chromedp or a hardcoded snapshot updated via monthly PR, not a feature added to this plan.

R-3. **Cash extractor JSON-key drift.** The SvelteKit response shape is sniffed-not-contracted; field names could change. Mitigation: the walker uses the same fallback-list pattern as `extractLowestAwardPrice` (try multiple candidate keys, tolerate missing fields). Test coverage on synthetic fixtures + real-fixture smoke validation in U2 verification. The existing award extractor has survived for the previous amend cycle — same tolerance applies.

R-4. **TPG valuation methodology change.** TPG occasionally re-bases their methodology (e.g. moves from "max value" to "data-backed average"), which would cause a noticeable cpp shift on a single month. Not a bug, but the cached 30-day-old value could disagree with a fresh fetch by ~30%. Mitigation: the meta envelope surfaces `cpp_baseline_fetched_at` so agents reading historical data know the as-of date. Acceptable.

R-5. **Round-trip vs one-way mismatch.** If the user requests a round-trip but the cheapest cash itinerary is via airport A↔B and the cheapest award is A↔C (different routing), the comparison is "valid" but possibly misleading. Mitigation: the envelope surfaces carrier + stops + cabin per leg so the agent can detect a mismatch and re-query with `--max-stops 0` or explicit destination. Documented in SKILL.md.

R-6. **Concurrent cache writes.** If two value-compare calls fire simultaneously and both miss the cache, both will scrape TPG. Mitigation: the writes are idempotent — both will write the same JSON file with similar mod-times. No file-lock needed for a 30-day-TTL one-key cache.

R-7. **Schema additions are not gated.** v1 ships `--program atmos` only but reserves the flag; if v2 adds `--program united` and the registry is not updated, the command surfaces a clean error from `BySlug` returning `(_, false)`. Validated in U1 test.

---

## Verification Strategy

Per-unit verification is defined inline above. End-to-end checks before opening PR:

1. `go build ./...` from the alaska-airlines CLI root — success.
2. `go vet ./...` — clean.
3. `go test ./...` — all new unit tests pass; existing tests still pass.
4. `python3 .github/scripts/verify-skill/verify_skill.py --dir library/travel/alaska-airlines/` — passes (no flag drift between SKILL.md and cobra).
5. `govulncheck ./...` — clean.
6. End-to-end: blow away `~/.cache/alaska-airlines-pp-cli/valuations/` and run `alaska-airlines-pp-cli flights value-compare --origin FCO --destination SEA --depart 2026-08-30 --json --agent`. Confirm:
   - Both cash and award are populated.
   - `meta.cpp_baseline_source` is `tpg-live` and `meta.cpp_baseline_fetched_at` is now-ish.
   - `comparison.effective_cpp_cents` is in the right ballpark (~5.67 for the documented FCO-SEA example).
   - Re-run immediately and confirm `meta.cpp_baseline_source` flips to `tpg-cached`.
   - Run again with `--cpp 2.0` and confirm `cpp_baseline_source` flips to `override` and `multiple` shifts accordingly.
7. End-to-end MCP: start `alaska-airlines-pp-mcp` and confirm `flights_value_compare` appears in the tool listing with `read_only_hint: true`.
8. Manual fallback test: temporarily point the TPG URL constant at `https://httpstat.us/503`, blow away cache, invoke command — confirm stderr warning fires and envelope `cpp_baseline_source` is `fallback-constant` with the 1.4 baseline applied. Restore URL.
9. Per AGENTS.md preflight: open the PR against `printing-press-library`, let Greptile review, resolve any P0/P1 before merge.

---

## Open Questions Deferred to Implementation

- **Exact cash JSON field name.** Resolved by inspecting `/tmp/sea-nrt-hydrated.json` (already on disk from the user's manual session) during U2 implementation. If the keys are not stable across response variants, the walker's candidate-key list grows — same pattern as the existing award extractor.
- **Optimal TPG row matcher.** `def.TPGRowMatch` is a substring prefix today ("Alaska Airlines Atmos"). If TPG renames the program (it was "Mileage Plan" before Atmos rebrand in 2025-2026), the matcher needs adjustment. Implementation-time decision: prefer substring containment over exact match.
- **Compact vs full table layout for TTY mode.** `printValueCompareTable` should fit on one screen; if rendering looks cramped, fall back to `printAutoTable` (which auto-picks card layout above 8 fields). Decided at implementation time after eyeballing real output.
- **Whether to surface a `verdict` string** (`good_redemption`, `fair`, `poor`) keyed off `multiple`. Punted — agents can compute this themselves from `multiple`; baking thresholds into the CLI is the kind of taste claim that ages badly. If the user asks, add it as a v1.1.
