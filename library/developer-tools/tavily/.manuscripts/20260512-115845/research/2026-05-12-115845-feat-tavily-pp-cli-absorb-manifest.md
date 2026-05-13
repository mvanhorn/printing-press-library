# Tavily CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Web search with all params | tvly CLI (Python) | Full param coverage | Single compiled binary, no Python required |
| 2 | `--search-depth` (basic/fast/ultra-fast/advanced) | tvly CLI | Same + `--depth` alias | |
| 3 | `--include-answer` (basic/advanced) | tvly CLI | Same | |
| 4 | `--include-raw-content` (markdown/text) | tvly CLI | Same | |
| 5 | `--time-range` and `--start-date`/`--end-date` | tvly CLI | Same | |
| 6 | `--include-domains` / `--exclude-domains` | tvly CLI, MCP | Same | |
| 7 | `--topic` (general/news/finance) | tvly CLI | Same | |
| 8 | `--country` geo-boost | tvly CLI | Same | |
| 9 | `--include-images` / `--include-image-descriptions` | tvly CLI | Same | |
| 10 | `--exact-match` phrase search | Python SDK, JS SDK (not in CLI/MCP!) | Exposed as `--exact` flag | Fills gap in official CLI |
| 11 | `--max-results` | tvly CLI | Same | |
| 12 | `--chunks-per-source` | tvly CLI | Same | |
| 13 | `--include-favicon` | tvly CLI | Same | |
| 14 | `--json` output mode | tvly CLI | `--json` + `--select` + `--compact` | Better agent-native with field selection |
| 15 | URL extraction (1-20 URLs) | tvly CLI, MCP | `extract` command | |
| 16 | `--extract-depth` (basic/advanced) | tvly CLI | Same | |
| 17 | `--query` for chunk reranking | tvly CLI | Same | |
| 18 | `--format` (markdown/text) | tvly CLI | Same | |
| 19 | `--include-images` on extract | tvly CLI | Same | |
| 20 | `--timeout` on extract | tvly CLI | Same | |
| 21 | Site crawl BFS | tvly CLI, MCP | `crawl` command | |
| 22 | `--max-depth` / `--max-breadth` / `--limit` | tvly CLI | Same | |
| 23 | `--instructions` (natural language guide) | tvly CLI | Same | |
| 24 | `--select-paths` / `--exclude-paths` regex | tvly CLI | Same | |
| 25 | `--select-domains` / `--exclude-domains` regex | tvly CLI | Same | |
| 26 | `--allow-external` / `--no-external` | tvly CLI | Same | |
| 27 | `--output-dir` (save pages as .md files) | tvly CLI | Same | |
| 28 | Site map / URL discovery | tvly CLI, MCP | `map` command | |
| 29 | Async deep research (submit) | tvly CLI, Python SDK | `research run` | |
| 30 | Research `--model` (mini/pro/auto) | tvly CLI | Same | |
| 31 | Research `--stream` (SSE streaming) | tvly CLI, Python SDK | Same | |
| 32 | Research `--output-schema` (JSON schema) | tvly CLI | Same | |
| 33 | Research `--citation-format` | tvly CLI | Same | |
| 34 | Research `--no-wait` (fire-and-forget) | tvly CLI | `--no-wait` + `research status` | |
| 35 | Research status polling | tvly CLI (`research poll`) | `research poll` | |
| 36 | Auth via env var (`TAVILY_API_KEY`) | All SDKs/CLI | Same | |
| 37 | Auth via stored config | tvly CLI | `auth set-token` + `~/.tavily-pp-cli/config.json` | |
| 38 | `doctor` health check | Printing Press default | auth check + /usage probe | |
| 39 | `get_search_context()` helper | Python SDK only | `context` command | First CLI with this! |
| 40 | `qna_search()` shortcut | Python SDK only | `qna` command | First CLI with this! |
| 41 | Usage / credit balance check | API `/usage` | `usage` command | |
| 42 | Multi-key load balancing + notifications | tavily-mcp-multikey (3rd party) | `--api-key` flag accepts comma-list (stub) | |
| 43 | DEFAULT_PARAMETERS config | Official MCP (env var) | `config set search.depth advanced` | Persistent config per-setting |
| 44 | Session/human tracking headers | Python SDK, JS SDK, MCP | `--session-id`, `--human-id` global flags | |

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | Search result deduplication cache | `search --cache-ttl 24h` | Local SQLite query+results cache; 0 credits on cache hit within TTL |
| 2 | Budget watch daemon | `budget watch --daily-limit 500` | Local usage time-series; computes burn rate from stored snapshots; API gives only totals |
| 3 | Cost attribution report | `cost report --by endpoint --since 7d` | Joins timestamped call records in SQLite; API only shows aggregate balance |
| 4 | RAG corpus builder | `corpus build --topic "RAG" --pages 200` | Fan-out search→extract→crawl with URL dedup across SQLite tables |
| 5 | Agent session replay | `replay --session abc123 --dry-run` | SQLite logs all calls per session; replay from cache, zero API spend |
| 6 | Offline FTS over cached content | `local search "attention mechanism"` | FTS5 index on all extracted content; queries without any API call |
| 7 | Research background tracker | `research track --notify` | Local polling daemon with SQLite state; persist across terminal restarts |
| 8 | Web result drift detector | `drift detect --query "openai pricing" --since 7d` | Diffs current vs. historical result sets from SQLite; measures URL overlap, score shifts |
| 9 | Corpus coverage gap analysis | `corpus gaps --topic "llm fine-tuning"` | Joins search result URLs vs. extract table to find uncrawled links |
| 10 | Batch plan + execute with budget | `batch plan queries.txt --budget 200` | Pre-flight cache check + cost estimation from local history; runs only net-new queries |
| 11 | Crawl resume | `crawl resume --run-id run_abc123` | SQLite-persisted crawl frontier; resumes BFS without re-fetching stored pages |
| 12 | Content freshness checker | `freshness check --max-age 30d` | Per-URL fetch timestamps in SQLite; flags stale pages with re-fetch priority |
| 13 | Source reputation tracker | `sources rank --since 90d` | Aggregates per-domain score observations across hundreds of stored searches |

