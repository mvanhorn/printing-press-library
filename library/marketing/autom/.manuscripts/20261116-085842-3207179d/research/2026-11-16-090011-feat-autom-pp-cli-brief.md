# Autom CLI Brief

## API Identity
- Domain: real-time search engine results (SERP) API. One key reaches Google (Search, Search Light, Autocomplete, News, Images, Videos, Shopping, Maps, Resolve), Bing Search, Brave (Search, Images, Videos), DuckDuckGo, Yahoo, and a multi-engine fallback `GET /v1/search`.
- Users: SEO and rank-tracking teams, growth and market-research analysts, AI agent builders who need grounded web results, developers replacing SerpApi/Serper.
- Data profile: read-only, query-shaped. No list or collection endpoints: every call is a search keyed by query, engine, location, language and page. Free account endpoints: `GET /v1/usage` (quota, packs, credits, rate limits, key details) and `GET /v1/stats` (per-engine infrastructure health, last hour, needs `engine` or `path`). Free finders: `/v1/finder/google-countries`, `google-languages`, `google-locations` (reject an empty `query`, cap at 100 results, 404 on `page`). Async jobs: `is_async=true` returns 202 + `job_id`, polled free on `GET /v1/jobs/{id}`.
- Homepage: https://autom.dev (evidence: docs header links "Website", "Pricing", "Dashboard"; the OpenAPI is served at https://app.autom.dev/openapi).

## Reachability Risk
- Low. Official API with a published OpenAPI 3.1 spec (spec source: `webapi-integrations/packages/printing-press/autom-openapi.json`, trimmed copy of https://app.autom.dev/openapi: GET only plus `POST /v1/google/resolve`, without the failing `/v1/google/jobs` and `/v1/google/search/ai`).
- Known server-side issues seen in live tests on 2026-11-11 and 2026-11-12 (to handle as errors, not to paper over): `GET /v1/bing/search` returns 500 when `page` is passed; Google Images, Shopping and Videos returned 500 for `query=coffee`; `/v1/search` sometimes answers 404 "Not Found" (no engine had a result) while single engines answer the same query; `/v1/stats` requires `engine` or `path` although the spec marks both optional; the google-locations finder error message names a `search` parameter instead of `query`.
- Rate limits answer HTTP 401 with a JSON "Rate limit exceeded" body (not 429). The client must classify 401 + "rate limit" as a retryable rate limit, and a plain 401 as a bad key.
- Probe-safe endpoint: `GET /v1/usage` (free, never billed).

## Platform facts (docs.autom.dev, fetched raw 2026-11-16)
- Auth: `x-api-key` header (the `x_api_key` query parameter is deprecated). Env var: `AUTOM_API_KEY`.
- Billing: sync 200 billed; async 202 not billed until the job completes; `/v1/jobs/{id}` polling free; failed jobs (500) free; other 4xx/5xx free; 402 = out of credits. Balances drawn in order: subscription quota, one-time packs, credits.
- Timeouts: hard 60 s server limit; typical 2-3 s, up to 30 s. Client timeout must be at least 60 s; longer work goes async.
- Global parameters on every endpoint: `soft_fail`, `is_async`, `fields` (comma list, dot notation for nested fields; shrinks payloads).
- Official MCP server at https://mcp.autom.dev/ (Streamable HTTP, same operations as REST). Official SDKs and integrations exist (LangChain, LlamaIndex, Haystack, crewAI, Dify, n8n, etc.), but no CLI.

## Competitors
- SerpApi (https://serpapi.com): multi-engine SERP API with an official MCP server and Claude plugin; rank-tracking tooling built on it (seo-position-tracker, SerpTrail).
- Serper (https://serper.dev): Google SERP API.
- Serply (https://serply.io): SERP API with a printed CLI in the public library (`rank`, `serp diff`, `research`).

## Top Workflows
1. Rank tracking: where does my domain rank for a keyword, by country and language, and how did it move since last time.
2. SERP snapshots and diffs: save a results page, replay it offline, diff two snapshots (entered, left, moved).
3. Cross-engine comparison: same query on Google, Bing, Brave, DuckDuckGo, Yahoo; overlap and unique domains.
4. Grounding for agents: one deduplicated, cited source list for a question across engines and news.
5. Monitoring: news watch for a brand or keyword with dedupe against the local store; batch runs from a keyword file with a credit budget.

## Table Stakes
- One command per engine endpoint with every documented parameter (location, gl/hl/lr, num, page, safe, time range, device).
- JSON output, `--select`/`fields` projection, `--dry-run` showing the request, async mode with job polling.
- Credit visibility before a batch (`usage`), clear errors for 401 key vs 401 rate limit, 402 out of credits.
- Finder lookups for valid countries, languages and locations.

## Data Layer
- Primary entities: SERP snapshots (query, engine, params, timestamp, organic results with position, URL, domain, title, snippet), news articles, credit usage samples.
- Sync cursor: none from the API (no list endpoint). The store is fed by searches the user runs (`--save` / snapshot commands), not by a sync crawl.
- FTS/search: full-text over saved result titles and snippets; queries by domain and query for rank history.

## Product Thesis
- Name: autom-pp-cli
- Why it should exist: Autom has an MCP server and SDKs but no CLI. Serply's printed CLI (peer SERP API) shows the bar: rank, serp diff, research. Autom covers more engines (Google family, Bing, Brave, DuckDuckGo, Yahoo, multi-engine fallback), so the CLI can go further on cross-engine comparison, geo comparison and credit-aware batching, with a local SQLite history that turns one-off searches into rank tracking.

## Build Priorities
1. Every spec endpoint as a typed command, with `fields`, `is_async` + job polling, and correct 401/402/429 classification.
2. Rank check, history and movers backed by saved snapshots; `serp replay` and `serp diff`.
3. `engines compare` (overlap across engines) and `geo compare` (same query across countries).
4. `ground` (cited, deduplicated sources across engines and news) and `news watch` (dedupe against the store).
5. `batch run` from a keyword file with a credit budget checked against `/v1/usage`; `credits` summary.
