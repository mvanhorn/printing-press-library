# Yahoo Fantasy Sports CLI Brief

## API Identity
- **Domain:** Fantasy Sports Management & Analytics (`fantasysports.yahooapis.com/fantasy/v2`)
- **Users:**
  1. **Active Fantasy Football League Managers ("Sunday Morning Lineup Setter & Tuesday Night Waiver Grinder"):** Manages 1–4 Yahoo NFL fantasy leagues every week from September through January. Needs fast visibility into roster injuries/byes, weekly matchup projections, free-agent/waiver targets, and safe, repeatable commands to change starting lineups (`PUT /team/{team_key}/roster`) and submit add/drop or FAAB waiver claims (`POST /league/{league_key}/transactions`).
  2. **Multi-League Commissioner & Trade Negotiator:** Tracks league-wide standings, points-for vs. schedule luck, transaction velocity, FAAB budget burn rates across all 10–14 managers, and pending trade proposals (`accept_trade`, `reject_trade`, `propose_trade`).
  3. **Draft & Season-Long Quantitative Analyst:** Pulls historical draft results (`draftresults`), weekly scoreboard matchups (`scoreboard`), ownership percentages (`percent_owned`), and multi-week player scoring trends into local SQLite for SQL and cross-week regression analysis.
- **Data profile:** Hierarchical key-addressed REST API (`game_key`, `league_key` = `{game_id}.l.{league_id}`, `team_key` = `{game_id}.l.{league_id}.t.{team_id}`, `player_key` = `{game_id}.p.{player_id}`, `transaction_key` = `{game_id}.l.{league_id}.tr.{trans_id}`). Reads support `?format=json` over OAuth 2.0 Bearer tokens; writes (`PUT /team/{team_key}/roster`, `POST /league/{league_key}/transactions`, `PUT /transaction/{transaction_key}`) accept XML payloads (`Content-Type: application/xml`) or JSON/XML envelopes.

## Reachability Risk
- **Low** — Direct HTTP probe to `GET https://fantasysports.yahooapis.com/fantasy/v2/game/nfl?format=json` returns clean `HTTP 401` JSON without WAF/Cloudflare challenges.
- Tier/permission hints from 4xx body: `{"error":{"lang":"en-US","description":"Please provide valid credentials. OAuth oauth_problem=\"unable_to_determine_oauth_type\", realm=\"yahooapis.com\""}}`
- Probe-safe endpoint used: `GET /game/nfl?format=json`

## Top Workflows
1. **Weekly Lineup Optimization & Position Swaps (`lineup set` / `lineup optimize`):** Inspect a team's active roster for a given NFL week or date, detect `OUT`/`IR`/`BYE` starters stuck in active slots while healthy bench (`BN`) players are available, preview the swap with `--dry-run`, and execute `PUT /team/{team_key}/roster` to move players between `QB`, `RB`, `WR`, `TE`, `W/R/T`, `K`, `DEF`, `BN`, and `IR`.
2. **Add / Drop & FAAB Waiver Claims (`transactions add-drop` / `waiver-wire`):** Scan available free agents (`status=FA`) and waiver players (`status=W`) at a position with ownership percentages and recent stats, compare against the weakest bench droppable player on the user's roster, and submit an atomic add, drop, or add/drop transaction (with optional `<faab_bid>`) via `POST /league/{league_key}/transactions`.
3. **Live Matchup & Win-Probability Tracker (`matchup` / `scoreboard`):** Pull the weekly scoreboard (`league/{league_key}/scoreboard;week={week}`) and head-to-head team matchups (`team/{team_key}/matchups`), comparing projected vs. actual points, remaining active players, and positional advantages.
4. **League Standings, Power Rankings & Schedule Luck (`standings` / `power-rankings`):** Fetch league standings (`league/{league_key}/standings`) and historical weekly scoreboards to compute "all-play" win-loss records, points-for vs. points-against schedule luck, and playoff seeding scenarios.
5. **Trade & Waiver Ledger Audit (`transactions` / `faab-tracker`):** Track all adds, drops, waiver claims, FAAB bids, and pending trades across the league (`league/{league_key}/transactions`), plus accept/reject/cancel pending transactions.

## Table Stakes
- List user's games and leagues (`users;use_login=1/games/leagues`)
- League settings, scoring stat categories, and roster position slots (`league/{league_key}/settings`)
- League standings (`league/{league_key}/standings`)
- League teams and manager metadata (`league/{league_key}/teams`)
- Weekly scoreboard and matchups (`league/{league_key}/scoreboard`)
- Team details, roster by week/date, and matchup opponent (`team/{team_key}/roster`, `team/{team_key}/matchups`)
- Free agents, waivers, taken players, and player search/details (`league/{league_key}/players`)
- Player stats by season, week, average, or date (`league/{league_key}/players;player_keys=.../stats`)
- Player percent-owned and league ownership (`percent_owned`, `ownership`)
- Draft results (`league/{league_key}/draftresults`)
- League and team transactions (`league/{league_key}/transactions`)
- **Write:** Change starting lineup positions (`PUT /team/{team_key}/roster`)
- **Write:** Add player, drop player, atomic add/drop, and FAAB waiver claim (`POST /league/{league_key}/transactions`)
- **Write:** Edit/cancel waiver claim or accept/reject pending trade (`PUT /transaction/{transaction_key}`, `DELETE /transaction/{transaction_key}`)

## Data Layer
- **Primary entities:** `games`, `leagues`, `teams`, `standings`, `rosters`, `players`, `matchups`, `transactions`, `draft_results`
- **Sync cursor:** Season/week-based sync into local SQLite (`resources` table + FTS5 search over player names, team names, manager nicknames, and transaction history)
- **FTS/search:** Instant local search across synced players, rostered assets, free agents, and league transactions without burning Yahoo API rate limits

## Codebase Intelligence
- **Source:** Ground-truth inspection of `michaelfromorg/mcp-yahoo-fantasy`, `derekrbreese/fantasy-football-mcp-public`, `spilchen/yahoo_fantasy_mcp`, `spilchen/yahoo_fantasy_api`, and `uberfastman/yfpy`
- **Auth:** OAuth 2.0 Bearer token (`Authorization: Bearer <token>`) with automatic refresh via `https://api.login.yahoo.com/oauth2/get_token` using `YAHOO_CLIENT_ID`, `YAHOO_CLIENT_SECRET`, and `YAHOO_REFRESH_TOKEN` (or direct `YAHOO_ACCESS_TOKEN` / `YAHOO_FANTASY_TOKEN`).
- **Data model:** Yahoo's `?format=json` wraps collections inside deeply nested alternating list/dict arrays under `fantasy_content`; write operations (`PUT /team/{team_key}/roster` and `POST /league/{league_key}/transactions`) require `<fantasy_content>` XML payloads with `Content-Type: application/xml` (while our CLI/MCP exposes clean flags like `--week`, `--set QB=449.p.31862,BN=449.p.30123`, `--add-player`, `--drop-player`, `--faab-bid` and builds the exact Yahoo XML envelope automatically).
- **Rate limiting:** Yahoo enforces per-IP and per-token burst limits (~300–500 req/hr); caching league settings, stat categories, and player pools in SQLite drastically reduces live calls.

## User Vision
- Hybrid Official Yahoo Fantasy Sports v2 API (OAuth2) + local SQLite analytics, with first-class support to **add players, drop players, submit waiver/FAAB claims, and change starting lineups** (`lineup set`, `transactions add`, `transactions drop`, `transactions add-drop`, `transactions claim`) safely from both the CLI and MCP server.

## Product Thesis
- **Name:** `yahoo-fantasy-pp-cli` / `yahoo-fantasy-pp-mcp`
- **Why it should exist:** Existing Python MCP servers (`derekrbreese`, `michaelfromorg`, `spilchen`) either lack write operations, expose raw 50KB Yahoo JSON trees that blow up agent context, or have no local SQLite store for cross-week analytics. `yahoo-fantasy-pp-cli` is a single static Go binary + MCP server that normalizes Yahoo's arcane XML write payloads (`PUT /roster`, `POST /transactions`) into clean, `--dry-run`-safe CLI flags and MCP tools while syncing leagues, rosters, matchups, and transactions into a local SQLite database.

## Build Priorities
1. Complete Yahoo Fantasy v2 read + write surface (games, leagues, settings, standings, scoreboard, teams, rosters, players, stats, ownership, draft results, transactions, plus XML-backed lineup changes and add/drop/waiver/trade mutations).
2. Ergonomic high-level mutation commands (`lineup set`, `lineup swap`, `transactions add`, `transactions drop`, `transactions add-drop`, `transactions claim`) that construct Yahoo's required `<fantasy_content>` XML payload, validate `--dry-run`, and work cleanly in both CLI and MCP.
3. Transcendence commands powered by local SQLite and compound multi-endpoint synthesis (lineup audit for injured/bye starters, add/drop waiver upgrade recommender, schedule-luck & all-play power rankings, FAAB burn & waiver intelligence).
