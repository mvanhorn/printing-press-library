# soccer-goat Absorb Manifest

## Ecosystem scan
No existing CLI joins these sources. Component tools to absorb:
- **felipeall/transfermarkt-api** (Python FastAPI wrapper) — its 13 endpoints ARE the Transfermarkt surface we absorb.
- **EA Sports FC ratings site** (drop-api.ea.com) — official rating + stats.
- **sofifa / fifacm** — potential ratings (Cloudflare-walled).
- **ESPN unofficial API** — club/fixture context (a pp-espn CLI already exists; we absorb only best-effort soccer context, not its full surface).

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search players by name | transfermarkt-api /players/search | (generated endpoint) players search | Offline cache, --json, feeds the join spine |
| 2 | Player profile (club, position, foot, age, citizenship) | transfermarkt-api /players/{id}/profile | (generated endpoint) players profile | Merged into unified report |
| 3 | Current market value | transfermarkt-api /players/{id}/market_value | (generated endpoint) players market-value | History time series in SQLite |
| 4 | Market value history | transfermarkt-api /players/{id}/market_value | (behavior in soccer-goat-pp-cli players market-value) --history | Local time series for trend math |
| 5 | Player transfers | transfermarkt-api /players/{id}/transfers | (generated endpoint) players transfers | Offline |
| 6 | Player injuries | transfermarkt-api /players/{id}/injuries | (generated endpoint) players injuries | Offline |
| 7 | Player achievements | transfermarkt-api /players/{id}/achievements | (generated endpoint) players achievements | Offline |
| 8 | Player TM stats | transfermarkt-api /players/{id}/stats | (generated endpoint) players stats | Offline |
| 9 | Search clubs | transfermarkt-api /clubs/search | (generated endpoint) clubs search | Feeds team report |
| 10 | Club profile | transfermarkt-api /clubs/{id}/profile | (generated endpoint) clubs profile | Squad value totals |
| 11 | Club squad roster | transfermarkt-api /clubs/{id}/players | (generated endpoint) clubs players | Joined with ratings for the team board |
| 12 | Competition search + clubs | transfermarkt-api /competitions/* | (generated endpoint) competitions | Offline |
| 13 | EA FC rating (overall) | drop-api.ea.com | (behavior in soccer-goat-pp-cli rating) EA client | Joined into unified report |
| 14 | EA FC attribute stats (pace/shoot/pass/dribble/defend/physical + 40 sub-attrs) | drop-api.ea.com | soccer-goat-pp-cli rating --stats | Full attribute breakdown, offline cache |
| 15 | FIFA-style potential rating | sofifa/fifacm | (behavior in soccer-goat-pp-cli rating) browser-clearance client | Best-effort; nobody else joins this to value |
| 16 | Club/league context | ESPN api | (behavior in soccer-goat-pp-cli player) ESPN best-effort | Recent fixtures/standings where resolvable |

Every absorbed row maps to a generated endpoint command (TM) or a hand-authored source client folded into the unified report.

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|-----------------|
| 1 | Unified player dossier (FLAGSHIP) | player | hand-code | Resolves a name once, fans out to TM value + EA rating/stats + potential + ESPN, merges into one report. No tool joins these silos. | The headline. `soccer-goat player schjelderup` → value, rating, potential, key stats in one report. |
| 2 | Team squad value+rating board | team | hand-code | Joins TM squad roster with per-player EA ratings + potential in one board with squad totals. | `soccer-goat team benfica` → every player's value + rating, squad totals, over/under-rated flags. |
| 3 | Market-vs-game divergence | over-under-rated | hand-code | Requires local join of TM market value + EA rating; flags players the market rates far above/below the game. | Find bargains and hype. Do NOT use for raw rating; use `rating`. |
| 4 | Potential growth gap ranking | potential-gap | hand-code | Requires potential (sofifa/fifacm) + current (EA) joined locally, ranked by headroom. | Rank a team/list by (potential - current). Best-effort; needs potential source. |
| 5 | Cross-source head-to-head | compare | hand-code | Side-by-side value + rating + potential + stats for two players from a single local join. | `soccer-goat compare mbappe haaland`. |
| 6 | Wonderkid scout query | wonderkids | hand-code | Joins TM age + rising value + sofifa potential — the classic scout filter no single source can answer. | Young + high-potential + rising value. Best-effort on potential. |

Hand-code transcendence rows: **6**. Spec-emits absorbed rows: TM endpoints (auto). Stubs: **0**.

## Notes
- ESPN + potential are **best-effort**: the unified report renders cleanly (field shown as `unavailable`/null) when a source can't be reached, never blocking the rest.
- Transfermarkt base URL is env-configurable (`SOCCER_GOAT_TM_BASE_URL`) so users can point at a self-hosted transfermarkt-api instead of the public fly.dev instance.
