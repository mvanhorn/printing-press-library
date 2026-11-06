# cloro CLI Brief

## API Identity
- Domain: cloro (cloro.dev), an API that returns the answers of AI search engines and Google results as structured JSON. This CLI covers ChatGPT, Google AI Mode, Gemini, Google Search with the AI Overview, and Google News, plus the async task API.
- Users: SEO and GEO (generative engine optimization) teams that track whether their brand or domain is cited in AI answers; AI-visibility tool builders; agents that need a grounded answer with its sources.
- Data profile: Stateless request and response. Each scrape returns the answer text, the cited sources (`position`, `label`, `url`), citation pills and, for Google, organic results, People Also Ask and the AI Overview. Results are geo-targeted by country (and US state for some engines). The account has a credit balance; each sync scrape costs credits.
- Homepage: https://cloro.dev (the `servers` and `info.contact` of the published spec, and the docs at https://cloro.dev/docs).

## Reachability Risk
- None. Official, published OpenAPI 3.1 spec. `GET /v1/credits` returned HTTP 200 with the operator's key on 2026-11-06.
- Auth: `Authorization: Bearer <key>`. Env var `CLORO_API_KEY`. Keys at https://dashboard.cloro.dev/api-keys. The free plan has 500 credits a month.
- Cost: sync ChatGPT 7 credits, AI Mode 6, Gemini 6, Google 5 (7 with the AI Overview). Async tasks cost 2 credits less. Countries, states, credits and queue status are free.
- Errors: `{success: false, error: {code, message}}`; 401 bad key, 403 `INSUFFICIENT_CREDITS`, 429 over the concurrency limit.

## Top Workflows
1. Check one prompt in one engine: `chatgpt --prompt ... --country US`, read the answer and its sources.
2. Brand or domain citation check: is a domain (for example `nike.com` for "best running shoes for flat feet") cited for a prompt by ChatGPT, AI Mode, Gemini and the Google AI Overview? Today this needs four calls and a manual scan of four source lists.
3. Google results with the AI Overview for a keyword and country, with the organic positions.
4. Google News coverage for a topic and country.
5. Bulk runs through the async API: submit tasks, poll them, read the results (2 credits less per task than sync).

## Table Stakes
- One command per engine with every documented request field (`prompt`, `country`, `state`, `include.*` add-ons), `--json`, `--select`, `--dry-run`, typed exit codes, and an MCP server that exposes each command as a tool.
- Countries and states lookups for geo-targeting; credit balance.
- Async task create, batch, get and queue status.
- Existing tools: cloro's hosted MCP server (`cloro-dev/mcp-server`, tools `scrape_chatgpt`, `scrape_google_ai_mode`, `scrape_gemini`, `scrape_google`, `scrape_google_news`, `list_countries`, `list_states`, `resolve_google_redirect`). It needs an MCP host; there is no cloro CLI. The DataForSEO CLI in this library has an AI-visibility tracker over its own LLM endpoints, but no live AI Mode or Google AI Overview citation check.

## Data Layer
- Primary entities: none that need sync. Scrapes are per request and cost credits, so a background sync would spend credits without a request from the user.
- Sync cursor: none.
- FTS/search: not needed for v1.

## Product Thesis
- Name: cloro-pp-cli
- Why it should exist: cloro is reachable today through raw HTTP or the hosted MCP server. A CLI gives shell users and agents one-line access to each engine with typed flags, and a citation check across engines answers the main GEO question ("does this engine cite my site for this prompt?") in one command.

## Build Priorities
1. Typed commands for ChatGPT, AI Mode, Gemini, Google (with `--include-aioverview`) and Google News, generated from the spec.
2. `cited`: one prompt, several engines in parallel, report per engine whether a domain is in the cited sources and at which positions.
3. Async task commands (generated) for bulk work.

## Reachability Gate
- Decision: PASS
- Evidence: on 2026-11-06, `GET https://api.cloro.dev/v1/countries` without a key returned HTTP 401 with the JSON error envelope (`MISSING_API_KEY`), so the API answers and needs auth. `GET /v1/credits` with the operator's key returned 200.
