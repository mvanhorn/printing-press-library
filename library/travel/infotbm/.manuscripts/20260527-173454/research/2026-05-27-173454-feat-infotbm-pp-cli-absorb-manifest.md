# InfoTBM CLI Absorb Manifest

## Sources Analyzed
1. drawbu/nextbus (Go TUI) — simple terminal wait time viewer
2. JulienLavocat/InfoTBM-Client (npm) — listLines, search, stopArea, nextPass, alerts
3. Almtesh/infotbm (Python) — stop search, real-time passages, VCub bikes
4. Catatomik/TBMHelper (Vue.js, archived) — real-time schedule viewer
5. rreau/realtime-tbm-stops (TypeScript) — real-time stop viewer

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List all lines | InfoTBM-Client listLines | infotbm-pp-cli lines list | Offline via SQLite, filterable by mode (tram/bus), --json |
| 2 | Search stops by name | InfoTBM-Client search, Almtesh search | infotbm-pp-cli stops search --query "Quinconces" | FTS5 offline search, regex, coordinates in output |
| 3 | Stop area details | InfoTBM-Client stopArea | infotbm-pp-cli stops get <stop-id> | Shows all lines serving the stop, coordinates, accessibility |
| 4 | Real-time next arrivals | InfoTBM-Client nextPass, nextbus, Almtesh | infotbm-pp-cli arrivals <stop-id> | Multi-line view, delay indicators, --json for agents |
| 5 | Service alerts | InfoTBM-Client alerts | infotbm-pp-cli alerts list | Filterable by line, offline cache, severity indicators |
| 6 | Route/line details | TBMHelper route view | infotbm-pp-cli lines get <line-id> | Full stop sequence, directions, colors, schedule info |
| 7 | Vehicle tracking | GTFS-RT vehicles feed | infotbm-pp-cli vehicles list | Real-time positions, filterable by line, --json |
| 8 | Stop search by location | Mecatran REST stops?lat&lon | infotbm-pp-cli stops nearby --lat 44.84 --lon -0.57 | Radius search, sorted by distance, shows serving lines |
| 9 | Feed/system status | SIRI check-status | (behavior in infotbm-pp-cli doctor) health check + data freshness | API reachability + GTFS freshness + store state |
| 10 | Fare information | Mecatran REST fares | (generated endpoint) fares list | Fare rules, transfer policies |
| 11 | GTFS data sync | None (no competitor has this) | infotbm-pp-cli sync --full | Full GTFS download and SQLite import for offline use |
| 12 | Offline schedule lookup | None | infotbm-pp-cli schedule <stop-id> --line A | Query stop_times from local SQLite without API calls |
| 13 | Estimated timetable | SIRI estimated-timetable | (generated endpoint) estimated-timetable get --line A | Calculated real-time timetable for a line |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Ghost service detector | schedule diff | hand-code | Joins GTFS stop_times (SQLite) with live estimated-timetable to surface trips scheduled but absent from real-time | none |
| 2 | Last viable departure | trips last-departure | hand-code | Walks GTFS stop_times backward from cutoff time across transfer chains | none |
| 3 | Delay-triggered reroute | trips reroute | hand-code | Given live delay at a transfer stop, recomputes fastest onward path using local GTFS graph | Use for reactive mid-journey rerouting when a leg is delayed. Do NOT use for proactive pre-trip planning; use trips plan instead. |
| 4 | Alert impact filter | alerts impact | hand-code | Filters disruptions to only those intersecting user-supplied line/stop IDs | none |
| 5 | Line stop sequence | lines stops | hand-code | Ordered stop list for a line/direction with optional scheduled times, offline and pipe-friendly | none |
| 6 | Timetable change diff | schedule changes | hand-code | Diffs two GTFS sync snapshots to show added/removed/shifted trips between sync timestamps | Use to detect week-over-week timetable changes. Do NOT use for live vs. scheduled comparison; use schedule diff instead. |
| 7 | Journey planner | trips plan | hand-code | Offline multi-modal journey planning (tram+bus+ferry) using local GTFS graph with live disruption overlay | Use for proactive pre-trip planning from origin. Do NOT use for mid-journey rerouting with active delays; use trips reroute instead. |
| 8 | Headway frequency report | lines frequency | hand-code | Aggregates GTFS stop_times to show average headways by hour and day-of-week | none |

