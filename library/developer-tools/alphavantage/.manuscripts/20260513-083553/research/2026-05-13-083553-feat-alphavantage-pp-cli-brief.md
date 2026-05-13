# Alpha Vantage CLI Brief

## API Identity
- **Domain:** Financial market data — stock quotes, fundamentals, news sentiment, technical indicators, FX/crypto/commodities, economic indicators.
- **Users:** Retail traders, fintech developers, quant analysts, AI agents doing equity research.
- **Data profile:** Single endpoint at `https://www.alphavantage.co/query` with `function=` parameter selecting the resource. JSON (some endpoints also CSV). Free tier = **25 requests/day** hard cap (down from older 5/min, 500/day). No OpenAPI spec — docs are HTML at alphavantage.co/documentation/.

## Reachability Risk
- **Low.** Probe to `NEWS_SENTIMENT&tickers=AAPL&limit=1` returned HTTP 200, 121KB payload, 50 items. Auth via `apikey=` query param works.
- **Quota risk: High.** 25 req/day burns fast. Every wrapper in the ecosystem fails on rate-limit detection (RomelTorres #125, multiple downstream consumers). AV mutates error shape: sometimes `Note`, sometimes `Information`, sometimes `Error Message` — most wrappers only check one.

## Top Workflows
1. **News + sentiment for a watchlist** — pull `NEWS_SENTIMENT&tickers=X,Y,Z`, see what moved sentiment, identify high-relevance articles.
2. **Pre-earnings check** — fetch `EARNINGS` + `EARNINGS_CALENDAR` + `NEWS_SENTIMENT topic=earnings` for a ticker before a print.
3. **Top movers scan** — `TOP_GAINERS_LOSERS` daily; cross-reference with news sentiment for stories driving moves.
4. **Quote + fundamentals snapshot** — `GLOBAL_QUOTE` + `OVERVIEW` + recent earnings for a one-shot research card.
5. **Macro context** — `CPI`, `FEDERAL_FUNDS_RATE`, `TREASURY_YIELD`, `UNEMPLOYMENT` for the dashboard or pulse report.

## Table Stakes (must match)
- All 106 functions across 9 categories (Core Stock 10, Options 3, Alpha Intelligence 7, Fundamentals 8, Forex 4, Crypto 5, Commodities 13, Economic 10, Technical 45).
- All filter params for NEWS_SENTIMENT (tickers, topics, time_from, time_to, sort, limit).
- CSV output for time-series (the AV native format for those).
- API key from env var or config (most wrappers force config files only).

## Data Layer
- **Primary entities:** ticker_news (sentiment articles, deduped by URL), sentiment_history (rolling per-ticker), top_movers (daily snapshots), fundamentals (per ticker, quarterly), earnings (history + estimates + calendar), economic_indicator_series (per indicator name), quota_log (per call), price_quote_cache (intraday TTL, daily/weekly/monthly long-lived).
- **Sync cursor:** time_published for news (most recent on top); date for time series; quarter for fundamentals.
- **FTS5:** news titles + summaries + topics for offline grep across pulled articles. This is huge — most users will accumulate hundreds of articles and need to grep "Apple AND tariff" without re-fetching.

## Codebase Intelligence
- **Auth:** `apikey=` query string param, no header. Simple. CLI should read from env var `ALPHAVANTAGE_API_KEY` (or `ALPHA_VANTAGE_API_KEY` alias common in community wrappers).
- **Rate-limit shape:** API returns 200 with body `{"Note": "..."}` OR `{"Information": "..."}` OR `{"Error Message": "..."}` when quota hit. **All three patterns must be checked** — this is the single biggest community pain point (RomelTorres #125, finance-quote #130, TradingAgents #305).
- **Quota:** Free tier = 25/day, premium tiers up to 1200/min. AV does NOT return a `429` — they return 200 with a Note field. This is a major footgun.
- **Architecture insight:** Single endpoint `/query` for everything. Function selection via `function=` param. This makes a "send raw" escape hatch trivial — agents who need premium endpoints they don't know yet can fall back.

## Ecosystem (competitors to absorb-and-beat)
- **alphavantage/alpha_vantage_mcp** (OFFICIAL) — 106 tools across 9 cats. Uses "Progressive Tool Discovery" to limit token burn. **Hosted at mcp.alphavantage.co.** This is the boss to beat.
- **portfoliotree/alphavantage** (Go) — full function coverage via `Commodities()`, `Crypto()`, `Economic()`, etc. methods. Has bare `av` CLI. **Closest competitor in Go terrain.**
- **RomelTorres/alpha_vantage** (Python, most-starred) — wraps most endpoints, no CLI, no MCP, no SQLite, weak rate-limit handling.
- **ashleydavis/alpha-vantage-cli** (TS) — CSV-download focused, narrow scope.
- **berlinbra/alpha-vantage-mcp, matteoantoci/mcp-alphavantage, calvernaz/alphavantage** — community MCPs, subset coverage.

## Product Thesis
- **Name:** `alphavantage-pp-cli`
- **Display name:** Alpha Vantage
- **Why it should exist:** The ecosystem fails three users:
  1. **Quota-blind users** burn their 25/day before realizing it (no community tool tracks usage, all silently fail on the 200-with-Note pattern).
  2. **Agents** can't grep historical news across pulled sentiment data — no local store.
  3. **Power users** want compound queries (high-sentiment tickers in last 7d + pre-earnings + macro-tailwind) that need joins, which a single REST API call can't do.
- The Cloudflare-pattern MCP (search + execute) is right for 106 tools — agents shouldn't load 106 tool descriptions at startup.

## Build Priorities
1. **Quota tracking + intelligent throttling** — every call logs to `quota_log`; the CLI refuses to call when budget is exhausted; `quota status` shows daily burn.
2. **NEWS_SENTIMENT first-class** with filters wired (tickers, topics, time_from/to, sort, limit) and SQLite-backed sentiment history.
3. **All 106 functions reachable** via typed subcommands AND a raw `query` escape hatch (`function=X&param=Y`).
4. **FTS5 news search** offline across pulled articles.
5. **Cross-call workflows** (transcendence): pre-earnings check, sentiment timeline for ticker, top-movers-with-news, macro snapshot, watchlist sentiment scan.
