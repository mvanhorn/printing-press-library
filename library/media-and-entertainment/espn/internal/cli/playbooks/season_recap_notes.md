# Season recap query family

The `espn-pp-cli leaders <sport> <league>` command silently drops any --team filter; the underlying endpoint doesn't support it. Do not waste calls trying to filter at the leaders command level.

Instead, hit `byathlete` directly:

```
https://site.api.espn.com/apis/common/v3/sports/<sport>/<league>/statistics/byathlete?limit=50&page=<n>&seasontype=2
```

- `seasontype=2` is REGULAR season. `seasontype=3` is postseason. The default (no param) is ambiguous and may return only postseason stats during playoff weeks.
- The endpoint paginates. For NBA expect ~12 pages of 50; for MLB ~30 pages. Paginate to completion then filter client-side.
- Use https (not http) and pass `-k` to curl if you hit SSL handshake errors (some endpoints have stale chain configs).

Payload gotchas:
- Each athlete's `categories[]` array contains overlapping label names. Use `categories[i].keys[j]` (machine-readable stat key) instead of `categories[i].labels[j]` (the display label, which has duplicates like "MIN" appearing twice). The values index in `categories[i].totals[j]` lines up with `keys[]`, not `labels[]`.
- `team.abbreviation` on each athlete is what filters to a team. Resolve $TEAM via entity_lookups (espn-pp-cli teach-lookup output) to get the abbreviation.
- Postseason stats are sparse for players who missed playoffs; cross-check `GP` (games played) before quoting per-game averages.

Recall query normalization gotcha:
- The entity extractor auto-promotes ALL-CAPS tokens like `PPG`, `RPG`, `SPG` to entities (the `nfl`/`mlb`/`nba` -caps rule). That changes the query family. When firing the recall call, lowercase stat abbreviations: `ppg rpg spg`. Caps versions like `PPG, RPG, SPG` land in a DIFFERENT family and miss this playbook.

Schedule context:
- `espn-pp-cli teams <sport> <league> <team_id> --season 2026 --agent` returns the team's full schedule under `.events[]` with `competitions.status.type.completed` to identify finished games.
- The team's regular-season finish appears in `standings` not on the team object's `record`. The team object only carries current `record` (live, may be midseason).
