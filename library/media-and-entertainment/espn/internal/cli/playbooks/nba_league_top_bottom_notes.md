# NBA top/bottom per division query family

ESPN's `standings basketball nba --agent` endpoint returns standings grouped by CONFERENCE (Eastern / Western), NOT by division. The `.children[]` array of the league has `.standings.entries[]` flat under each conference.

Like MLB, you must group by division client-side. The NBA division-to-abbreviation map (current; stable across seasons):

**Eastern Conference:**
- Atlantic: BOS, BKN, NY, PHI, TOR
- Central: CHI, CLE, DET, IND, MIL
- Southeast: ATL, CHA, MIA, ORL, WAS

**Western Conference:**
- Northwest: DEN, MIN, OKC, POR, UTAH
- Pacific: GS, LAC, LAL, PHX, SAC
- Southwest: DAL, HOU, MEM, NO, SA

Watch for: `NO` (New Orleans Pelicans), `SA` (San Antonio Spurs), `GS` (Golden State Warriors) - shorter abbrevs than other leagues use. `UTAH` is the Jazz; some ESPN payloads use `UTA` instead. Treat both as Northwest.

## Reading the standings entries

- `entries[i].team.abbreviation` is the key for the division map above.
- `entries[i].stats[]` is keyed by name; pull win % via the entry whose `stats[j].name == "winPercent"`. Other useful stat names: `wins`, `losses`, `gamesBehind`, `playoffSeed`, `streak`, `pointsFor`, `pointsAgainst`.
- Float precision matters: two teams can show the same `.value` for winPercent in display but differ at higher precision. Sort by `wins` as the tiebreaker.

## Direction interpretation

User words map to direction:
- "best", "top" -> sort by winPercent DESC, take top N
- "worst", "bottom" -> sort by winPercent ASC, take bottom N

Top and bottom families share the same playbook structure but produce different families via queryStructural (the words "top" / "best" / "worst" / "bottom" survive the structural strip).

## Sport vocabulary note

The NBA league path on ESPN endpoints is `basketball/nba`, NOT `basketball/mens-basketball` or `basketball/nba-l`. Some ESPN sub-leagues use longer paths (`mens-college-basketball` for NCAAM) but the pro NBA is short.

## Cross-shape relationship to MLB top/bottom

The MLB playbook (league_top_bottom_mlb.json) has the same shape as this one: one standings call, group by division client-side, rank by winPercent. Only the division map differs (and the path: `baseball/mlb` vs `basketball/nba`). If a future query asks for NFL or NHL top-3-per-division, the same recipe applies with the appropriate per-league division map.
