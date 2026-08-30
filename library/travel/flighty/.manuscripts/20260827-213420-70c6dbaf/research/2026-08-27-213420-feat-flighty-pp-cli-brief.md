# Flighty Airports CLI Brief

## API Identity
- **Domain:** `flighty.com/airports` — Flighty's "Airport Intelligence" web app. Flighty is the Apple Design Award-winning flight tracker app; the airports web app is a free, public companion surface (Pro features like delay forecasts are app-only; the web basics are free).
- **Users:** Frequent flyers, aviation enthusiasts, travel agents, ops folks who want "is my airport melting down right now" without opening an app; AI agents that need live airport status, weather, and flight-board data.
- **Data profile:** 156 tracked airports worldwide across 8 regions; per-airport live status (NORMAL_OPERATIONS / MINOR_ISSUES / MAJOR_ISSUES), arrival/departure delay summaries (current + today), warnings, cumulative delay, weather (METAR/TAF-derived: temp, condition, ceiling, wind, visibility, flight rules, raw METAR), today's departure/arrival performance (on-time/delayed/canceled/diverted + disrupted routes/airlines), and live arrivals/departures boards with per-flight status, times, gates, belts, terminals.

## Reachability Risk
- **None.** `probe-reachability` returned `standard_http` (confidence 0.95): 200 via both stdlib and Surf. No WAF, no Cloudflare, no clearance cookie needed. Site is Vercel-hosted Next.js SSR.
- Probe-safe endpoint used: `GET /airports` (200, 226KB SSR HTML).
- Data is fully embedded in the SSR HTML as Next.js RSC flight chunks (`self.__next_f.push([1,"..."])`) — no XHR/JSON API exists. Replayable with plain HTTP + a small RSC-payload parser.

## Top Workflows
1. **"Is my airport melting down?"** — `airports show <iata>` → status, weather, delay summary, warnings, performance, disrupted routes/airlines.
2. **"What's disrupted right now?"** — `airports list --region <region>` / `airports tv` → the meltdown map catalog filtered/sorted by status.
3. **"When is my flight actually leaving?"** — `airports departures <iata>` → live departures board (flight number, airline, scheduled/original vs new time, gate, terminal, status).
4. **"What's landing?"** — `airports arrivals <iata>` → live arrivals board with belt/terminal info.
5. **"Which routes/airlines are worst today?"** — `airports show <iata>` performance section → disruptedRoutes[] / disruptedAirlines[] with delayed/canceled percentages.

## Table Stakes
- Flighty's own web app: search by IATA/name/city, region filter, TV mode (disrupted dashboard), per-airport boards.
- Community Flighty MCP server (`CPLX/flighty-mcp-server`): `flighty_search_airports` (search by IATA/ICAO/name/city), `flighty_get_airport_status` etc. — but it reads the **private app API** (`api.flightyapp.com`, protobuf, JWT from app DB + build token) requiring the installed app. Ours reads the **free public web surface**.
- Competing data CLIs (airhint, opensky, flightradar24 wrappers) don't cover Flighty's airport-intelligence UX (status + plain-English warnings + boards in one place).

## Data Layer
- **Primary entities:** `airport` (catalog: id, slug, name, iata, city, lat/lon, region), `airport_detail` (status + weather + performance), `flight` (board entries: flightNumber, airline, times, gates).
- **Sync cursor:** none upstream — each page is a full snapshot. `sync` can fetch catalog + selected airport details + boards into SQLite.
- **FTS/search:** airport names/cities/IATA/ICAO → FTS5 table for offline `search`.

## Codebase Intelligence
- Source: browser-sniff of `flighty.com/airports` (chrome-devtools MCP + raw SSR captures) — see `discovery/browser-sniff-report.md`.
- Auth: **none** (public site).
- Data model: RSC-embedded JSON (documented in the browser-sniff report §7 samples).
- Rate limiting: none observed; light pages (6 fetches, all 200).
- Architecture: Next.js App Router SSR; all data server-rendered into `self.__next_f.push` chunks; client-side search/region filtering over the embedded catalog. Slug URL scheme: `/airports/{slug}-{iata}`, sub-paths `/arrivals`, `/departures`, `/tv`.

## Product Thesis
- **Name:** `flighty` — "Flighty Airports from your terminal."
- **Why it should exist:** The web app's data (live airport status, METAR weather, performance, boards) is fully public and SSR-embeddable, but there is no CLI. Agents and terminal users can't answer "is my airport melting down" in one command today. A CLI that parses the SSR payload gives instant, offline-searchable, scriptable airport intelligence — faster than opening the map, and composable with `--json` for agents.

## Build Priorities
1. `airports list` — catalog + status + delay summaries, region filter, status filter (from homepage/tv SSR).
2. `airports show <iata>` — detail: status, weather (METAR), performance, disrupted routes/airlines.
3. `airports arrivals|departures <iata>` — live boards.
4. `airports search <query>` — offline FTS over synced catalog.
5. `sync` + SQLite store — persist catalog/details/boards for offline use and cross-command joins.
