# Season recap query family

`espn-pp-cli leaders <sport> <league>` silently drops any --team filter; the underlying endpoint doesn't accept one. Do not waste calls trying to filter at the leaders command level.

Instead, hit `byathlete` directly:

```
https://site.api.espn.com/apis/common/v3/sports/<sport>/<league>/statistics/byathlete?limit=50&page=<n>&seasontype=2
```

- `seasontype=2` is REGULAR season. `seasontype=3` is postseason. The default (no param) is ambiguous and during playoff weeks returns sparse postseason stats only.
- The endpoint paginates. NBA: ~12 pages of 50 athletes. MLB: ~30 pages. Paginate to completion then filter client-side.
- Use https (not http). The endpoint is `site.api.espn.com`; pages can also be fetched from `site.web.api.espn.com` with the same path if the first host throws SSL/handshake errors.

## byathlete payload schema (this is the part the original notes got wrong)

Per-athlete categories DO NOT carry the stat schema. Each `athlete.categories[i]` has:
- `name` (e.g., "general", "offensive", "defensive")
- `displayName`
- `totals[]` (positional values)
- `values[]` (sometimes, same as totals)
- `ranks` (positional)
- NO `names[]` or `labels[]` per athlete.

The stat schema (which stat each totals[] index represents) lives at the TOP-LEVEL `categories[]` block (the response's root `categories`, not the athlete's). At top level:
- `categories[i].name` matches the athlete's `categories[i].name`
- `categories[i].names[j]` gives the machine-readable stat key (e.g., `"avgPoints"`, `"avgRebounds"`)
- `categories[i].labels[j]` gives the human label (DUPLICATES exist - "MIN", "REB", "PF" appear twice within `general`). Always prefer `names[]` for keying.
- Index `j` in the top-level `names[]` lines up with index `j` in the per-athlete `totals[]`.

To pull a specific stat for an athlete: find `j = top.categories[i].names.indexOf(<wanted_name>)`, then read `athlete.categories[i].totals[j]`.

## Stat index lookup pattern (use runtime indexOf - DO NOT hardcode positions)

Index positions in `top.categories[i].names[]` DRIFT across seasons. The 2026-05-25 dogfood observed `avgRebounds` at general[11] in one Pistons session while an earlier note had it at general[9]. ESPN reshuffles ordering between seasons (and occasionally mid-season as fields are added).

Always look up by name at runtime:

```
j = top.categories[i].names.indexOf("avgPoints")
value = athlete.categories[0].totals[j]
```

Common stat-name keys to look up (NBA):
- offensive: `avgPoints`, `avgFieldGoalsMade`, `avgFieldGoalsAttempted`, `fieldGoalPct`, `avgThreePointFieldGoalsMade`, `threePointFieldGoalPct`, `avgFreeThrowsMade`, `freeThrowPct`, `avgAssists`, `avgTurnovers`, `points`
- general: `gamesPlayed`, `avgMinutes`, `avgRebounds`, `avgFouls`, `doubleDouble`, `tripleDouble`
- defensive: `avgSteals`, `avgBlocks`

If a `names.indexOf(<key>)` returns -1, the category doesn't carry that stat in this season's schema — fall back to a different category (e.g., `avgRebounds` sometimes appears under general, sometimes under defensive — check both).

## Team filter

The byathlete response wraps each athlete entry one level deep:

```
.athletes[i] = {
  athlete: {
    id, displayName, teamShortName, teamId, teamName, teams[], teamUId, ...
  },
  categories: [ ... per-category stat arrays ... ]
}
```

To filter by team, access `.athletes[i].athlete.teamShortName` (e.g., `"GSW"`, `"LAL"`, `"DET"`, `"NY"`) or `.athletes[i].athlete.teamId` (numeric string, e.g., `"9"` for Warriors, `"18"` for Knicks). NOT `.athletes[i].teamShortName` — there is no top-level team field on the entry. The 2026-05-25 Knicks session burned 1 call discovering the nesting; this note captures the fix.

The athlete sub-object also carries `teams[]` (this-season team history, usually 1 entry), `teamName` (display, e.g., "Warriors"), and `teamUId`.

## Recall query normalization

The entity extractor auto-promotes ALL-CAPS tokens like `PPG`, `RPG`, `SPG` to entities (the league-abbrev rule). That changes the structural query family and misses this playbook. When firing the recall call, lowercase stat abbreviations: `ppg rpg spg`, not `PPG, RPG, SPG`.

## Schedule + standings

- `espn-pp-cli teams <sport> <league> <team_id> --season <year> --agent` returns the team's schedule under `.events[]` with `competitions.status.type.completed` flagging finished games. The `--season <year>` argument is required for past seasons; without it the endpoint returns the CURRENT season's schedule (which may be empty in the offseason).
- The team's final regular-season record lives in `standings` (run `espn-pp-cli standings <sport> <league> --agent`), NOT on the team object's `record` field. The team object's `record` is live and may be midseason.
- For NBA standings: response is grouped by conference (East/West). For MLB: grouped by league (AL/NL). Neither is pre-grouped by division. To do division-level breakdowns, see the league_top_bottom playbook for the static team-to-division maps.
