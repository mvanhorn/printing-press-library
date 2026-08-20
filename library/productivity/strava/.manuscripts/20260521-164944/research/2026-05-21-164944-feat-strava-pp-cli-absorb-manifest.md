# Strava CLI Absorb Manifest

## Sources Absorbed
| Source | Type | Language | Stars | Key Contribution |
|--------|------|----------|-------|-----------------|
| eddmann/strava-cli | CLI | Python | 400+ | Most complete Strava CLI; AI-friendly; training analysis commands |
| bwilczynski/strava-cli | CLI | Python | 200+ | pip-installable; clean auth flow |
| lorenzobenvenuti/strava-cli | CLI | Python | 50+ | Early Python implementation |
| dlenski/stravacli | CLI | Python | 100+ | Upload focus; GPX/FIT/TCX |
| eddmann/strava-mcp | MCP | Python | 300+ | High-level analysis tools; training analysis, comparison, context resource |
| Guutong/strava-mcp-kit | MCP | TypeScript | 50+ | Full 34-endpoint coverage; OAuth meta tools; describe-endpoint |
| kw510/strava-mcp | MCP | TypeScript | 30+ | Cloudflare Workers; remote MCP |
| liskin/strava-offline | Sync tool | Python | 1k+ | SQLite mirror with incremental sync |
| stravalib | SDK | Python | 600+ | Reference implementation; segment efforts, zones |
| strava-v3 (npm) | SDK | JS | 300+ | Node.js client with auto-refresh |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | OAuth2 login flow (browser + local callback) | eddmann/strava-cli auth login | `auth login` — local HTTP server on configurable port, opens browser, exchanges code | Token stored in config, auto-refresh, `STRAVA_ACCESS_TOKEN` override |
| 2 | Auth logout + revoke | eddmann/strava-cli auth logout | `auth logout` — revokes token at Strava + clears local config | Full revocation, not just local delete |
| 3 | Auth status + refresh | eddmann/strava-cli auth status/refresh | `auth status`, `auth refresh` | Shows token expiry, scopes, athlete ID |
| 4 | Get logged-in athlete | all sources `athlete get` | `athlete get` — profile, stats, zones in one call | `--json`, `--select`, offline-capable after sync |
| 5 | Athlete stats (YTD/all-time) | strava-mcp `get_athlete_profile` | `athlete stats` | By activity type filter |
| 6 | Athlete heart rate + power zones | eddmann `athlete zones` | `athlete zones` | Structured zone table output |
| 7 | Activities list (paginated) | all sources | `activities list` | `--type`, `--before`, `--after`, `--limit`, `--json`, cursor-based |
| 8 | Activity get (detailed) | all sources | `activities get <id>` | `--json`, `--select` |
| 9 | Activity create | eddmann + strava-mcp-kit | `activities create` | `--name`, `--type`, `--start-date`, `--elapsed-time`, `--description`, `--dry-run` |
| 10 | Activity update | eddmann + strava-mcp-kit | `activities update <id>` | `--name`, `--description`, `--gear-id`, `--private`, `--commute`, `--dry-run` |
| 11 | Activity delete | eddmann/strava-cli | `activities delete <id>` | `--dry-run` guard |
| 12 | Activity streams (HR, power, GPS, cadence) | all sources | `activities streams <id>` | `--keys` to select stream types; `--json` for raw data |
| 13 | Activity laps | eddmann + strava-mcp-kit | `activities laps <id>` | Lap breakdown with pace/HR/elevation per lap |
| 14 | Activity zones | eddmann + strava-mcp-kit | `activities zones <id>` | Distribution buckets per zone |
| 15 | Activity comments | eddmann + strava-mcp-kit | `activities comments <id>` | Paginated, `--json` |
| 16 | Activity kudos (who liked) | eddmann + strava-mcp-kit | `activities kudos <id>` | Paginated list of athletes who kudosed |
| 17 | Segment get (metadata) | all sources | `segments get <id>` | Name, grade, elevation, category, distance |
| 18 | Starred segments list | eddmann + strava-mcp | `segments starred` | `--json`, filter by activity type |
| 19 | Star / unstar segment | eddmann + strava-mcp | `segments star <id>` / `segments unstar <id>` | `--dry-run` |
| 20 | Explore segments by lat/lng | eddmann + strava-mcp | `segments explore` | `--bounds`, `--activity-type`, `--min-cat`, `--max-cat` |
| 21 | Segment efforts list (personal history) | all sources | `segment-efforts list --segment <id>` | Date range filter; `--json` |
| 22 | Segment effort get (single) | strava-mcp-kit | `segment-efforts get <id>` | Detailed with streams |
| 23 | Routes list | eddmann + strava-mcp | `routes list` | Paginated; `--json` |
| 24 | Route get | eddmann + strava-mcp | `routes get <id>` | With segment list |
| 25 | Route GPX export | eddmann + strava-mcp | `routes export <id>` | `--format gpx/tcx`, `--output <file>` |
| 26 | Route streams | strava-mcp-kit | `routes streams <id>` | time/distance/latlng/altitude/grade |
| 27 | Clubs list (athlete's clubs) | eddmann + strava-mcp | `clubs list` | `--json` |
| 28 | Club get | eddmann + strava-mcp | `clubs get <id>` | Full club detail |
| 29 | Club members | eddmann + strava-mcp | `clubs members <id>` | Paginated; `--json` |
| 30 | Club activities feed | eddmann + strava-mcp | `clubs activities <id>` | Recent club feed; `--limit` |
| 31 | Gear get (bike/shoes) | all sources | `gear get <id>` | Distance, brand/model, retired status |
| 32 | Upload FIT/GPX/TCX file | eddmann + dlenski | `upload <file>` | `--activity-type`, `--name`, poll for completion |
| 33 | Upload status | strava-mcp-kit | `upload status <id>` | Processing state + activity ID on success |
| 34 | Full SQLite sync (incremental) | liskin/strava-offline | `sync` | `--full` for full refresh, incremental by default; cursor = newest activity date |
| 35 | Doctor / health check | eddmann/strava-cli | `doctor` | Token valid, scopes, rate limit remaining, API reachable |
| 36 | FTS search | eddmann MCP `query_activities` | `search <query>` | Offline FTS5 over activities, segments, routes |
| 37 | SQL query (power users) | liskin/strava-offline `strava-offline sqlite` | `sql <query>` | Readonly SQLite queries |
| 38 | AI context export | eddmann/strava-cli context | `context` | Full athlete + recent activities + segments in agent-ready JSON |
| 39 | Training analysis | eddmann/strava-mcp `analyze_training` | `training analyze` | Date range, type filter; moving_time/distance/elevation aggregates |
| 40 | Activity comparison | eddmann/strava-mcp `compare_activities` | `activities compare <id1> <id2>` | Side-by-side pace/HR/power/elevation |
| 41 | Find similar activities | eddmann/strava-mcp `find_similar_activities` | `activities similar <id>` | Same route/segment proximity match from local store |
| 42 | OAuth URL generator | strava-mcp-kit `strava_oauth_authorize_url` | `auth url` | Generate OAuth URL without triggering browser |
| 43 | Segment leaderboard | eddmann/strava-mcp (Python MCP calls `/segments/{id}/leaderboard`) | `segments leaderboard <id>` | `--per-page`, `--age-group`, `--weight-class` |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Segment Effort Progression | `segments progress <id>` | 9/10 | hand-code | Queries local segment_efforts table for all efforts ordered by date; joins streams blobs for avg_watts and avg_hr; renders chronological table with elapsed_time, delta-from-PR, avg_power, avg_HR, rank-at-time | Jake persona; Strava web only shows PR; stravalib issues #142/#217; r/Strava "segment history" monthly posts |
| 2 | Training Load Timeline (CTL/ATL/TSB) | `training load` | 9/10 | hand-code | Reads local activities (moving_time + suffer_score or weighted_avg_watts); computes TSS per day; CTL (42-day), ATL (7-day), TSB=CTL−ATL as ASCII sparklines with fresh/fatigued/overreached label | Marcus Sunday ritual; strava-offline README requests; r/Velo and r/triathlon CTL threads |
| 3 | Zone Time Distribution | `training zones` | 8/10 | hand-code | Fetches athlete HR/power zone thresholds via /athlete/zones; decodes streams second-by-second; bins each second into matching zone; outputs per-week/month table of minutes-per-zone | Elena zone review; stravalib/strava-offline issues; r/cycling zone-time posts |
| 4 | Power Curve (best mean power) | `athlete power-curve` | 8/10 | hand-code | Iterates activities with power streams in SQLite; sliding-window max over watts JSON blob for windows 1s/5s/30s/1m/5m/20m/60m; outputs W and W/kg per window; optional date-range filter for season comparison | Marcus; most-requested stravalib feature; r/Velo; no native Strava equivalent for custom ranges |
| 5 | HR Drift Detector | `activities drift [id]` | 7/10 | hand-code | Decodes heartrate + velocity_smooth streams; splits effort at midpoint by time; computes mean HR each half; outputs decoupling % with velocity normalization; flags above --threshold (default 5%) | Aerobic decoupling metric (Joe Friel); r/artc and r/triathlon threads; no Strava tool computes this |
| 6 | Bulk Activity Updater | `activities bulk-update` | 7/10 | hand-code | Filters activities from local SQLite; previews candidate list; loops PUT /activities/{id} with --set-gear/--set-name-template/--set-description; requires activity:write scope; rate-limit guard built-in | Priya; #1 Strava community feature request 2000+ votes; r/Strava "bulk edit"; no native web equivalent |
| 7 | Gear Retirement Tracker | `gear status` | 7/10 | hand-code | Groups activities by gear_id summing distance; calls GET /gear/{id} for name/brand; reads user-configured thresholds from config or --threshold flag; outputs table with % threshold consumed and estimated replacement date from weekly average | Priya; r/running shoe mileage threads; Strava web has no threshold alerting |
| 8 | KOM Gap Tracker | `segments kom-gap` | 6/10 | hand-code | Reads starred segments; retrieves user's best effort from local segment_efforts; fetches live leaderboard top-1 via /segments/{id}/leaderboard; computes gap in seconds + %; previous snapshot delta; ranks by closable gap | Jake's Notion-table frustration; r/cycling KOM threads; unique local+live data combination |
