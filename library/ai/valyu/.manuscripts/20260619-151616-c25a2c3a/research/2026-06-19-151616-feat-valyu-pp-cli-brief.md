# Valyu CLI Brief

## API Identity
- Domain: AI-native search and research for LLMs and autonomous agents
- Users: AI agent developers, RAG system builders, researchers in finance/biotech/academia, enterprises building copilots
- Data profile: web search, 36+ proprietary sources (arXiv, PubMed, SEC filings, USPTO patents, FRED, ChEMBL, clinical trials, financial data)

## Reachability Risk
- Low — official REST API at api.valyu.ai, well-documented, $10 free credits on signup
- API Key required (X-API-Key header), VALYU_API_KEY env var
- No known 403/blocking issues

## Top Workflows
1. **Semantic search across 40+ sources** — `POST /v1/search` with source filtering (web, proprietary, or all), date ranges, country code, result length
2. **AI-synthesized answers with attribution** — `POST /v1/answer` with SSE streaming, structured output schemas, system instructions
3. **Deep multi-step research reports** — `POST /v1/deepresearch/tasks` with modes (fast/standard/heavy), output in markdown/PDF/JSON, async polling
4. **URL content extraction** — `POST /v1/contents` batch up to 10 URLs, async webhook, AI summarization
5. **Research workflows** — `GET/POST /v1/workflows` templated pipelines with versioning and preview

## Table Stakes
- Source filtering (include/exclude specific data sources by name)
- Date range filtering for time-sensitive queries
- Result count control (max_num_results)
- Search type selection (web / proprietary / all / news)
- Cost tracking per request (max_price cap)
- Async job polling with status checks
- Streaming support for Answer API (SSE)
- Discover available datasources

## Data Layer
- Primary entities: SearchResult (title, content, url, source, date), ResearchTask (id, status, query, mode, output_formats, result), ContentJob (id, status, url, content), Workflow (slug, version, parameters)
- Sync cursor: task IDs for deepresearch polling, job IDs for async contents
- FTS/search: local cache of search results with FTS for offline re-querying and dedup

## Codebase Intelligence
- Source: valyuAI/valyu-mcp + valyuAI/valyu-py
- Auth: X-API-Key header, env var VALYU_API_KEY; SDK: `Valyu(api_key=VALYU_API_KEY)`
- Data model: SearchResult with title/content/url/source; ResearchTask with id/status/query/mode
- Rate limiting: No hard limits, pay-per-use with per-query max_price cap
- Architecture: REST POST endpoints for search/answer/research; GET endpoints for polling/listing; OpenAPI 3.0.3 spec at docs.valyu.ai/api-reference/openapi.json

## Product Thesis
- Name: valyu-pp-cli
- Why it should exist: The Valyu Python/JS SDKs exist but there's no general-purpose CLI for shell scripting, agent pipelines, or offline analysis of cached research results. A CLI enables: shell composability (pipe to jq), local SQLite caching of expensive research results, batch research scripts, and agent-native `--json` output for LLM agents that need to invoke Valyu without a Python/Node runtime.

## Build Priorities
1. `search` — core semantic search with all filters, `--json`, `--select`, source include/exclude, date ranges
2. `answer` — AI-synthesized answers with streaming support
3. `research` (deepresearch) — create/poll/list/cancel async research tasks, save output to file
4. `extract` (contents) — batch URL extraction with async polling
5. `datasources` — list available sources with categories and pricing info
6. `workflows` — CRUD for research workflow templates
7. `sync` — cache search results locally for offline FTS re-use
