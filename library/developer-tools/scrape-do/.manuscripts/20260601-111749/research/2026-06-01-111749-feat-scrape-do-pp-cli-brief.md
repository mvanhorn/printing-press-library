# Scrape.do CLI Brief

## API Identity
- **Domain:** Web-scraping proxy API. One token unlocks proxy rotation, headless JS rendering, anti-bot bypass, geo-targeting, and a family of pre-parsed "Ready API" scrapers (Google, Amazon, YouTube, ChatGPT).
- **Base URL:** `https://api.scrape.do/` — auth via `token` **query param**. Async API at `https://q.scrape.do` (header `X-Token`).
- **Users:** Engineers and AI agents extracting public web data at scale. This user runs **multiple AI agents concurrently** against one Scrape.do account.
- **Data profile:** Per-request scrape results (HTML / markdown / JSON / screenshot), structured Google SERP records, and account usage/credit/concurrency state. High-gravity entities: **SERP results** (organic/ads/PAA/AI-overview), **scrape jobs**, **account usage snapshots**.

## Reachability Risk
- **None.** Scrape.do is an actively marketed (2026) scraping proxy; reachable by design. No GitHub issues report it blocked/broken/deprecated (the official wrapper repos have ~0 issues — absence of signal, not proven reliability). Token available → live testing on.

## User Vision (from briefing)
- **Primary workflow: Google Search scraping** — `/plugin/google/search` is where most of the user's work happens.
- Will use the **majority of endpoints** → broad coverage required, not just a thin scrape wrapper.
- **HEADLINE REQUIREMENT — credit-budget + concurrency-limit governor.** Multiple AI agents must share one Scrape.do account concurrently **without** exceeding the plan's concurrent-request cap or burning monthly credits. The CLI must track both live, locally, and gate requests against the limits. This is the differentiating feature, not a nice-to-have.

## Top Workflows
1. **Scrape a Google SERP and get clean structured JSON** — `/plugin/google/search?q=...` → organic/ads/PAA/knowledge-graph/local/AI-overview. Store rows offline for rank tracking.
2. **General page scrape with the right knobs** — `GET /?url=...` with `render`/`super`/`geoCode`/`output=markdown`, returning HTML/markdown/JSON/screenshot.
3. **Govern concurrent multi-agent usage** — never exceed `ConcurrentRequest`; track `RemainingMonthlyRequest`; estimate cost before spending; debit actual cost from the `Scrape.do-Request-Cost` header.
4. **Track SERP rank drift over time** — re-run a query, diff against the last stored SERP, surface position changes — all offline, no re-spend.
5. **Batch / fan-out scraping** — submit many URLs/queries with a concurrency-respecting worker pool and auto-retry on 429/502.

## Table Stakes (competitor parity to absorb)
- Structured Google SERP JSON parity with **SerpApi** (organic w/ position, ads, PAA, knowledge_graph, answer_box, local_results, related_searches, pagination via `start`).
- JS render, residential/mobile proxy, geo + regional targeting, device emulation, sticky sessions, markdown output, screenshots, browser-interaction scripts (Bright Data / ScrapingBee parity).
- Batch URL submission + async jobs + webhook delivery (ScraperAPI / Bright Data parity).
- A **budget/usage** command (Bright Data `budget`, ScraperAPI dashboards).
- Built-in auto-retry + backoff (every competitor hides this server-side; we expose + govern it).

## Data Layer
- **Primary entities:**
  - `serp_results` — flattened Google SERP rows (query, engine, position, title, url, snippet, type, geo, device, scraped_at) → powers offline search + rank drift.
  - `serp_snapshots` — full raw SERP JSON per query+params+timestamp → powers diff/rank-drift without re-spend.
  - `scrape_jobs` — every scrape request: url, params, status, cost (from header), bytes, scraped_at, result-cache → powers usage analytics + result reuse.
  - `usage_snapshots` — periodic `/info` captures: concurrent/remaining-concurrent, monthly/remaining credits, captured_at → powers the governor + spend trends.
  - `credit_ledger` — per-request credit debits keyed off `Scrape.do-Request-Cost` → authoritative local spend accounting.
- **Sync cursor:** `scraped_at` / `captured_at` timestamps; SERP snapshots keyed by (query, params-hash).
- **FTS/search:** FTS5 over `serp_results` (title/url/snippet) and over cached scrape bodies.

## Codebase Intelligence (official SDKs)
- `@scrape-do/client` (TS, github.com/scrape-do/node-client): `sendRequest(method, options)`, `statistics()`. Options mirror the query params (super/geoCode/render/sessionId/playWithBrowser). `statistics()` == `/info`.
- `scrapy-scrapedo` (Python, github.com/scrape-do/scrapy-scrapedo): Scrapy middleware; `RequestParameters` with token/geoCode/super/render/playWithBrowser/proxy_mode.
- Both thin and low-traffic → **absorb the HTTP API directly**, credit the SDKs in the README.

## Auth
- **Type:** api_key, in **query**, param name **`token`**.
- **Env var (canonical, user-set):** `SCRAPEDO_API_KEY` — must be the generated CLI's env var (NOT slug-derived `SCRAPE_DO_API_KEY`). Enrich the spec accordingly.
- No login session; no OAuth. Live testing read-only (credits are consumed by every successful call).

## Credit Cost Model (for cost-estimation + governor)
| Mode | Credits |
|---|---|
| datacenter (default) | 1 |
| datacenter + render | 5 |
| super (residential/mobile) | 10 |
| super + render | 25 |
| any `/plugin/google/*` | 10 (super auto-applied) |
| domain overrides | e.g. LinkedIn 30, Shopee 100 |
- **Billing:** only 2xx / 400(target) / 404 / 410 charged. 401 / 429 / 502 / 510 are **free**.
- **Authoritative cost** = `Scrape.do-Request-Cost` response header (debit budget from this, not an estimate).

## Concurrency & Rate Limits (for governor)
- Plan-tiered concurrent requests: Free 5, Hobby 10, Pro 50, Business 100, Advanced 200, Enterprise unlimited.
- Exceed concurrency → **429** (not billed) → back off + retry.
- `/info` (== statistics) is itself capped at **10 req/min** → poll sparingly, cache locally.
- `/info` fields: `IsActive`, `ConcurrentRequest`, `RemainingConcurrentRequest`, `MaxMonthlyRequest`, `RemainingMonthlyRequest`.
- No `X-RateLimit-*` / `Retry-After` headers; the 429 status is the signal.

## Error Codes
401 (no credits / suspended / auth) · 429 (concurrency or `/info` >10/min) · 502 (transient, retry) · 510 (client canceled) · 404 / 410 (charged) · 400 (charged only if target-caused). Repeated bad-token requests → IP throttle.

## Product Thesis
- **Name:** `scrape-do` (binary `scrape-do-pp-cli`), display **Scrape.do**.
- **Why it should exist:** It's the **first** CLI and MCP server for Scrape.do — and the only scraping client that treats credits and concurrency as first-class, locally-governed resources. Multiple AI agents can hammer one account safely: the CLI estimates cost before every call, gates against the live concurrent-request ceiling, debits a local credit ledger from the authoritative cost header, and turns one-shot Google SERPs into a queryable offline history with rank-drift diffing — none of which any Scrape.do tool (or competitor) offers today.

## Build Priorities
1. **Data layer** for all five entities; sync/search/SQL path.
2. **Core scrape** command (full param surface) + **Google SERP** command (primary) with flattened-rows + raw-snapshot persistence.
3. **The governor**: `budget`/`usage` (live `/info` + local ledger), pre-flight `cost` estimator, a concurrency-gated request path, and a multi-agent-safe fan-out/`batch` command honoring the plan's concurrent cap.
4. **Transcendence**: rank-drift diffing, SERP offline search, cost analytics, concurrency-headroom watch — built on the local store.
5. Full Google family + Amazon/YouTube scrapers + async jobs as broad coverage.
