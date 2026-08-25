# Benzinga API — Ecosystem Research

Research compiled 2026-06-27 to inform building a best-in-class CLI for the Benzinga developer/data API. Evidence is drawn from the official docs (`docs.benzinga.com`, including the machine-readable `llms.txt` / `llms-full.txt`), the official `Benzinga/benzinga-python-client` source on GitHub (ground-truth endpoint URLs), npm/PyPI package pages, and the Benzinga GitHub org. Source URLs are cited inline. Where something could not be confirmed it is flagged explicitly.

---

## 1. API Identity & Products

Benzinga is a financial-media + market-data company. Its developer API ("Benzinga Cloud" / "Benzinga Data") is a licensed, tier-gated REST + streaming product covering news, calendars, fundamentals, market data, analyst data, and real-time signals. The canonical docs site is **https://docs.benzinga.com** (legacy mirror: `docs.benzinga.io`). The machine-readable doc index lives at **https://docs.benzinga.com/llms.txt** and the full text at **https://docs.benzinga.com/llms-full.txt** — these are the single best source for endpoint coverage and were used heavily here.

### Base URLs (IMPORTANT — there are several, and they differ per product)

Ground-truth from the official Python client source ([`financial_data.py`](https://github.com/Benzinga/benzinga-python-client/blob/master/benzinga/financial_data.py)) and docs:

| Base URL | Used for |
|---|---|
| `https://api.benzinga.com` | News (`/api/v2/news`), Calendar (`/api/v2/calendar/*` and `/api/v2.x/calendar/*`), bars (`/api/v2/bars`), delayed quote (`/api/v1/quoteDelayed`), logos (`/api/v1.1/logos`), signals (`/api/v1/signal/*`), gov trades, insider, transcripts, analyst insights — the primary REST host |
| `https://data-api.benzinga.com` | Fundamentals (`/rest/v3/fundamentals/*`), quote (`/rest/v2/quote`), batch price history (`/rest/v2/batchhistory`), autocomplete (`/rest/v2/autocomplete`), movers (`/rest/movers`), instruments (`/rest/v3/instruments`), ticker detail (`/rest/v3/tickerDetail`), ownership (`/rest/v3/ownership/summary`) |
| `https://api.benzinga.io/dataapi` | `security` (`/rest/v2/security`), `chart` (`/rest/v2/chart`) — older `.io` host still referenced by the SDK |

Note the **version sprawl**: the docs (`llms-full.txt`) show calendar endpoints at `v2.1`/`v2.2` (e.g. `/api/v2.1/calendar/earnings`, `/api/v2.2/calendar/dividends`), while the Python SDK still calls `/api/v2/calendar/earnings`. Both appear live; v2.x is the newer documented surface. A CLI should default to the documented `v2.1`/`v2.2` paths but be aware the `v2` paths the SDK uses still resolve.

### Product / endpoint families

Auth param style for all of the below: **`?token=<KEY>`** query param (see §2). Response format JSON unless noted; the News API historically also supports XML.

**News & content**
- **News (Newsfeed/Stories)** — `GET https://api.benzinga.com/api/v2/news`. Key params: `tickers`, `channels`, `dateFrom`, `dateTo`, `date`, `updatedSince`, `publishedSince`, `lastId`, `displayOutput` (`headline` | `abstract` | `full`), `pageSize`, `page`, `sort`. Returns author, created, updated, title, teaser, body (HTML), url, image, channels, stocks, tags. JSON or XML (XML historically the default; JSON via `Accept: application/json`). Docs: news-api/get-news-items. ([massive.com mirror](https://massive.com/docs/rest/partners/benzinga/news))
- **Removed News** — `GET /api/v2/news/removed` (for syncing a local store and purging retracted items).
- **News Channels** — list of available channels (`news-api/channels`).
- **Press Releases** — `news-api/press-releases`.
- **Why Is It Moving (WIIMs)** — `news-api/wiims`; short structured explanations of why a ticker is moving.
- **NewsQuantified** — `newsquantified-api`; sentiment scores, relevance metrics, market-impact scoring on news.

**Calendar APIs** (base `https://api.benzinga.com`; common params on all: `parameters[date_from]`, `parameters[date_to]`, `parameters[date]`, `parameters[tickers]`, `parameters[updated]`, `parameters[importance]`, `page`, `pagesize`, `date_sort`):
- **Earnings** — `GET /api/v2.1/calendar/earnings` — eps, eps_est, revenue, revenue_est, surprises.
- **Dividends** — `GET /api/v2.2/calendar/dividends` — dividend amount, ex_date, record_date, yield (params `div_yield`, `div_yield_operation`).
- **Splits** — `GET /api/v2.1/calendar/splits`.
- **Economics** — `GET /api/v2.1/calendar/economics` — event_name, actual, consensus, prior; param `country`.
- **Guidance** — `GET /api/v2.1/calendar/guidance` — eps/revenue guidance ranges.
- **IPOs** — `GET /api/v2.1/calendar/ipos`.
- **Offerings (secondary)** — `GET /api/v2.1/calendar/offerings`.
- **M&A** — `GET /api/v2.1/calendar/ma`.
- **Conference Calls** — `GET /api/v2.1/calendar/conference-calls`.
- **Ratings (analyst)** — `GET /api/v2.1/calendar/ratings` — action_company, rating_current, pt_current; param `action`.
- **Ratings → Firms** — `GET /api/v2.1/calendar/ratings/firms`.
- **Ratings → Analysts** — `GET /api/v2.1/calendar/ratings/analysts` — includes ratings_accuracy metrics.
- **FDA** — `GET /api/v2.1/calendar/fda` — drug, event_type, PDUFA/outcome.
- **Retail** — `GET /api/v2/calendar/retail` (per SDK).
- **Events (corporate)** — `GET /api/v2/calendar/events`.
- **Removed** — `GET /api/v2.1/calendar-removed/` — purge cancelled/removed calendar rows from a local store.

**Signals (real-time / unusual activity)** (base `https://api.benzinga.com`):
- **Unusual Options Activity** — `GET /api/v1/signal/option_activity` — strike_price, put_call, sentiment, volume; params `date`, `date_from`, `date_to`, `updated`, `page`, `pagesize`.
- **Block Trades** — `GET /api/v1/signal/block_trade`.
- **Halt / Resume** — `GET /api/v1/signal/halt_resume`.
- **Squawk audio / breaking-news signal** — delivered via the Squawk SDK / streaming (not a REST endpoint; see §4 Squawk SDKs and the WebSocket/TCP layer below).

**Analyst / sentiment intelligence**:
- **Analyst Insights** — `GET /api/v1/analyst/insights` — action, rating, price target, analyst_id.
- **Consensus Ratings** — `GET /api/v1/consensus-ratings` — consensus_rating, consensus_price_target.
- **Bulls Say / Bears Say** — `GET /api/v1/bulls_bears_say` — bull_case, bear_case; param `ticker`.
- **ERX Gaps (earnings-reaction gaps)** — `GET /api/v1/erx_gaps`.

**Government / regulatory**:
- **Congressional / Government Trades** — `GET /api/v1/gov/usa/congress/trades` and `…/trades/reports`.
- **Insider Transactions (SEC Form 4)** — `GET /api/v1/sec/insider_transactions/{view_type}` where view_type ∈ `filings` | `transactions`; plus `…/insider_transactions/owners`.

**Fundamentals** (base `https://data-api.benzinga.com/rest/v3/fundamentals`; common params `symbols`/`company_tickers`, `isin`, `cik`, `date_asof`):
- `fundamentals` (root), `financials` (params `period`, `reporttype`), `valuationRatios`, `earningRatios`, `operationRatios`, `shareClass`, `shareClassProfile`, `earningReports`, `alphaBeta`, `companyProfile`, `company`, `assetClassification`, plus docs-listed `balance-sheet`, `income-statement`, `cash-flow`, `share-price-ratios`, `derived`.

**Market data**:
- **Bars / OHLCV** — `GET https://api.benzinga.com/api/v2/bars` — params `symbols`, `from`, `to`, `interval` (e.g. 1MONTH, 1D, 5M), `session`. Also SDK `chart` at `api.benzinga.io/dataapi/rest/v2/chart`.
- **Quote** — `GET https://data-api.benzinga.com/rest/v2/quote` (real-time, licensed).
- **Delayed Quote** — `GET https://api.benzinga.com/api/v1/quoteDelayed` — params `symbols`, `isin`, `cik`.
- **Batch price history** — `GET https://data-api.benzinga.com/rest/v2/batchhistory`.
- **Movers (gainers/losers)** — `GET https://data-api.benzinga.com/rest/movers` — params `session` (REGULAR/PRE_MARKET/AFTER_MARKET), `period_from`, `period_to`, `maxResults`, `marketCap` gt/lt, `close_gt`, `sector`, `industry`.
- **Instruments (screener)** — `GET https://data-api.benzinga.com/rest/v3/instruments` — params market_cap gt/lt, close_gt, sector, sort_field/sort_dir, date range.
- **Short Interest** — `market-data/get-short-interest-data` — short volume, days-to-cover.
- **Ticker Detail** — `GET https://data-api.benzinga.com/rest/v3/tickerDetail`.
- **Autocomplete / ticker search** — `GET https://data-api.benzinga.com/rest/v2/autocomplete` — params `query`, `limit`, `search_method`, `exchanges`, `types`.
- **Logos** — `GET https://api.benzinga.com/api/v1.1/logos` — params `symbols`, `filters` (search vs sync variants in docs: logos-api search / sync).
- **Security lookup** — `GET https://api.benzinga.io/dataapi/rest/v2/security` — params `symbols`, `cusip`.

**Ticker Trends / market sentiment** — `ticker-trends-api`: `get-ticker-trend-data` and `get-ticker-trend-list-data` (trend scores, ranked trending securities).

**Transcripts (earnings calls)** — Delivery API: `GET /api/v1/transcripts/calls` (filter `symbol`, `status`, date range, `pageSize`), `GET /api/v1/transcripts/calls/{call_id}`, `GET /api/v1/transcripts/summaries`, `GET /api/v1/transcripts/summaries/{call_id}` (summary text + sentiment).

**Streaming / push** (not REST — for a CLI these are optional `stream`/`watch` subcommands):
- **WebSocket** (`ws-reference`): news-stream, calendar-earnings-stream, calendar-ratings-stream, consensus-ratings-stream, analyst-insights-stream, bulls-bears-say-stream, transcripts-stream. AsyncAPI specs published.
- **TCP streaming** (`tcp-reference`): persistent TCP feed for news/market data; official Go client `go-bztcp` and Python client `python-bztcp`.
- **Webhooks** (`webhook-reference`): filterable push of calendar/signal/sentiment events; `test-webhook-delivery` endpoint.
- **Widgets** (`widgets`): embeddable visualizations (out of scope for a CLI).

Sources: [docs.benzinga.com/llms.txt](https://docs.benzinga.com/llms.txt), [docs.benzinga.com/llms-full.txt](https://docs.benzinga.com/llms-full.txt), [Benzinga/benzinga-python-client financial_data.py](https://github.com/Benzinga/benzinga-python-client/blob/master/benzinga/financial_data.py), [calendar docs (docs.benzinga.io)](https://docs.benzinga.io/benzinga/calendar-v2.html).

---

## 2. Auth Model

- **Mechanism:** static API token (key). No OAuth, no refresh.
- **Primary param:** **`token`** as a URL query parameter, e.g. `https://api.benzinga.com/api/v2/news?token=YOUR_KEY&pageSize=1`. Confirmed by docs authentication section and every official SDK call.
- **Header variant:** the docs also document an `Authorization: token <YOUR_API_KEY>` header (note the literal word `token` prefix, not `Bearer`), recommended for production. Source: [docs.benzinga.com/llms-full.txt](https://docs.benzinga.com/llms-full.txt) (Authentication section).
- **Tier-gating:** products are separately licensed. A valid token can still get **403 Forbidden** on an endpoint not included in the plan (vs **401 Unauthorized** for an invalid/expired token). Error envelope: `{"ok": false, "errors": [{"code": "...", "id": "...", "value": "..."}]}` with codes like `auth_failed`, `bad_request`, `no_data_found`, `internal_server_error`, `upstream_api_error`.
- **Env-var convention:** community/tooling convention is **`BENZINGA_API_KEY`** (used by OpenBB's `openbb-benzinga` provider and most wrappers). Some scripts use `BZ_API_KEY` / `BENZINGA_TOKEN`. A CLI should accept `BENZINGA_API_KEY` as the canonical env var and pass it as `?token=`.

---

## 3. Reachability / Risk

- **Reachable programmatically:** yes. Plain HTTPS GET with `?token=` works; no signing, no handshake. This makes it trivially scriptable from a CLI.
- **Almost everything is paid / licensed.** There is no documented free public tier; you need a key issued by Benzinga sales, and each product family is licensed separately. A keyless user gets nothing useful. **Logos** is sometimes bundled cheaply, but assume all endpoints require a paid token. (No confirmed keyless/free endpoint found.)
- **Base-URL drift is the main risk.** Three hosts coexist — `api.benzinga.com` (primary), `data-api.benzinga.com` (fundamentals/quote/movers/autocomplete), and the older `api.benzinga.io/dataapi` (security/chart). The Python SDK mixes all three. A CLI must hardcode the correct host per endpoint rather than assume one base URL.
- **Version drift:** SDK uses `/api/v2/calendar/*`; docs publish `/api/v2.1` and `/api/v2.2`. Both resolve today but a CLI should prefer the documented v2.x.
- **GitHub issue signal:** the official `benzinga-python-client` is low-traffic (≈31 stars, 17 forks, ~4 open issues; last updated March 2026). Open issues are mostly install/usage (e.g. issue #16 about `pip install git+ssh://…`), **not** systemic 401/deprecation reports. No widespread "endpoints broke" thread surfaced. The `api-docs` repo is **archived (2017)**; current docs live in `benzinga-docs` (MDX, updated June 2026) → the docs site is the source of truth, the old repo is stale.
- **No rate-limit numbers are publicly documented;** the docs emphasize narrowing queries (use `parameters[updated]` deltas, `tickers`, `date`) to stay performant — implying server-side throttling exists. Treat unknown limits as a risk; build polite pagination + delta-sync.

Sources: [benzinga-python-client repo](https://github.com/Benzinga/benzinga-python-client), [Benzinga org repos](https://github.com/orgs/Benzinga/repositories), [docs.benzinga.com/llms-full.txt](https://docs.benzinga.com/llms-full.txt).

---

## 4. Competing / Absorbable Tools (for the absorb manifest)

### 4a. `benzinga` — official PyPI Python client (PRIMARY absorb target)
- **URL:** https://pypi.org/project/benzinga/ · source https://github.com/Benzinga/benzinga-python-client
- **Language:** Python 3.x · **License:** MIT · **Stars:** ~31 · last PyPI release v1.21 (the repo itself updated through 2026).
- **Why it matters:** this is the de-facto reference for real endpoint URLs and param names. A best-in-class CLI should match/exceed its entire surface. Two modules:

**`financial_data.Benzinga(api_key, log=True)` — methods (ground truth from source):**
- Quotes/prices: `price_history(tickers, date_from, date_to)` → `data-api…/rest/v2/batchhistory`; `quote(tickers)` → `data-api…/rest/v2/quote`; `delayed_quote(tickers, isin, cik)` → `api…/api/v1/quoteDelayed`; `security(tickers, cusip)` → `api.benzinga.io/dataapi/rest/v2/security`.
- Charts/bars: `chart(...)` → `api.benzinga.io/dataapi/rest/v2/chart`; `bars(...)` → `api…/api/v2/bars`.
- Search/reference: `auto_complete(tickers, limit, search_method, exchanges, types)` → `…/rest/v2/autocomplete`; `ticker_detail(tickers)` → `…/rest/v3/tickerDetail`; `instruments(...)` → `…/rest/v3/instruments`.
- Movers/logos: `movers(session, period_from, period_to, max_results, market_cap_gt/lt, close_gt, sector, industry)` → `…/rest/movers`; `logos(tickers, filters)` → `api…/api/v1.1/logos`.
- Calendar: `dividends`, `earnings`, `splits`, `economics`, `guidance`, `ipo`, `retail`, `ratings`, `conference_calls` — all `api…/api/v2/calendar/<name>` with `page, pagesize, date_asof, date_from, date_to, company_tickers, importance, date_sort, updated_params` (plus `action` for ratings, `country` for economics, `div_yield`/`div_yield_operation` for dividends).
- Fundamentals: `fundamentals`, `financials(period, reporttype)`, `valuation_ratios`, `earning_ratios`, `operation_ratios`, `share_class`, `share_class_profile`, `earning_reports`, `alpha_beta`, `company_profile`, `company`, `asset_classification` — all `data-api…/rest/v3/fundamentals/<name>` with `(tickers, isin, cik, date_asof)`.
- Ownership: `summary(...)` → `…/rest/v3/ownership/summary`.
- Options: `options_activity(tickers, date, date_from, date_to, page, pagesize, updated)` → `api…/api/v1/signal/option_activity`.
- Utility: `output(json)` pretty-prints.

**`news_data.News(api_key)` — methods:** only `news(pagesize, page, display_output, base_date, date_from, date_to, last_id, updated_since, publish_since, company_tickers, channel)` is publicly implemented → `http://api.benzinga.com/api/v2/news/`. (Snake→camel mapping: `pagesize→pageSize`, `display_output→displayOutput`, `base_date→date`, `date_from→dateFrom`, `date_to→dateTo`, `last_id→lastId`, `updated_since→updatedSince`, `publish_since→publishedSince`, `company_tickers→tickers`, `channel→channels`.) The internal `__url_call` dict references `top_news`/`channels`/`quantified` resource types but no public wrappers ship for them — a gap a new CLI can fill.

### 4b. `@benzinga/*` — official npm scope (JS/TS)
Source: [npmjs.com search](https://www.npmjs.com/search?q=benzinga), [Benzinga org](https://github.com/orgs/Benzinga/repositories). These are mostly **session/infra plumbing**, not a data-API SDK:
- `@benzinga/session`, `@benzinga/session-context` — session/manager framework (LoggingManager etc.).
- `@benzinga/safe-await` — error-safe promise wrapper.
- `@benzinga/benzinga-squawk-sdk` — **Squawk audio** SDK (real-time breaking-news audio feed). The most data-relevant npm package.
- Tooling: `babel-preset-benzinga-webpack`, ESLint/TSLint configs.
- `benzinga-javascript-client` (TypeScript, GitHub, last updated 2023) — the JS analog of the python client.
- `squawk-sdk-js` / `benzinga-squawk-client` — Squawk protocol clients (updated 2026).
**Feature takeaway for a CLI:** the only unique capability here vs the Python client is **Squawk real-time audio streaming** — a niche `squawk` subcommand candidate, but it's audio/websocket, not REST.

### 4c. `openbb-benzinga` — OpenBB Platform provider extension
- **URL:** https://pypi.org/project/openbb-benzinga/ · **License:** AGPL-3.0 · latest v1.6.1 (May 2026), Python 3.10–3.14.
- Integrates Benzinga as a data provider inside OpenBB. Exposes Benzinga-backed models for **analyst price targets, analyst ratings/search, company news, and world news** (OpenBB's standard fetcher set). Uses `BENZINGA_API_KEY`. The full model list isn't on the PyPI page (it's in the OpenBB monorepo `openbb_platform/providers/benzinga`); confirmed surface centers on news + analyst data. Good prior art for env-var naming and for which Benzinga endpoints downstream users actually want.

### 4d. Official streaming clients
- **`go-bztcp`** — https://github.com/Benzinga/go-bztcp — Benzinga TCP client in Go, ISC license (last pushed 2021). Pure-Go TCP news feed client.
- **`python-bztcp`** — https://github.com/Benzinga/python-bztcp — Python TCP client, ISC (2021).
- Both implement the persistent-TCP news protocol (auth + JSON message frames). Relevant only if the CLI adds a streaming `tail`/`watch` mode.

### 4e. MCP servers
- **No dedicated Benzinga MCP server exists** (confirmed via multiple searches of GitHub, npm, and MCP registries as of June 2026). Adjacent finance MCP servers exist (Financial Modeling Prep, Yahoo Finance, finance-tools-mcp) but none wrap Benzinga. **This is whitespace** — shipping a Benzinga CLI with an MCP wrapper would be the first of its kind. No ground-truth MCP source to extract; the Python client (§4a) is the authoritative endpoint map instead.

### 4f. Claude skills / plugins / automation
- No public Claude skill or plugin specifically targeting Benzinga was found. Some n8n/Pipedream/Zapier connectors reference Benzinga ([Pipedream Benzinga integrations](https://pipedream.com/apps/benzinga)) but expose only thin news/calendar triggers, not a full surface. `dltHub` publishes a `benzinga-bars` dlt source for the bars endpoint ([dlthub.com/context/source/benzinga-bars](https://dlthub.com/context/source/benzinga-bars)). The partner **Massive** mirrors Benzinga's News REST docs ([massive.com/docs/rest/partners/benzinga/news](https://massive.com/docs/rest/partners/benzinga/news)).

**Net:** the only comprehensive prior art is the official `benzinga` Python client. To beat the field, a CLI must (1) cover everything that client covers, (2) add the news `channels`/`top_news`/WIIM surface it omits, (3) add an MCP wrapper (none exists), and (4) add offline persistence + compound queries.

---

## 5. Data Layer Candidates (local SQLite)

Highest-gravity entities to persist, with their natural sync cursor and FTS fields:

| Entity | Endpoint | Sync cursor | FTS / index fields |
|---|---|---|---|
| **News stories** | `/api/v2/news` | `updatedSince` (Unix ts) or `lastId` — the API explicitly recommends delta-sync by `updated` | title, teaser, body (strip HTML), channels, tags, ticker symbols |
| **Analyst ratings** | `/api/v2.1/calendar/ratings` | `parameters[updated]` + `date` | ticker, analyst, firm, action, rating_current, pt_current |
| **Earnings rows** | `/api/v2.1/calendar/earnings` | `parameters[updated]` + `date` | ticker, period, eps/eps_est, revenue/revenue_est |
| **Economic events** | `/api/v2.1/calendar/economics` | `parameters[updated]` + `date` | event_name, country, actual/consensus/prior |
| **Dividends** | `/api/v2.2/calendar/dividends` | `parameters[updated]` | ticker, ex_date, amount, yield |
| **Government (congress) trades** | `/api/v1/gov/usa/congress/trades` | report/transaction date | filer, ticker, transaction_type, amount |
| **Insider transactions** | `/api/v1/sec/insider_transactions/transactions` | filing date | ticker, owner, transaction_code, shares |
| **Unusual options activity** | `/api/v1/signal/option_activity` | `updated` + `date` | ticker, sentiment, put_call, strike |
| **WIIMs** | `news-api/wiims` | updated/date | ticker, why-text |
| **Splits / IPOs / Guidance / M&A / FDA** | respective calendar paths | `parameters[updated]` | ticker, date, type-specific fields |

**Design notes:** Every calendar family shares the `parameters[updated]` delta cursor and a `(date, ticker)` natural key, so a single generic `sync(family, since)` routine covers all of them — this is the core of an offline store. News is the strongest FTS candidate (full body HTML → FTS5 over stripped text, joined to a `story_tickers` table). The `calendar-removed` and `news/removed` endpoints exist precisely to reconcile deletions during incremental sync — a quality CLI should call them to keep the local store honest.

---

## 6. User Pain Points & Workflows

**Who uses it:** quant/algo traders, fintech apps and brokerages embedding news + calendars, financial-news aggregators, hedge-fund/retail-research desks, and trading-bot builders who need low-latency analyst/ratings/earnings data with reliable tickers.

**Top power-user workflows:**
1. **Watchlist rating-change scan** — "show today's analyst upgrades/downgrades + price-target changes for my 40 tickers" via `/calendar/ratings?parameters[tickers]=…&parameters[date_from]=today`.
2. **Earnings-season tracker** — pull `/calendar/earnings` for the week, join estimates vs actuals, flag surprises; pair with `/calendar/conference-calls` and transcripts.
3. **Weekly economic calendar** — `/calendar/economics?country=US` for the week, sorted by importance, actual-vs-consensus.
4. **Breaking-news stream filtered by ticker/channel** — poll `/api/v2/news?updatedSince=…&tickers=…&channels=…` (or the WebSocket/TCP/Squawk feeds) to drive alerts.
5. **Unusual-activity + WIIM scanner** — combine `/signal/option_activity`, block trades, and WIIMs to answer "why is X moving right now."

**Concrete pain points:**
- **Base-URL/version fragmentation** (three hosts, v2 vs v2.1 vs v2.2) makes hand-rolling requests error-prone — the #1 thing a CLI abstracts away.
- **No free tier + opaque per-endpoint licensing** → 403s that look like bugs; users can't tell "broken" from "not licensed." A CLI should surface the 403 plan-gating message clearly.
- **Thin official tooling:** the Python client is the only full wrapper, ships no CLI, omits parts of the news surface (channels/top_news), pretty-prints JSON but offers no filtering/persistence; the npm scope is session-infra, not data. No MCP server at all. Users end up writing ad-hoc `requests` scripts and re-implementing pagination + delta-sync every time.
- **Undocumented rate limits** force defensive pagination.

---

## 7. Product Thesis

**Name:** **`benzinga`** (binary `bz`), "the agent-native Benzinga terminal."

**Thesis:** Benzinga ships a licensed, high-value financial-data API spread across three base URLs, three version prefixes, and ~40 endpoint families — but its only complete client is a low-traffic Python library with no CLI, no offline store, an incomplete news surface, and **zero MCP presence**. `bz` collapses that fragmentation into one coherent command surface: every calendar, news, fundamentals, signal, and analyst endpoint as a first-class subcommand with the correct host/version baked in; a built-in SQLite cache that delta-syncs via the API's own `parameters[updated]`/`updatedSince` cursors (and reconciles deletions through `calendar-removed`/`news/removed`), giving instant offline FTS5 search over news bodies and compound cross-entity queries ("ratings changes AND unusual options on my watchlist this week") that the REST API can't express in one call; agent-native JSON/JSONL output plus a bundled MCP server — the first for Benzinga — so Claude and other agents can query Benzinga directly. It matches the official Python client's entire endpoint map, fills the gaps it leaves (news channels/top-news/WIIM, transcripts, gov/insider trades), and adds the persistence + composition + agent layer no incumbent offers — beating them not on data access (everyone hits the same token-gated REST) but on ergonomics, offline gravity, and being the only agent-ready front door to Benzinga.

---

### Appendix — Key sources
- Docs index: https://docs.benzinga.com/llms.txt · Full text: https://docs.benzinga.com/llms-full.txt · Home: https://docs.benzinga.com/home
- Calendar (legacy mirror): https://docs.benzinga.io/benzinga/calendar-v2.html
- Official Python client: https://github.com/Benzinga/benzinga-python-client (`financial_data.py`, `news_data.py`) · PyPI: https://pypi.org/project/benzinga/
- Benzinga GitHub org repos: https://github.com/orgs/Benzinga/repositories (benzinga-javascript-client, go-bztcp, python-bztcp, benzinga-squawk-client, squawk-sdk-js, benzinga-docs)
- npm scope: https://www.npmjs.com/package/@benzinga/benzinga-squawk-sdk and related `@benzinga/*`
- OpenBB provider: https://pypi.org/project/openbb-benzinga/
- Partner mirror (News REST): https://massive.com/docs/rest/partners/benzinga/news
- dlt bars source: https://dlthub.com/context/source/benzinga-bars
</content>
</invoke>
