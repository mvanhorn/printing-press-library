# Catapult OpenField Connect CLI Brief

## API Identity
- Domain: Elite sport GPS athlete monitoring (Catapult OpenField Cloud, "Connect" API)
- Users: Sport scientists, S&C coaches, performance analysts (the operator runs Baylor Athletics Applied Performance)
- Data profile: athletes, activities (sessions/matches), periods, 200+ parameters (metric catalog), stats (computed metrics via POST query engine), annotations, teams, venues, sensor/10Hz data, IMA events, velocity/acceleration efforts

## Reachability Risk
- Low. Official first-party API; the operator's prior CLI ran against it daily. Regional base hosts (main stage): America → https://us.catapultsports.com. Auth required for every endpoint.

## Top Workflows
1. Morning squad ACWR check with risk zones (the #1 daily ritual)
2. Post-session stats pull: player load / distance / max velocity grouped by athlete
3. Match-to-match comparison (diff) and period-by-period intensity breakdown
4. Weekly load report exported to markdown/CSV for coach briefings
5. Return-to-play progression tracking vs squad average or pre-injury baseline

## Table Stakes (from catapultR, the only real incumbent)
- Token auth (password grant + refresh; long-lived API token works as Bearer directly)
- List athletes, activities (with date filters), periods, parameters, teams, venues, annotations, customer info, settings
- Stats query engine: POST /api/v6/stats?requested_only=TRUE with {parameters[], group_by[], filters[{name,comparison,values}]}
  - group_by: athlete, activity, period, position, team (+annotation dims with &source=annotation_stats)
  - filter names: date, period_id, activity_id, athlete_id, position_id, team_id, athlete_group_id, lastActivities, timeRange, period_name, snapshot_name, rotationNumber, tag_id, month_id, day_id
- Sensor data: {activities|periods}/{id}/athletes/{aid}/sensor (10Hz), /events?event_types=, /efforts?effort_types=velocity|acceleration, sensor/devices
- Athlete velocity bands: athletes/{id}/bands

## Data Layer
- Primary entities: athletes, activities, periods, parameters (metric catalog), stats rows, annotations
- Sync cursor: activities by start_time; stats fetched per-activity window
- FTS/search: parameter names/slugs (the 200+ metric catalog), athlete names, activity names

## Codebase Intelligence
- Source: SBGSports/catapultr R/ofCloud.R (read in full — every endpoint path extracted; API v6 confirmed)
- Auth: POST /api/v6/oauth/token (username+password+client_id+client_secret, grant_type=password), /api/v6/oauth/refresh; OR long-lived Bearer API token (CATAPULT_TOKEN) — the operator uses the latter
- Note from operator memory: Baylor tenant uses v6 /stats, NOT the older v4 /statistics — confirmed by R source (all endpoints are /api/v6/*)
- Rate limiting: not documented; be conservative
- /api/v6/modules returns HTML (quirk recorded in R source)

## User Vision
- Regenerate the lost catapult-openfield-connect-pp-cli with the same feature set: acwr, pb, benchmark, rtp, periods heatmap, diff, report, params search — offline SQLite sync, agent-native output. Use CATAPULT_TOKEN for read-only smoke tests.

## Product Thesis
- Name: catapult-openfield-connect-pp-cli
- Why: the only CLI/agent surface for OpenField data; ACWR/PB/benchmark math the API cannot do alone, computed from a local SQLite mirror — what took custom R scripts takes one command.

## Build Priorities
1. Data layer + sync (athletes, activities, periods, parameters, stats)
2. Full endpoint absorb (catapultR parity)
3. Transcendence: acwr, pb, benchmark, rtp, periods, diff, report, params search
