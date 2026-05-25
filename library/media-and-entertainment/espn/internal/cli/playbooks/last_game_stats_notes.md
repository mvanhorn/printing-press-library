# Last-game stats query family

ESPN's player search has weak coverage for rookies and recently-signed players. Two gotchas worth recording:

- `espn-pp-cli search "<player name>"` returns empty for many active rookies (e.g., Carter Bryant). Do NOT rely on it as the resolution step.
- `https://site.api.espn.com/apps/searchapi/search` returns 403 Forbidden without a `User-Agent: Mozilla/5.0` header. Even with the header it filters out a lot of players. Avoid as a primary path.

Resolution that works:
- `espn-pp-cli compare "$ATHLETE_NAME" "<any other athlete>" --sport <s> --league <l> --agent` accepts free-text athlete names and resolves both. The response includes `athlete1.id` and `athlete1.team.id` for the queried athlete. Pick any second athlete you know exists in the same league to satisfy the comparison.

Once you have `athlete.team.id`, walk the team's recent events:

```
espn-pp-cli teams <sport> <league> <team.id> --agent
```

Returns `events[]` with `competitions.status.type.completed: true` on finished games. Sort by date DESC, take the first.

Boxscore parsing:
- The athlete stats live at `.players[teamIdx].statistics[0].athletes[playerIdx]` where `teamIdx` is 0 or 1 depending on which side the athlete played for.
- Stat keys appear in `.statistics[0].keys[]` and values in `.statistics[0].athletes[i].stats[]` (same index). Use keys[], not labels[].
- A player who didn't play has `active: false` and stats may be empty strings; treat as DNP.
- Box score `header.competitions.competitors[].score` carries the final team scores. Don't read it from `events` -- those competitors may be empty before the game completes.

For the user-facing answer, format `+/-` from `stats[plusMinus_index]` (not from a separate field) and `MIN` (minutes played) from `stats[min_index]` — DNP players have empty MIN.
