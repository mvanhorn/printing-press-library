# Exa CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Neural web search with ranked results | exa-js SDK | `exa-pp-cli search` | Offline result store, `--json`, `--select`, typed exit codes, FTS over cached results |
| 2 | Search type selector (auto/neural/keyword) | exa-js SDK | `(behavior in exa-pp-cli search) --type auto\|neural\|keyword` | Full type surface stored per-query |
| 3 | Instant/fast/deep-lite/deep/deep-reasoning types | Exa API | `(behavior in exa-pp-cli search) --type instant\|fast\|deep-lite\|deep\|deep-reasoning` | All type variants exposed |
| 4 | Category filter | exa-js SDK | `(behavior in exa-pp-cli search) --category company\|people\|"research paper"\|news\|"personal site"\|"financial report"\|github\|pdf` | All categories; stored per-query |
| 5 | Domain allow-list | exa-cli.mjs | `(behavior in exa-pp-cli search) --include-domains a.com,b.com` | Comma-separated or repeatable |
| 6 | Domain block-list | exa-cli.mjs | `(behavior in exa-pp-cli search) --exclude-domains a.com,b.com` | Stored per-query |
| 7 | Result count control | exa-cli.mjs | `(behavior in exa-pp-cli search) --limit N` | Default 10; stored per-query |
| 8 | Full page text extraction in results | exa-js SDK | `(behavior in exa-pp-cli search) --text` | Configurable --max-chars; stored in SQLite |
| 9 | Highlight extraction in results | Exa API | `(behavior in exa-pp-cli search) --highlights` | Stored alongside result rows |
| 10 | AI summary per result | Exa API | `(behavior in exa-pp-cli search) --summary <question>` | Cached by URL+question hash |
| 11 | Date filter (start/end) | Exa API | `(behavior in exa-pp-cli search) --since <date> --until <date>` | ISO 8601 or relative (--since 7d) |
| 12 | Structured data extraction / outputSchema | Exa API | `(behavior in exa-pp-cli search) --schema <file-or-inline-json>` | Schema persisted; output validated |
| 13 | Streaming search results | Exa API | `(behavior in exa-pp-cli search) --stream` | NDJSON to stdout; written to SQLite incrementally |
| 14 | Live crawl preference | exa-js deepresearch | `(behavior in exa-pp-cli search) --livecrawl preferred\|always\|never` | Three modes; stored in search profile |
| 15 | Batch URL content extraction | exa-js / MCP | `exa-pp-cli contents` | URL list from stdin/file/args; deduplicates against cache |
| 16 | Per-URL text extraction | exa-cli.mjs | `(behavior in exa-pp-cli contents) --text` | Configurable --max-chars; stores by URL+timestamp |
| 17 | Per-URL highlight extraction | Exa API | `(behavior in exa-pp-cli contents) --highlights` | Stored; comparable across fetch dates |
| 18 | Per-URL AI summary | exa-cli.mjs | `(behavior in exa-pp-cli contents) --summary <question>` | Cached by URL+question hash |
| 19 | Subpage extraction | Exa API | `(behavior in exa-pp-cli contents) --subpages N` | Depth-limited; all sub-URLs stored |
| 20 | Content freshness filter | Exa API | `(behavior in exa-pp-cli contents) --max-age 24h` | Human duration; stored per-fetch |
| 21 | Grounded Q&A with citations | exa-js / MCP | `exa-pp-cli answer` | Citations stored in SQLite; answer deduped by question hash |
| 22 | Streaming answer | exa-js streamAnswer | `(behavior in exa-pp-cli answer) --stream` | SSE tokens to stdout; final answer stored |
| 23 | Semantic similarity search | exa-js / MCP | `exa-pp-cli similar` | Results cached by source URL; diff against prior run |
| 24 | Exclude source from similar results | Exa API | `(behavior in exa-pp-cli similar) --exclude-source` | Default on |
| 25 | Monitors — create recurring search | Exa API | `exa-pp-cli monitor create` | Config stored in SQLite |
| 26 | Monitors — list | Exa API | `exa-pp-cli monitor list` | Shows local last-fired timestamp |
| 27 | Monitors — get single | Exa API | `exa-pp-cli monitor get <id>` | Includes local delivery history |
| 28 | Monitors — delete | Exa API | `exa-pp-cli monitor delete <id>` | Dry-run supported |
| 29 | Monitors — webhook | Exa API | `(behavior in exa-pp-cli monitor create) --webhook <url>` | Webhook URL + local delivery log |
| 30 | Async agent research runs — create | Exa API | `exa-pp-cli agent run` | Run ID + status stored immediately |
| 31 | Agent runs — effort levels | Exa API | `(behavior in exa-pp-cli agent run) --effort minimal\|low\|medium\|high\|x-high` | Effort stored with run record |
| 32 | Agent runs — poll | Exa API | `exa-pp-cli agent poll <run-id>` | Polling loop with --timeout; output written to file and SQLite |
| 33 | Websets — create pipeline | Exa API | `exa-pp-cli webset create` | Pipeline config stored; results appended |
| 34 | Websets — list | Exa API | `exa-pp-cli webset list` | Local row counts alongside remote status |
| 35 | Websets — get results | Exa API | `exa-pp-cli webset get <id>` | Results paged into SQLite; --select to filter |
| 36 | Output to file | exa-cli.mjs | `(behavior in exa-pp-cli search) -o <file>` | Writes JSON; -o - for stdout |
| 37 | JSON output mode | MCP tools | `(behavior in exa-pp-cli search) --json` | Structured JSON composable with jq |
| 38 | Field selection / projection | (missing in all tools) | `(behavior in exa-pp-cli search) --select url,title,text` | Dotted-path selectors; strips noise from LLM payloads |
| 39 | Parallel deep research (async) | parallel-cli | `exa-pp-cli agent run --effort high` | Maps to Exa agent runs with effort level |
| 40 | Deep research poll | parallel-cli | `exa-pp-cli agent poll <id> --timeout 540` | Markdown report written to disk |
| 41 | Context chaining for follow-ups | parallel-cli | `(behavior in exa-pp-cli agent run) --continue <run-id>` | Prior run ID from SQLite; --continue last |
| 42 | Goal / objective narrowing | parallel-cli | `(behavior in exa-pp-cli search) --goal <text>` | Stored with search record |
| 43 | Boolean / advanced query mode | parallel-cli | `(behavior in exa-pp-cli search) --advanced` | AND/OR/NOT/phrase syntax |
| 44 | Geo / country filter | parallel-cli | `(behavior in exa-pp-cli search) --geo <iso2>` | Default from user profile in SQLite |
| 45 | Keyword boost | parallel-cli | `(behavior in exa-pp-cli search) --boost <keyword>` | Repeatable; stored with query |
| 46 | Balance / credit check | parallel-cli | `exa-pp-cli doctor` | Includes API key, reachability, quota, DB integrity |
| 47 | URL scraping to markdown | firecrawl | `exa-pp-cli contents --format markdown` | Exa text extractor; JS-rendered via --livecrawl always |
| 48 | Multiple output formats | firecrawl | `(behavior in exa-pp-cli contents) --format markdown\|html\|links\|text` | Each format stored in own column |
| 49 | Screenshot of page | firecrawl | `(stub - requires headless browser not provided by Exa API)` | Not exposed by Exa; gap acknowledged |
| 50 | Site URL map | firecrawl | `exa-pp-cli map` | findSimilar + subpages to enumerate URLs; stored in SQLite |
| 51 | Async full-site crawl | firecrawl | `exa-pp-cli crawl` | Uses Exa websets/agent for multi-URL extraction |
| 52 | Crawl job status polling | firecrawl | `exa-pp-cli crawl status <job-id>` | Unified status tracking in SQLite |
| 53 | Academic paper search | valyu-cli | `(behavior in exa-pp-cli search) --category "research paper"` | Maps to Exa category |
| 54 | Financial report search | valyu-cli | `(behavior in exa-pp-cli search) --category "financial report"` | Maps to Exa category |
| 55 | News search | valyu-cli | `(behavior in exa-pp-cli search) --category news --since 7d` | Date-windowed news |
| 56 | Multi-source single call | valyu-cli | `(behavior in exa-pp-cli search) --sources web,papers,finance` | Fan-out to multiple Exa queries; deduped by URL |
| 57 | Progress logging to stderr | exa-cli.mjs / valyu-cli | `(behavior in exa-pp-cli search) stderr JSON events` | {step, query, elapsed_ms} per operation |
| 58 | Dry-run mode | (missing in all tools) | `(behavior in exa-pp-cli search) --dry-run` | Prints request payload; exit 0 without sending |
| 59 | Compact / high-gravity fields | (missing in all tools) | `(behavior in exa-pp-cli search) --compact` | url, title, published, one-line excerpt |
| 60 | Agent-native output mode | (missing in all tools) | `(behavior in exa-pp-cli search) --agent` | Structured JSON + --compact defaults |
| 61 | Local FTS search over cached results | (missing in all tools) | `exa-pp-cli search --local <query>` | FTS5 over stored result text; offline |
| 62 | SQL query over result store | (missing in all tools) | `exa-pp-cli sql "<SELECT ...>"` | Read-only SQL against SQLite |
| 63 | Sync / re-run saved queries | (missing in all tools) | `exa-pp-cli sync` | Diffs new vs prior results; exit 3 if unchanged |
| 64 | Doctor / health check | (missing in all tools) | `exa-pp-cli doctor` | API key, reachability, quota, local DB integrity |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Query drift — compare result sets for a fixed query across time | `drift` | hand-code | Requires local history of result sets keyed by query hash; Exa API has no historical result endpoint | Use to see which URLs entered or left top-N since a prior run. Returns new/dropped/stable rows. Do NOT use for ad-hoc diffs; use search --json piped to diff instead. |
| 2 | Cross-query entity co-occurrence | `entities` | hand-code | Requires JOIN across result rows from multiple stored queries + in-process NER; no API combines two queries into an entity surface | Use to find recurring companies/people/papers across a research sweep. Do NOT use for single-query entity extraction; use search --text --json \| jq for that. |
| 3 | Monitor digest — weekly roll-up of all monitor firings | `monitor digest` | hand-code | Requires aggregating local delivery log written per monitor fire; Exa webhooks deliver individually with no server-side aggregation | Use after a week of monitor firings for a deduped, ranked report. Do NOT use for real-time results; use monitor list for current status. |
| 4 | Topic velocity — count new results per query per time bucket | `velocity` | hand-code | Requires time-series of result counts stored across runs; no Exa endpoint returns trend data | Use to detect whether a topic is gaining or losing momentum. Requires at least 2 prior sync runs. |
| 5 | Answer contradiction detector | `contradiction` | hand-code | Requires storing two /answer responses for the same question and diffing them; impossible without local store because each API call is stateless | Use for fact-checking before publishing. Fires two answer calls (auto + neural), stores both, returns divergent claims. Do NOT use for routine Q&A; use answer directly. |
| 6 | Stored query templates with variable interpolation | `template run` | hand-code | Requires local template table with slot definitions, variable binding at run time, execution history log | Use for recurring daily research sweeps. Templates store last-run timestamp, default variables, category/domain presets. Do NOT confuse with monitor; templates are manually triggered. |
| 7 | Budget guard — daily credit cap with spend tracking | `budget set` | hand-code | Requires local spend-log table updated on every API call; Exa API has no server-side daily budget gate | Use to prevent runaway agent spend. `budget set 100` caps daily spend at $1.00; search/contents/answer auto-check tally before firing. |
| 8 | URL intersection — find URLs appearing in two or more saved searches | `intersect` | hand-code | Requires SQL JOIN across result rows from multiple stored query runs; no API takes two queries and returns shared URLs | Use to find authoritative sources that independently surface across different research angles. |
