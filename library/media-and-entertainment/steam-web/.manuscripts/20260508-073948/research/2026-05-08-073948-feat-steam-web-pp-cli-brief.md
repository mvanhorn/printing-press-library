# Steam Web API CLI Brief

## API Identity
- **Domain:** Gaming platform — player profiles, game libraries, achievements, friends, stats, store, news, reviews, community.
- **Users:** Achievement hunters, library auditors, game developers tracking concurrent players, data analysts comparing playtimes across friend graphs, Steam profile flexers (`steamfetch`-style), AI agents driving Steam workflows.
- **Data profile:** ~169 documented endpoints across `api.steampowered.com` (IPlayerService, ISteamUser, ISteamUserStats, ISteamApps, ISteamNews, IAuthenticationService, ICSGOPlayers_*, IClientStats_*, ISteamWebAPIUtil) plus the undocumented-but-stable `store.steampowered.com/api/*` (appdetails, appreviews, wishlistdata).
- **Auth:** API key from steamcommunity.com/dev/apikey, sent as `?key=<value>` query parameter. Env var: `STEAM_WEB_API_KEY`.
- **Protocol:** REST/JSON. Some endpoints accept both GET and POST.

## Reachability Risk
- **Low — confirmed reachable** with the user's API key (HTTP 200 from `/ISteamUser/GetPlayerSummaries/v2/`). Server time endpoint reachable without auth (`/ISteamWebAPIUtil/GetServerInfo/v1/`).
- **Rate limits tightened ~2025-06**: GetPlayerSummaries went from ~100 req/s to ~25 req/s with burst lock; 429 returns `x-eresult: 25` (LimitExceeded) or `84` (RateLimitExceeded). Inventory endpoints aggressively throttled.
- **Privacy gotchas:** private profile -> `GetOwnedGames` returns `{}` (200 + empty payload), not a clear "private" signal. `GetUserStatsForGame` empty for the first ~hours after purchase.
- **Schema drift on undocumented store endpoints** (`appdetails` field shapes change silently). The `Philipp15b/go-steam` README explicitly warns "expect things to break."

## Spec Sources
- **Local 169-endpoint OpenAPI 3.0.3** at `~/printing-press/manuscripts/steam-web-pp-cli/20260404-003212/research/steam-public-spec.json` (auto-generated from Steam's GetSupportedAPIList; covers public + many undocumented interfaces, including `IAuthenticationService` for QR login). Auth scheme `apiKey` in query, name `key`. **This is the spec we will reuse — comprehensive and recent.**
- xPaw/SteamWebAPIDocumentation (HTML, 200+ interfaces) is a reference, not OpenAPI.
- store.steampowered.com endpoints not in the spec — captured as additional commands in the absorb manifest.

## Top Workflows
1. **Achievement completionism.** "What's my next-easiest achievement to unlock (highest global %, currently locked) across my whole library?" — `GetPlayerAchievements` × `GetGlobalAchievementPercentagesForApp` × `GetSchemaForGame`, joined locally.
2. **Friend-vs-me playtime comparison on a specific game.** "Who in my friends list has the most hours in Elden Ring? Who owns it but has zero hours?" — `GetFriendList` -> N × `GetOwnedGames`, fanout naturally needs throttle + cache.
3. **Library audit / backlog analysis.** "Games I own but never launched", "$ spent on games <2hr playtime", "genres I overbuy" — needs `GetOwnedGames(include_appinfo=1, include_played_free_games=1)` joined with `appdetails` from the store API.
4. **Game-dev / publisher analytics.** Live concurrent players (`GetNumberOfCurrentPlayers`), review velocity (`appreviews/<appid>` cursor-paginated), price-history alerting via store `appdetails`.
5. **Steam profile flexing / `steamfetch`-style display.** Top played, rare achievements, total hours — single command, terminal-pretty output.

## Table Stakes (absorbed)
- Player lookup by SteamID, vanity URL resolution.
- List owned games for a SteamID with playtime breakdowns.
- Player achievements per game, global achievement percentages, game schema.
- Friend list, friend-of-friend traversal.
- Recently played games, online status, playtime in 2 weeks.
- App list, news for a given app, current player counts.
- Store appdetails (price, genres, release date, header image), appreviews (cursor pagination).
- CS-specific (CSGO/CS2 match codes, sharing codes).
- Auth: API key configuration, validation against `GetServerInfo`/`GetPlayerSummaries`.
- Output flags: `--json`, `--csv`, `--select`, `--compact`.

## Data Layer
Primary entities (SQLite with FTS5 over text fields):

- `players` (steamid PK, persona_name, profile_url, country, last_logoff, visibility_state, fetched_at)
- `apps` (appid PK, name, type, is_free, release_date, header_image, fetched_at) — populate from `GetAppList` + `appdetails`
- `owned_games` (steamid + appid PK, playtime_forever, playtime_2weeks, playtime_windows/mac/linux, last_played_at) — `playtime_2weeks` is the high-signal "what to play next" column
- `achievements_schema` (appid + apiname PK, display_name, description, hidden, icon, global_pct) — merges `GetSchemaForGame` + `GetGlobalAchievementPercentagesForApp`
- `player_achievements` (steamid + appid + apiname PK, achieved, unlock_time)
- `friends` (steamid + friend_steamid PK, relationship, friend_since)
- `app_news` (gid PK, appid, title, url, contents, date)
- `app_reviews` (recommendationid PK, appid, author_steamid, voted_up, votes_up, playtime_at_review, timestamp_created)
- `app_player_counts` (appid + sampled_at PK, current_players) — time-series, enables trending SQL

**Sync cursor:** per-table `fetched_at` for snapshot semantics; `appreviews` uses Steam's cursor parameter; `app_player_counts` is append-only sampling.

**FTS/search:** `apps.name`, `app_news.title + contents`, `achievements_schema.display_name + description`, `players.persona_name`.

**Drop/defer:** wishlist (privacy-gated, brittle endpoint), recently_played (subset of `owned_games.playtime_2weeks` — derive in SQL).

## Codebase Intelligence
- **Source signals (from MCP repos and SDK READMEs):**
  - Auth header: none — query parameter `key` only.
  - All 5+ existing MCP servers (TMHSDigital, matheusslg, algorhythmic, dsp/mcp-server-steam, mcp-steam) ship 20-25 tools each; **none of them ship a local store, SQL, FTS, or endpoint mirror at scale.**
  - No Go MCP exists. Two narrow Go SDKs: `Philipp15b/go-steamapi` (low-maintenance) and `ljesus/steam-go`.
  - The "no bulk endpoints" pain point is repeated across every wrapper's issue list.
- **Auth:** `apiKey` query param `key`. Token format is hex; `IAuthenticationService` is for Steam's first-party login flow and does NOT use the Web API key — flag those endpoints as side-effecting / `mcp:hidden`.

## User Vision (reprint motivation)
The prior CLI was generated on Printing Press v0.4.0 (2026-04-04). v4.0.6 includes substantial machine upgrades the prior CLI did not benefit from: typed exit codes, MCP transport options (stdio + http), code-orchestration MCP surface for large APIs, agent-native architecture (cobratree walker, mcp:read-only annotations), per-source rate limiting (`cliutil.AdaptiveLimiter`), narrative validation, output-review pass. **Treat this as a redo against the current machine** — let novel features and MCP surface be re-evaluated, not carried forward verbatim. The 169-endpoint spec is reusable; everything wrapping it should be re-derived.

## Product Thesis
- **Name:** `steam-web-pp-cli` (binary), `steam-web` (slug)
- **Display:** Steam Web
- **Why it should exist:**
  - Only Go-native CLI + MCP for Steam — every existing MCP is Node/Python with a 20-25 tool snapshot of the API.
  - Local SQLite with FTS lets the friend-playtime, library-audit, and completionism workflows run as one SQL query instead of an N+1 fanout app rewrite.
  - Built-in throttle for the post-June-2025 25 req/s reality (`AdaptiveLimiter` + `*RateLimitError` propagation).
  - Both `api.steampowered.com` AND undocumented `store.steampowered.com` covered in one tool.
  - Agent-first: an agent can answer "which of my friends owns Hades II and has <5h playtime" without writing code.

## Build Priorities
1. Generate from the 169-endpoint spec; all read-side endpoint mirrors land for free.
2. Data layer: 9 tables above, `sync` per-resource, FTS over apps/news/achievements/players.
3. Sync wiring for the 5 highest-gravity resources (players, owned_games, achievements_schema, player_achievements, friends).
4. **Novel feature absorption** (Phase 1.5 subagent decides which of these survive):
   - `library audit` — backlog SQL on `owned_games`
   - `friends compare <appid>` — fan-out playtime across friend list with `AdaptiveLimiter`
   - `next-achievement` — easiest unlocked-globally achievement still locked for me
   - `play-trend <appid>` — current_players over time from `app_player_counts`
   - `profile fetch` — `steamfetch`-style summary
5. Mark `IAuthenticationService` operations as `mcp:hidden` (interactive Steam login, not Web API auth).
6. MCP surface: spec has 169 endpoints + ~5-15 novel commands -> > 50 tool threshold. Apply Cloudflare pattern: `mcp.transport: [stdio, http]`, `mcp.orchestration: code`, `mcp.endpoint_tools: hidden`.
