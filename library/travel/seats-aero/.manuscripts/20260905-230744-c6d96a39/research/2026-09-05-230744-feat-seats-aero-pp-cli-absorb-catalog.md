# Seats.aero Ecosystem Absorb Catalog (raw — Phase 1.5a)

Raw findings from the 10 required searches + source reads of the top 2 public repos. Orchestrator assembles the final absorb manifest separately; this is the unfiltered catalog.

Search coverage: all 10 required queries run. Queries 1 ("Claude Code plugin"), 2 ("MCP server"), 3 ("CLI site:github.com"), 6 ("API wrapper github"), 7 (marketplace sites), 8 (language-specific wrapper) substantially overlapped on the same ~6 repos — deduped below. Query 4 (npmjs.com) and 5 (pypi.org) returned almost nothing seats.aero-specific (see notes). Query 9 (award flight search CLI) surfaced independent, non-seats.aero-API tools (scrapers) — included for competitive context, marked accordingly. Query 10 (automation/alert) returned only seats.aero's own first-party web-app alert feature, which is **not part of the Partner API** — no third-party alert-automation tool found; noted as a gap.

| Tool | URL | Lang | Feature/command | Notes |
|---|---|---|---|---|
| **seats-aero-pp-cli (prior printed CLI)** | `~/printing-press/library/seats-aero` (local) | Go | `availability` (bulk availability by source/cabin/date/region) | Prior CLI's own surface, from `--help`. |
| seats-aero-pp-cli (prior) | (local) | Go | `routes` (route list by source) | |
| seats-aero-pp-cli (prior) | (local) | Go | `seats-aero-partner-search` (cached search: origin/dest/cabin/dates/carriers/direct/order-by) | |
| seats-aero-pp-cli (prior) | (local) | Go | `trips <id>` (trip detail by availability ID) | |
| seats-aero-pp-cli (prior) | (local) | Go | `sync` / `search` / `analytics` / `export` / `import` / `workflow archive` / `workflow status` / `api` / `which` / `agent-context` / `profile` / `feedback` / `doctor` | Generic Printing-Press-generator scaffolding commands (store-backed but on a single generic `resources` table per User Vision — not seats.aero-specific). |
| **gavgrego/seats.aero-mcp-server** | https://github.com/gavgrego/seats.aero-mcp-server | TypeScript | `get_flights` — Cached Search wrapper | **(source)** Read README + `src/` listing. 25 stars, last pushed 2026-08-10. MPL-2.0. "Not affiliated with seats.aero." |
| gavgrego/seats.aero-mcp-server | same | TypeScript | `get_bulk_avail` — Bulk Availability wrapper | (source) |
| gavgrego/seats.aero-mcp-server | same | TypeScript | `get_routes` — Get Routes wrapper | (source) |
| gavgrego/seats.aero-mcp-server | same | TypeScript | `get_destinations` — Get Destinations wrapper | (source) **Endpoint our prior CLI does not cover.** |
| gavgrego/seats.aero-mcp-server | same | TypeScript | `get_trips` — Get Trips wrapper | (source) |
| gavgrego/seats.aero-mcp-server | same | TypeScript | `refresh_cached_data` — queue/poll refresh of stale Availability IDs, gated "Pro users; not commercial" | (source) **Endpoint our prior CLI does not cover.** |
| gavgrego/seats.aero-mcp-server | same | TypeScript | `live_search` — live search, gated "Commercial agreement only; not Pro" | (source) Notes 5–15s latency, advises exponential backoff, prefer cached search when possible. |
| gavgrego/seats.aero-mcp-server | same | TypeScript | Env: `SEATS_API_KEY` | (source) Differs from our CLI's `SEATS_AERO_API_KEY`. |
| gavgrego/seats.aero-mcp-server | same | TypeScript | Explicitly states it covers "all seven active Partner-Authorization endpoints"; excludes OAuth `/consent`,`/token`,`/userinfo` as "application authorization flows rather than award-availability operations" | (source) Confirms our Spec-currency finding of 7 endpoints + 3 OAuth-only endpoints. |
| **denverquane/seats-aero-go** | https://github.com/denverquane/seats-aero-go | Go | `CachedSearch(origin, dest, cabin, start, end)` client method | (source) Read README. 1 star, last pushed 2025-01-28 (stale relative to current API — predates `/destinations`,`/live` in its own checklist). |
| denverquane/seats-aero-go | same | Go | `TripSearch(id)` client method | (source) |
| denverquane/seats-aero-go | same | Go | Implementation-status checklist: Cached Search done, Get Trips done; **Bulk Availability, Get Routes, Live Search unimplemented** ("Live Search... probably won't implement" — commercial gate cited as the reason) | (source) Useful negative signal: even a dedicated Go wrapper stalled on the commercial-gated endpoint. |
| denverquane/seats-aero-go | same | Go | Env: `SEATS_AERO_API_KEY` (matches our CLI's convention) | (source) |
| @skybluu/seats-aero-mcp | https://www.npmjs.com/package/@skybluu/seats-aero-mcp | TypeScript (FastMCP) | "Curated tools for Seats.aero's Partner API" | Not read (npm page 403'd; not investigated further — WebSearch snippet only). Env var: `SEATS_AERO_PARTNER_TOKEN` — a third naming convention, noted in brief's Auth section. |
| seats-aero-mcp (LobeHub listing, author cjmcenery) | https://lobehub.com/mcp/cjmcenery-seats-aero-mcp | unknown | "search for award flight availability in real time"; env `SEATS_AERO_API_KEY` or `.env` | Marketplace listing only, not independently verified against source; likely same feature set as gavgrego's server (search/trips/routes/destinations). |
| seats-aero-mcp (Glama, author kwonye) | https://glama.ai/mcp/servers/kwonye/seats-aero-mcp | unknown | Marketplace mirror/listing | Not independently read; likely duplicate/fork of the same small MCP-server pattern. |
| Seats.aero (mcpmarket.com listing) | https://mcpmarket.com/server/seats-aero | unknown | "AI Award Flight Search API for Coding Agents" — search availability, mileage costs, seat counts, itineraries; integrates with OpenCode + Claude Desktop | Marketplace description only. |
| Seats Aero (LobeHub Skills Marketplace, chatandbuild) | https://lobehub.com/skills/chatandbuild-chatchat-skills-seats-aero | Python (per WebSearch summary: "search_availability function and seats_api.py client") | Skill-wrapped client for chat agents | Not read directly; listed as part of a larger chatandbuild skills bundle, not a standalone package. |
| seats-aero (OpenClaw skill directory) | https://openclawdir.com/skills/seats-aero-yf50t4 | unknown | OpenClaw-format skill listing | Directory listing only, surfaced via reachability-risk search; not independently opened. |
| **AwardTravelFinder/award-travel-finder-plugin** | https://github.com/AwardTravelFinder/award-travel-finder-plugin | hosted remote MCP (client-agnostic) | 127 tools total; `search_availability` (28 airlines, one at a time) | **(source, README read)** **NOT seats.aero-API-backed** — independent product, its own scraper/data pipeline across 28 airlines directly. OAuth 2.1 remote auth (API-key auth retired 2026-06-07 per its own README warning). Included for competitive-workflow ideas only, not as a seats.aero wrapper. |
| AwardTravelFinder | same | — | `search_all_airlines` — 11-airline fan-out in one call | (source) Competitive idea: multi-program fan-out in a single tool call, beyond what seats.aero's own `/search` `sources` param does across only *seats.aero-tracked* programs. |
| AwardTravelFinder | same | — | `search_monthly_availability` — whole month, day by day | (source) Competitive idea for a CLI workflow command. |
| AwardTravelFinder | same | — | `search_hybrid` — cash-and-points combinations | (source) Not replicable via seats.aero API (no cash-fare data). |
| AwardTravelFinder | same | — | `get_pricing` / `get_program_rates` — 23-program award chart data | (source) Not replicable via seats.aero API (no award-chart endpoint). |
| AwardTravelFinder | same | — | `get_points_valuation`, `find_transfer_paths`, `compare_transfer_options` | (source) Not replicable via seats.aero API; proprietary valuation/transfer data. |
| AwardTravelFinder | same | — | `get_status_matches` — elite status match opportunities | (source) Out of API scope. |
| AwardTravelFinder | same | — | `search_hotels` / `get_hotel_availability` — 4 hotel programs | (source) Out of API scope (seats.aero is flights-only). |
| AwardTravelFinder | same | — | `get_portfolio` / `list_points_balances` / `add_flight_booking` / `list_flight_bookings` — personal points/trip tracking | (source) Out of API scope; a personal-data-tracking pattern our CLI's local store could echo for *our own* synced availability/trip history, not points balances. |
| point.me | https://point.me | web app | Day-by-day search, 30+ programs, direct booking link | Independent, not seats.aero-backed. Table-stakes reference only (per WebSearch summary of comparison articles). |
| PointsYeah | https://pointsyeah.com | web app | Real-time search 4–365 days out, alerts, hotel award search | Independent. Named "best award travel search tool of 2026" by NerdWallet per a review summary — competitive pressure worth noting in Product Thesis. |
| Roame.travel | https://roame.travel | web app | Global map / visual "explore anywhere" interface | Independent. UX pattern (not API-replicable) most similar in *intent* to our proposed `/destinations`-backed "explore from airport" workflow. |
| AwardTool | (per review articles; no repo found) | web app | Live search across 32 simultaneous searches, advanced alerts | Independent. Sets the live-search UX bar if a commercial key is ever available to this CLI. |
| AwardFares | (per review articles; no repo found) | web app | Deep coverage of a narrow program set (SAS EuroBonus, United, Finnair, Avianca LifeMiles) | Independent, niche-program specialist — contrasts with seats.aero's broad-but-shallower 26-program coverage. |
| flights-points-tool (andrew-demers / demersaj) | https://github.com/andrew-demers/flights-points-tool | unknown | Uses Google Flights MCP server to convert cash fares to "total points" equivalent; searches date ranges for cheapest award-equivalent flights | Independent — **not** seats.aero-backed (uses Google Flights, not seats.aero data). Surfaced under queries 1/2 by keyword overlap only. |
| aa_flight_search_tool (tszumowski) | https://github.com/tszumowski/aa_flight_search_tool | Python (scraper) | Scrapes AA's own award search page directly | Independent, non-API, surfaced under query 9 ("award flight search CLI"). No seats.aero relation. |
| flight-seeker (alloy) | https://github.com/alloy/flight-seeker | unknown | FlyingBlue + select SkyTeam award search | Independent, non-seats.aero. Surfaced under query 9. |
| flightplan (flightplan-tool) | https://github.com/flightplan-tool/flightplan | Node.js + headless Chrome | Multi-engine award-inventory scraper, interactive CLI mode | Independent, non-seats.aero. Surfaced under query 9. |
| awardwiz (lg) | https://github.com/lg/awardwiz | unknown | Scrapes AA/Aeroplan/Alaska/JetBlue/Southwest etc. directly for award tickets | Independent, non-seats.aero. Surfaced under query 9. |

**Query 4 (`site:npmjs.com`) note:** only one genuinely relevant hit (`@skybluu/seats-aero-mcp`, listed above); the rest of the result set was unrelated seat-map/seating-chart packages (`seatsio`, `seat-picker`, etc.) — different "seats" domain entirely.

**Query 5 (`site:pypi.org`) note:** zero relevant hits. No PyPI package for seats.aero's Partner API was found; the closest Python client is bundled inside the LobeHub `chatandbuild` skills repo (not a standalone published package) — **a real gap**: no independently-installable Python wrapper exists in the wild as of this search.

**Query 10 (automation/alert) note:** no third-party automation/alert tool wraps the Partner API for alerting. Seats.aero's own web app has first-party "one-off" and "continuous" alerts (docs.seats.aero), but this is a **web-app feature, not a Partner API endpoint** — nothing to absorb from it directly; the closest our CLI can offer is a `sync` + `analytics`/`search` diff pattern run on a schedule by the *user's own* cron/agent, not a vendor-side push.

## Summary counts
- Total distinct tools/products cataloged: **~24** (7 rows are the prior printed CLI's own commands).
- Seats.aero-Partner-API-backed third-party tools: **gavgrego/seats.aero-mcp-server** (source-read, most complete — matches our 7-endpoint spec-currency finding exactly), **denverquane/seats-aero-go** (source-read, partial/stale), **@skybluu/seats-aero-mcp**, plus 2–3 marketplace-listed mirrors not independently verified.
- Independent (non-seats.aero-API) competitors cataloged for competitive-workflow context: **point.me, PointsYeah, Roame, AwardTool, AwardFares, AwardTravelFinder (127-tool hosted MCP), flights-points-tool, aa_flight_search_tool, flight-seeker, flightplan, awardwiz**.
- No PyPI package found; no third-party alert/automation wrapper found.
