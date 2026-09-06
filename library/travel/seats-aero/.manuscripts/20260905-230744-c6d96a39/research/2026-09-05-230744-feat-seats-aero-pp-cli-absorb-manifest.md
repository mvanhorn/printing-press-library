# seats-aero — Absorb Manifest (2026-09-05, reprint under press 4.31.7)

Sources: `2026-09-05-230744-feat-seats-aero-pp-cli-brief.md`, `…-absorb-catalog.md` (24 tools cataloged; 3 Partner-API-backed wrappers, 11 independent competitors), `…-novel-features-brainstorm.md` (subagent audit trail). Reprint: prior CLI 2026.6.1 (press 3.10.0) had 3 novel features; all three are **reframed** below, none dropped.

## Absorbed (match or beat everything that exists)

Endpoint commands are generator-emitted from the merged spec (`seats-aero-openapi-2026-09.yaml`). The `/search` operation carries `x-pp-resource: awards` so its command is `awards …`, not the reserved framework `search`.

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Cached award search across programs with cabins/direct/carriers/date/order filters | gavgrego MCP `get_flights`; prior CLI `seats-aero-partner-search` | (generated endpoint) awards GET /search | `--sources` multi-program in one call, plural `cabins`, `--json/--select/--agent`, results upserted into typed `availability` table |
| 2 | Bulk per-program availability calendar | gavgrego `get_bulk_avail`; prior CLI `availability` | (generated endpoint) availability GET /availability | cursor pagination handled by sync; typed `availability` table with `synced_at` + `first_seen_at` |
| 3 | Flight-level trip detail for an Availability row | gavgrego `get_trips`; prior CLI `trips` | (generated endpoint) trips GET /trips/{id} | cached in `trips` table keyed by availability id |
| 4 | Route coverage listing per program | gavgrego `get_routes`; prior CLI `routes` | (generated endpoint) routes GET /routes | fully synced (~87k rows), FTS5 over airports/regions/program, `x-pp-pagination: none` |
| 5 | Nonstop "where can I go" fan-out from one airport | gavgrego `get_destinations` | (generated endpoint) destinations GET /destinations | new since the prior CLI (zero prior coverage) |
| 6 | Refresh stale cached availability before booking | gavgrego `refresh_cached_data` | (generated endpoint) refresh POST /refresh | response `quota{}` surfaced; Pro-only gating in description; live dogfood tier-gated so credits are never spent by the harness |
| 7 | Live point-in-time search | gavgrego `live_search`; AwardTool | (generated endpoint) live POST /live | commercial-only gating stated in help + tool description; `x-pp-mutation: false` so the body prints; never retried on 403 |
| 8 | Daily-quota awareness (`X-RateLimit-Remaining`) | vendor docs only — no wrapper surfaces it | (behavior in seats-aero-pp-cli doctor) probe one cheap call and report remaining daily calls + inferred key tier | no community tool does this |
| 9 | Multi-airline fan-out in one call | AwardTravelFinder `search_all_airlines` | (behavior in seats-aero-pp-cli awards) `--sources` csv across all 26 programs | one call, not N |
| 10 | Day-by-day month scan | AwardTravelFinder `search_monthly_availability` | (behavior in seats-aero-pp-cli availability) date window + cursor paging | offline re-query after sync via `calendar` |
| 11 | Local sync / FTS search / analytics / export / import / workflow / doctor / which / agent-context / learn loop | prior CLI framework commands | framework (generator-emitted) | typed `routes`/`availability`/`trips`/`destinations` tables replace the generic `resources` table; `x-rate-class: daily` → sync concurrency 1 |
| — | Award charts, points valuation, transfer paths, hotels, cash+points hybrid, push alerts, OAuth "Login with Seats.aero" (`/token` `/consent` `/userinfo`) | AwardTravelFinder, point.me, PointsYeah, seats.aero web app | OUT OF SCOPE — not in the Partner-Authorization API surface (OAuth flow is a consumer-app surface, documented as known-but-unimplemented) | contrast only |

No stubs. Every absorbed row ships.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Source | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------|--------------|----------|------------------|
| 1 | New-since award watch | `new-since --origin JFK --destination NRT --cabin business --since 24h` | 10/10 | hand-code | prior (reframed from `seats-aero-partner-search`) | Reads the typed `availability` table's per-row `first_seen_at` (set on first upsert during `sync`) to list rows newly visible after a cutoff; local-only, no live call. | Brief § Table Stakes: alerts are a web-app-only feature; poll-and-diff is the local approximation. Top Workflow #1/#5. | Use this command to see which cached availability rows are newly visible since a past point in time. Do NOT use this to re-verify a specific already-known Availability ID is still bookable before booking; use 'recheck' instead. |
| 2 | Cabin/date calendar matrix | `calendar --origin JFK --destination NRT --source united --start 2026-10-01 --end 2026-12-31` | 8/10 | hand-code | prior (reframed from `availability`) | Pivots one route's synced `availability` rows into a date × Y/W/J/F matrix (available / lowest mileage / direct); local-only. | Brief Top Workflow #2 (region/calendar scan builds shortlists); Data Layer (`availability` dated by `Date`). | Use this command to view one route's full cabin-by-date matrix from already-synced availability. Do NOT use this to filter across multiple routes/programs for direct-only options under a mileage ceiling; use 'direct-scan' instead. |
| 3 | Nonstop reach finder | `reach --origin JFK --cabin business --max-mileage 90000 --top 10` | 8/10 | hand-code | new | Live `GET /destinations` fan-out (1 call) joined with the local `availability` table for dated seats per candidate; bounded live `/search` fallback per top-N only when local has nothing. `--data-source auto`. | Brief § Table Stakes (destinations missing from prior CLI); Top Workflow #4; MCP intent `explore_from_airport`. | Use this command to discover which destinations are reachable nonstop from one origin airport, ranked by mileage cost. Do NOT use this to filter already-synced availability for a route you already know; use 'direct-scan' or 'calendar' instead. |
| 4 | Credit-aware recheck | `recheck --origin JFK --destination NRT --cabin business --max-mileage 90000 --dry-run` | 10/10 | hand-code | new | Drain-first read of local `availability` to shortlist matching, aging IDs (reports `synced_at` age even under `--dry-run`), then `POST /refresh` on the shortlist while checking `quota.remaining`; refuses when remaining quota is below the batch. Default is print-only; the live refresh is opt-in. | Brief Top Workflow #5; § Auth tier asymmetry; MCP notes `refresh_before_booking`. | Use this command to re-verify specific already-known Availability rows are still live before booking, with a quota guard. Do NOT use this to discover newly appeared availability across a route; use 'new-since' instead. |
| 5 | Cross-program direct-only scan | `direct-scan --origin JFK --destination NRT --cabin business --max-mileage 90000 --sources united,virginatlantic,aeroplan` | 9/10 | hand-code | prior (reframed from `routes`) | Cross-program filter over the local `availability` table for direct-only seats under a mileage ceiling — `/availability` is one program per call upstream, so this join has no single live equivalent; local-only. | Brief § Data Layer (`/availability` scoped to one program); catalog row `search_all_airlines` contrast. | Use this command to filter already-synced availability across ALL routes and programs for direct-only flights under a mileage ceiling. Do NOT use this to view a single route's full date-by-cabin matrix (use 'calendar') or to discover new destinations from an origin with no fixed route in mind (use 'reach'). |

Dropped prior features: **none**. All three prior novel features are reframed (see Source column); their raw endpoint mirrors survive as absorbed rows 1, 2, 4.

Hand-code commitment: **5 of 5** transcendence rows are `hand-code` (~50–150 LoC each plus registration). 0 `spec-emits`.

### Data-layer requirements the transcendence rows impose (Priority 0)

- Typed `availability` table with `first_seen_at` (set once on first upsert) and `synced_at` (updated every upsert). `new-since` and `recheck` depend on these.
- Typed `routes` table (FTS5 over OriginAirport/DestinationAirport/OriginRegion/DestinationRegion/Source).
- `trips` cached per AvailabilityID; `destinations` cached per origin.
- All local commands call `hintIfUnsynced` / `hintIfStale` before returning; SQLite drain-first pattern; no `store.Upsert` inside an open write tx.

### Harness-safety notes for Phase 3

- `recheck`: `pp:happy-args` must include `--dry-run`; the real `/refresh` call is opt-in (`--apply`-style flag) and refuses under `cliutil.IsAnyHarness()`.
- Local-only commands (`new-since`, `calendar`, `direct-scan`) declare `pp:typed-exit-codes: "0,3"` and `pp:happy-args` with realistic positionals/flags so the publish hollow-coverage gate can count the graceful-empty exit.
- `reach` curtails to `--top 3` under `cliutil.IsDogfoodEnv()`.

## Phase Gate 1.5

- **Approved — generate now** (user, 2026-09-05 via AskUserQuestion). Full 16-row manifest (11 absorbed + 5 novel), no trims, no additions.
