# Absorb Manifest — sncf-connect-pp-cli

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Journey search by city names | mcp-sncf plan_journey_by_city_names | `journey search Paris Lyon --date 2026-06-01` with rich table + --json | --json, --dry-run, --direct-only, --class, --via, typed exit codes |
| 2 | Station detail (lines, nearby, stop_id) | mcp-sncf get_station_details | `station get "Bordeaux" --nearby` | Offline-first via SQLite; --json for agent consumption |
| 3 | Station schedule (departures/arrivals) | mcp-sncf get_station_schedule | `departures "Gare de Lyon" --count 20 --freshness realtime` | --freshness flag, real-time vs base schedule, --json |
| 4 | Disruption check | mcp-sncf check_disruptions | `disruptions list --coverage sncf` | --since/--until filter, --json, exit 2 on active disruption |
| 5 | Train search by city/station code | locomotive | Absorbed into `journey search` | Supports UIC/stop codes as well as city names |
| 6 | Date/class filter | locomotive | `journey search Paris Lyon --date 2026-06-01 --class first` | Standard flags |
| 7 | Station autocomplete | juliuste/sncf | `station search "Mont"` with ranked completions | FTS offline via synced SQLite |
| 8 | FPTF-format output | juliuste/sncf | `journey search --json` emits structured sections | `--json` is agent-native; typed fields |
| 9 | Direct-only and via-station filter | juliuste/sncf | `journey search Paris Lyon --direct` `--via "Lyon Part-Dieu"` | Standard flags |
| 10 | Coverage list (regions) | navitia_client | `coverage list` | --json, paginates to all regions |
| 11 | Places nearby (lat/lon) | navitia_client | `places nearby --lat 48.85 --lon 2.35 --radius 500` | --radius flag, --type filter |
| 12 | Line info for a region | navitia_client | `lines list --coverage sncf` | Synced to SQLite; offline search |
| 13 | Route schedules for a line | navitia_client | `schedule route --line LINE_ID` | --date, --from-stop, --to-stop |
| 14 | Real-time train status | SNCF.js / sncf-node | `departures "Gare du Nord" --freshness realtime` | Merged into departures with --freshness flag |
| 15 | Sync stations to local SQLite | (store layer) | `sync stations --coverage sncf` | Full FTS5 index; offline-capable |
| 16 | Sync lines to local SQLite | (store layer) | `sync lines --coverage sncf` | Full FTS5 index |
| 17 | Offline FTS search over synced stops | (search layer) | `search "Montpellier"` | Works offline; ranked by relevance |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Commute tracker | `commute check [--route NAME]` | 8/10 | Saved route in SQLite commute table; /departures filtered by direction + /disruptions overlay; exits non-zero on disruption | Marine persona. Cron-scriptable. Cross-endpoint join the API doesn't expose as one call. |
| 2 | Disruption digest | `disruptions digest [--all]` | 8/10 | Fan-out /traffic_reports over SQLite-keyed lines; severity-ranked output (blocked > delayed > info) | Théo and Marine. Multi-line aggregation not exposed by a single Navitia call. |
| 3 | Isochrone station list | `isochrone stations --from PLACE --duration MINS` | 7/10 | /isochrones GeoJSON boundary × bounding-box filter on SQLite stops table | Isabelle + Karim. Live API + SQLite cross-source join. No equivalent in any absorb tool. |
| 4 | Accessibility report | `journey accessibility --from PLACE --to PLACE` | 7/10 | /journeys + /equipment_reports per stop_point; per-leg elevator/escalator status aggregation | Isabelle (elderly parents). Two real Navitia endpoints chained. /equipment_reports unused in absorb. |
| 5 | Timetable frequency heatmap | `timetable heatmap --line LINE --stop STOP` | 7/10 | /stop_schedules full day; hourly bucketing; ASCII bar chart with peak annotation | Théo + Marine. Navitia doesn't expose aggregated frequency. Mechanical computation. |
| 6 | Board export | `board export --station STATION [--format csv\|jsonl]` | 7/10 | /departures high-count fetch; flat CSV/JSONL pipeline output | Théo ETL. Distinct output contract from human-formatted board. |
| 7 | Vehicle journey trace | `vehicle trace --line LINE --date DATE` | 6/10 | /vehicle_journeys filtered by line; ordered stop sequence with arrival/departure times | Théo schedule DB population; Karim. /vehicle_journeys entirely unused in absorb manifest. |
| 8 | Stop cluster | `stops cluster --near PLACE [--radius M]` | 6/10 | /places geocode + Haversine grouping over SQLite stops; 100m proximity buckets | Théo deduplicating station index. Pure SQLite local computation. |
