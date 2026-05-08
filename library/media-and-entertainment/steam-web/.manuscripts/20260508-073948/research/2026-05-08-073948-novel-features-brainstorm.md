# Steam Web Novel Features — Brainstorm Audit Trail

> Output of the novel-features subagent. Persisted for retro / dogfood debugging.
> Reprint mode: prior research.json absent (degraded reprint), so Pass 2(d)
> reconciliation did not fire and `## Reprint verdicts` is omitted.

## Customer model (Pass 1)

1. **Achievement hunter** — completionist; wants the easiest still-locked achievement across the whole library, plus rarest unlocks for flexing.
2. **Library auditor / backlog warrior** — owns hundreds of titles; wants sunk-cost, never-launched, and genre-overspend views in one query.
3. **Friend-graph competitor** — "who in my friends owns this and beats me"; needs throttled fan-out across the friend list.
4. **Game-dev / publisher analyst** — tracks concurrent players over time and review velocity for a shipped title.
5. **Profile flexer + agent driver** — single-pane status display for humans; one MCP tool call for agents asking "find friends with <5h in $game".

## Candidates (pre-cut, Pass 2)

30 candidates generated; folded/cut as noted:

| C# | Candidate | Disposition |
|---|---|---|
| C1 | library audit | survives → folds C10 (library never-launched), C11 (library bounce), C12 (genre-overspend), C27 (most-played-genre) |
| C2 | friends compare <appid> | survives → folds C9 (friends owns), C16 (friends idle) via filters |
| C3 | next-achievement | survives |
| C4 | play-trend <appid> | survives, low-confidence flag |
| C5 | profile fetch | cut — duplicate of absorbed feature A13 (`steamfetch`-style summary) |
| C6 | rare-achievements | survives |
| C7 | review-velocity <appid> | survives |
| C8 | price-drop | cut — needs persistent price-history sampling not in data layer; without it falls back to current-price mirror already covered by store appdetails (A1) |
| C13 | news search <query> | survives |
| C14 | achievement-hunt <appid> | survives |
| C15 | friends overlap <steamid> | reframed → merged into C20 as `library compare` |
| C17 | currently-playing | survives |
| C18 | wishlist diff | cut — brief explicitly defers wishlist (privacy-gated, brittle endpoint) |
| C19 | recently-played | cut — already an endpoint mirror |
| C20 | library compare <steamid> <steamid> | survives (absorbs C15) |
| C21 | app trending | cut on score (5/10) and sample-density: useful only after weeks of broad sampling; output is sparse and silent on a fresh DB, dogfood-verifiability concern |
| C22 | review sentiment | cut — LLM dependency kill-check |
| C23 | achievement leaderboard <appid> | survives |
| C24 | profile heatmap | cut — scope creep (needs persistent 2-week sampling) |
| C25 | library deals | cut on score (3/10): you don't usually buy what you own; the high-value deals query is for non-owned wishlist which was deferred |
| C26 | news digest <appid> | cut — redundant with `news search` filtered by appid |
| C28 | dormant-friends | cut on score (3/10) — no community evidence anyone wants this |
| C29 | cs-match-history | cut — already endpoint-mirrored |
| C30 | app fingerprint <appid> | cut on redundancy with cobratree walker — agents can compose the four reads via per-endpoint typed tools; convenience for humans is marginal |

## Survivors and kills (Pass 3)

### Survivors

11 features, all >= 6/10. Final transcendence table:

| # | Feature | Command | Score | Persona | Buildability proof |
|---|---------|---------|-------|---------|--------------------|
| 1 | Library backlog audit | `library audit [--never-launched\|--bounce\|--genre-spend]` | 10/10 | Library auditor | Reads local SQLite (`owned_games` joined with `apps.genres` and `apps.is_free`) to surface never-launched titles, paid-titles-with-<2h-playtime sunk cost, and genre-by-playtime distribution — no API calls at query time. |
| 2 | Friend playtime comparison on one app | `friends compare <appid> [--filter owns-zero-hours\|--filter owners]` | 10/10 | Friend-graph competitor | Fans out `GetFriendList` then per-friend `GetOwnedGames` through `cliutil.AdaptiveLimiter` (post-2025-06 25 req/s budget), filters to `appid`, ranks by `playtime_forever`, persists to `owned_games` so subsequent runs hit the local store. |
| 3 | Next easiest achievement to unlock (cross-library) | `next-achievement [--app <appid>] [--limit N]` | 10/10 | Achievement hunter | Reads local SQLite `player_achievements LEFT JOIN achievements_schema WHERE achieved=0` sorted by `global_pct DESC`, optionally scoped to one `appid`. The `global_pct` column is populated at sync time by merging `GetSchemaForGame` with `GetGlobalAchievementPercentagesForApp`. |
| 4 | Per-app achievement workbench | `achievement-hunt <appid>` | 9/10 | Achievement hunter | Joins `player_achievements`, `achievements_schema`, and `achievements_schema.global_pct` for one `appid` to render schema + unlock state + global rarity in one table; supports `--locked` / `--rare` filters. |
| 5 | Rarest unlocked achievements (flex view) | `rare-achievements [--steamid <id>] [--limit N]` | 8/10 | Achievement hunter / Profile flexer | Reads `player_achievements JOIN achievements_schema WHERE achieved=1 ORDER BY global_pct ASC` — inverse query of `next-achievement`, surfaces the user's rarest unlocks. |
| 6 | Library diff between two SteamIDs | `library compare <steamid> [--mine-only\|--theirs-only\|--shared]` | 8/10 | Friend-graph competitor | Set operations on `owned_games` rows for two SteamIDs (mine-only, theirs-only, shared) plus playtime delta on the shared set. |
| 7 | Currently-playing friends | `currently-playing` | 8/10 | Profile flexer + agent driver | Single batched `GetPlayerSummaries` call across the friend list, filters to records with non-null `gameextrainfo` / `gameid`, joins to local `apps` for the title — one API call, no fanout. |
| 8 | Achievement leaderboard among friends | `achievement-leaderboard <appid>` | 8/10 | Achievement hunter + Friend-graph competitor | Fans out `GetFriendList` + per-friend `GetPlayerAchievements(<appid>)` through `AdaptiveLimiter`, persists to `player_achievements`, ranks by % achieved for the given app. |
| 9 | Review velocity over time for one app | `review-velocity <appid> [--window 7d\|30d]` | 7/10 | Game-dev/publisher analyst | Date-bucket aggregation on `app_reviews.timestamp_created` (populated by the cursor-paginated `appreviews` sync from absorb A2) to compute reviews/day and `voted_up` share over a rolling window. |
| 10 | News full-text search across tracked apps | `news search <query> [--app <appid>] [--since <date>]` | 7/10 | All personas | SQLite FTS5 over `app_news.title + contents` (already the brief-specified FTS field), optionally scoped by `appid` or date range. |
| 11 | Concurrent-player trend for one app | `play-trend <appid> [--window 24h\|7d\|30d]` | 6/10 | Game-dev/publisher analyst | Window query over `app_player_counts` (append-only sampling of `GetNumberOfCurrentPlayers`) plotted as ascii sparkline + numeric min/max/last for the requested window. **Low-confidence flag:** value scales with sample density; first-day output is sparse and that sparseness is visible to the user (vs. `app trending` where it's silent), so dogfood-verifiability is preserved. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| C5 profile fetch | Duplicate of absorbed feature A13 | (absorbed, not surfaced as novel) |
| C8 price-drop | Needs persistent price sampling not in data layer | none — falls back to mirror |
| C18 wishlist diff | Brief explicitly defers wishlist | none |
| C21 app trending | Sparse-output failure on fresh DB; dogfood-verifiability concern | play-trend (scoped to one app, sparse output is visible) |
| C22 review sentiment | LLM dependency kill-check | review-velocity (date-bucket aggregation, no LLM) |
| C24 profile heatmap | Scope creep (persistent sampling cron) | rare-achievements (achievement-hunter flex without sampling) |
| C25 library deals | Score 3/10; you don't buy what you own | library audit (covers the `--bounce` cousin: paid-but-low-playtime) |
| C26 news digest | Redundant with `news search --app <appid>` | news search |
| C28 dormant-friends | Score 3/10; no community evidence | none |
| C30 app fingerprint | Redundant with cobratree-walker per-endpoint tools for agents; marginal for humans | (cobratree exposes constituents) |
| C9/C16/C19/C29 | Folded/already-mirrored | (subsumed) |
