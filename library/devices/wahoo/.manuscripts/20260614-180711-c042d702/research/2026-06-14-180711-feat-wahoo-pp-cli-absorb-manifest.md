# Wahoo Cloud API CLI — Absorb Manifest

Every competitor (wahoolib Python/Typer CLI, go-wahoo-cloud-api, armonge/wahoo-mcp, ClawHub skill, HA integration) is a thin API wrapper. We match the full 28-op surface AND add a local SQLite training database with offline analysis none of them have.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Get authenticated user | wahoolib, wahoo-mcp | (generated endpoint) user get | offline cache, --json/--select |
| 2 | Update user profile | wahoolib | (generated endpoint) user update | --dry-run, scriptable |
| 3 | List workouts (paginated) | wahoolib, wahoo-mcp | (generated endpoint) workouts list | local mirror, --limit |
| 4 | Get workout | wahoolib | (generated endpoint) workouts get | embedded summary parsed |
| 5 | Create workout | wahoolib | (generated endpoint) workouts create | --dry-run |
| 6 | Update workout | wahoolib | (generated endpoint) workouts update | --dry-run |
| 7 | Delete workout | wahoolib | (generated endpoint) workouts delete | typed exit codes |
| 8 | Get workout summary | wahoolib | (generated endpoint) workouts workout-summary get | parsed metrics into store |
| 9 | Upload workout FIT file | wahoolib, go-wahoo-cloud-api | (generated endpoint) workout-file-uploads create | multipart handling |
| 10 | Poll FIT upload status | wahoolib | (generated endpoint) workout-file-uploads get | token poll |
| 11 | List plans | wahoolib | (generated endpoint) plans list | offline cache |
| 12 | Get/create/update/delete plan | wahoolib | (generated endpoint) plans get/create/update/delete | --dry-run |
| 13 | List plans for workout | wahoolib | (generated endpoint) plans for-workout | offline join |
| 14 | List routes | wahoolib | (generated endpoint) routes list | offline cache |
| 15 | Get/create/update/delete route | wahoolib | (generated endpoint) routes get/create/update/delete | multipart FIT upload |
| 16 | List power zones | wahoolib | (generated endpoint) power-zones list | FTP/zones cache |
| 17 | Get/create/update/delete power zones | wahoolib | (generated endpoint) power-zones get/create/update/delete | --dry-run |
| 18 | Revoke app access | wahoolib | (generated endpoint) permissions revoke | typed exit codes |
| 19 | OAuth2 PKCE login + auto-refresh | wahoolib | (behavior in wahoo-pp-cli auth login) | browser flow, single-use refresh handling, status |
| 20 | Cross-resource offline search | (none — competitors thin) | (behavior in wahoo-pp-cli search) | FTS over workout/route/plan names offline |
| 21 | SQL over local mirror | (none) | (behavior in wahoo-pp-cli sql) | composable; no competitor offers |
| 22 | Rate-limit-aware sync | ClawHub skill | (behavior in wahoo-pp-cli sync) | fixed-window pacing, incremental, --resources/--since |
| 23 | Health/auth/cache check | (none) | (behavior in wahoo-pp-cli doctor) | auth + rate-limit + cache report |

## Transcendence (only possible with our local-SQLite + offline-math approach)
Minimum 5 — six survivors from the Phase 1.5c.5 subagent (all scored ≥7/10, all hand-code).

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Ride + FIT archive | backup --out ./dir [--since] [--full] | 9/10 | hand-code | Iterates the local workouts mirror, downloads each workout's `file.url` FIT + writes record JSON to a resumable directory tree; FIT downloads are rate-limit-exempt | Brief Top Workflow #1 + Product Thesis; go-wahoo-cloud-api stores FIT files (2 sources) | none |
| 2 | Training load (CTL/ATL/TSB) | load [--since] [--days 90] [--json] | 9/10 | hand-code | Parses string-typed summary metrics NULL-safe into per-ride training stress, computes 42d/7d EW Fitness/Fatigue and Form=CTL−ATL over local SQLite | Brief Top Workflow #2 + TrainingPeaks paywall gap (2 sources) | Use for the multi-day Fitness/Fatigue/Form trend. For all-time single-metric records use 'bests'; for a fixed recent window total use 'digest'. |
| 3 | Offline route finder | routes find [--distance A-B] [--ascent] [--near LAT,LNG --radius] [--json] | 8/10 | hand-code | Range-filters the routes mirror on stored distance/ascent/descent + haversine from starting_lat/lng — local SQLite | Brief Top Workflows #3/#4 + routes geo fields (2 sources) | Use to pick a saved route by distance/elevation/location. For free-text name matching use 'search --type route'. |
| 4 | FTP progression | ftp-history [--json] | 7/10 | hand-code | Reads FTP/critical-power from power_zones records + sync snapshots over time, emits dated progression with watts/kg when weight known | Brief Top Workflow #2 + power_zones FTP fields (2 sources) | Use for the FTP-over-time view from power zones. For training-load trend use 'load'; this does not compute Form. |
| 5 | Recent-window digest | digest [--days 7] [--json] | 8/10 | hand-code | Aggregates count/distance/time/ascent/work + load delta for last N days from local mirror; plain output pipeable | Brief Top Workflow #4 + agent-native thesis (2 sources) | Use for a one-shot rollup of any recent window (--days 365 = year in review). For the continuous load curve use 'load'; for all-time records use 'bests'. |
| 6 | Personal bests | bests [--metric power\|distance\|ascent\|duration] [--json] | 7/10 | hand-code | Sorts/maxes NULL-safe-parsed summary fields across the workouts mirror for all-time + per-period records | Brief Top Workflow #2 (PRs) + workout_summary metrics (2 sources) | Use for record summary-metric values per ride. Uses stored ride summaries, not per-second streams (so avg/total records, not mean-maximal power). For trends use 'load'. |

Dropped candidates (with reasons) recorded in the brainstorm audit: push, recap, webhook serve, gear, calendar, streaks, workouts dupes, zones. Notable hard-cut: `zones` (time-in-zone needs per-second power streams the API does not expose).

## Stubs
None. All 6 transcendence rows are full shipping scope (hand-code).

## Hand-code commitment
6 transcendence features, all `hand-code` (~50-150 LoC each + root.go wiring). 0 spec-emits novel rows. 23 absorbed rows are generator-emitted endpoints + framework behaviors.
