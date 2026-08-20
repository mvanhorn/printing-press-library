# SNCF Connect CLI Brief

## API Identity
- **Domain:** French national railway travel data — journey planning, timetables, real-time departures/arrivals, station info, disruptions. Powered by the Navitia open-source multimodal routing engine.
- **Users:** Travellers in France (TGV, TER, Transilien, Intercités, metro, bus), developers building transport apps, agents orchestrating travel workflows.
- **Data profile:** Journey plans (legs, transfers, sections, fares), stops and stations, line schedules, real-time departure boards, traffic disruptions, isochrones (reachability maps), vehicle positions. Mostly read-only, deep pagination via HATEOAS links.
- **Official API:** `https://api.sncf.com/v1` and `https://api.navitia.io/v1` (same engine, SNCF coverage is `sncf`)
- **Spec:** Swagger 2.0 at `https://api.navitia.io/v1/schema` — 269 endpoints, `basicAuth` (API key as username)

## Reachability Risk
- **Low.** Both probe checks on `api.navitia.io` returned 200. API has been stable since ~2014. No GitHub issues about 403 blocks reported against the official key-authenticated API. Rate limit is 5,000 req/day on free tier; 429 is possible but not a hard block.

## Top Workflows
1. **Journey search** — Find trains between two cities/stations with date, time, class, and direct-only filters. Core use case.
2. **Live departure board** — "What trains leave Paris Gare de Lyon in the next 30 minutes?" Real-time and scheduled.
3. **Station info lookup** — Find a station by city name, get its stop ID, lines served, and nearby connections.
4. **Disruption monitoring** — Check current and upcoming service disruptions on a line or around a station.
5. **Timetable by line** — Full day schedule for a line at a stop; useful for planning recurring commutes.
6. **Isochrone** — "Where can I reach from Lyon in 2 hours by train?" Reachability map query.

## Table Stakes
- Journey search by city name (what locomotive did, what mcp-sncf does)
- Station autocomplete / lookup by name
- Real-time departures from a station
- Disruption alerts for a region
- Schedule for a line at a stop
- `--json` output for agent consumption
- Offline store for synced stops/lines

## Data Layer
- **Primary entities:** `stop_areas` (stations), `lines`, `vehicle_journeys` (trips), `disruptions`
- **Sync cursor:** `since`/`until` on time-windowed endpoints; coverage-scoped pagination
- **FTS/search:** Stations and lines by name — `places` endpoint autocompletes; local FTS over synced stops for offline use
- **SNCF coverage ID:** `sncf` (covers intercity rail); `fr-idf` for Île-de-France/Transilien

## Codebase Intelligence
- **Source:** mcp-sncf (`github.com/Kryzo/mcp-sncf`) — Python MCP server wrapping Navitia
- **Auth:** `Authorization` header with Basic Auth: `base64(api_key + ":")` — or equivalently HTTP Basic with username=key, password=""
- **Env var:** `SNCF_API_KEY`
- **Endpoints confirmed working in mcp-sncf:** `/journeys`, `/places`, `/stop_areas`, `/stop_schedules`, `/departures`, `/arrivals`, `/disruptions`
- **Response fields power users need:** journeys → `sections[].from/to.name`, `sections[].type`, `sections[].display_informations.name`, `durations.total`, `co2_emissions`; departures → `route.name`, `stop_date_time.departure_date_time`, `display_informations.direction`

## Product Thesis
- **Name:** `sncf-connect-pp-cli`
- **Why it should exist:** No maintained Go CLI for French rail exists. `locomotive` and `juliuste/sncf` are both archived (2025/2026). The mcp-sncf server covers only 4 tools. The Navitia API has 269 endpoints of which most CLIs use fewer than 10. A properly built Go CLI with local SQLite store, full endpoint coverage, offline search, and `--json` output would be the definitive developer/agent tool for French rail.

## Build Priorities
1. **Journey search** — the flagship command: `journey Paris Lyon` with rich output and `--json`
2. **Departures board** — real-time: `departures "Gare de Lyon"` with `--count` and `--data-freshness realtime`
3. **Station search + detail** — `station "Bordeaux"` with lines, connections
4. **Disruptions** — `disruptions --coverage sncf` with filtering
5. **Timetables** — `schedule` command for a line at a stop
6. **Sync + offline** — `sync stations`, `sync lines`, `search "Montpellier"` offline FTS
7. **Isochrones** — `isochrone "Paris" --duration 2h` — novel, not in any existing tool
8. **SNCF Open Data punctuality** — `punctuality --line TGV` from separate free dataset endpoint — novel
