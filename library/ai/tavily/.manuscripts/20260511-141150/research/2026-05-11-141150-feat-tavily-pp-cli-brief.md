# Tavily CLI Brief

## API Identity
- Domain: AI-native web search, content extraction, site crawling, URL mapping, deep research
- Users: AI/LLM developers, RAG builders, data researchers, content aggregators, AI agent builders
- Data profile: Search results (title, URL, content, score, images), extracted page content, sitemap URLs, research reports with citations. All responses include request_id and response_time. Credit-based billing.
- Owner: Tavily AI (acquired by Nebius, Feb 2026)

## Reachability Risk
- None. API is well-documented, public, bearer-token authenticated. Only 4 open issues on tavily-python (1.2k stars), none about 403/blocked/broken endpoints. API returned 200 on usage endpoint during preflight.

## Top Workflows
1. **Search + RAG context** — Search for a topic, get clean content chunks for LLM consumption
2. **Content extraction** — Pull clean markdown/text from specific URLs for analysis or ingestion
3. **Site crawling** — Systematically extract all content from a website section
4. **Site mapping** — Discover URL structure before targeted extraction
5. **Deep research** — Multi-source investigation with citations (mini/pro models, streaming)

## Table Stakes
- Web search with domain filtering, date ranges, topic categories
- Content extraction from URLs (single and batch)
- Site crawling with depth/breadth controls
- URL mapping/sitemap discovery
- Research reports with streaming and structured output
- Credit usage tracking
- All output in JSON
- API key management (enterprise)

## Data Layer
- Primary entities: SearchResult (query + results + answer), ExtractedPage (url + content), CrawlResult (base_url + pages), MapResult (base_url + urls), ResearchReport (input + report + sources), UsageSnapshot (credits by endpoint)
- Sync cursor: Usage endpoint provides current-cycle snapshot; search/extract/crawl/map are stateless queries
- FTS/search: Offline full-text search across cached search results and extracted content

## Competitors
1. **tavily-cli (official)** — Python CLI by Tavily (tvly command). Covers search, extract, crawl, map, research. JSON output, file output. No offline storage, no credit tracking history, no compound workflows.
2. **tavily-mcp (official)** — MCP server with tavily-search, tavily-extract, tavily-map, tavily-crawl tools. Remote and local modes. No offline, no research endpoint.
3. **@tavily/core (npm)** — Official JS SDK. Full API coverage. Library, not CLI.
4. **tavily-python (PyPI)** — Official Python SDK. Full API coverage including get_search_context() and qna_search() convenience methods. Library, not CLI.

## Product Thesis
- Name: tavily-pp-cli
- Why it should exist: The official tvly CLI is a thin API wrapper with no local persistence, no credit tracking over time, no offline search across past results, no compound workflows (search → extract → store). A Go CLI with SQLite persistence turns Tavily from a stateless API into a local research database that compounds over time.

## Build Priorities
1. Full API coverage: search, extract, crawl, map, research, usage, enterprise key management
2. SQLite persistence for all search results, extracted content, crawl outputs, research reports
3. Offline FTS search across all cached data
4. Credit usage tracking with history and budgeting
5. Compound workflows: search → extract top results → store
6. Agent-native output (--json, --select, --compact, --csv)
7. Research streaming with progress display
