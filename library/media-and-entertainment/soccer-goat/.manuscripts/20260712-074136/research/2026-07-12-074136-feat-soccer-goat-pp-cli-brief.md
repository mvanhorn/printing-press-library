# soccer-goat CLI Brief

## API Identity
- Domain: Football/soccer player intelligence — market value, game ratings, potential, stats, all keyed by player or team name.
- Users: Scouts, FPL/Football-Manager/FC-Ultimate-Team players, football data nerds, journalists, agents. Anyone who wants "what's this player worth, how good is he, how good will he get" in one shot.
- Data profile: Player entity (identity, value, rating, potential, per-attribute stats), Club/squad entity (roster with values + ratings), market-value history time series.

## Reachability Risk
- **Low overall**, with one walled enrichment field. Verified 2026-07-12 with live probes:
  - **Transfermarkt** (market value): main site bot-walled (`unknown`/301 shells), but the open-source community wrapper `transfermarkt-api` (felipeall/transfermarkt-api, self-hostable FastAPI) serves clean JSON with a full OpenAPI at `/openapi.json`. Confirmed `marketValue 30000000` for Schjelderup (id 670103, SL Benfica). Public instance `transfermarkt-api.fly.dev`; base URL must be env-configurable for self-hosting.
  - **EA Sports FC ratings**: official JSON API `drop-api.ea.com/rating/ea-sports-fc` reachable via direct HTTP (Origin header). Returns overall rating + 40+ per-attribute stats. Confirmed Schjelderup overall 72. **No `potential` field** (null) — matches user's observation.
  - **sofifa** → `browser_clearance_http` (Cloudflare). **fifacm** → 403 "Attention Required! | Cloudflare". These are the ONLY source of **potential** rating. User chose best-effort browser-clearance capture; field degrades to null when clearance is stale.
  - **ESPN**: `site.api.espn.com` soccer scoreboards/standings reachable (Primeira Liga confirmed). Player-level season stat lines are thin — soccer player search returns []. Best-effort club/fixture context only.

## Top Workflows
1. "Type a player name, get everything" — value + current rating + potential + key stats in one report. (THE headline.)
2. "Type a team, get the squad board" — every player's value + rating, squad totals, over/under-rated flags.
3. Compare two players head-to-head across all sources.
4. Scout query: young + high-potential + rising value ("wonderkids").
5. Find market-vs-game divergence (market overrates / EA overrates).

## Table Stakes
- Player search + profile + market value (+ history) — Transfermarkt.
- Player rating + full attribute stats — EA FC.
- Club search + squad roster with values — Transfermarkt.
- Potential rating — sofifa/fifacm (best-effort).

## Data Layer
- Primary entities: `players` (merged cross-source row), `clubs`, `market_value_history`, `ea_ratings`, `potential_ratings`.
- Sync cursor: on-demand fetch per name; cache merged player rows in SQLite for offline compare/rank/scan queries.
- FTS/search: player + club name search over the local store.

## Source Priority
- Primary: **transfermarkt** (community API) — identity + value spine. Official OpenAPI available. Free.
- Secondary: **ea-fc** (drop-api.ea.com) — rating + attribute stats. Free. No potential.
- Tertiary: **sofifa-fifacm** — potential only, best-effort via browser-clearance. Free.
- Tertiary: **espn** — club/fixture context, best-effort. Free.
- **Economics:** all sources free; potential + ESPN are best-effort enrichment that never blocks the report.
- **Inversion risk:** EA drop-api and TM OpenAPI are both clean; do NOT let EA's richer stats invert TM as the identity/value headline. TM name→id→value is the join spine.

## User Vision
- "Type in any player's name OR a team and get all that stuff in one report." — the unified `player <name>` / `team <name>` report is the product, not a set of disconnected source commands.
- Concrete example given: `schjelderup benfica` → €30M value, FC rating (~76) + ~86 potential, season stats.
- Named it `soccer-goat`.

## Product Thesis
- Name: **soccer-goat**
- Why it should exist: No tool joins Transfermarkt market value + EA FC rating + FIFA-style potential + stats into one name-keyed report. Each lives in a walled silo (TM bot-walled, sofifa/fifacm Cloudflare, EA has no potential). soccer-goat is the aggregator that resolves a name once and fans out to every source, with a local SQLite store that unlocks cross-source queries (over/underrated, potential-gap, wonderkids) nobody can run today.

## Build Priorities
1. Generate base CLI from Transfermarkt OpenAPI (player/club/competition endpoints).
2. EA drop-api client (rating + stats) + ESPN client (best-effort context).
3. sofifa/fifacm browser-clearance potential client (best-effort).
4. Flagship `player <name>` and `team <name>` unified aggregator commands + SQLite merge.
5. Transcendence: compare, over/underrated, potential-gap, wonderkids.
