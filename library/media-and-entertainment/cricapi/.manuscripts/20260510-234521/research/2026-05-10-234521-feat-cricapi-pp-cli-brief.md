# Cricket Data (CricAPI) CLI Brief

## API Identity
- **Service**: Cricket Data API (rebranded from CricAPI). Same `api.cricapi.com/v1` endpoint, new docs/signup at cricketdata.org.
- **Domain**: Live cricket scores, fixtures, series, players, fantasy data. International + major domestic leagues (IPL, BBL, PSL, BPL, CPL, Hundred, etc.).
- **Users**: Cricket fans, fantasy-cricket players, sports-data app developers, AI agents needing cricket facts.
- **Auth**: Single GUID-format API key passed as `?apikey=` query param. Free tier ≈ 100 req/day. Signup is one form on cricketdata.org.
- **Data profile**: REST + JSON. Top-level envelope `{ status: "success"|"failure", data: [...], reason: "..." }`. No GraphQL, no WebSockets, no streaming. Pagination via `offset=` (page size effectively fixed by API).

## Reachability Risk
- **None.** `curl https://api.cricapi.com/v1/countries?apikey=DEMO` → HTTP 200 with clean `{"status":"failure","reason":"Invalid API Key"}`. No Akamai/Cloudflare gate, no bot detection, no IP-block reports in research. Free tier publicly advertised. Owner actively maintains a GitHub samples repo (`CricketData/Python-Code-Samples`).

## Canonical Endpoints (12, from official samples)

| Endpoint | Purpose | Params |
|---|---|---|
| `GET /countries` | List supported countries | `offset` |
| `GET /currentMatches` | Live + about-to-start matches across all cricket | `offset` |
| `GET /matches` | Generic match list with optional search | `offset`, `search` |
| `GET /match_info` | Single match details | `id`, `offset` |
| `GET /match_scorecard` | Full scorecard (fantasy + general) | `id`, `offset` |
| `GET /match_squad` | Match squads | `id` |
| `GET /match_points` | Fantasy match points | `id` |
| `GET /cricScore` | Quick live-score snapshot | (apikey only) |
| `GET /series` | Series list | `offset` |
| `GET /series_info` | Series details (squads, matches, points) | `id`, `offset` |
| `GET /players` | Player search/list | `offset`, `search` |
| `GET /players_info` | Player details (career stats) | `id`, `offset` |

## Top Workflows
1. **"What's on right now"** — fan checks live + upcoming during the day → `currentMatches`
2. **"When does my team play next"** — team-aware fixture lookup → `matches?search=Pakistan` or filter `currentMatches`
3. **"Show me the scorecard"** — after a match → `match_scorecard`
4. **"Player career stats"** — `players?search=kohli` → `players_info?id=...`
5. **"Series fixture board"** — tournament-following → `series` → `series_info?id=...`
6. **"Fantasy match points"** — fantasy players → `match_points?id=...`

## Table Stakes (from competitor CLIs)
- **cbirajdar/cricket-cli** (Python, 19★, stale since 2017): `scores`, `rankings`, `standings`
- **Nshul/cricket-cli** (Node, 6★): `calendar`, `configure`
- **mikeesto/cricket-cli** (Node, 0★): live score watch
- **cricketlive** npm (2024): live match details lib
- **No cricket MCP servers exist on GitHub.** This CLI's MCP would be the **first** for cricket data — that's a real novelty signal.

So a CLI in this space at minimum needs: live scores, fixtures/calendar, scorecards, rankings/standings (out of scope for this API but the ESPN CLI covers it), and API-key configuration.

## Data Layer (SQLite store rationale)
- **Primary entities**: `matches`, `series`, `players`, `countries`. Each has a stable `id`.
- **Refresh cadence**: matches/series change daily; players/countries change rarely.
- **Why a store**: free tier is 100 req/day. Without local caching, a user opening the CLI 10x a day burns the budget on the same data. Sync once, query SQLite locally.
- **FTS targets**: match names (`India vs Pakistan, 5th T20I`), player names, series names.

## Codebase Intelligence
- No DeepWiki entry found for cricapi/cricketdata. Skipping.
- MCP source-code analysis: no MCP servers exist, so no source to mine. The `CricketData/Python-Code-Samples` repo IS the canonical reference (used above).
- Auth pattern: single GUID API key, query-param only. Owner enforces format on `/cricScore` (returns `"Guid should contain 32 digits with 4 dashes"`).

## Product Thesis
- **Name**: `cricapi-pp-cli` (preserves the API hostname; "cricapi" is more searchable than "cricketdata")
- **Display name**: Cricket Data
- **Why it should exist**:
  - First MCP server for cricket data — direct agent access via Claude Desktop, Cursor, etc.
  - Offline SQLite store + FTS — search matches/players/series without burning the 100 req/day budget
  - Team-aware fixture queries — solve the natural-language "next Pakistan game" problem the user already cares about
  - Fantasy-data layer — most existing cricket CLIs ignore fantasy; this API exposes it cleanly

## Build Priorities
1. **Foundation**: 12 endpoint commands, SQLite store for `matches`/`series`/`players`/`countries`, sync command, FTS search.
2. **Absorb**: live scores (`scores`/`live`), upcoming calendar (`calendar`/`today`), player search, series listing, scorecards, fantasy points/squad.
3. **Transcend**: team-aware fixture filter (`team pakistan next`), series timeline view, format-split player stats, fantasy aggregator, watch-loop for live scores, MCP tool surface for agent access.

## Source Priority
- Single-source CLI. No combo. No priority gate needed.
