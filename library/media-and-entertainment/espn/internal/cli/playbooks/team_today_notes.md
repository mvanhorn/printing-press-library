# Team today/tonight query family

The cheapest path is the team object — it already carries `nextEvent` and a slice of recent `events[]`. One call gets you almost everything:

```
espn-pp-cli teams $SPORT $LEAGUE <team_id> --agent
```

The response has:
- `team.nextEvent`: the upcoming game (or null in offseason)
- `team.previousEvent`: the most recently completed game
- `events[]`: ~10-20 scheduled + recent events with status

If `nextEvent.date` falls in the user's window ("tonight" = today's UTC date, "today" = same, "this weekend" = Sat/Sun), you're done. The `nextEvent.competitions[]` has team + opponent + venue + status.

If the user's query is more specific ("Tuesday's game", "this weekend") or `nextEvent` falls outside the window, scan `events[]` for events whose date matches. The `events[i].date` is ISO 8601 with timezone.

Gotchas:
- The `scoreboard` command uses `--dates` (plural), not `--date`. Single-day query: `--dates 20260415`. Range: `--dates 20260415-20260425`.
- `nextEvent` may be stale during the offseason or just after a season ends — verify the date is plausible before trusting it.
- For in-progress games, `events[i].competitions[0].status.type.completed` is `false` and `status.type.detail` carries the live status string. For final games, `completed: true` and `detail: "Final"`.
- Some sports (MLS, NHL) have different status detail strings; trust the `completed` boolean over parsing `detail`.

When the user wants live scoring detail beyond just "is the game on tonight", call `boxscore <event_id>` after picking the event from the team object. This is the optional second step.
