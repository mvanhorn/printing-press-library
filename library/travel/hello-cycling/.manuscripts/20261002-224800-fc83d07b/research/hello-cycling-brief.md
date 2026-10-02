# HELLO CYCLING CLI brief
## API identity and user vision
A read-only station planner for Japan travelers and cycling commuters: find an explicitly named station or nearby pickup and dropoff options, inspect snapshot bikes/return spaces, compare compatible pairs, and hand off to the official app. The user preauthorized focused public-source implementation, routine scope choices, browser discovery, direct building, and exactly one fresh-context MAX reviewer.
## Sources and reachability
Native Chrome anonymous station-map tab opened successfully. Full CDP reload inspection stalled; the public observed app.js asset contains the exact GET /app/top/port_json?data=data request and the rendered count/name/address mappings. Direct HTTP replay returned 200 and station 5112 matched the parent's observed 1 bike and 11 spaces. The website payload is 42.7 MB and has per-bike records; runtime will avoid this costly feed.
First-party OpenStreet article https://note.com/openstreet/n/n2f4b51cd52b3 publishes HELLO CYCLING GBFS at https://api-public.odpt.org/api/v4/gbfs/hellocycling/gbfs.json. Its live index advertises station_information, station_status, vehicle_types, and system_information; all return 200 anonymously. GBFS station 5112 also matches the map with stable name/address/coordinates and 1 bike/11 spaces. No account, location permission, API key, reservation, payment, or private endpoint is used.
GBFS is 2.3, Japanese, metadata ttl 60, status last_reported Unix seconds. Information 8.04 MB, status 4.35 MB. Counts and operational flags are snapshots; missing/unknown/stale are never zero. Vehicle type 2 currently describes a generic electric_assist bicycle, not a specific model. Compatibility is supported by vehicle_types_available and vehicle_docks_available; do not assign city/sport/e-Bike models from type 2. First-party website explains special return restrictions and electric-cycle eligibility; more model detail remains in app.
## User workflows and pain
1. Search Japanese names and addresses without navigating many map pins.
2. Rank nearby pickup/return choices from explicit coordinates, checking freshness and vehicle compatibility.
3. Compare both ends with independent distances and counts; avoid a full return station.
4. Save an explicit SQLite snapshot for offline discovery and inspect changes since it; old counts remain marked stale.
5. Inspect current source price rows including municipal exceptions and hand off to app for the selected bike price.
Pain points: empty pickups, full returns, stale counts, special vehicle compatibility, and model-dependent regional rates.
## Table stakes and ecosystem
HELLO CYCLING native map gives station counts/address and app handoff. GBFS-NOW QGIS plugin (https://github.com/hiskoh/GBFS-NOW) fetches/display GBFS information/status; a March 2026 GBFS MCP talk demonstrates nearby station lookup and basic info/status. No dedicated HELLO CYCLING CLI/SDK identified. Generic GBFS consumers are alternatives, not provider runtime authorities. Registry has no match; blocked journal returns 404. Source interfaces are fully established, so extra community endpoint discovery is unnecessary.
## Product thesis
HELLO CYCLING CLI: bounded Japanese station discovery and compatible pickup/dropoff comparison with source freshness and precise unknowns. Data layer: explicit singleton SQLite snapshot containing the joined station input feeds, observed timestamp, original per-feed timestamps; no automatic refresh of offline data. Local station-change comparison is an observation difference, not a ride prediction.
## Pricing and licensing
https://www.hellocycling.jp/price/ says rates vary by area/model even at one station; seconds round up; no initial 30-minute rate means 15-minute pricing starts immediately. Tokyo page has separate base, Chiyoda, and Itabashi tables. Rates are displayed as source rows; no quotation or model assumption. Feed license offers CC BY 4.0, selected here with OpenStreet attribution and license link retained.
## Bounds
Four feed GETs maximum for a live station snapshot (index plus information/status/types), no retry, 20-second per-request and 60-second whole-operation ceiling; 16 MiB per feed; <=50 station rows, <=100 comparison pairs, <=50 change rows; no geocoding or hidden user-location access. Pricing reads index and one advertised area page, <=1 MiB each. Source errors fail or yield explicit incomplete snapshot where information remains available.
## Build priorities
Implement every approved manifest row, deterministic parsing/status/compatibility/freshness tests, live anonymous provider checks, one independent reviewer and fixes, Press gates and atomic promotion.
