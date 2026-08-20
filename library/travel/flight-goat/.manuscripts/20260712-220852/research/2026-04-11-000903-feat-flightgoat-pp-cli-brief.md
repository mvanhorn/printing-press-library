# flightgoat CLI Brief

## API Identity

- **Domain:** Flight search + route exploration + live tracking (combo CLI)
- **Users:** Travelers planning trips, travel hackers hunting long-haul redemptions, operators watching live status, agents automating flight workflows
- **Data profile:** Mostly read-only; three data sources stitched into a local SQLite store so compound queries work offline

## Sources (three independent services unified)

| Source | Type | Auth | Coverage |
|---|---|---|---|
| FlightAware AeroAPI v4.17.1 | OpenAPI 3.0.2 (53 endpoints) | `x-apikey` header | Live flight status, airports, operators, routes, history, alerts, weather, disruptions |
| Google Flights via fli | Reverse-engineered (punitarani/fli, Python, MIT) | None | Search flights, date-range price discovery, filters (stops, cabin, airlines, time windows) |
| Kayak /direct nonstop matrix | Client-rendered web | None | Nonstop routes from an airport + duration + airlines (we substitute FlightAware route enumeration) |

## Reachability Risk

- **AeroAPI**: Low. Official documented API with published OpenAPI spec. Key required for actual calls but spec is public.
- **fli (Google Flights)**: Low-Medium. 1.7k stars, 158 commits, actively maintained as of 2026. Reverse-engineered APIs can break but fli has stayed current.
- **Kayak /direct**: Medium. Scraping client-rendered pages is brittle. Mitigation: derive nonstop matrix from FlightAware `/airports/{id}/flights/scheduled_departures` instead of scraping Kayak. Same answer, documented source.

## Top Workflows

1. "Find me nonstop flights from SEA that are 8+ hours this May" - combine AeroAPI scheduled_departures + route lookup + duration filter
2. "Cheapest dates JFK -> LHR in June, nonstop only" - fli dates mode with filters
3. "Is flight AA123 on time right now?" - AeroAPI /flights/{ident} with position
4. "What long-haul destinations does my home airport serve?" - AeroAPI /airports/{id}/routes enumerated
5. "Show me every disruption at SFO in the last 24 hours" - AeroAPI /disruption_counts

## Table Stakes (must match every competing tool)

- One-way, round-trip, multi-city search (fli baseline)
- Filters: max stops, cabin class, airlines, time windows (fli baseline)
- Date-range cheapest-day discovery (fli dates)
- Live flight position + track + route + map (AeroAPI)
- Airport lookup with delays + weather (AeroAPI)
- Flight alerts with callback endpoints (AeroAPI)
- Historical flight lookup (AeroAPI history)
- JSON output for agents (everyone has this, we make it better)
- MCP compatibility (multiple Google Flights MCPs exist)

## Data Layer

Primary entities to persist in SQLite:

- `airports` (id, code, name, country, lat/lon, timezone) - seeded from AeroAPI /airports
- `airlines` / `operators` (id, code, name, country) - from /operators
- `routes` (origin, destination, distance_miles, typical_duration_min, airlines) - derived from scheduled flights
- `scheduled_departures` (flight_id, ident, origin, dest, sched_out, sched_in, duration_min, operator, aircraft_type)
- `flight_searches` (query, filters_json, results_json, fetched_at) - cached fli/AeroAPI search results
- `flight_status` (ident, fa_flight_id, status, position, on_time, delay_min, last_updated) - cached live status

FTS5 on airports.name/city, flight_searches.query, operators.name. Sync cursor on scheduled_departures.fetched_at.

## Product Thesis

- **Name:** flightgoat
- **Thesis:** The first unified CLI that combines the three things flight nerds juggle today: flight search (Google Flights), route discovery (Kayak /direct style), and live tracking (FlightAware). Everything in one SQLite store so you can ask questions no single service answers, like "show me all nonstop departures from SEA that are over 8 hours long, then tell me which are on time right now."
- **Why it should exist:** Every competitor does ONE of these. fli does search beautifully. aeroapps shows live tracking. Scrapers pull Kayak's nonstop matrix. Nobody joins them. flightgoat joins them.

## Build Priorities

1. **P0 foundation:** AeroAPI client (OpenAPI-generated), SQLite store with airports/operators/routes/flights/searches, sync command, FTS5 search
2. **P1 absorb:** All 53 AeroAPI commands (flights, foresight, airports, operators, alerts, history, aircraft, schedules, disruptions), Google Flights search via fli subprocess wrapper or native Go port, agent-native `--json --select --csv --dry-run` everywhere
3. **P2 transcend:** The compound queries that make flightgoat unique:
   - `longhaul` - list nonstop departures from an airport filtered by minimum duration
   - `explore` - Kayak /direct equivalent from AeroAPI route data
   - `cheapest-longhaul` - fli dates + duration filter (cheap + long flights)
   - `ontime-now` - live on-time status for all departures from an airport today
   - `reliability` - historical on-time % for a route from AeroAPI history
   - `compare` - side-by-side fli price + AeroAPI reliability for same route
   - `trip-monitor` - watch a flight from booking through arrival (alerts + position + notifications)
