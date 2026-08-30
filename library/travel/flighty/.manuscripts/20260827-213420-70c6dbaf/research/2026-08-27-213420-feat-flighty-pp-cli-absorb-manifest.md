# Flighty CLI — Absorb Manifest (Phase 1.5d)

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Meltdown map airport list with delay status | Flighty web app homepage (SSR) | flighty-pp-cli airports list --region --status --json | Offline FTS, SQL composable, typed exit codes, --select |
| 2 | TV disrupted dashboard | Flighty web app /airports/tv | (generated endpoint) airports tv | Status filter no web UI offers (--status MAJOR_ISSUES) |
| 3 | Airport detail: status + weather + performance | Flighty web app airport page | (generated endpoint) airports show <iata> | Raw METAR + flight rules in --json for agents |
| 4 | Arrivals board | Flighty web app /arrivals | (generated endpoint) airports arrivals <iata> | JSON board with gate/belt/terminal structured |
| 5 | Departures board | Flighty web app /departures | (generated endpoint) airports departures <iata> | Original vs new time structured for diffing |
| 6 | Airport search by IATA/ICAO/name/city | Flighty web app ⌘K search + CPLX MCP flighty_search_airports | (behavior in flighty-pp-cli airports show <query>) resolves any identifier | Works offline after sync, no app install required (unlike MCP server which needs macOS app + JWT) |
| 7 | Region browsing (8 regions) | Flighty web app region tabs | (behavior in flighty-pp-cli airports list) --region flag | Composable with --status and --json |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|------------------------|------------------|
| 1 | Worst-airports ranking | airports worst --region --limit | hand-code | 9/10 | Local SQLite sorts synced catalog by cumulativeDelay/canceledPercentage/status — the web app only color-codes, never ranks | Use this command for magnitude-ranked answers ("which airports are worst right now"). Do NOT use it for browsing or filtering the airport catalog; use 'airports list' instead. |
| 2 | Flight lookup by number | airports find-flight <iata> <flight-number> | hand-code | 9/10 | Joins arrivals + departures boards filtering by flightNumber — web UI separates boards and requires eyeballing | Use this command to look up one flight by number across arrivals and departures. Do NOT use it to browse the full board; use 'airports departures' or 'airports arrivals' instead. |
| 3 | Network airline disruption | airports airline <airline-iata> | hand-code | 8/10 | Aggregates disruptedAirlines[] across all synced airport details, weighted by numOperations — site is per-airport only | Use this command for network-wide airline disruption aggregation. Do NOT use it for one airport's disrupted airlines; use 'airports show' instead. |
| 4 | Airport comparison | airports compare <iata> <iata> | hand-code | 7/10 | Joins two airport_detail records into one side-by-side diff — web shows one airport at a time | Use this command to compare two airports side by side. Do NOT use it for a single airport's full detail; use 'airports show' instead. |
| 5 | Route disruption check | airports route <origin-iata> <dest-iata> | hand-code | 7/10 | disruptedRoutes are directional (origin-only); joining both directions requires two fetches + reconciliation no surface does | Use this command for a single origin-destination pair. Do NOT use it for full side-by-side comparison; use 'airports compare', or 'airports show' for one airport's disrupted-route list. |
| 6 | Healthy-alternates finder | airports nearby <iata> --healthy-only --limit | hand-code | 7/10 | Haversine over catalog lat/lon joined to status — the map renders but never ranks "nearest healthy airport" | Use this command for distance-ranked alternates near one airport. Do NOT use it for region browsing; use 'airports list' instead. |
| 7 | Change diff since last sync | airports diff | hand-code | 6/10 | No upstream history exists (snapshot-only); deltas exist only in local SQLite snapshot history | Use this command to see what changed since the last sync. Do NOT use it for current live status; use 'airports list' instead (sync first). |

## Stub items
None. All 7 transcendence features ship fully.

## Hand-code commitment
7 transcendence rows, all `hand-code`: airports worst, airports find-flight, airports airline, airports compare, airports route, airports nearby, airports diff.

## Data layer plan
- SQLite resources: `airport` (catalog), `airport_detail` (per-airport), `flight` (board entries)
- `airports diff` requires a snapshot-history table added by the syncer (hand-written migration)
- FTS over airport name/city/iata/slug
