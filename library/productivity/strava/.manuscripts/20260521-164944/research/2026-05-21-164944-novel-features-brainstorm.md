# Novel Features Brainstorm: strava-pp-cli

## Customer Model

**Persona 1: Marcus, the Data-Driven Cyclist**
- Today: Exports CSVs from Strava web app manually, pastes into Excel or Google Sheets, writes VLOOKUP formulas to find his best segment times by season
- Weekly ritual: Every Sunday, reviews his week's rides — TSS, IF, CTL trend — by cross-referencing Garmin Connect and Strava web UI in two browser tabs
- Frustration: Can't query "show me all rides where I was in zone 4+ for more than 60 minutes" without exporting data and filtering manually

**Persona 2: Elena, the Triathlete Coach**
- Today: Manually scrolls through each athlete's Strava feed, screenshots power/HR charts, pastes into a Google Doc training review
- Weekly ritual: Monday morning debrief prep — compares each athlete's weekend long run HR drift against their threshold zones
- Frustration: No way to compare her own multi-discipline season splits (swim/bike/run volume by week) in one view without building a spreadsheet from scratch

**Persona 3: Jake, the Segment Hunter**
- Today: Refreshes Strava segment leaderboards manually, keeps a Notion table of his top-10 PRs and the gap to the KOM
- Weekly ritual: Before a training ride, checks which key segments he hasn't attempted in >30 days to target them fresh
- Frustration: Can't see his own progression on a segment over time — Strava only shows his PR, not the trend of his last 10 efforts

**Persona 4: Priya, the Gear Manager / Running Club Admin**
- Today: Manually updates activity descriptions with gear notes after races, one by one via the Strava web editor
- Weekly ritual: Logs race results for club members by visiting each activity page
- Frustration: After a shoe model retires, has to click into every run to change gear assignment — no bulk operation exists

## Candidates (pre-cut) — omitted for brevity; see Survivors and Kills

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Segment Effort Progression | `segments progress <id>` | 9/10 | hand-code | Queries local segment_efforts table for all efforts on a segment ordered by start_date; joins streams blobs to pull avg_watts and avg_hr; renders chronological table with elapsed_time, delta-from-PR, avg_power, avg_HR | Jake persona; Strava web only shows PR; stravalib issues #142/#217; r/Strava "segment history" monthly posts |
| 2 | Training Load Timeline (CTL/ATL/TSB) | `training load` | 9/10 | hand-code | Reads local activities (moving_time + suffer_score or weighted_avg_watts); computes TSS per day, CTL (42-day exp decay), ATL (7-day exp decay), TSB=CTL−ATL as ASCII sparklines | Marcus Sunday ritual; strava-offline README requests; r/Velo and r/triathlon weekly threads |
| 3 | Zone Time Distribution | `training zones` | 8/10 | hand-code | Fetches athlete HR/power zone thresholds via /athlete/zones; decodes streams heartrate/watts JSON second-by-second; bins each second into matching zone; outputs per-week/month table of minutes-in-zone | Elena zone review; stravalib/strava-offline issues; r/cycling posts |
| 4 | Power Curve | `athlete power-curve` | 8/10 | hand-code | Iterates activities with power streams; for each window (1s, 5s, 30s, 1m, 5m, 20m, 60m), runs sliding window max over watts array; outputs best mean power per window, normalized to W/kg if weight supplied | Marcus; stravalib most-requested feature; r/Velo; no native Strava equivalent for custom date ranges |
| 5 | HR Drift Detector | `activities drift` | 7/10 | hand-code | Decodes heartrate stream; splits effort into two halves by time; computes mean HR each half; outputs decoupling % using velocity_smooth for pace normalization; flags above --threshold (default 5%) | Joe Friel aerobic decoupling; r/artc and r/triathlon threads; no Strava tool computes it |
| 6 | Bulk Activity Updater | `activities bulk-update` | 7/10 | hand-code | Filters activities from local SQLite by date range/sport/name-regex/gear; previews changes; loops PUT /activities/{id} with --set-gear, --set-name-template, --set-description; rate-limit guard | Priya; #1 most-upvoted Strava community feature request (2000+ votes); r/Strava; no native bulk edit |
| 7 | Gear Retirement Tracker | `gear status` | 7/10 | hand-code | Groups activities by gear_id summing distance/moving_time; calls GET /gear/{id} per unique gear; reads user thresholds from config; outputs table with distance, hours, % threshold, estimated replacement date | Priya; r/running shoe mileage threads; Strava web shows distance but no threshold alerting |
| 8 | KOM Gap Tracker | `segments kom-gap` | 6/10 | hand-code | Gets starred segments; retrieves user's best effort from local segment_efforts; fetches live leaderboard top-1 via /segments/{id}/leaderboard?per_page=1; computes gap in seconds + %; outputs ranked table by closable gap | Jake's Notion-tracking frustration; r/cycling KOM hunting; unique combination of local+live data |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| Activity name bulk-rename with template | Strict subset of Bulk Activity Updater; absorbed as --set-name-template flag | Bulk Activity Updater |
| Personal leaderboard rank history | Requires snapshot polling infrastructure; too large for single feature | KOM Gap Tracker |
| Multi-sport weekly volume summary | Single SQLite GROUP BY; achievable via sql command in absorbed table | Training Load Timeline |
| Route difficulty scorer | Made-up composite formula; no industry-standard backing | HR Drift Detector |
| Segment effort heatmap (day/hour grid) | Curiosity output; low actionable value; low weekly use | Training Load Timeline |
| Club activity leaderboard | API returns limited data on free tier; unverifiable in dogfood | None — cut |
| Fitness trend projection | Linear extrapolation gimmick; absorbed by CTL sparkline output | Training Load Timeline |
| Segment effort anomaly detector | Requires 10+ efforts per segment; garbage output for 90% of segments | Segment Effort Progression |
