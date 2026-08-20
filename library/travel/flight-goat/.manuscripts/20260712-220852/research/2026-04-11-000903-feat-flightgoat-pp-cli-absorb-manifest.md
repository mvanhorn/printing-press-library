# flightgoat Absorb Manifest

## Source Tools Surveyed

| Tool | Type | Stars | License | Role |
|---|---|---|---|---|
| punitarani/fli | Python CLI + MCP | 1.7k | MIT | Google Flights reverse-engineered client (search + dates) |
| flightaware/aeroapps | Sample code repo | - | - | Official AeroAPI examples |
| FlightAware AeroAPI | OpenAPI 3.0.2 spec, 53 endpoints | - | Commercial | Live flight data, tracking, history, alerts |
| mikedarke/mcp-server-flight-aware-aeroapi | MCP server | - | - | AeroAPI MCP wrapper |
| smamidipaka6/flights-mcp-server | MCP server | - | - | Google Flights MCP |
| HaroldLeo/google-flights-mcp | MCP server | - | - | Google Flights MCP |
| salamentic/google-flights-mcp | MCP server | - | - | Google Flights MCP with trip planning |
| aweirddev/fast-flights | Python lib | - | MIT | Alternative Google Flights client |
| jaebradley/flights-search-cli | npm | - | - | Flight search CLI |
| danielmoraes/fly-cli | npm | - | - | Lowest-fare search CLI |
| IonicaBizau/flight-tracker | npm | - | - | Flight tracker CLI |
| giuseppecampanelli/google-flights-cli | Selenium CLI | - | - | Browser-driven GF search |
| Various Kayak scrapers (Apify, ScrapeGraph, fnneves) | Web scrapers | - | - | Kayak price/route extraction |

## Absorbed Features (match or beat everything that exists)

### Flight Search (from fli + fast-flights + Google Flights MCPs + npm CLIs)

| # | Feature | Best Source | Our Command | Added Value |
|---|---|---|---|---|
| 1 | One-way flight search | fli flights | `flightgoat search <origin> <dest> <date>` | SQLite cache, --json, --select, agent-native |
| 2 | Round-trip search | fli flights | `flightgoat search --return <date>` | Composable with other sources |
| 3 | Multi-city search | salamentic/google-flights-mcp | `flightgoat search --legs "JFK:LHR:2026-05-01,LHR:CDG:2026-05-10"` | CSV/JSON output, scriptable |
| 4 | Cheapest-date discovery | fli dates | `flightgoat dates <origin> <dest> --from <d1> --to <d2>` | Cached, queryable via SQL |
| 5 | Filter by max stops | fli | `--max-stops 0|1|2` | Used in transcend commands |
| 6 | Filter by cabin class | fli | `--cabin economy|premium|business|first` | Standard flag |
| 7 | Filter by airlines (IATA) | fli | `--airline UA,AA,DL` | Multi-value |
| 8 | Filter by time window | fli | `--depart-after 06:00 --depart-before 20:00` | Human-readable |
| 9 | Filter by max price | flights-search-cli | `--max-price 800` | USD default, --currency |
| 10 | Passenger count | fli | `--adults 2 --children 1 --infants 0` | Standard flags |
| 11 | Sort by price/duration/depart/arrive | fli | `--sort price|duration|depart|arrive` | Default price |
| 12 | JSON output | fli, all MCPs | `--json` | Valid JSON, streaming-safe |
| 13 | Human-readable table | fli | default | Colored, compact |
| 14 | Browser opener fallback | evolve2k/fly, fly-cli | `flightgoat search --open` | Opens Google Flights URL |

### Live Flight Tracking (from AeroAPI + flight-tracker)

| # | Feature | Best Source | Our Command | Added Value |
|---|---|---|---|---|
| 15 | Get flight by ident | AeroAPI /flights/{ident} | `flightgoat flight get <ident>` | Cached, --json |
| 16 | Get flight position | AeroAPI /flights/{id}/position | `flightgoat flight position <id>` | Single call, mapped |
| 17 | Get flight track (full history) | AeroAPI /flights/{id}/track | `flightgoat flight track <id>` | Compact mode shows trajectory |
| 18 | Get flight route | AeroAPI /flights/{id}/route | `flightgoat flight route <id>` | Great-circle distance included |
| 19 | Get flight map | AeroAPI /flights/{id}/map | `flightgoat flight map <id> --out <file>` | Saves image locally |
| 20 | Foresight prediction | AeroAPI /foresight/flights/{ident} | `flightgoat flight foresight <ident>` | Predictive ETA |
| 21 | Search airborne flights geospatially | AeroAPI /flights/search | `flightgoat flight search-live --box "44,-111,40,-104"` | Named --origin, --dest flags |
| 22 | Advanced live search | AeroAPI /flights/search/advanced | `flightgoat flight search-advanced "query"` | Docs inlined |
| 23 | Count flights matching query | AeroAPI /flights/search/count | `flightgoat flight count "query"` | Useful for monitoring |
| 24 | Flights between airports | AeroAPI /airports/{id}/flights/to/{dest_id} | `flightgoat route flights SEA LHR` | Shortcut for common case |
| 25 | Canonical flight lookup | AeroAPI /flights/{ident}/canonical | `flightgoat flight canonical <ident>` | Disambiguate codeshares |
| 26 | Flight intents | AeroAPI /flights/{ident}/intents | `flightgoat flight intents <ident>` | Analytics data |

### Airports (from AeroAPI + various)

| # | Feature | Best Source | Our Command | Added Value |
|---|---|---|---|---|
| 27 | List all airports | AeroAPI /airports | `flightgoat airports list` | Paginated, FTS-searchable offline |
| 28 | Get airport by code | AeroAPI /airports/{id} | `flightgoat airports get SEA` | Cached |
| 29 | Canonical airport | AeroAPI /airports/{id}/canonical | `flightgoat airports canonical KSEA` | Normalize IATA/ICAO |
| 30 | Nearby airports | AeroAPI /airports/{id}/nearby | `flightgoat airports nearby SEA --radius 50` | Radius in miles |
| 31 | Airport delays (all) | AeroAPI /airports/delays | `flightgoat delays` | Sortable by delay-minutes |
| 32 | Airport delays (one) | AeroAPI /airports/{id}/delays | `flightgoat delays SEA` | Scopes to single airport |
| 33 | Airport current flights | AeroAPI /airports/{id}/flights | `flightgoat airports flights SEA` | Paginated |
| 34 | Airport arrivals | AeroAPI /airports/{id}/flights/arrivals | `flightgoat arrivals SEA` | Top-level shortcut |
| 35 | Airport departures | AeroAPI /airports/{id}/flights/departures | `flightgoat departures SEA` | Top-level shortcut |
| 36 | Scheduled arrivals | AeroAPI /airports/{id}/flights/scheduled_arrivals | `flightgoat arrivals SEA --scheduled` | Future flights |
| 37 | Scheduled departures | AeroAPI /airports/{id}/flights/scheduled_departures | `flightgoat departures SEA --scheduled` | Future flights |
| 38 | Flights to another airport | AeroAPI /airports/{id}/flights/to/{dest_id} | `flightgoat route flights SEA HNL` | Same as #24 |
| 39 | Airport flight counts | AeroAPI /airports/{id}/flights/counts | `flightgoat airports counts SEA` | Activity metrics |
| 40 | Airport weather observations | AeroAPI /airports/{id}/weather/observations | `flightgoat weather SEA` | METAR data |
| 41 | Airport weather forecast | AeroAPI /airports/{id}/weather/forecast | `flightgoat weather SEA --forecast` | TAF data |
| 42 | Nearby airports (global) | AeroAPI /airports/nearby | `flightgoat airports nearby --lat 47.6 --lon -122.3` | Lat/lon query |
| 43 | Routes between airports | AeroAPI /airports/{id}/routes/{dest_id} | `flightgoat route info SEA LHR` | Route metadata |

### Operators / Airlines (from AeroAPI)

| # | Feature | Best Source | Our Command | Added Value |
|---|---|---|---|---|
| 44 | List operators | AeroAPI /operators | `flightgoat airlines list` | Cached, FTS-searchable |
| 45 | Get operator by code | AeroAPI /operators/{id} | `flightgoat airlines get UA` | Cached |
| 46 | Canonical operator | AeroAPI /operators/{id}/canonical | `flightgoat airlines canonical UA` | IATA/ICAO normalize |
| 47 | Operator current flights | AeroAPI /operators/{id}/flights | `flightgoat airlines flights UA` | Live |
| 48 | Operator scheduled flights | AeroAPI /operators/{id}/flights/scheduled | `flightgoat airlines flights UA --scheduled` | Future |
| 49 | Operator arrivals | AeroAPI /operators/{id}/flights/arrivals | `flightgoat airlines arrivals UA` | Live arrivals |
| 50 | Operator enroute | AeroAPI /operators/{id}/flights/enroute | `flightgoat airlines enroute UA` | Currently in air |
| 51 | Operator flight counts | AeroAPI /operators/{id}/flights/counts | `flightgoat airlines counts UA` | Fleet activity |

### Alerts (from AeroAPI)

| # | Feature | Best Source | Our Command | Added Value |
|---|---|---|---|---|
| 52 | List alerts | AeroAPI /alerts | `flightgoat alerts list` | Local cache |
| 53 | Get alert by id | AeroAPI /alerts/{id} | `flightgoat alerts get <id>` | Direct lookup |
| 54 | Create alert | AeroAPI POST /alerts | `flightgoat alerts create --ident <f> --event <e>` | --dry-run for safety |
| 55 | Delete alert | AeroAPI DELETE /alerts/{id} | `flightgoat alerts delete <id>` | Confirmation prompt |
| 56 | Get alert endpoint | AeroAPI /alerts/endpoint | `flightgoat alerts endpoint` | Webhook URL |
| 57 | Set alert endpoint | AeroAPI PUT /alerts/endpoint | `flightgoat alerts endpoint set <url>` | Idempotent |

### History (from AeroAPI)

| # | Feature | Best Source | Our Command | Added Value |
|---|---|---|---|---|
| 58 | Historical flight by ident | AeroAPI /history/flights/{ident} | `flightgoat history flight <ident>` | Paginated |
| 59 | Historical flight track | AeroAPI /history/flights/{id}/track | `flightgoat history track <id>` | Full breadcrumb |
| 60 | Historical flight map | AeroAPI /history/flights/{id}/map | `flightgoat history map <id> --out <file>` | Image saved |
| 61 | Historical flight route | AeroAPI /history/flights/{id}/route | `flightgoat history route <id>` | Planned vs actual |
| 62 | Last flight of aircraft | AeroAPI /history/aircraft/{registration}/last_flight | `flightgoat aircraft last <registration>` | By tail number |

### Aircraft + Schedules + Disruptions (from AeroAPI)

| # | Feature | Best Source | Our Command | Added Value |
|---|---|---|---|---|
| 63 | Is aircraft blocked from tracking | AeroAPI /aircraft/{ident}/blocked | `flightgoat aircraft blocked <ident>` | Boolean result |
| 64 | Aircraft owner | AeroAPI /aircraft/{ident}/owner | `flightgoat aircraft owner <ident>` | Ownership lookup |
| 65 | Aircraft type info | AeroAPI /aircraft/types/{type} | `flightgoat aircraft type B738` | Specs |
| 66 | Schedules by date range | AeroAPI /schedules/{start}/{end} | `flightgoat schedules --from 2026-05-01 --to 2026-05-07` | Future schedules |
| 67 | Disruption counts by entity | AeroAPI /disruption_counts/{entity_type} | `flightgoat disruptions <type>` | airport, operator, origin, destination |
| 68 | Disruption counts for entity | AeroAPI /disruption_counts/{entity_type}/{id} | `flightgoat disruptions <type> <id>` | Scoped |

### Infrastructure (table stakes, from every modern CLI)

| # | Feature | Our Command | Notes |
|---|---|---|---|
| 69 | Health check | `flightgoat doctor` | Verifies key, cache, connectivity |
| 70 | Full sync to local store | `flightgoat sync` | Airports, operators, seed data |
| 71 | Offline FTS search | `flightgoat search-local "<query>"` | Works without network |
| 72 | Raw SQL | `flightgoat sql "<query>"` | Read-only SQLite access |
| 73 | JSON output | `--json` on every command | Schema documented |
| 74 | Field selection | `--select <csv>` | Projection |
| 75 | CSV output | `--csv` | For spreadsheets |
| 76 | Dry-run for mutations | `--dry-run` on every POST/PUT/DELETE | Prints request only |
| 77 | Rate limiting | Built-in | Respects AeroAPI quotas |
| 78 | Typed exit codes | 0/2/3/4/5/7 | Script-friendly |
| 79 | MCP compatibility | `flightgoat mcp` | stdio MCP server |

**Total absorbed: 79 features.** Every competing tool, matched and beaten.

## Transcendence (only possible with our local data layer + multi-source fusion)

These are the NOI commands. Each requires joining data from sources nobody else joins.

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---|---|---|---|
| T1 | **Longhaul nonstop finder** (the user's original ask) | `flightgoat longhaul SEA --min-hours 8 --month 2026-05` | 10/10 | Requires scheduled_departures + route_info joined with duration filter. Kayak /direct shows this on the web but no CLI has it. fli has duration sort but no "show me all airports reachable nonstop with flights >= N hours" query. |
| T2 | **Route explore** (Kayak /direct in a terminal) | `flightgoat explore SEA` | 10/10 | Lists every nonstop destination from an airport with typical duration, operating airlines, frequency. Derived from scheduled_departures aggregation. No existing CLI has this. |
| T3 | **Cheapest longhaul dates** | `flightgoat cheapest-longhaul SEA --min-hours 8 --from 2026-05-01 --to 2026-05-31` | 9/10 | fli dates + longhaul filter. Cross-source: route catalog from AeroAPI, prices from Google Flights. Answer: the cheapest 8+ hour nonstop flight from SEA in May. |
| T4 | **On-time now** | `flightgoat ontime-now SEA` | 9/10 | All departures from SEA today, with live on-time status joined from AeroAPI positions. Single command answers "how's SEA running today." |
| T5 | **Route reliability** | `flightgoat reliability SEA LHR --days 30` | 9/10 | Historical on-time percentage for a route derived from /history/flights. Nobody surfaces this as a simple metric. |
| T6 | **Search + reliability compare** | `flightgoat compare SEA LHR 2026-06-15` | 8/10 | Joins fli price results with historical reliability per flight/airline. "Cheapest flight that's also likely to be on time." |
| T7 | **Trip monitor** | `flightgoat monitor <ident> --until-arrival` | 8/10 | Watches a flight through its lifecycle: booked -> gate -> pushback -> airborne -> landed, using AeroAPI alerts + position polling. Exits when landed. Agent-friendly. |
| T8 | **Disruption heatmap** | `flightgoat heatmap --region US --hours 6` | 7/10 | Combines /airports/delays + /disruption_counts across every major airport. Single table showing where trouble is right now. |
| T9 | **Codeshare resolver** | `flightgoat resolve <ident>` | 7/10 | Uses /flights/{ident}/canonical + operator lookup to show every code for a single physical flight. Travelers hate hunting this down. |
| T10 | **Aircraft biography** | `flightgoat aircraft bio <registration>` | 6/10 | Full history of a tail number from /history/aircraft + /aircraft/{ident}/owner. Where has this plane been and who owns it. |
| T11 | **Weather-adjusted ETA** | `flightgoat eta <ident>` | 6/10 | Combines foresight prediction with destination weather forecast. Flags when inbound weather might delay arrival. |
| T12 | **Alerts from search** | `flightgoat search --alert-if-under 600` | 6/10 | Run a search, automatically create an AeroAPI alert if target price found. Cross-source automation nobody has. |
| T13 | **Home airport digest** | `flightgoat digest SEA` | 5/10 | Daily brief: departures today, delays, weather, notable disruptions, top outbound destinations. One command, one answer. |

**Total transcend: 13 novel features.** All scored >= 5/10.

**Total CLI surface: 79 + 13 = 92 commands.** That is more than every existing tool combined.
