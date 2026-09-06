# Seats.aero CLI Brief

## API Identity

**Domain.** Seats.aero's Partner API is the programmatic surface behind seats.aero's own award-search web app: pre-computed ("cached") availability for ~70,000+ origin-destination routes across mileage programs, refreshed on a schedule, plus a small live-search surface for on-demand checks. It answers "where/when can I redeem miles for a seat" — not booking, not pricing in cash, not fare search.

**Users.** Per docs.seats.aero and developers.seats.aero: (1) **seats.aero Pro subscribers** building personal tooling (scripts/bots/Discord alerts) against their own 1,000-call/day allotment — the dominant real-world user per every wrapper found; (2) **commercial partners** with a written agreement (travel agencies, OTA-adjacent tools, "Login with Seats.aero" consumer apps) who get live search + higher/negotiated limits; (3) points/miles bloggers and "award hacking" communities (FlyerTalk, Frequent Miler, Thrifty Traveler) who use seats.aero (the product) as a research tool but rarely touch the API directly; (4) agent/tool builders wiring award search into Claude/MCP workflows (5+ community MCP servers found — see absorb catalog).

**Data profile.** Read-mostly, moderate-cardinality, time-windowed. Core objects: `Route` (O-D pair + program + distance + region, ~70k+ tracked), `Availability` (a route+date+program summary row with 4 cabins × {available, direct, mileage cost, remaining seats, airlines}), `Trip` (flight-level segments under an Availability, fetched on demand via a paid "credit" — `refresh`/`trips`), plus new `Destinations` (cheapest-nonstop-per-cabin fan-out from one airport). No user PII in the API itself; the OAuth "Login with Seats.aero" flow (new since 2026-05) does carry per-user tokens/consent if a partner app is built on it, which is out of scope for a personal single-key CLI.

## Reachability Risk

- **`https://seats.aero/openapi.json` (marketing host) returns HTTP 403 "Just a moment..."** — a Cloudflare interstitial on `seats.aero` the marketing/app domain, NOT the API. Do not read this as the Partner API being blocked.
- **API host is `https://seats.aero/partnerapi`**, distinct from the Cloudflare-challenged marketing host and from `developers.seats.aero` (ReadMe-hosted docs, plain HTTP 200, no challenge observed). No probe was made against `partnerapi` itself (no key sent per instructions); no evidence of Cloudflare gating on that host was found in vendor docs or community repos.
- **No GitHub issues found reporting "403"/"blocked"/"deprecated" against the Partner API itself** across the wrapper repos checked (gavgrego/seats.aero-mcp-server, denverquane/seats-aero-go). The one operational risk repeatedly documented by the vendor and echoed by wrappers is the **daily quota**, not blocking:
  > "Pro API access includes 1,000 API calls per calendar day... resets daily at midnight UTC... Every API response includes a header showing how many API calls you have left for the current day: `X-RateLimit-Remaining`. Once this reaches zero, API requests will be rejected until the daily reset." — docs.seats.aero/article/68-seatsaero-pro-api-access-limits-and-usage
- **Commercial vs Pro tier gating is asymmetric and a real integration trap**, confirmed by both the vendor's own KB and an independent MCP server's README:
  - `live_search` (`POST /live`): **NOT available to Pro API keys, regardless of use case** — commercial-agreement partners only.
  - `refresh_cached_data` (`POST /refresh`): available to Pro users; **"cannot currently be used by commercial users; the API documentation directs commercial users to `live_search` instead"** (gavgrego/seats.aero-mcp-server README).
  - This means no single API key can exercise both premium endpoints — a CLI that assumes "Pro key = full access" will silently 4xx on `/live`.
- **Geography/eligibility gating**: "Not all Pro users can access the partner API key... We may limit access to the API by geographical location or for any reason in our sole discretion" (developers.seats.aero/reference/getting-started-p, repeated on the KB article). A `doctor` check should treat "API tab not visible" / a 403 on first call as an eligibility question, not a code bug.
- **Commercial use is prohibited without written approval** — "The Seats.aero partner API can be used by Pro users for non-commercial purposes and only by written agreement for commercial use" (getting-started-p). Relevant to how this CLI's README should frame itself (personal-use tool, not a resale product).

## Top Workflows

1. **"Find me a business/first award to X in the next N months across every program I care about"** — the flagship seats.aero use case (cached search across all sources, `order_by=lowest_mileage`, `cabins=business,first`). This is `/search`, and it's the #1 reason people use seats.aero over checking each airline site.
2. **"Scan a whole region's calendar for one program"** (Bulk Availability) — e.g. "show me every Delta SkyMiles J/F seat from North America to Europe this summer" — used to build shortlists before drilling into specific dates. This is the CLI's existing "Bulk award calendar" feature.
3. **"I found an Availability row — get me the actual bookable flight numbers/times/taxes"** (Get Trips) — the mandatory second step before booking anything, since Availability is a same-day summary, not a bookable itinerary.
4. **"Where can I fly nonstop from my home airport in [cabin] for the fewest miles, across everything?"** (Get Destinations, new endpoint) — a "where can my miles take me" discovery flow that didn't exist in the prior spec; strong candidate for a new novel-feature command.
5. **"Alert me / re-check this specific award before it's gone"** (Refresh Cached Data, new endpoint) — power users hit the credit-metered `/refresh` on a shortlist of Availability IDs right before booking, since cached data can be stale; distinct from seats.aero's own web-app "Alerts" feature (which is not part of the Partner API).

## Table Stakes

| Feature | Tool | Seats.aero-API-backed? | Notes |
|---|---|---|---|
| Cached multi-airport, multi-program search with cabin/direct/carrier filters | seats.aero web app, gavgrego MCP (`get_flights`), this CLI (`seats-aero-partner-search`) | Yes | Table stakes; our CLI already has it via `/search`, but is missing the current `sources`/`cabins`(plural)/`include_filtered`/`minify_trips`/`min_cabin_pct` filters (see Spec currency). |
| Bulk per-program calendar scan | seats.aero "Explore", gavgrego (`get_bulk_avail`) | Yes | Have it (`availability`). |
| Flight-level trip detail from a summary row | seats.aero web app, gavgrego (`get_trips`) | Yes | Have it (`trips`). |
| Route/coverage listing per program | gavgrego (`get_routes`) | Yes | Have it (`routes`). |
| **Nonstop "where can I go" fan-out from one airport** | gavgrego (`get_destinations`) | Yes | **Missing** — new endpoint, zero prior coverage. High-value novel-feature candidate. |
| **On-demand refresh of stale cached rows before booking** | gavgrego (`refresh_cached_data`) | Yes | **Missing** — new endpoint, credit-metered, has its own quota object in the response. |
| Live (non-cached) point-in-time search | gavgrego (`live_search`), AwardTool | Yes (Partner API `/live`), commercial-only | Gate behind a clear "requires commercial agreement" doctor check rather than silently failing. |
| Cross-provider aggregation beyond seats.aero's own program list (30+ programs incl. hotel/transfer) | point.me, PointsYeah, Roame, Award Travel Finder (127 tools, OAuth) | **No** — independent scrapers/aggregators, not seats.aero API consumers | Out of scope for this CLI; useful only as a "why choose us" contrast (see Product Thesis). |
| Program-specific live search with deep filters (32 simultaneous searches) | AwardTool | No — independent | Sets the UX bar for "live search" ergonomics if/when a commercial key is available. |
| Global map / visual "explore anywhere" | Roame.travel | No — independent | Not replicable via this API (no geo-fan-out endpoint); Destinations endpoint is the closest analog. |
| Award-chart / points-valuation / transfer-path tools | Award Travel Finder | No — independent, proprietary data | Out of scope; seats.aero API has no award-chart or transfer-partner data. |
| Alerts (push notification on new availability) | seats.aero web app itself | No — web-app-only feature, not exposed via Partner API | Cannot be replicated server-side by this CLI without seats.aero's own alert infra; a poll-and-diff pattern via `sync` + `search`/`analytics` is the closest local approximation. |

## Data Layer

**Primary entities to store** (replacing the current single generic `resources` table, per User Vision item 3):
- `routes` — `ID, OriginAirport, OriginRegion, DestinationAirport, DestinationRegion, NumDaysOut, Distance, Source` (one row per tracked O-D-program triple; ~70k+ rows per the docs' "over 70,000 routes" claim, matching the installed CLI's observed 87k synced rows).
- `availability` — the `/search` and `/availability` row shape (see Spec currency for the full/renamed field list): per-cabin available/direct/mileage-cost/remaining-seats/airlines, keyed by `ID`, foreign-keyed to `RouteID`, dated by `Date`. This is the natural sync target with a `cursor`-based incremental fetch (the API's own pagination cursor is a Unix timestamp — reuse it directly as the sync checkpoint rather than inventing a new one).
- `trips` — flight-segment detail fetched lazily per `AvailabilityID` (paid/credit endpoint; do not bulk-sync — cache on demand, keyed by availability ID, with a freshness/TTL check before re-spending a credit).
- `destinations` — small, cheap to sync per origin airport; good FTS/browse target for the "where can I go" workflow.

**Sync cursor.** Use the API's native `cursor` (Unix timestamp of first response) + `skip`, matching the documented pagination contract exactly ("you should treat it as an opaque integer when possible" — concepts-copy). Store last-successful cursor per `(source, cabin?)` tuple for `/availability` bulk syncs since that endpoint is scoped to one program at a time.

**FTS.** Full-text search has real value here — `routes` and `destinations` are essentially airport/region/program lookup tables well-suited to FTS5 over `OriginAirport, DestinationAirport, OriginRegion, DestinationRegion, Source`. `availability`/`trips` are better served by structured filtering (cabin, date range, mileage cost) than FTS, though a combined search over airline/carrier names in `YAirlines`/`Carriers` fields is reasonable.

**Cache freshness.** The installed CLI's "cache freshness scored 5/10" (User Vision) is explained by the generic `resources` table having no per-resource-type TTL logic — `availability` data is known-stale by design (that's the whole cached-vs-live distinction the vendor documents in "Cached Search vs Bulk Availability" and solved server-side via `/refresh`), so the local store should expose `synced_at` per availability row and surface it in the response envelope (`meta.source`/`meta.synced_at`, already partially present per the prior SKILL.md's documented envelope) rather than pretending local data is live.

## Spec currency

**Verdict: PATCH, not reuse-as-is.** The prior `spec.yaml` (4 endpoints: `/search`, `/availability`, `/trips/{id}`, `/routes`) is a strict subset of the current Partner API (7 active `Partner-Authorization` endpoints), confirmed independently by developers.seats.aero's own embedded OAS *and* by the community MCP server's README: *"This covers all seven active `Partner-Authorization` endpoints in the current seats.aero API reference."* (gavgrego/seats.aero-mcp-server README).

**New endpoints since the 2026-05 spec (add to spec.yaml):**
| Path | Method | operationId | Summary |
|---|---|---|---|
| `/destinations` | GET | `get-destinations` | Nonstop-only, cheapest-per-cabin fan-out from/to one airport, aggregated across all sources. |
| `/live` | POST | `live-search` | Point-in-time live search, JSON body, commercial-agreement-gated. |
| `/refresh` | POST | `create-refresh` | Queue a refresh of 1–250 stale Availability IDs; credit-metered with its own `quota{limit,used,remaining,reset_seconds}` block in the response. |

(Also present but **out of scope** for a Partner-Authorization-keyed CLI: `/token`, `/consent`, `/userinfo` — these belong to the separate "Login with Seats.aero" OAuth2 flow for consumer apps acting on behalf of end users, not to a personal API-key CLI. Note them in the manifest as known-but-unimplemented rather than silently dropping them.)

**Changed parameters on existing endpoints:**
- `/search` (Cached Search): **`cabin` (singular) is gone**, replaced by **`cabins`** (comma-delimited, e.g. `economy,business`) — a breaking rename from the prior spec's single-value `cabin` param. New params added: `sources` (comma-delimited program filter — search now spans *multiple* programs per call, not just filtered post-hoc), `include_filtered` (bypass dynamic-price filtering), `minify_trips` (trim `include_trips` payload), `min_cabin_pct` (mixed-cabin itinerary tolerance, 0–100).
- `/availability` (Bulk Availability): `cabin` (singular) is **retained** here (unlike `/search`) — do not conflate the two endpoints' cabin params when patching the spec. New params: `include_filtered`, `min_cabin_pct` (same semantics as above).
- `/trips/{id}`: new params `include_filtered`, `min_cabin_pct`.
- Response schema drift: `YMileageCost`/`WMileageCost`/`JMileageCost`/`FMileageCost` are now documented as **strings** (`"5000"`) in the vendor's own worked example (concepts-copy), not the prior spec's `integer, nullable` — verify against a live sample before committing to a type in the generated client, but treat the vendor's own example as authoritative over the 2026-05 spec's guess. `TripSegment` gained `AircraftName`+`AircraftCode` (prior spec had one `Aircraft` field) and `DepartsAt`/`ArrivesAt` (prior had `DepartureTime`/`ArrivalTime`); `Trip` gained `AvailabilityID`, `AvailabilitySegments` (renamed from `Segments`), `TotalDuration`/`TotalTaxes` (renamed from `Duration`/`Taxes`), `AllianceCost`, `FlightNumbers`, `Carriers`, `MixedCabinPct` (new, mixed-cabin support).
- `Route` gained `NumDaysOut` (not in prior spec).

**`Source` enum drift — 4 new mileage programs, real gap:** current docs list 26 sources; the prior spec's enum has only 22. **Missing from spec.yaml:** `finnair`, `lufthansa`, `frontier`, `spirit` (all confirmed live in the current "Sources" concepts table at developers.seats.aero/reference/concepts-copy, with cabin/seat-count/trip-data support columns). One anomaly to flag, not silently "fix": a worked example elsewhere on the same concepts page uses `"Source": "lifemiles"` (Avianca LifeMiles), which does **not** appear in the current 26-row Sources table — likely a stale example carried over from an older doc revision; do not add `lifemiles` to the enum without independently confirming it against a live `/routes?source=lifemiles` call (not attempted here per the no-key-usage instruction).

**ReadMe/OpenAPI export:** No downloadable OpenAPI JSON/YAML link was found in the reference page HTML (`href` scan for `openapi`/`.json`/`.yaml` returned nothing), and the marketing-host `seats.aero/openapi.json` 403s behind Cloudflare (not the API — see Reachability Risk). The current spec was reconstructed here by reading each ReadMe reference page's embedded OAS JSON (`document.api.schema`, OpenAPI 3.1.0) directly out of the fetched HTML — this is the same mechanism ReadMe uses to render its own docs, and it is complete and internally consistent (same `paths` object recurs verbatim across every reference sub-page checked). **No `seats-aero-openapi-current.json` was saved** — no single-URL machine-readable export was reachable; the reconstructed operation list above is the authoritative current-state input for patching `spec.yaml` by hand.

## Auth

- **Header:** `Partner-Authorization`, `apiKey`-type per the embedded OAS (`securitySchemes.sec0`), matching the prior spec exactly. Vendor KB shows the plain key form `pro_xxxxxxxxxxxxxxxxxxxxx` sent bare in the header (no `Bearer` prefix) for a personal Pro key; the *new* OAuth access-token form (`seats:ota:...`) is sent as `Partner-Authorization: Bearer seats:ota:123` — **two different header formats for the same header name** depending on auth mode (personal key vs. OAuth-obtained token). A CLI built purely around a personal API key never needs the `Bearer` form.
- **Env var:** the published CLI's own convention is `SEATS_AERO_API_KEY` (primary, per README/SKILL) with `SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION` also accepted "for generator compatibility" (a Printing-Press-generator artifact, not a vendor convention). **No canonical env var name exists in vendor docs** — the vendor's docs only ever say "send your API key in the `Partner-Authorization` header," with no suggested env var. Community tools disagree with each other: gavgrego's MCP server uses `SEATS_API_KEY`; the npm `@skybluu/seats-aero-mcp` package uses `SEATS_AERO_PARTNER_TOKEN`. **Recommendation:** keep `SEATS_AERO_API_KEY` as primary (it's already the installed CLI's convention and is the most self-describing), keep the generator-compat alias for continuity, and do not adopt either community variant.
- **OAuth:** Yes, but **only for the "Login with Seats.aero" consumer-app flow** (new since 2026-05) — a standards-compliant OAuth2 flow with `/consent`, `/token`, `/userinfo`, requiring a registered OAuth2 app (client ID/secret from `seats.aero/settings` → Apps tab) and a `redirect_uri`. This is for building multi-user consumer products, not for a single-operator personal CLI — **out of scope for this reprint**; document its existence in the manifest/README as a known-but-unimplemented surface so a future amend isn't surprised by it.
- **Key-tier gating:** a single key is either a **personal Pro key** (1,000 calls/day, no `/live`, has `/refresh`) or an **OAuth access token acting on behalf of a Pro user** (same 1,000/day limit, *shared* across all apps the user has authorized plus their own personal-key usage) or a **commercial key** (negotiated volume, has `/live`, no `/refresh`). The CLI cannot introspect which tier its configured key has — `doctor` should attempt a cheap, cache-friendly call (e.g. `/routes` for a single source) and surface the `X-RateLimit-Remaining` header value rather than assuming tier from configuration.

## MCP surface notes

Declare `mcp.transport: [stdio, http]`. Candidate multi-step intents (each chains ≥2 of the 7 endpoints):

1. **`find_best_award`** — "cheapest/best [cabin] award [origin]→[destination] in [date range] across all programs, then trip-detail the top N." Chains `/search` (with `order_by=lowest_mileage`, `sources`, `cabins`) → `/trips/{id}` for the top matches. This is the single highest-value intent; it's exactly what point.me/PointsYeah/AwardTool users ask for in one breath.
2. **`explore_from_airport`** — "where can I fly in [cabin] for under [N] miles from [origin]?" Chains `/destinations` (fan-out, cheap) → optionally `/search` per promising destination for actual dated availability. New since the prior CLI had no destinations coverage.
3. **`refresh_before_booking`** — "re-verify these N Availability IDs are still live right before I book." Chains a prior `/search`/`/availability` result set → `/refresh` on the shortlist → poll `/refresh` status until `complete:true` → surface `quota.remaining` so the agent doesn't blow the daily credit budget mid-workflow.
4. **`program_calendar_scan`** — "scan [program]'s [cabin] availability across [region]→[region] for the next N days, ranked." Chains `/availability` (bulk, paginated via `cursor`) with client-side ranking — the CLI's existing "Bulk award calendar" feature, worth promoting to an explicit intent since it's genuinely multi-page (agents should not hand-loop pagination).

Tool descriptions should explicitly state the `/live` and `/refresh` tier gating (see Auth) so an agent doesn't repeatedly retry a 4xx it can never fix with its current key.

## User Vision

> Installed seats-aero is CLI 2026.6.1 / MCP 1.0.0 from generator 3.10.0; the fleet CDR records this drift against press 4.31.7 and this reprint reconciles it. Priorities: (1) a modern MCP surface — remote transport, intents for the award-search workflow, tool design and descriptions the 3.10.0 manifest never scored; (2) apply the canonical MCP output-bounding contract via the generated internal/mcp/bound package; (3) revisit the local store: routes are synced (87k rows) into a single generic resources table and cache freshness scored 5/10; (4) the fresh module must declare go 1.26.5 or newer.

## Product Thesis

**Name:** keep `seats-aero` (slug), binaries `seats-aero-pp-cli` / `seats-aero-pp-mcp`.

**Thesis:** Seats.aero's own web app and every third-party wrapper found (gavgrego's MCP server, AwardTravelFinder, denverquane's Go lib) treat the Partner API as a thin, mostly-complete-but-stale pass-through — none combine local FTS/analytics over a fully-synced route+availability corpus with a credit-aware `/refresh` workflow and destination-discovery in one agent-native surface. The reprint's edge over "just call the API" or "use the seats.aero web app" is: (a) **offline-capable local search/analytics** over routes and synced availability (the web app requires a live session; wrappers have no local store at all); (b) **credit-budget-aware tooling** — surfacing `X-RateLimit-Remaining` and `/refresh` quota proactively so an agent doesn't silently exhaust the 1,000/day Pro allotment mid-session, which no community wrapper currently does; (c) **agent-native design** (structured envelopes, `--select`, `--agent`, `which`, `agent-context`) that the closest community analog (gavgrego's MCP server, 25 stars, actively maintained) doesn't attempt — it's a thin tool-calling shim, not a full CLI+store+MCP fleet member.

## Build Priorities

1. Patch `spec.yaml` to the current 7-endpoint surface: add `/destinations` (GET), `/live` (POST), `/refresh` (POST); rename `/search`'s `cabin`→`cabins` and add `sources`, `include_filtered`, `minify_trips`, `min_cabin_pct`; add matching params to `/availability` and `/trips/{id}`; update the `Source` enum to add `finnair`, `lufthansa`, `frontier`, `spirit` (26 total); update `Availability`/`Trip`/`Route`/`TripSegment` response schemas per the field renames/additions documented above (verify `*MileageCost` type against a live sample rather than assuming string).
2. Redesign the local store off the generic `resources` table into typed `routes`/`availability`/`trips`/`destinations` tables with the API's native `cursor` as the sync checkpoint and per-row `synced_at` for freshness scoring; wire FTS5 over the route/destination lookup fields.
3. Apply the canonical MCP output-bounding contract (`internal/mcp/bound`) to every MCP tool, given `/availability` and `/search` responses can be large (bulk pages up to 1000 rows).
4. Build the 4 MCP intents above (`find_best_award`, `explore_from_airport`, `refresh_before_booking`, `program_calendar_scan`); declare `mcp.transport: [stdio, http]`.
5. Add 2 new novel-feature commands for the 2 new table-stakes endpoints: a `destinations` command ("Nonstop reach finder") and a `refresh` command ("Award recheck / credit-aware revalidation"), each documented with their tier gating.
6. Doctor check: probe a cheap call, surface `X-RateLimit-Remaining` and key tier (Pro vs. commercial) inference (has `/refresh` worked? has `/live` worked?) rather than assuming.
7. Env var: keep `SEATS_AERO_API_KEY` primary + the existing generator-compat alias; do not adopt community-variant names (`SEATS_API_KEY`, `SEATS_AERO_PARTNER_TOKEN`).
8. Document the OAuth "Login with Seats.aero" surface (`/token`,`/consent`,`/userinfo`) as known-but-out-of-scope in the manifest/README so it isn't silently rediscovered as "missing" in a future amend.
9. go.mod: declare `go 1.26.5` or newer per User Vision item 4.

## Reachability Gate
- Decision: PASS
- Probe (2026-09-05, read-only, with the operator's Pro key): `GET https://seats.aero/partnerapi/routes?source=aeroplan` → HTTP 200, `content-type: application/json`, 1.5 MB bare JSON array; served via Cloudflare (`cf-cache-status: DYNAMIC`) with no challenge.
- Quota headers observed: `x-ratelimit-limit: 1000`, `x-ratelimit-remaining: 999`, `x-ratelimit-reset: 73587` (seconds).
- Two further read-only probes used to pin response shapes: `GET /search?origin_airport=JFK&destination_airport=LHR&cabins=business&take=1&order_by=lowest_mileage` → 200 (`*MileageCost` are JSON strings, `*Raw` are numbers); `GET /destinations?origin_airport=JFK` → 200 (`{success, origin_airport, destinations[{airport, economy, premium, business, first}]}`).
- Tier/permission hints from 4xx body: n/a (no 4xx observed). Probe-safe endpoint used: none (GET only).
