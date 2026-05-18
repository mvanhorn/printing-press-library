# Novel-features brainstorm — Bandsintown CLI

Full audit trail from the Phase 1.5c.5 subagent invocation.

## Customer model

**Persona 1: Aisha — Tour Router at Double Deer (Jakarta)**

*Today (without this CLI):* Aisha keeps 15-20 browser tabs open across Bandsintown artist pages, Songkick, Spotify, and a Google Sheet titled "SEA Routing Q2-Q3". When her boss says "we want a mid-tier indie act for the August single show", she manually opens 30-40 artists' Bandsintown pages, scans for upcoming dates in Singapore/KL/Bangkok/Manila/Tokyo, and writes by hand which ones land in the ±7 day window around her target Jakarta date.

*Weekly ritual:* Every Monday morning she pulls a "this week's routing candidates" list — artists currently touring SEA who could be diverted to Jakarta on the four event slots. She updates the Google Sheet, flags candidates for the lineup-planner, and DMs initial outreach.

*Frustration:* Bandsintown's web UI has no "show me everyone who plays Singapore between June 1-15" query. She is reduced to checking artists one by one. The single most tedious thing is the manual ±N day window math against a target date in Jakarta.

**Persona 2: Reza — Lineup Curator at Double Deer**

*Today (without this CLI):* Reza is building four 2026 lineups (Festival Market, Single Show, Multi-Day, Touring, Destination). He needs to know which artists frequently co-bill — useful for "Headliner X works well with Mid-tier Y" intuition. Today he scrolls past events on Bandsintown manually, eyeballing lineup arrays.

*Weekly ritual:* Reviews 5-10 candidate headliners per week; for each, scans their last 12 months of past events to find who they've shared stages with. Writes a "frequent collaborators" note in Obsidian.

*Frustration:* Co-bill mining is invisible work — Bandsintown shows lineups one event at a time. There's no aggregation. He cannot ask "across Glastonbury, Coachella, and Fuji Rock 2025, which mid-tier artists appeared on 2+ of them?"

**Persona 3: Maya — Demand Analyst / Booking Risk Owner at Double Deer**

*Today (without this CLI):* Maya tracks ~80 artists Double Deer is considering. She wants to know who is rising — `tracker_count` going up — vs. who has plateaued. Bandsintown shows a number today but not the delta. She screenshots artist pages weekly and pastes counts into a spreadsheet.

*Weekly ritual:* Friday "demand snapshot" — open every tracked artist, copy tracker_count, log to sheet, compute week-over-week delta in Excel.

*Frustration:* No history. Bandsintown is a "now" view. She is hand-rolling a time-series out of screenshots.

**Persona 4: Pak Bayu — Daily-Refresh Agent (machine persona, 06:00 WIB cron)**

*Today (without this CLI):* The `daily-refresh` agent in Double Deer's Hermes Nous runtime needs structured JSON for ~100 tracked artists every morning, piped into Postgres. Today the agent would have to scrape or hand-roll an HTTP client per artist with retry/escape/throttle logic.

*Weekly ritual:* Runs every morning at 06:00 Asia/Jakarta. Pulls fresh events for every tracked artist, diffs against yesterday, emits a "new shows added" feed to the team Slack and Notion.

*Frustration:* Existing wrappers are not agent-shaped — they print pretty tables, not stable JSON. They have no `--select`, no idempotent sync mode, no exit-code contract.

## Survivors (transcendence features, ≥5/10)

| # | Feature | Command | Score | Persona | Why only us |
|---|---------|---------|-------|---------|-------------|
| 1 | Tour-routing feasibility | `bandsintown-pp-cli route --to <city> --on <date> --window <Nd> --tracked` | 9/10 | Aisha, Pak Bayu | Cross-entity local query (events × venues × watchlist); haversine + date-window math; no API endpoint does this |
| 2 | Calendar-gap detection | `bandsintown-pp-cli gaps <artist> --min <Nd> --max <Nd> --in <region>` | 8/10 | Reza, Aisha | SQL window function over synced events; gap analysis impossible from a single API call |
| 3 | Lineup co-bill mining | `bandsintown-pp-cli lineup co-bill <artist> --since <date> --min-shared <N>` | 9/10 | Reza | Aggregates `lineup[]` arrays across many events; ranks by shared appearances |
| 4 | Tracker trend over time | `bandsintown-pp-cli snapshot` + `bandsintown-pp-cli trend --top <N> --period <Nd>` | 9/10 | Maya, Pak Bayu | Turns Bandsintown's stateless `tracker_count` scalar into a time-series via local snapshot table; LAG() delta |
| 5 | Idempotent sync with diff | `bandsintown-pp-cli sync --tracked --since-stale <h>` | 8/10 | Pak Bayu | Re-fetches every tracked artist's events with a staleness window; emits structured diff (added/removed/changed events); agent-shaped exit codes |
| 6 | SEA routing radar | `bandsintown-pp-cli sea-radar --date <range> --tier <t>` | 8/10 | Aisha | Composed query over synced events × watchlist × tracker_count percentile, regionally scoped to SEA; Aisha's Monday-morning briefing in one command |
| 7 | Watchlist primitive | `bandsintown-pp-cli track add/list/remove` | 7/10 | Maya, Pak Bayu | First-class table not present in the API surface; drives sync, snapshot, route, sea-radar |

## Killed candidates

| Feature | Kill reason | Closest survivor |
|---------|-------------|------------------|
| lineup festivals (auto-detect) | Reza knows festival names already; he wants co-bills inside them. Wrapperish. | lineup co-bill |
| nearby | Strict subset of `route`. | route |
| route export to Notion | Notion sync belongs to `sync-orchestrator` agent reading our `--json`. | route --json |
| offers watch | Offers status is noisy; weekly-use case weak. | snapshot + trend |
| venue density | Quarterly use, not weekly. | gaps |
| mbid bridge | Should be a flag on `artists get`, not a command. | absorbed |
| feasibility-score composite | Flag on `route`, not standalone. | route --score |
| calendar .ics export | Generic, not service-specific. | dropped |
| weekly digest | Overlaps sea-radar; better composed externally. | sea-radar |
