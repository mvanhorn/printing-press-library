# Tavily CLI Absorb Manifest

## Sources Analyzed
1. **tavily-cli** (official Python CLI) — search, extract, crawl, map, research commands
2. **tavily-mcp** (official MCP server) — tavily_search, tavily_extract, tavily_crawl, tavily_map, tavily_research tools
3. **tavily-python** (Python SDK, 1.2k stars) — TavilyClient with 8 public methods including get_search_context() and qna_search()
4. **@tavily/core** (npm SDK) — full API coverage for JS/TS

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Web search | tavily-cli search, tavily-mcp | search command | Results cached in SQLite, searchable offline |
| 2 | Advanced search depth | tavily-cli, API docs | search --depth advanced | Cost tracked per query |
| 3 | News topic search | tavily-mcp topic param | search --topic news | Time-filtered with local queries |
| 4 | Domain filtering | tavily-cli, tavily-mcp | search --include-domains, --exclude-domains | Saved domain profiles |
| 5 | Date range filtering | API docs | search --start-date, --end-date, --time-range | Offline date filtering on cached results |
| 6 | AI-generated answers | tavily-python include_answer | search --include-answer | Answers cached alongside results |
| 7 | Raw content inclusion | API docs | search --include-raw-content markdown/text | Full page content stored in SQLite |
| 8 | Image inclusion | tavily-mcp | search --include-images | Images cached locally |
| 9 | Country-specific results | API docs | search --country | Combined with time range for local analysis |
| 10 | Exact match search | API docs | search --exact-match | Precision filtering stored |
| 11 | Auto parameters | API docs | search --auto-parameters | AI-configured search (2 credits) |
| 12 | URL content extraction | tavily-cli extract, tavily-mcp | extract command | Extracted content in SQLite, searchable |
| 13 | Batch URL extraction | tavily-python | extract with multiple URLs | Up to 20 URLs, all stored |
| 14 | Extract with query reranking | API docs | extract --query "..." | Chunks reranked and stored |
| 15 | Extract depth modes | API docs | extract --depth advanced | Cost tracking per extraction |
| 16 | Site crawling | tavily-cli crawl, tavily-mcp | crawl command | Full crawl results in SQLite |
| 17 | Crawl with instructions | API docs | crawl --instructions "..." | Natural language crawl guidance |
| 18 | Crawl depth/breadth control | tavily-mcp | crawl --max-depth, --max-breadth, --limit | Crawl history tracked |
| 19 | Crawl path filtering | API docs | crawl --select-paths, --exclude-paths | Regex URL filtering |
| 20 | URL mapping | tavily-cli map, tavily-mcp | map command | Sitemaps stored in SQLite |
| 21 | Map with instructions | API docs | map --instructions "..." | Guided URL discovery |
| 22 | Deep research | tavily-cli research, tavily-mcp | research command | Reports stored, searchable |
| 23 | Research model selection | API docs | research --model mini/pro/auto | Cost tracked per model |
| 24 | Research streaming | API docs | research --stream | Real-time progress display |
| 25 | Structured research output | API docs | research --output-schema | Custom JSON schema responses |
| 26 | Citation formatting | API docs | research --citation-format apa/mla/chicago | Multiple citation styles |
| 27 | Credit usage | API /usage | usage command | Current cycle usage |
| 28 | Search context for RAG | tavily-python get_search_context() | search --context | Token-budgeted context string |
| 29 | QnA search | tavily-python qna_search() | search --qna | Direct answer extraction |
| 30 | Enterprise key generation | API /generate-keys | keys generate | Key lifecycle management |
| 31 | Enterprise key deactivation | API /deactivate-keys | keys deactivate | Bulk deactivation |
| 32 | Enterprise key info | API /key-info | keys info | Key status checking |
| 33 | Output format control | tavily-mcp format param | extract/crawl --format markdown/text | Consistent format |
| 34 | Favicon inclusion | API docs | search/extract/crawl --include-favicon | Site identity |
| 35 | Safe search | API docs | search --safe-search | Enterprise content filtering |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Credit burn timeline | usage history | 9/10 | Stores daily /usage API snapshots in SQLite usage_snapshots table; queries show per-endpoint burn rate over time with no API equivalent for historical data | Competitor analysis: official CLI has "no credit tracking history"; product thesis calls out credit tracking |
| 2 | Sitemap diff | map diff <url> | 9/10 | Calls /map API, compares returned URL set against most recent stored map_results row for same base_url in SQLite; outputs added/removed URLs as deterministic set difference | Competitor analysis: "no compound workflows"; product thesis: "compounds over time" |
| 3 | Search result diff | search diff <query> | 8/10 | Calls /search with same parameters as prior stored query, joins new results against stored search_results by URL to compute rank changes, new entries, dropped entries | Competitor analysis: no offline storage means no cross-run comparison; brief Top Workflow #1 |
| 4 | Research archive search | research search <terms> | 9/10 | FTS5 query against research_reports.report_text in SQLite; returns matching excerpts with original query, date, model used | Product thesis: "local research database"; build priority #3: offline FTS |
| 5 | Search-then-extract pipeline | search --extract-top N | 9/10 | Calls /search, takes top N result URLs, calls /extract for each, stores both search results and extracted content with shared pipeline_id | Brief Top Workflow #1; product thesis: "search to extract to store"; competitor analysis: "no compound workflows" |
| 6 | Stale content detector | stale --days N | 7/10 | Queries extracted_pages and crawl_pages tables for rows where fetched_at < now - N days, ranked by age; no API call needed | Product thesis: "compounds over time" implies content aging; competitor gap: no offline storage |
