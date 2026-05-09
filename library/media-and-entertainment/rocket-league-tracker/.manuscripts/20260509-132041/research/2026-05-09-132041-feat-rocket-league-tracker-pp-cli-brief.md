# Rocket League Tracker CLI Brief

> Slug: `rocket-league-tracker` · Binary: `rocket-league-tracker-pp-cli`
> Spec source: research-derived (no authoritative spec exists)
> Target: `rocket-league1.p.rapidapi.com` via RapidAPI marketplace

## API Identity

- **Domain:** Rocket League player rank/MMR/profile lookups, plus item shop, tournament, and announcement metadata. Game data is owned by Psyonix; the RapidAPI listing is a third-party relay (proxies upstream sources).
- **Users (concrete):**
  - **The grinder.** Plays 2-3 hours of competitive every night, needs to know their MMR/tier change after each session without opening tracker.gg in a browser. Cares about playlist deltas (1v1 vs 2v2) and "am I close to next promo?" Today: refreshes their tracker.gg profile every 10-15 minutes, watches the chart by eye.
  - **The friend-group sweat.** Has 4-5 friends they queue with; wants to see who tilted last night, who's improved the most this week, and who's not actually GC like they claim. Today: copy-pastes everyone's tracker.gg URL into a group chat once a week.
  - **The Discord-bot author.** Maintains a 50-200-member RL Discord server, currently runs Tusk/FlipReStat (TRN scraper), watches it break every few months when TRN tightens. Wants a CLI they can call from a bot script that won't 403 next quarter.
  - **The agentic player coach.** Uses an AI agent for self-review; pipes their own rank/match data into a coaching loop that suggests playlists to grind or rotations to drill. Today: hand-curates a CSV.
- **Adjacent:** This very project's collector pipeline (apps/collector reads the local Stats API and stores snapshots — a CLI surfaces those same data shapes for agent and terminal use).
- **Data profile:** Per-player profile (name, tag, presence), per-playlist rank (tier, division, MMR, total games, streak), aggregate stat values (goals, wins, MVPs, etc.), playlist populations, item shop snapshots, current tournaments, recent announcements.

## Reachability Risk

- **Status: Low (verified live).** `printing-press probe-reachability https://rocket-league1.p.rapidapi.com/` → `mode: standard_http`, `401` without key. `curl /ranks/<player>` without key → `429 Too many requests` (RapidAPI gateway throttles unauthenticated callers, as designed). With a free RapidAPI key the endpoint returns 200 JSON.
- **Upstream risk:** RapidAPI listings can be removed by the operator at any time. The "Rocket League by Stannis" listing is unofficial and ultimately re-hosts data that originates with TRN/Psyonix; if upstream tightens, this listing dies. Two community wrappers already died from upstream changes (PannH/trn-rocket-league archived 2025-11; kilroy-2/rl-rank-stats-web-scrap broken since 2023-04).
- **Tracker Network direct:** Available technically via Surf-Chrome TLS fingerprinting (probe-reachability returned `browser_http`), but TRN's own block page explicitly says *"do not scrape... Legal will get involved."* Out of scope by user decision.
- **Psyonix Public API:** Closed beta (`api.rocketleague.com`, OAuth client_id/client_secret). Effectively unobtainable for casual projects. Out of scope.

## Top Workflows

1. **`rank <player> [--platform <p>]`** — fastest lookup of every playlist's tier/division/MMR for a given player. The single most-asked-for feature in every wrapper readme.
2. **`profile <player>`** — name, tag, presence; the "is this person who I think it is" check before deeper queries.
3. **`stat <player> <stat>`** — single named stat (goals, wins, MVPs, saves, assists). Composable in shell pipelines; the building block for "who's improved most this week" workflows.
4. **`track <player>` + `trend <player> --days N`** — local SQLite snapshots over time. The transcendence move: TRN/RapidAPI/Psyonix all give you "right now" — nobody gives you a historical curve unless you persist it yourself.
5. **`shop` / `tournaments` / `announcements`** — daily-driver metadata not specific to one player; useful for Discord-bot-style "what's new" queries.

## Table Stakes (must-match)

- Multi-platform input (steam, epic, psn, xbox) — every competing wrapper supports at least 2-3.
- `--json` output for every command — RapidAPI itself returns JSON.
- Per-playlist breakdown (1v1, 2v2, 3v3, Hoops, Rumble, Dropshot, Snowday, Tournament playlists).
- Rate-limit awareness — RapidAPI's free tier caps requests; the CLI must surface 429s clearly, not silently drop to empty.
- `X-RapidAPI-Key` env var auth (`RAPIDAPI_KEY`) — the standard pattern across every RapidAPI listing.

## Data Layer

- **Primary entities:** `players` (storage_key, platform, account_id, display_name, tag), `playlists` (player_storage_key, playlist_key, tier, division, mmr, games, streak, captured_at), `stats` (player_storage_key, stat_name, value, captured_at), `shops`, `tournaments`, `announcements`.
- **Sync cursor:** `players` and `playlists` are append-only snapshot rows keyed by `(storage_key, captured_at)`. No upstream cursor — every sync is a full pull for the requested player.
- **FTS/search:** FTS5 over `players.display_name`, `players.tag`, plus an `aliases` table mapping platform-specific user input to the canonical storage_key (mirrors this project's player-directory.ts pattern).

## Codebase Intelligence

- Surrounding project (rocket-league-stats) already has `apps/web/lib/rocket-league/player-identity.ts`, `player-directory.ts`, `match-history.ts` — the storage-key pattern (`platform:account_id` + alias resolution) is proven and worth mirroring in the CLI's local store. Same canonical-key shape would let the CLI cross-reference data the project already collects via OCR + Stats API.
- Auth: `X-RapidAPI-Key` (per-call) + `X-RapidAPI-Host: rocket-league1.p.rapidapi.com` (constant). Env var: `RAPIDAPI_KEY`. Standard RapidAPI marketplace pattern.
- Rate limiting: free tier is roughly 50 req/day on RapidAPI's "Basic" plan for this listing (typical for community-relay listings; exact figure varies). Adaptive limiter required.

## Product Thesis

- **Name:** Rocket League Tracker CLI (`rocket-league-tracker-pp-cli`)
- **Why it should exist:** Every RL rank lookup today happens through a website (tracker.gg, rocketleague.tracker.network) that is heavy, ad-supported, and fights you with Cloudflare if you script it. The RapidAPI listing exists to solve this for developers but every published wrapper is either dead, partial (one or two endpoints), or aimed at Discord bots only. Nobody ships an offline-capable CLI that absorbs the full RapidAPI surface, persists snapshots locally, and computes time-series rank deltas. The transcendence work is in `track` + `trend` + `peek` — questions only answerable with a local store.
- **Differentiator:** Agent-native (`--json`, `--select`, typed exit codes, dry-run on every mutation), offline historical curve (`trend --days N` after seeding with `track`), tight rate-limit handling that surfaces typed 429s rather than dropping rows.

## Build Priorities

1. **P0 (foundation):** Local SQLite store with `players`, `playlists`, `stats`, `aliases`, FTS5 over `players`. Sync command that pulls one player + all playlists in one shot.
2. **P1 (absorb table-stakes):** `rank`, `profile`, `stat`, `search`, `playlists`, `shop`, `tournaments`, `announcements`, `titles`, `clubs`, `population`, `leaderboard` (skill + stat). All with `--json`, `--platform`, `--dry-run` where applicable.
3. **P2 (transcendence):** `track`, `trend --days N`, `peek` (one-line daily summary), `compare <p1> <p2>`, `delta` (diff between two snapshots), `streak-history`, `mmr-curve`, `playlist-distribution`.
4. **P3 (polish):** README cookbook with realistic invocations, `auth` subcommand for key setup, `doctor` for env + reachability check, `agent-context` MCP surface for AI agent integration.

## Honest Caveats

- The spec is research-derived. Some endpoints in the absorb manifest may not exist on the RapidAPI listing exactly as written — the listing's own JS-rendered docs page couldn't be parsed, so endpoint inventory comes from yataknemogy/rocket-league-api, the AngaBlue blog post, and the RapidAPI marketplace summary. Phase 5 dogfood would catch divergence; without a key, divergence is discovered at first user-run.
- Live smoke testing in Phase 5 will be skipped unless the user provides a `RAPIDAPI_KEY` before that phase.
- The upstream listing may go dark unilaterally. The CLI should ship with a clear `doctor` story for "the API is now 404" and a README note pointing to the same risk.
