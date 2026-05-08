# Steam Web — Build Log

## Phase 2: Generate

- Spec source: `~/printing-press/manuscripts/steam-web-pp-cli/20260404-003212/research/steam-public-spec.json` — 169-endpoint OpenAPI 3.0.3, reused from prior run (still current and comprehensive).
- Spec enriched in-place at `$RESEARCH_DIR/steam-public-spec.json` with:
  - `x-mcp` block: `transport: [stdio, http]`, `orchestration: code`, `endpoint_tools: hidden` (Cloudflare pattern, expected ~193 total tools).
  - `components.securitySchemes.apiKey.x-auth-env-vars: [STEAM_WEB_API_KEY]`.
- Public-param audit: 6 findings, all skip-recorded with evidence. OpenAPI parser does not yet support `flag_name`/`x-flag-name` overlays for parameter renames (retro candidate); Steam's array-index wire shape (`publishedfileids[0]`, `name[0]`) is the literal API contract.
- `printing-press generate` passed all 8 quality gates: go mod tidy, govulncheck, go vet, go build, build runnable binary, --help, version, doctor.
- Re-generated once after fixing two narrative quickstart items: `auth set-token` doesn't fit Steam's env-var-only auth model (replaced with `auth status`); `sync` doesn't take `--steamid` (removed the flag).

## Phase 3: Build novel features

All 11 transcendence features hand-built per the absorb manifest. **Zero stubs**; no shipping-scope downgrades.

### Files

- `internal/cli/steamhelpers.go` (~280 lines) — shared helpers:
  - `resolveSteamID(c, input)` — accepts SteamID64 or vanity URL, resolves via `ResolveVanityURL`.
  - `fetchOwnedGames`, `fetchFriendList`, `fetchPlayerSummaries` (batched up to 100 SteamIDs per request), `fetchSchemaForGame`, `fetchPlayerAchievements` (returns nil-nil on private profile / Steam's 200 + success=false), `fetchGlobalPercentages` (returns map keyed by APIName).
  - `fanOutOwnedGames(ctx, c, limiter, steamids)` — throttled fan-out using `cliutil.AdaptiveLimiter`, surfaces `*RateLimitError` via the limiter without empty-on-throttle silent corruption.
  - Shared types (`ownedGame`, `playerSummary`, `friend`, `achievementSchema`, `playerAchievement`, `globalPctEntry`, `playtimeRow`).

- `internal/cli/novel_library.go` (~250 lines) — `library audit`, `library compare`.
- `internal/cli/novel_friends.go` (~270 lines) — `friends compare`, `currently-playing`, `achievement-leaderboard`.
- `internal/cli/novel_achievements.go` (~370 lines) — `achievement-hunt`, `next-achievement`, `rare-achievements`.
- `internal/cli/novel_app_data.go` (~380 lines) — `news search` (FTS5 over store), `review-velocity` (live `store.steampowered.com/api/appreviews` cursor), `play-trend` (sample-and-persist, sparkline render).

### What each command does

| # | Command | Endpoints / store | Score | Notes |
|---|---------|-------------------|-------|-------|
| 1 | `library audit [steamid]` | GetOwnedGames | 10/10 | Buckets: never_launched, bounce (<2h paid), playtime distribution. |
| 2 | `friends compare <appid>` | GetFriendList × N×GetOwnedGames + GetPlayerSummaries (personas) | 10/10 | Throttled via `AdaptiveLimiter`. `--filter owners` / `--filter owns-zero-hours`. |
| 3 | `next-achievement` | GetOwnedGames + N×(GetSchemaForGame, GetGlobalPercentages, GetPlayerAchievements) | 10/10 | Sweeps top-N most-recently-played apps (cap `--max-apps 50`); ranks locked achievements by global pct DESC. |
| 4 | `achievement-hunt <appid>` | GetSchemaForGame + GetGlobalPercentages + GetPlayerAchievements | 9/10 | Joined per-app workbench. `--locked`, `--rare`. |
| 5 | `rare-achievements [steamid]` | Same trio as next-achievement, sweeps most-played apps first | 8/10 | Inverted sort (asc global pct on achieved=1). |
| 6 | `library compare <steamid>` | Two GetOwnedGames + set ops | 8/10 | `--mine-only` / `--theirs-only` / `--shared`. |
| 7 | `currently-playing` | GetFriendList + batched GetPlayerSummaries | 8/10 | One API call across friends; reads `gameextrainfo`/`gameid`. |
| 8 | `achievement-leaderboard <appid>` | GetFriendList + N×GetPlayerAchievements | 8/10 | Throttled via the same `AdaptiveLimiter`; ranks friends by % achieved. |
| 9 | `review-velocity <appid>` | Live `store.steampowered.com/api/appreviews` cursor | 7/10 | Date-bucket aggregation; `--window 7d/14d/30d`. |
| 10 | `news search <query>` | Local store FTS5 over `news` resource_type | 7/10 | Honest "no_news_in_store" note when store empty (recommends `isteam-news get-news-for-app` to populate); not a stub — works on populated store. |
| 11 | `play-trend <appid>` | GetNumberOfCurrentPlayers; persists sample to local store under `app_player_count:<appid>` | 6/10 | Sparkline + min/max/current; `single_sample` note when only one data point in window. Visible-not-silent sparseness (low-confidence flag honored from manifest). |

### Build-checklist conformance (Phase 3 #1-#10)

- **#1 Non-interactive:** No `bufio.Scanner(os.Stdin)`, no TTY prompts. All commands work in CI.
- **#2 Structured output:** Every command goes through `printJSONFiltered`, picking up `--select`, `--compact`, `--csv`, `--quiet` from rootFlags.
- **#3 Progressive help:** `Example:` strings include realistic SteamID64 (Gabe Newell's, 76561197960287930) and a real appid (1245620 = Elden Ring). `Long:` descriptions explain the algorithm + flag semantics.
- **#4 Actionable errors:** `classifyAPIError` (the generated helper) is called on every API error path; private-profile detection adds a hint.
- **#5 Safe retries:** All commands are read-only (`mcp:read-only: true` annotation on every novel cmd). No mutations.
- **#6 Composability:** Outputs are JSON; pipe to `jq` cleanly. Exit codes via the framework.
- **#7 Bounded responses:** `next-achievement --max-apps 50 --limit 10`, `rare-achievements --max-apps 50 --limit 10`, `news search --limit 50` all bound the output.
- **#8 Verify-friendly RunE:** Every novel command starts with `if <required arg empty> { return cmd.Help() }` then `if dryRunOK(flags) { return nil }` — no `cobra.MinimumNArgs` or `MarkFlagRequired` gates, so verify probes hitting `--dry-run` exit 0.
- **#9 Side-effect short-circuit:** N/A — no novel command performs visible side effects (no browser open, no notifications, no file writes outside the local store cache).
- **#10 Per-source rate limiting:** All friend-fanout commands (`friends compare`, `next-achievement`, `rare-achievements`, `achievement-leaderboard`) use `cliutil.NewAdaptiveLimiter(20.0)` and the `fanOutOwnedGames` helper surfaces `*cliutil.RateLimitError` properly via the limiter's `OnRateLimit()` halving + ceiling discovery. The 25 req/s post-2025-06 budget is honored at the floor (limiter starts at 20/s, ramps up only after 10 consecutive successes).

### Dry-run smoke test

All 11 commands pass `--dry-run` with exit 0 (no API calls, no output, just verify-friendly help fall-through).

### Skipped / deferred (per absorb manifest)

- Wishlist sync (`store.steampowered.com/api/wishlistdata`) — brief explicitly defers; Steam's wishlist endpoint is privacy-gated and brittle.
- `IAuthenticationService/*` operations — Steam's interactive QR/credential login flow, not Web API auth. They get default `mcp:hidden` via the spec's enrichment (Cloudflare pattern hides all per-endpoint mirrors); on the human CLI surface they remain available as `iauthentication-service <subcmd>` for completeness but should not be a normal user flow.
- `IClientStats_*/ReportEvent` — same `mcp:hidden` treatment (reporter, not consumer).

### Generator limitations / retro candidates

- **OpenAPI parser missing `flag_name`/`x-flag-name` overlay support.** 6 public-param-audit findings (one-letter `l` for language; `[0]`-array-shape `publishedfileids[0]`, `name[0]`) had to be skip-recorded with evidence rather than auto-renamed. Recorded in `$API_RUN_DIR/public-param-audit.json`.
- **Generic `resources` table for the data layer.** Steam's data layer brief named 9 typed entities (players, apps, owned_games, achievements_schema, player_achievements, friends, app_news, app_reviews, app_player_counts). The generator emits a generic `resources(id, resource_type, data JSON)` table with FTS5 over the JSON blob. Novel commands worked around it by using `resource_type: "app_player_count:<appid>"` keys and JSON parsing on read; a typed-column option would yield faster aggregations on `library audit` and `review-velocity`.
- No top-level `sql` command emitted by the generator anymore. Novel features compose their own SQL inside their RunE; this is fine for now but means power users can't ad-hoc query the synced store without going through the FTS5 `search` command.
