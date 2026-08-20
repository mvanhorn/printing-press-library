# League top/bottom per division (all leagues)

One playbook covers MLB, NBA, NFL, NHL, MLS. The choreography is identical (standings -> group by division -> rank); only the per-league division map differs.

ESPN's standings endpoint groups responses differently per sport:
- **MLB**: grouped by league (American League / National League). All teams flat under each league. Division mapping must be applied client-side.
- **NBA**: grouped by conference (Eastern / Western). All teams flat under each conference. Division mapping must be applied client-side.
- **NFL**: grouped by division natively. The `.children[].children[]` is the division level. Easier — no client-side mapping needed.
- **NHL**: grouped by conference (Eastern / Western). Like NBA, divisions need client-side mapping.
- **MLS**: grouped by conference (Eastern / Western). MLS doesn't use divisions in the modern format.

## Per-league division maps (by team abbreviation)

### MLB (6 divisions)

- **AL East**: BAL, BOS, NYY, TB, TOR
- **AL Central**: CHW, CLE, DET, KC, MIN
- **AL West**: HOU, LAA, OAK, SEA, TEX (Athletics may appear as `OAK` or `ATH` depending on the season)
- **NL East**: ATL, MIA, NYM, PHI, WSH
- **NL Central**: CHC, CIN, MIL, PIT, STL
- **NL West**: ARI, COL, LAD, SD, SF

### NBA (6 divisions)

- **Atlantic**: BOS, BKN, NY, PHI, TOR
- **Central**: CHI, CLE, DET, IND, MIL
- **Southeast**: ATL, CHA, MIA, ORL, WAS
- **Northwest**: DEN, MIN, OKC, POR, UTAH (some payloads use `UTA` for Utah)
- **Pacific**: GS, LAC, LAL, PHX, SAC
- **Southwest**: DAL, HOU, MEM, NO, SA

### NFL (8 divisions; pre-grouped natively by ESPN)

NFL's standings response already nests teams under their division: `.children[]` are conferences (AFC/NFC), `.children[].children[]` are divisions (East/North/South/West per conference). No client-side mapping needed for NFL.

### NHL (4 divisions)

- **Atlantic**: BOS, BUF, DET, FLA, MTL, OTT, TB, TOR
- **Metropolitan**: CAR, CBJ, NJ, NYI, NYR, PHI, PIT, WSH
- **Central**: CHI, COL, DAL, MIN, NSH, STL, UTA, WPG
- **Pacific**: ANA, CGY, EDM, LA, SEA, SJ, VAN, VGK

### MLS (2 conferences; no divisions)

MLS uses Eastern Conference and Western Conference; no divisions inside. For an MLS "top 3 per division" query, treat the conferences as the divisions.

## Reading the standings entries

- `entries[i].team.abbreviation` keys the division map.
- `entries[i].stats[]` is keyed by name; pull win % via the entry whose `stats[j].name == "winPercent"`. Useful stat names: `wins`, `losses`, `gamesBehind`, `playoffSeed`, `streak`, `pointsFor`, `pointsAgainst`.
- Float precision: two teams may show the same displayed win % but differ in higher precision. Sort by `wins` as the tiebreaker.

## CLI gotcha: `--select` silently no-ops on nested standings paths

The `standings` command's `--select` flag silently ignores dotted paths into `children.standings.entries.*`. The 2026-05-25 NBA session tried `--select children.standings.entries.team.abbreviation,children.standings.entries.stats` and got back the full ~325KB payload unfiltered. Do NOT rely on `--select` to trim the response for division-level data — pipe the full response through `jq` or `python3` for client-side extraction instead. Top-level `--select` paths (e.g., `--select name,abbreviation`) work, but anything reaching into the nested entries does not.

## Direction interpretation

User words map to direction:
- "top", "best" -> sort by winPercent DESC, take top N
- "bottom", "worst" -> sort by winPercent ASC, take bottom N
- Default N when not specified: 3

## League routing

The query's structural family does NOT contain the league token (stopword stripped). Look at the playbook's `$LEAGUE` slot resolution — or read the query directly to determine which sport/league applies. The agent passes the right `<sport> <league>` pair to `espn-pp-cli standings`.

If neither slot nor query-text disambiguates (rare), default to MLB (most active during summer / regular season) and surface a clarifying note in the user-facing response.
