# Benzinga CLI Brief

## API Identity
- **Domain:** Licensed financial-data + market-news API ("Benzinga Cloud / Data"). News, 14+ calendar products, fundamentals, market data, analyst intelligence, real-time signals, transcripts.
- **Users:** quant/algo traders, fintech apps & brokerages embedding news+calendars, financial-research desks, trading-bot builders, options/event traders.
- **Data profile:** Time-series + entity rows keyed by `(date, ticker)`. News stories with HTML bodies. Calendar rows (earnings/dividends/ratings/economics/...). Signals (options/blocks/halts). Fundamentals snapshots. All delta-syncable via `parameters[updated]` / `updatedSince`.

## Spec Source (DECIDED)
- **Official OpenAPI specs exist** in the public `Benzinga/benzinga-docs` repo under `openapi/*.spec.yml` (English root; `ar/es/ja/` are translations). Pulled to `research/benzinga-openapi/`.
- **All specs target one host: `https://api.benzinga.com`** (the docs front fundamentals/market data through the same host — the SDK's 3-host split is collapsed in the official docs). This removes the multi-host risk.
- **Auth is uniform `?token=<KEY>` query param** (`securitySchemes.ApiKeyAuth: {in: query, name: token, type: apiKey}`) on 9/11 specs. The generator's `in: query` api_key path (`client.go.tmpl:1441`) emits exactly `?token=`.
- **Generation plan:** merge 6 read-only query-token specs under one `--name benzinga`:
  - `calendar-api` (28 ops), `data-api-proxy` (25 ops: fundamentals+bars+movers+quote+short-interest), `news-api` (3), `logo-api` (2), `ticker-trends-api` (2), `earnings-call-transcripts-api` (2) = **~62 read-only endpoint commands**.
  - **Dropped:** `press-releases` (duplicate `/api/v2/news` path → merge collision), `delivery_api` (14 ops, mostly mutating transcript-management PUT/DELETE/POST — niche/write-licensed), `newsquantified` + `webhook` (header auth, not query-token; 1 op each), `analyst-reports-raw-text` (niche, 1 op).
- No operationId collisions across the 6 core specs; schema collisions are only on Benzinga's shared envelope types (`bzhttp.Error/Resp`) → `--lenient` handles it.

## Reachability Risk
- **Low.** Plain HTTPS GET + `?token=`; no signing/handshake. Reachable programmatically.
- **Everything is paid/tier-gated.** A valid token can still 403 on an endpoint not in the plan (vs 401 for a bad token). No documented free tier. The CLI must surface the 403 plan-gating message clearly so users distinguish "not licensed" from "broken."
- Official `benzinga-python-client` (~31★) shows no systemic 401/deprecation issue threads; old `api-docs` repo archived 2017, current docs (`benzinga-docs`) updated 2026.
- Rate limits undocumented → build polite pagination + delta-sync (docs explicitly recommend narrowing by `tickers`/`date`/`updated`).

## Top Workflows
1. **Watchlist rating-change scan** — today's analyst upgrades/downgrades + price-target changes for a set of tickers (`/calendar/ratings`).
2. **Earnings-season tracker** — week's `/calendar/earnings`, estimates vs actuals, surprises; pair with conference-calls + transcripts.
3. **Weekly economic calendar** — `/calendar/economics?country=US`, sorted by importance, actual-vs-consensus.
4. **Breaking-news stream filtered by ticker/channel** — poll `/api/v2/news?updatedSince=&tickers=&channels=` to drive alerts.
5. **"Why is X moving" unusual-activity scan** — combine `/signal/option_activity`, block trades, halts, ratings, news for one ticker.

## Data Layer
- **Primary entities (syncable):** news stories, ratings, earnings, economics, dividends, splits, guidance, ipos, ma, conference-calls, fda, congress-trades, insider-transactions, option-activity, block-trades, halts.
- **Sync cursor:** `parameters[updated]` (calendars) / `updatedSince` (news). Reconcile deletes via `/calendar-removed/` and `/news-removed`.
- **FTS/search:** news body (strip HTML) + title/teaser/channels/tickers via FTS5; calendar rows by ticker/date.

## Codebase Intelligence
- Ground-truth endpoint map cross-checked against the official `benzinga-python-client` (`financial_data.py`, `news_data.py`). The Python client wraps a SUBSET of the documented surface (omits news channels/top_news/WIIM, transcripts, gov/insider) — the CLI's full-spec coverage already beats it.

## Absorb Landscape (competitors)
- **`benzinga` PyPI client** (~31★, MIT) — the only complete-ish wrapper. Methods → endpoints fully mapped in `ecosystem-research.md`. Covers a subset of the official spec.
- **`@benzinga/*` npm** — session-infra; only data-relevant piece is the Squawk audio SDK (websocket, out of REST scope).
- **`openbb-benzinga`** — OpenBB provider; news + analyst price targets/ratings only. Confirms `BENZINGA_API_KEY` env convention.
- **`go-bztcp` / `python-bztcp`** — official TCP streaming clients (news feed).
- **MCP servers: NONE exist for Benzinga** — confirmed whitespace. Shipping an MCP wrapper is a first.

## Product Thesis
- **Name:** `benzinga` (binary `benzinga-pp-cli`), "the agent-native Benzinga terminal."
- **Why it should exist:** Benzinga's high-value licensed API has only one complete client — a low-traffic Python lib with no CLI, no offline store, an incomplete news surface, and zero MCP presence. This CLI covers the full official documented REST surface as first-class typed commands, adds an offline SQLite store that delta-syncs via the API's own `updated` cursors (instant FTS over news bodies + cross-entity compound queries the REST API can't express in one call), agent-native JSON/JSONL output, and the first-ever Benzinga MCP server. It beats incumbents on ergonomics, offline gravity, and agent-readiness — not on data access (everyone hits the same token-gated REST).

## Build Priorities
1. **P0 foundation:** SQLite store for the syncable calendar/news/signal entities; `sync` (delta via `updated`), `search` (FTS5 over news), `sql`.
2. **P1 absorb:** all ~62 generated endpoint commands (calendar/news/fundamentals/market/signals/analyst/gov/insider/transcripts/logos/trends), each with `--json`/`--select`/`--dry-run`/typed exits.
3. **P2 transcend:** offline + cross-entity novel commands (watchlist scans, "why moving", earnings-season tracker, rating-change drift, calendar-week digest) — from the brainstorm subagent.
