# Catapult OpenField Connect — Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Token auth + refresh | catapultR ofCloudGetToken | catapult-openfield-connect-pp-cli auth set-token | Long-lived Bearer token (CATAPULT_TOKEN), doctor check |
| 2 | List athletes | catapultR ofCloudGetAthletes | (generated endpoint) athletes list | Offline SQLite, FTS, --json/--select |
| 3 | Metric catalog (200+ params) | catapultR ofCloudGetParameters | (generated endpoint) parameters list | Offline FTS over slugs via framework search --type parameters |
| 4 | List activities w/ date filters | catapultR ofCloudGetActivities | (generated endpoint) activities list | Sync cursor, offline |
| 5 | Activity detail + embed | catapultR | (generated endpoint) activities get | --select |
| 6 | Periods in activity | catapultR ofCloudGetPeriods | (generated endpoint) activities periods | Offline |
| 7 | Athletes in activity/period | catapultR | (generated endpoint) activities athletes / periods athletes | Offline |
| 8 | Devices | catapultR | (generated endpoint) activities devices | --json (standalone /sensor/devices query dropped: duplicate surface, requires runtime id) |
| 9 | 10Hz sensor stream | catapultR ofCloudGetSensorData | (generated endpoint) sensor data | Agent-native |
| 10 | IMA events | catapultR ofCloudGetEvents | (generated endpoint) sensor events | Typed flags |
| 11 | Velocity/accel efforts | catapultR ofCloudGetEfforts | (generated endpoint) sensor efforts | Typed flags |
| 12 | Stats query engine | catapultR ofCloudGetStatistics | (generated endpoint) stats query | Full body flags, --json, --dry-run preview |
| 13 | Athlete velocity bands | catapultR ofCloudGetAthleteBands | (generated endpoint) athletes bands | Offline |
| 14 | Annotations (4 surfaces) | catapultR | (generated endpoint) activities annotations / periods annotations / athletes annotations / annotations categories | Offline FTS |
| 15 | Teams/venues/customer/settings | catapultR | (generated endpoint) teams list / venues list / venues get / customer info / customer settings | --json |

### Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | ACWR dashboard | acwr --squad --metric total_player_load --flag-risk | 10/10 | hand-code | 7d acute / 28d chronic rolling load ratios per athlete from local SQLite stats rows; fixed risk zones (0.8-1.3 safe, >1.5 danger) | Brief workflow #1 + prior SKILL; catapultR has no ACWR | Use this command for acute:chronic workload ratios and risk zones. Do NOT use it for weekly summaries; use 'report'. Do NOT use it for a single rehabbing athlete's progression; use 'rtp'. |
| 2 | Weekly load report | report --week --squad --format markdown --export ./week_load.md | 9/10 | hand-code | Aggregates local stats for the window, reusing acwr/pb internals: total load, session count, ACWR, max vel vs PB, risk flags, monotony/strain columns; markdown/CSV export | Brief workflow #4; saves 30 min of Monday wrangling | Use this command for the multi-metric weekly export. Do NOT use it for a live morning risk check; use 'acwr'. |
| 3 | Personal bests | pb --athlete "J. Smith" --metric max_vel --vs-peak | 8/10 | hand-code | SQL window functions over full local stats history: MAX() PB, last-3-session trend arrows, current/PB readiness % | Prior SKILL (velocity-as-readiness is stated practice) | Use this command to compare an athlete against their OWN historical best. Do NOT use it to compare against other athletes; use 'benchmark'. |
| 4 | Session diff | diff 20260308 20260315 --squad --metrics total_player_load,max_vel | 8/10 | hand-code | Self-joins two local stats slices on athlete+metric; absolute and percentage deltas; args resolve by id or date | Brief workflow #3 (match-to-match comparison) | Use this command to compare TWO sessions. Do NOT use it for period-by-period breakdown within one session; use 'heatmap'. |
| 5 | Positional benchmark | benchmark --metric total_distance --position "Centre Back" --percentile | 7/10 | hand-code | Joins local stats to athlete position; percentile rank, squad median, position median, outlier flags | Prior SKILL (match-debrief peer comparison); stats engine groups but cannot rank | Use this command to rank an athlete against positional peers. Do NOT use it for self-comparison against personal history; use 'pb'. |
| 6 | Return-to-play tracker | rtp --athlete "M. Lee" --target-metric total_player_load --vs-squad --threshold 85 | 7/10 | hand-code | Per-session athlete load / concurrent squad average, or / --baseline <a>..<b> pre-injury window; threshold-crossing progression | Brief workflow #5 + prior SKILL | Use this command for one athlete's rehab progression vs squad or baseline. Do NOT use it for squad-wide risk screening; use 'acwr'. |
| 7 | Period intensity heatmap | heatmap --activity 2026-03-15 --metric meterage_per_minute --all-athletes | 7/10 | hand-code | Pivots local stats grouped by period_id into athlete x period matrix; ANSI heatmap on TTY, JSON matrix under --agent | Brief workflow #3 + prior SKILL ("10-minute R task") | Use this command for within-one-session period breakdown. Do NOT use it to compare two different sessions; use 'diff'. |

### Dropped prior features (user may override at gate)
| Prior feature | Verdict | Reason |
|---------------|---------|--------|
| params search | drop | Framework `search --type parameters` + offline FTS catalog (absorbed row #3) ships the identical capability; bespoke alias adds no leverage |
