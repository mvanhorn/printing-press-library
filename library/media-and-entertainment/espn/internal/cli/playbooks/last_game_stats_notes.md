# Last-game stats query family

ESPN's player search has weak coverage for rookies and recently-signed players. Two gotchas:

- `espn-pp-cli search "<player name>"` returns empty for many active rookies (e.g., Carter Bryant). Do NOT rely on it as the resolution step.
- `https://site.api.espn.com/apps/searchapi/search` returns 403 Forbidden without a `User-Agent: Mozilla/5.0` header. Even with the header it filters out a lot of players. Avoid as a primary path.

## Resolution that works

```
espn-pp-cli compare "$ATHLETE_NAME" "<any other athlete>" --sport <s> --league <l> --agent
```

Compare accepts free-text athlete names and resolves both. The response includes `athlete1.id` and `athlete1.team.id` for the queried athlete. Pick any second athlete you know exists in the same league to satisfy the comparison.

## Postseason mode failure (Stephen Curry case)

The compare-then-teams choreography breaks when the league is in postseason mode and the athlete's team isn't currently in the playoffs.

Symptoms:
- `currentSeason.type.name === "Postseason"` (visible in any ESPN API response with the season info).
- `espn-pp-cli compare ... --agent` returns `athlete1.team = ""` for athletes whose team is eliminated or didn't make the playoffs.
- `espn-pp-cli teams <sport> <league> <team.id> --agent` returns `events: []` because the team-schedule endpoint scopes to the active seasontype.

This is what bit the Stephen Curry session (Warriors were eliminated in the play-in; postseason mode = compare/teams choreography returned nothing).

## Fallback path for postseason mode

When step 1 or step 2 above returns empty:

```
espn-pp-cli scoreboard <sport> <league> --dates <YYYYMMDD>-<YYYYMMDD> --agent
```

Use a date window covering the last 30 days of regular season plus the play-in tournament. For NBA the regular season ends mid-April; the play-in runs Apr 14-Apr 18. So a window like `20260401-20260425` covers the recent completed games.

Filter the returned events by the team's abbreviation (e.g., `GS` for Warriors), sort by date DESC, take the first completed event. The event id flows into the boxscore step.

## Summary endpoint envelope (the 2026-05-25 dogfood gotcha)

`espn-pp-cli summary <sport> <league> --event <id> --agent` wraps its payload in `{meta, results}`. The boxscore-shaped data (header, competitions, competitors, status, gameNote, boxscore, leaders) lives at `.results`, NOT at the top level.

- WRONG: `--select header.competitions.competitors.score` (looks at top-level header, which is empty)
- RIGHT: `--select results.header.competitions.competitors.score` (looks under results)
- Or via jq: `jq '.results.header.competitions[0].competitors[].score' summary.json`

Both Carter Bryant and Stephen Curry sessions in 2026-05-25 dogfood burned 2 calls each rediscovering this. Always probe `.results.<field>` first when working with summary output.

## Boxscore endpoint shape (different from summary)

`espn-pp-cli boxscore <event_id> --sport <s> --league <l> --agent` does NOT wrap in meta/results. The shape is `{teams, players}` directly at the top level. NO `.header` field; pull final score / date from `summary --event <id>` (at `.results.header.competitions[]`) when you need it.

- Athlete stats live at `.players[teamIdx].statistics[0].athletes[playerIdx]` where teamIdx is 0 or 1 depending on which side the athlete played for.
- Stat schema lives at `.players[teamIdx].statistics[0].keys[]` (machine-readable) and `.players[teamIdx].statistics[0].names[]` (human label). Athlete values are at `.athletes[i].stats[]` indexed positionally. Use `keys[]` not `labels[]` (labels carry duplicates in some sports).
- DNP signal: use `didNotPlay: true`, NOT `active: false`. The 2026-05-25 Stephen Curry session observed `active: false` with reason "COACH'S DECISION" on an athlete who played 36 minutes with full stats. `active` reflects roster status, not game participation. A real DNP has `didNotPlay: true` AND empty MIN. If `didNotPlay` is absent, check whether the `stats` array has non-empty entries before declaring DNP.
- `+/-` comes from `stats[<plusMinus_index>]` where the index is found by `keys.indexOf("plusMinus")`.

## When you need both stats AND final score

Run boxscore + summary in parallel:
- boxscore: per-player stats (.players[][].statistics[0].athletes[])
- summary: final score + date + status (.results.header.competitions[0].competitors[].score, .results.header.competitions[0].date)

Don't try to extract final score from boxscore output - it isn't there.

## Output formatting

For the user-facing answer:
- MIN (minutes played) - empty string means DNP.
- PTS, FG, 3PT, REB, AST, STL, BLK, TO, PF - read positionally per the keys[] index.
- +/- - same approach; pull from keys.indexOf("plusMinus").
