# InfoTBM CLI Brief

## API Identity
- Domain: Public transit (bus, tram, ferry) for Bordeaux Métropole, France
- Operator: Keolis Bordeaux Métropole Mobilités (KB2M), branded TBM
- Users: Commuters, tourists, transit planners, developers building transit apps
- Data profile: 206 lines, 5177+ stops, real-time vehicle positions, service alerts, GTFS/SIRI-Lite

## Reachability Risk
- [None] All endpoints return 200 with valid data
- Public API key is embedded in URLs (no registration needed)
- GTFS-RT protobuf feeds confirmed active (61KB vehicle data)
- SIRI-Lite JSON endpoints confirmed working (check-status, lines-discovery, stoppoints-discovery)
- Real-time stop monitoring returns empty during late evening hours (expected - service dependent)

## API Surface

### Primary: Mecatran Urbiplan REST API (JSON)
- Base URL: https://bdx.mecatran.com/utw/ws/
- Auth: apiKey=opendata-bordeaux-metropole-flux-gtfs-rt (query param)
- Endpoints:
  - GET /ws/server-info — API version info
  - GET /ws/gtfs/feed-info/bordeaux — Feed metadata
  - GET /ws/gtfs/agencies/bordeaux — Agency info
  - GET /ws/gtfs/routes/bordeaux — All routes (filterable by lat/lon, name)
  - GET /ws/gtfs/stops/bordeaux — All stops (filterable by lat/lon, name, route)
  - GET /ws/gtfs/fares/bordeaux — Fare rules
  - GET /ws/realtime/stop/bordeaux/{stopId} — Real-time departures
  - GET /ws/realtime/vehicles/bordeaux — Vehicle positions
  - GET /ws/alerts/active/bordeaux — Service alerts
  - GET /ws/layer/kml/feed/bordeaux — KML geographic data

### Secondary: SIRI-Lite REST API (JSON)
- Base URL: https://bdx.mecatran.com/utw/ws/siri/2.0/bordeaux/
- Auth: AccountKey=opendata-bordeaux-metropole-flux-gtfs-rt (query param)
- Endpoints:
  - check-status.json — Health check (no key needed)
  - lines-discovery.json — All lines with destinations
  - stoppoints-discovery.json — All stops with coordinates
  - stop-monitoring.json — Real-time arrivals at a stop
  - estimated-timetable.json — Calculated timetables for a line
  - general-message.json — Service alerts

### Tertiary: GTFS/GTFS-RT feeds
- Static GTFS: ZIP download (routes, stops, trips, stop_times, calendar, shapes)
- GTFS-RT Vehicle Positions: Protobuf binary
- GTFS-RT Trip Updates: Protobuf binary
- GTFS-RT Alerts: Protobuf binary

## Top Workflows
1. Check when the next tram/bus arrives at my stop
2. Find the nearest stops and what lines serve them
3. Check for disruptions/alerts on my commute lines
4. Track a specific vehicle/tram in real-time
5. Browse schedules for a line (all stops, all times)

## Table Stakes
- Real-time arrival predictions at any stop
- Stop search by name and by GPS coordinates
- Line/route listing with destinations
- Service alerts and disruption notifications
- Vehicle position tracking on a map (or as coordinates)

## Data Layer
- Primary entities: stops (5177+), routes/lines (206), trips, stop_times, vehicles, alerts
- Sync cursor: GTFS feed timestamp + real-time feed timestamps
- FTS/search: Stop names, route names/numbers, alert text
- Offline value: GTFS schedules work offline; real-time needs live API

## Competitors / Community
- drawbu/nextbus (Go TUI) — simple `./nextbus bus 10 Peixotto` for wait times
- JulienLavocat/InfoTBM-Client (npm, inactive since 2019) — listLines, search, stopArea, nextPass, alerts
- Almtesh/infotbm (Python, inactive since 2022) — stop search, real-time passages, VCub bikes
- Catatomik/TBMHelper (Vue.js, archived 2024) — real-time schedule viewer
- No MCP server exists for InfoTBM

## Product Thesis
- Name: infotbm-pp-cli
- Display Name: TBM Bordeaux
- Why it should exist: No maintained CLI for Bordeaux transit. Existing tools are abandonware web/mobile apps. A CLI that syncs GTFS data locally, provides instant offline schedule lookups, real-time arrival predictions, and agent-native JSON output would serve commuters, developers, and AI agents building transit-aware apps for Bordeaux.

## Build Priorities
1. Foundation: GTFS sync + SQLite store for stops, routes, trips, stop_times
2. Real-time: Next arrivals at stop, vehicle tracking, service alerts
3. Discovery: Stop search by name/location, route info, line schedules
4. Transcendence: Commute monitoring, delay history, nearby transit, offline schedule queries
