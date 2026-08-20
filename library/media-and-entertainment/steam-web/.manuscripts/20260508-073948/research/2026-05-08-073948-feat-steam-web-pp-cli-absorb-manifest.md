# Steam Web Absorb Manifest

> 169 OpenAPI endpoints + 17 hand-rolled absorbed features (matched against
> 10 surveyed competitor MCPs/CLIs/SDKs) + 11 transcendence features.
> Total surface ~197 features (some endpoints shipped as `mcp:hidden`).

## Source survey

10 tools across MCP servers, CLIs, and SDKs. Every surveyed tool ships a
20-25-tool snapshot of the API; none ships a local store, SQL, FTS, or
endpoint mirror at scale. **No Go-native MCP exists.**

| Tool | Lang | Surface | Distinguishing feature |
|---|---|---|---|
| TMHSDigital/steam-mcp | Node/TS | 25 tools (18 read, 7 write) | Store + leaderboards + workshop |
| matheusslg/steam-mcp | Node/TS | ~20 tools | Standard Web API |
| algorhythmic/steam-mcp | Python | ~20 tools | Game stats focus |
| dsp/mcp-server-steam | — | — | Library + playtime |
| mcp-steam (PyPI) | Python | 20 tools | Library / achievements / store |
| Philipp15b/go-steamapi | Go | Web API type wrappers | Narrow surface |
| ljesus/steam-go | Go | Web API impl | Low maintenance signal |
| unhappychoice/steamfetch | — | profile display | `neofetch`-style flex |
| jakoch/csgo-cli | Go | CS:GO stats | CS-specific |
| xfxf/steam-friends-played | — | friend-fanout reference | Workflow exemplar, not a tool |

## Absorbed (matched or beat by every endpoint and every competitor feature)

### Endpoint-mirror absorbed (free from generator, ~169 commands)

| Family | Endpoints | What we get |
|---|---|---|
| ISteamUser | GetPlayerSummaries, GetFriendList, ResolveVanityURL, GetUserGroupList, GetPlayerBans, GetUserGroupListAllOf | Profile lookup, vanity-URL resolve, friends, group membership, ban status |
| IPlayerService | GetOwnedGames, GetRecentlyPlayedGames, GetSteamLevel, GetBadges, GetCommunityBadgeProgress, IsPlayingSharedGame | Library, recently played, level, badges |
| ISteamUserStats | GetPlayerAchievements, GetUserStatsForGame, GetGlobalAchievementPercentagesForApp, GetSchemaForGame, GetNumberOfCurrentPlayers, GetGlobalStatsForGame | Per-player and global achievement/stat data |
| ISteamApps | GetAppList, GetServersAtAddress, UpToDateCheck | App enumeration, server lookup, version check |
| ISteamNews | GetNewsForApp, GetNewsForAppAuthed | App news |
| IStoreService | GetAppList, GetItemPrice, GetItemPrices | Store catalog and pricing |
| ICSGOPlayers_730 | GetNextMatchSharingCode, GetMatchHistory | CS:GO/CS2 match codes |
| IClientStats_* | ReportEvent | (mark `mcp:hidden` — reporter, not consumer) |
| IAuthenticationService | BeginAuthSessionViaCredentials/QR, GetAuthSessionInfo, GetPasswordRSAPublicKey | (mark `mcp:hidden` — interactive Steam login flow, not Web API auth) |

### Hand-rolled absorbed (from competitor MCPs / SDKs)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| A1 | App store details (price, release date, header_image, genres) | TMHSDigital/steam-mcp `appdetails` tool | `store appdetails <appid>` calling `store.steampowered.com/api/appdetails` (not in spec) | Cached in local SQLite `apps` table; SQL queryable |
| A2 | App reviews (cursor-paginated) | TMHSDigital/steam-mcp `appreviews` tool | `store reviews <appid>` calling `store.steampowered.com/api/appreviews` | Persisted to `app_reviews` for trend SQL |
| A3 | Wishlist read | mcp-steam | `store wishlist <steamid>` calling store wishlistdata endpoint | Snapshotted; deltas via `since` |
| A4 | Steam level / badge progress | algorhythmic/steam-mcp | endpoint mirror via IPlayerService | n/a — mirror is enough |
| A5 | Friend list resolve | xfxf/steam-friends-played reference workflow | endpoint mirror via ISteamUser | Persisted to `friends` table |
| A6 | Game schema (achievements + stats) | All MCPs | endpoint mirror via ISteamUserStats | Persisted to `achievements_schema` |
| A7 | Per-player achievements | All MCPs | endpoint mirror | Persisted to `player_achievements` |
| A8 | Global achievement % | hhc97/steam_achievement_tracker | endpoint mirror | Joined into `achievements_schema.global_pct` at sync time |
| A9 | Current player count | TMHSDigital/steam-mcp | endpoint mirror | Time-series sampled into `app_player_counts` |
| A10 | News for app | TMHSDigital/steam-mcp | endpoint mirror | Persisted to `app_news` with FTS over title+contents |
| A11 | App list (full Steam catalog) | All MCPs | endpoint mirror via ISteamApps/GetAppList | Persisted to `apps` (FTS over name) |
| A12 | Resolve vanity URL → SteamID | All wrappers | endpoint mirror via ISteamUser | n/a — mirror is enough |
| A13 | `steamfetch`-style profile summary | unhappychoice/steamfetch | hand-rolled `profile fetch <steamid>` aggregating GetPlayerSummaries + GetSteamLevel + GetOwnedGames + GetRecentlyPlayedGames | Single command vs N tabs |
| A14 | Player ban status | mcp-steam | endpoint mirror via ISteamUser/GetPlayerBans | n/a |
| A15 | Recently played games | All MCPs | endpoint mirror | Persisted to `owned_games.playtime_2weeks` |
| A16 | Server time / health probe | All wrappers | endpoint mirror via ISteamWebAPIUtil/GetServerInfo | Used by `doctor` |
| A17 | CS:GO/CS2 match-sharing code lookup | jakoch/csgo-cli | endpoint mirror | n/a |

## Transcendence (only possible with our approach)

11 features, 9 scored 8/10 or higher. Cuts and audit trail in `2026-05-08-073948-novel-features-brainstorm.md`.

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Library backlog audit | `library audit [--never-launched\|--bounce\|--genre-spend]` | 10/10 | Reads local SQLite (`owned_games` joined with `apps.genres` and `apps.is_free`); no API calls at query time; no surveyed competitor MCP ships local SQL over `owned_games`. |
| 2 | Friend playtime comparison on one app | `friends compare <appid>` | 10/10 | Throttled `GetFriendList` × `GetOwnedGames` fan-out via `cliutil.AdaptiveLimiter` (post-2025-06 25 req/s budget); persists to local `owned_games` so subsequent runs hit the store. None of the 5 surveyed MCPs ship a throttled bulk-fanout primitive. |
| 3 | Next easiest achievement (cross-library) | `next-achievement [--app <appid>]` | 10/10 | Local SQLite `player_achievements LEFT JOIN achievements_schema WHERE achieved=0 ORDER BY global_pct DESC`; the `global_pct` column is populated at sync time by merging `GetSchemaForGame` with `GetGlobalAchievementPercentagesForApp`. No surveyed MCP exposes the cross-library "easiest locked" query. |
| 4 | Per-app achievement workbench | `achievement-hunt <appid>` | 9/10 | Local SQLite join (`player_achievements` × `achievements_schema` × `global_pct`) for one `appid` to render schema + unlock state + global rarity in one table; every surveyed MCP exposes the constituent endpoints separately but none ships the joined per-app view. |
| 5 | Rarest unlocked achievements (flex) | `rare-achievements [--steamid <id>]` | 8/10 | Inverse of `next-achievement` — `WHERE achieved=1 ORDER BY global_pct ASC`; surfaces the user's rarest unlocks for steamfetch-style flex. |
| 6 | Library diff between two SteamIDs | `library compare <steamid>` | 8/10 | Set operations on `owned_games` rows (mine-only, theirs-only, shared) plus playtime delta on the shared set; no surveyed competitor MCP ships the set-diff primitive. |
| 7 | Currently-playing friends | `currently-playing` | 8/10 | Single batched `GetPlayerSummaries` call across the friend list, filters to records with non-null `gameextrainfo`/`gameid`, joins to local `apps` for the title — one API call, no fanout. |
| 8 | Achievement leaderboard among friends | `achievement-leaderboard <appid>` | 8/10 | Throttled `GetFriendList` × per-friend `GetPlayerAchievements(<appid>)` fanout, ranked by % achieved for one app. Shares the `AdaptiveLimiter` infrastructure with `friends compare`. |
| 9 | Review velocity over time | `review-velocity <appid>` | 7/10 | Date-bucket aggregation on `app_reviews.timestamp_created` (populated by the cursor-paginated `appreviews` sync from absorb A2) for reviews/day and `voted_up` share over a rolling window. |
| 10 | News full-text search | `news search <query> [--app <appid>]` | 7/10 | SQLite FTS5 over `app_news.title + contents`; no surveyed competitor MCP ships news search — they ship `GetNewsForApp` raw and stop. |
| 11 | Concurrent-player trend | `play-trend <appid>` | 6/10 | Window query over `app_player_counts` (append-only sampling of `GetNumberOfCurrentPlayers`) plotted as ascii sparkline + numeric min/max/last. Low-confidence flag: value scales with sample density; first-day output is sparse but visible to the user. |

## MCP surface decision

Tool count: 169 endpoint mirrors + ~13 framework + 11 novel = ~193 total tools.
Above the 50-tool threshold. **Apply the Cloudflare pattern at generate-time:**

```yaml
mcp:
  transport: [stdio, http]    # remote-capable
  orchestration: code         # thin <api>_search + <api>_execute pair
  endpoint_tools: hidden      # suppress raw per-endpoint mirrors from default surface
```

`IClientStats_*/ReportEvent` and `IAuthenticationService/*` operations get
`mcp:hidden` annotations (interactive / side-effecting; not Web API auth).

Stub status: **none.** All 11 transcendence features are shipping-scope.
