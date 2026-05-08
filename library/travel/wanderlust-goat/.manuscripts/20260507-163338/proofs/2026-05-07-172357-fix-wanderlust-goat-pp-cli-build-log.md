# Build Log — wanderlust-goat v2 reprint

## Architecture inversion (v1 → v2)

v1 (broken): orchestrator planned 12-source fanouts; only 4 source clients (overpass, wikipedia, reddit, atlasobscura) were ever invoked. Other 12 (tabelog, retty, hotpepper, notecom, michelin, eater, timeout, nyt36hours, wikivoyage, lefooding, naverblog, navermap) were scaffolded with passing unit tests but never imported by the orchestrator.

v2 (this run): inverted to a two-stage funnel.
- **Stage 1**: seed candidates from Google Places (NearbySearch). Hard-filter `business_status != OPERATIONAL` at seed.
- **Stage 2**: deep-research each candidate against locale-aware sources from `internal/regions/regions.go` (data table, single source of truth). Closed-signal kill-gate drops anything any source confirms permanently closed.
- **Stage 3**: trust-weighted ranking (Google base + locale boost + Wikipedia notability + Reddit thresholds), walking-time radius (4.5 km/h × 1.3 tortuosity).

## What was built

**New packages:**
- `internal/regions/regions.go` — country → research-stage map (data table). 8 regions + fallback. Adding a country = adding one row.
- `internal/googleplaces/` — Google Places API (New) v1 client. NearbySearch + SearchText. CLOSED_PERMANENTLY filter at seed. `ErrMissingAPIKey` for the Stage-1 auth gate.
- `internal/walking/` — pure-Go haversine + walking-time helpers. 4.5 km/h × 1.3 tortuosity.
- `internal/closedsignal/` — every locale's "permanently closed" detector in one package: Tabelog "閉店"/"営業終了", Naver "폐업", Le Fooding "fermé définitivement", OSM `disused:amenity=*` / `opening_hours=closed`, Google `CLOSED_PERMANENTLY/CLOSED_TEMPORARILY`, recent-review keyword scan.
- `internal/sourcetypes/` — shared `Client` interface + `StubClient` reusable shape.
- `internal/dispatch/` — full v2 rewrite of the orchestrator. Two-stage funnel + ranking + closed-signal kill-gate. v1's `dispatch` and `goat_orchestrator` are gone.

**Stage-2 real-impl source clients (per brief: JP/KR/FR fully):**
- JP: `internal/tabelog/`, `internal/retty/` (v1's deferred body extraction promoted), `internal/hotpepper/` (API path when `HOTPEPPER_API_KEY` is set, HTML fallback otherwise).
- KR: `internal/navermap/`, `internal/naverblog/`.
- FR: `internal/lefooding/`.

Each carries a `Client` with `Slug() / Locale() / IsStub() / LookupByName(ctx, name, city, max) / CheckClosed(ctx, hit)`. Conservative per-instance throttle (1.5s between requests). httptest-backed parser tests.

**Stage-2 stub source clients (typed Go packages, real `LookupByName` returning `ErrNotImplemented`):**
- 19 stubs total: notecom, hatena (JP); kakaomap, mangoplate (KR); pudlo, lafourchette (FR); gamberorosso, slowfood, dissapore (IT); falstaff, derfeinschmecker (DE); verema, eltenedor (ES); squaremeal, hotdinners, observerfood (UK/IE); dianping, mafengwo, xiaohongshu (CN).
- All embed `sourcetypes.StubClient` and report a deferral reason via `coverage`.
- All are imported by `internal/dispatch/registry.go` so the wiring test passes (and so promoting any stub to a real source is a single-package edit).

**Compound commands (`internal/cli/*.go`):**
| Command | Source | Notes |
|---|---|---|
| `near` | new | Headline two-stage funnel; positional or `--anchor`; `--criteria`, `--identity`, `--minutes`, `--top`, `--seed-limit`, `--type`, `--llm` |
| `goat` | new | Same funnel as `near`, no-LLM; deterministic heuristic criteria match |
| `research-plan` | new | Typed JSON query plan emitter; pure metadata, no live calls |
| `status` | new (v2) | Per-source operational/closed lookup; surfaces conflicting signals explicitly |
| `why` | new | Score breakdown for one place; uses Google Places SearchText to seed |
| `coverage` | new | Per-region source coverage + stub deferral reasons |
| `sync-city` | new (v2) | Pre-cache geo-anchored sources AND prewarm every implemented Stage-2 source for the country (the v1 silent-drop fix) |
| `golden-hour` | ported v1 | Pure-Go SunCalc; works without API keys |
| `reddit-quotes` | ported v1 | Verbatim quotes from local `goat_reddit_threads`; cross-name lookup |
| `crossover` | ported v1 | Spatial join: food + culture pairs within 200m |
| `route-view` | ported v1 | OSRM walking polyline + spatial buffer |
| `quiet-hour` | ported v1 | Reddit "dead before, quiet on" + OSM hours intersect |

**Wiring invariant test (`internal/cli/wiring_test.go`):**
- Uses `go/parser` + `go/ast` to walk every `internal/<source>/` package.
- Asserts (a) the source is imported by either `cli/` or `dispatch/`, AND (b) at least one of its exported functions is called from one of those packages.
- Mutation-tested: dropping a source from the dispatcher registry, or creating an unwired source dir, both fail the test with a clear message naming the missing source.

## Quality gates

- `go fmt`, `go vet ./...`: clean.
- `go test ./...`: 43 packages pass.
- Phase 4 shipcheck umbrella: 5/5 legs PASS (dogfood, verify, workflow-verify, verify-skill, scorecard). Scorecard 85/100 Grade A.
- `internal/cli/wiring_test.go`: passes; mutation tests confirm it catches v1-class regressions.

## Intentional deferrals (per brief)

- Foursquare, Yelp, Mapillary, Flickr: not in scope.
- Atlas Obscura geo-near: kept as candidate-enrichment only (returns 0 silently when city slug isn't indexed).
- Stage-2 real impl for IT/DACH/ES/UK/CN: stubs only; ship JP/KR/FR fully.
- LLM (`--llm`) path: hooked in `Plan.UseLLM` for future wiring; v2 ships heuristic-only criteria match.

## Generator/machine-side notes (for retro)

- The generator emits the SKILL canonical install URL using research.json's `category`. `verify-skill` expects `library/other/` regardless. Workaround: SKILL ships with `library/other/`. Filing as a Printing Press machine inconsistency.
- `mcp_token_efficiency` scored 4/10 because the spec only declares 2 endpoints (Nominatim places search/reverse). The runtime cobra-walker exposes the rest of the surface dynamically as MCP tools at server start. Brief didn't call for MCP enrichment; not addressed.
