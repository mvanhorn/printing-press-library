# Tavily CLI Brief

## API Identity
- Domain: Web search and data extraction for AI agents and LLM applications
- Users: AI/ML engineers building RAG pipelines, agentic workflows, company intelligence tools, research automation, competitive monitoring
- Data profile: Time-series search results (query+results+timestamp), extracted web content (URL+markdown+timestamp), async research tasks (request_id+status+result), credit usage history
- Auth: Bearer token header `Authorization: Bearer tvly-YOUR_KEY` (or `TAVILY_API_KEY` env var). Key prefix: `tvly-`.

## Reachability Risk
- Low. The API is well-documented, commercially operated, recently acquired by Nebius (Feb 2026), and has an active MCP server on Smithery. Intermittent 502/429 issues documented in gpt-researcher issues under high load but routine use is stable.

## Top Workflows
1. **Agentic RAG search**: `search()` with `search_depth=advanced`, collect top N results, feed `content` into LLM context. Optional: `extract()` the top-scoring URLs for full page text.
2. **Two-step precision extraction**: `search()` to get URLs → `extract()` with `query=` for chunk reranking. Cheaper than `include_raw_content=true` on all results.
3. **Competitive intelligence crawling**: `crawl()` a competitor site with `select_paths` regex, diff results across time, store in SQLite for trend analysis.
4. **Site audit / documentation ingestion**: `map()` to discover URL structure → batch `extract()` targeted pages → save as markdown files for vector ingestion.
5. **Async deep research pipeline**: `POST /research` with `model=pro` and `output_schema` → poll `GET /research/{id}` → receive cited report in structured JSON.

## Table Stakes
- All 5 endpoints: search, extract, crawl, map, research
- `--include-answer` (basic/advanced AI-generated answer)
- `--include-raw-content` (markdown/text full page content)
- `--time-range` and `--start-date`/`--end-date` filters
- `--include-domains` / `--exclude-domains`
- `--json` mode for agent-native output
- Research async polling with status tracking
- Auth via `TAVILY_API_KEY` env var
- Usage stats / credit balance check

## Data Layer
- Primary entities: `searches` (query, results, score, timestamp), `extracts` (url, content, timestamp), `crawl_runs` (base_url, pages, run_id, timestamp), `research_tasks` (request_id, input, status, result, model), `usage_snapshots` (date, credits_used_by_endpoint, plan)
- Sync cursor: `timestamp` on searches/extracts, `run_id` on crawls, `request_id` on research tasks
- FTS/search: Full-text search across cached search results and extracted content (FTS5 on `content` column)
- Local SQLite = cache for deduplication + async research persistence across terminal sessions + usage trend tracking

## Codebase Intelligence
- No public server-side code; clients are in `tavily-ai/tavily-python` (1.2k stars, 11k+ dependent repos) and `tavily-ai/tavily-js` (89 stars)
- Auth pattern: `Authorization: Bearer tvly-<key>` header; env var `TAVILY_API_KEY`
- Data model: All endpoints are POST (except GET `/research/{id}` and GET `/usage`). Results paginated via `max_results` (1-20). Crawl/map/search all return `response_time`, `usage`, `request_id`.
- Rate limiting: 100 RPM for most endpoints (dev), 20 RPM for research. Credit-based billing.
- Architecture: Stateless HTTP API. Research endpoint is async (fire-and-forget POST → poll GET). All other endpoints are synchronous.

## Product Thesis
- Name: `tavily-pp-cli`
- Why it should exist: The official `tvly` Python CLI is thorough but requires Python/pip. The Printing Press Go CLI is a single compiled binary with all 5 endpoints, local SQLite caching to avoid redundant credits, offline FTS search across cached results, and novel compound commands (budget guardian, topic drift detector, research poll daemon, offline corpus builder) that no existing tool provides.

## Build Priorities
1. All 5 core endpoints: search, extract, crawl, map, research (full param coverage)
2. Local SQLite store: search deduplication cache, research task persistence, usage history
3. `qna` command: shortcut for `search` with `include_answer=advanced` and compact output
4. `context` command: outputs a ready-to-paste LLM context string from search results
5. Transcendence: budget guardian (warn before overspending), research poll daemon (background polling), offline search across cached results, topic drift detector
