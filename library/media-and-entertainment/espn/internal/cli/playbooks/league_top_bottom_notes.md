# League top/bottom per division query family

ESPN's `standings` endpoint returns standings at the LEAGUE level (AL + NL for MLB; East + West conferences for NBA/NFL/MLS) NOT at the division level. The `.children[]` of the league has `.standings.entries[]` with all teams flat.

This means you cannot ask the API "give me top 3 by division" — you must group client-side. The mapping is stable across seasons:

**MLB divisions (by team abbreviation):**
- AL East: BAL, BOS, NYY, TB, TOR
- AL Central: CHW, CLE, DET, KC, MIN
- AL West: HOU, LAA, OAK, SEA, TEX (Athletics may appear as OAK or ATH depending on season)
- NL East: ATL, MIA, NYM, PHI, WSH
- NL Central: CHC, CIN, MIL, PIT, STL
- NL West: ARI, COL, LAD, SD, SF

**NBA divisions (by team abbreviation):**
- Atlantic: BOS, BKN, NY, PHI, TOR
- Central: CHI, CLE, DET, IND, MIL
- Southeast: ATL, CHA, MIA, ORL, WAS
- Northwest: DEN, MIN, OKC, POR, UTAH
- Pacific: GS, LAC, LAL, PHX, SAC
- Southwest: DAL, HOU, MEM, NO, SA

**NFL divisions** group by team_abbr exactly as ESPN labels them; the standings response carries divisional groupings for NFL but not for MLB/NBA.

**MLS conferences** (Eastern / Western): ESPN's standings response uses .children[].name for "Eastern Conference" and "Western Conference". MLS doesn't have divisions inside conferences in the modern format.

Reading the standings entries:
- `entries[i].team.abbreviation` is the key for the division map above.
- `entries[i].stats[]` is keyed by name (`stats: 'wins'`, `stats: 'losses'`, `stats: 'winPercent'`). Pull win % via `entries[i].stats[j].value` where `stats[j].name == "winPercent"`. Watch for float precision.
- For a "bottom 3" query, the lowest win % wins (or loses, depending on how you frame it). Note that one division (often NL Central) can have all teams above .500, so "bottom 3" is still all winning records.

This is a single-call playbook in steady state: standings → group → rank. The actual cost in the dogfood transcripts was figuring out that ESPN doesn't pre-group by division. Now you know.

Synthetic resource_id `standings-mlb` (or `standings-nba`, etc.) is fine for the resource-learning side but pointless for cache invalidation — standings are always live-fetched. The teach call here is primarily about THIS playbook + notes pair, not about saving a fetchable id.
