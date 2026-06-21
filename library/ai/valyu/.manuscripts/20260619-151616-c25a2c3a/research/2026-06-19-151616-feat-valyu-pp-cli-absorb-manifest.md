# Absorb Manifest — valyu-pp-cli
**Run:** 20260619-151616-c25a2c3a | **Date:** 2026-06-19

---

## Table Stakes — Absorbed Features

Every feature from the ecosystem's 10 existing tools, matched to a generated command.

| # | Feature | Best Source | Our Implementation |
|---|---------|-------------|-------------------|
| 1 | Semantic search with filters | valyu-mcp (Python) | `valyu-pp-cli search [flags]` |
| 2 | Knowledge search with source/type control | valyu-mcp-js | `search list` (generated endpoint mirror) |
| 3 | Feedback submission for search transactions | valyu-mcp-js | `valyu-pp-cli feedback` |
| 4 | Web search | valyu-cli (community) | `valyu-pp-cli search --type web` |
| 5 | Finance search (stocks, earnings, crypto) | @valyu/ai-sdk | `valyu-pp-cli search --type finance` |
| 6 | Academic paper search (arXiv, PubMed) | @valyu/ai-sdk | `valyu-pp-cli search --type paper` |
| 7 | Biomedical search (ChEMBL, clinical trials) | @valyu/ai-sdk | `valyu-pp-cli search --type bio` |
| 8 | Patent search (USPTO) | @valyu/ai-sdk | `valyu-pp-cli search --type patent` |
| 9 | SEC filing search (10-K, 10-Q, 8-K) | @valyu/ai-sdk | `valyu-pp-cli search --type sec` |
| 10 | Economics data search (FRED, BLS, World Bank) | @valyu/ai-sdk | `valyu-pp-cli search --type economics` |
| 11 | News search with date/country filtering | @valyu/ai-sdk | `valyu-pp-cli search --type news` |
| 12 | URL content extraction (batch) | valyu-cli (community) | `valyu-pp-cli contents extract` |
| 13 | Contents async job status | valyu-js SDK | `valyu-pp-cli contents status <job-id>` |
| 14 | AI-synthesized answer with SSE streaming | valyu-js SDK | `valyu-pp-cli answer` |
| 15 | DeepResearch create | valyu-cli (community) | `valyu-pp-cli research create` |
| 16 | DeepResearch status | valyu-cli (community) | `valyu-pp-cli research status <id>` |
| 17 | DeepResearch wait/poll until complete | valyu-js SDK | `valyu-pp-cli research wait <id>` |
| 18 | DeepResearch stream result events | valyu-js SDK | `valyu-pp-cli research stream <id>` |
| 19 | DeepResearch list all tasks | valyu-js SDK | `valyu-pp-cli research list` |
| 20 | DeepResearch update task metadata | valyu-js SDK | `valyu-pp-cli research update <id>` |
| 21 | DeepResearch cancel in-progress task | valyu-js SDK | `valyu-pp-cli research cancel <id>` |
| 22 | DeepResearch delete task | valyu-js SDK | `valyu-pp-cli research delete <id>` |
| 23 | DeepResearch batch create | valyu-js SDK | `valyu-pp-cli batch create` |
| 24 | DeepResearch batch add tasks | valyu-js SDK | `valyu-pp-cli batch add-tasks <batch-id>` |
| 25 | DeepResearch batch wait for completion | valyu-js SDK | `valyu-pp-cli batch wait <batch-id>` |
| 26 | DeepResearch batch list | valyu-js SDK | `valyu-pp-cli batch list` |
| 27 | DeepResearch batch status | valyu-js SDK | `valyu-pp-cli batch status <batch-id>` |
| 28 | DeepResearch batch cancel | valyu-js SDK | `valyu-pp-cli batch cancel <batch-id>` |
| 29 | Workflow list templates | valyu-js SDK | `valyu-pp-cli workflows list` |
| 30 | Workflow create template | valyu-js SDK | `valyu-pp-cli workflows create` |
| 31 | Workflow get by slug | valyu-js SDK | `valyu-pp-cli workflows get <slug>` |
| 32 | Workflow update | valyu-js SDK | `valyu-pp-cli workflows update <slug>` |
| 33 | Workflow delete | valyu-js SDK | `valyu-pp-cli workflows delete <slug>` |
| 34 | Workflow preview | valyu-js SDK | `valyu-pp-cli workflows preview <slug>` |
| 35 | Workflow versions list | valyu-js SDK | `valyu-pp-cli workflows versions <slug>` |
| 36 | Datasource list all sources | valyu-js SDK | `valyu-pp-cli datasources list` |
| 37 | Datasource by category | valyu-js SDK | `valyu-pp-cli datasources categories` |

---

## Transcendence — Novel Features (hand-code scope)

All 8 survivors require hand-written Cobra commands + SQLite integration (tagged `hand-code`).

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|-------------|--------------|----------|-----------------|
| T1 | Search A/B comparison | `valyu-pp-cli search compare "query" --config-a "type=web" --config-b "type=paper"` | 9/10 | hand-code | Two sequential search calls with different param sets; diffs result URLs, titles, snippet overlap, cost delta; outputs unified table or JSON diff | Maya's explicit pain: evaluating source configs before committing to RAG production setup | Run the same query against two different source/type configurations and receive a side-by-side diff of result overlap, unique hits per config, and cost delta. `--config-a` and `--config-b` accept comma-separated key=value pairs mapping to search flags. Output includes a Jaccard similarity score for result URLs, per-config cost, and `--format json` for downstream processing. |
| T2 | Cost ledger | `valyu-pp-cli cost [--since 7d] [--budget 5.00] [--breakdown]` | 8/10 | hand-code | Every CLI invocation appends cost field from API response to local SQLite; `cost` command aggregates and reports; `--budget` warns if cumulative spend exceeds threshold | Carlos needs budget visibility; Valyu's usage-based pricing model makes cost tracking acute | Maintains a local SQLite ledger of every search, answer, and research API call with its cost, timestamp, and command type. `cost` shows daily/weekly rollup. `--budget N` adds a threshold warning. `--breakdown` shows per-command-type subtotals. Feeds into `search batch --max-total-cost` for run-level budgeting. |
| T3 | Multi-query batch search | `valyu-pp-cli search batch --file queries.txt [--parallel 3] [--max-total-cost 2.00] [--format jsonl]` | 9/10 | hand-code | Reads queries from file (one per line), executes searches sequentially or in parallel, accumulates costs client-side, aborts if `--max-total-cost` exceeded, emits JSONL | Carlos's 30-ticker watchlist; Maya's 50-query regression test suite; no existing tool covers file-driven batch search | Reads a query file (one query per line, optionally tab-separated with per-query flag overrides). Runs all searches and emits one JSON object per result set per line (JSONL). Tracks cumulative cost and aborts with partial results if `--max-total-cost` is hit. `--parallel N` runs N concurrent searches. Output is pipe-friendly. Use for bulk query execution from a file. Do NOT use for single queries; use `valyu-pp-cli search` instead. |
| T4 | Search history with replay | `valyu-pp-cli history list / history replay <id> [--diff]` | 9/10 | hand-code | Every search call writes query+params+result URLs+cost+timestamp to SQLite; `history replay` re-runs the stored search and diffs new results against stored snapshot | Carlos's "what changed this week vs. last week" pain; cross-entity local query joining history rows with cost data | Local SQLite stores every search invocation. `history list` shows past searches with timestamps and costs. `history replay <id>` re-executes the identical search and outputs a diff: new URLs since last run, dropped URLs, unchanged URLs, cost delta. `history export --format csv` for downstream analysis. |
| T5 | Saved search templates | `valyu-pp-cli search save <name> / search run <name> / search list-saved` | 8/10 | hand-code | Writes named search config (query + all flags) to local JSON config file; `search run` loads and executes; complementary to server-side Workflows | Carlos's repeatable weekly routine; complements server Workflows for local-only use cases | Saves a named search configuration locally (distinct from server-side Workflows). `search save earnings-scan --query "Q2 earnings" --type finance --sources SEC,news --max 20`. `search run earnings-scan` executes it. Use for client-side saved query templates. Do NOT use for server-side workflow templates; use `valyu-pp-cli workflows` instead. |
| T6 | Contents pipeline (search-to-extract) | `valyu-pp-cli search "query" \| valyu-pp-cli contents extract --from-stdin` | 9/10 | hand-code | `contents extract --from-stdin` reads JSON from stdin, extracts `url` fields from search results, submits batch extraction job; bridges search and content extraction in one pipe | Priya's two-step pain: manual URL copy from search results to batch extraction; common RAG ingest pattern with no single-command solution | `contents extract --from-stdin` accepts piped search result JSON (or JSONL from `search batch`), extracts all URLs, and submits a batch extraction job. Returns a job ID for `contents status`. `--max-urls N` caps extraction cost. `--wait` blocks until job completes and writes extracted text to stdout or `--out dir/`. |
| T7 | Citation export from answer | `valyu-pp-cli answer "question" --cite-output citations.json [--format ris\|bibtex\|json]` | 8/10 | hand-code | Parses SSE stream from answer endpoint, extracts source URLs/titles/snippets from response, writes structured citation list in requested format alongside answer text | RAG pipeline builders need provenance tracking; academic/legal users need citation formats | Streams the answer SSE and simultaneously captures all source attributions. On completion, writes a structured citation file in JSON (default), RIS (for citation managers), or BibTeX format. Answer text goes to stdout; citations to `--cite-output` file. |
| T8 | Research task queue | `valyu-pp-cli research queue --file topics.txt / research queue status / research queue download` | 9/10 | hand-code | Submits N `research create` calls from a topics file, stores all task IDs in local SQLite with labels, `queue status` polls all pending tasks and shows a live status table | Priya's Monday morning compound research batch; bridges gap between single `research create` and server-side batch API | Reads a topics file (one question per line), submits each as a `research create` call, stores task IDs with labels in local SQLite. `research queue status` polls all tracked tasks and renders state/elapsed table. `research queue download --dir ~/reports/` downloads all completed tasks. Use for client-side task lifecycle tracking. Do NOT use for server-side batch submission; use `valyu-pp-cli batch` instead. |

---

## Ecosystem Inventory

| Tool | Type | Stars | Auth | Commands / Features | Gap vs. our CLI |
|------|------|-------|------|---------------------|----------------|
| valyu-mcp (Python) | Official MCP server | 17 | X-API-Key | `valyu_context` (semantic search) | Single search tool only; no research, answer, or contents support |
| valyu-mcp-js (TypeScript) | Official MCP server | — | X-API-Key | `knowledge_search` with full filter params; `feedback` | No research/answer/batch support |
| valyu-js | Official SDK | — | X-API-Key | Full API surface (all 27 endpoints) | SDK only, no CLI |
| valyu-python | Official SDK | — | X-API-Key | Full API surface | SDK only, no CLI |
| @valyu/ai-sdk | Official AI SDK adapter | — | X-API-Key | Domain-specific search tools (finance, bio, patent, sec, economics, news, paper) | Vercel AI SDK integration only |
| claude-search-plugin | Official Claude plugin | — | X-API-Key | Domain search (finance, bio, patent, SEC, news, economics) | Plugin only, not a CLI |
| valyu-agent-skills | Official agent skills | — | X-API-Key | Agent-oriented search skills | Not a standalone CLI |
| valyu-cli (npm, ed3t) | Community CLI | — | X-API-Key | search, contents, deep-research create, deep-research status | Only 4 commands; no history, batch queries, cost tracking, citation export, or task queue |
| valyu-aws | Community AWS integration | — | X-API-Key | AWS Lambda wrapper for search | Not a CLI |
| valyu-agent (community) | Community agent | — | X-API-Key | Agent loop with Valyu search | Not a CLI |
