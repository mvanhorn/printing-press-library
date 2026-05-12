---
name: pp-tavily
description: "Every Tavily endpoint plus search caching, RAG corpus building, and budget management that no other Tavily tool has. Trigger phrases: `search the web for`, `use Tavily to find`, `extract content from`, `crawl this website`, `research the topic`, `build a RAG corpus`, `check my Tavily credits`, `run Tavily search`."
author: "Mani"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tavily-pp-cli
    install:
      - kind: go
        bins: [tavily-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/ai-tools/tavily/cmd/tavily-pp-cli
---

# Tavily — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `tavily-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install tavily --cli-only
   ```
2. Verify: `tavily-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai-tools/tavily/cmd/tavily-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

tavily-pp-cli is a single compiled binary that covers all five Tavily API endpoints with full parameter support. Unlike the official Python CLI, it adds a local SQLite layer that caches results to save credits, tracks usage over time, and enables novel workflows like offline full-text search, corpus gap analysis, and crawl resume — all without re-spending credits.

## When to Use This CLI

Use tavily-pp-cli when building AI agents, RAG pipelines, or research automation that needs reliable web search and content extraction. It is the right choice when you need to avoid redundant API credits through caching, build offline-queryable corpora, track costs across a project, or manage long-running async research tasks across terminal sessions.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### RAG corpus pipeline
- **`corpus build`** — Fan out search, extract, and crawl calls with automatic URL deduplication to build a JSONL corpus ready for embedding.

  _Use this to bootstrap a RAG knowledge base — the most common Tavily production use case — without writing orchestration code._

  ```bash
  tavily-pp-cli corpus build --query 'retrieval augmented generation' --max-results 20 --output corpus.jsonl
  ```
- **`corpus gaps`** — Find URLs mentioned in cached search results that have not yet been extracted, sorted by mention frequency.

  _Use this to identify which high-value pages your corpus is missing before running the next extraction batch._

  ```bash
  tavily-pp-cli corpus gaps --limit 50
  ```

### Agent debugging
- **`replay`** — Replay every search and extract call from a named agent session, substituting cached results so no credits are spent.

  _Use this to debug an agent's information-gathering trace offline without re-running expensive API calls._

  ```bash
  tavily-pp-cli replay --session abc123 --dry-run
  ```
- **`qna`** — Get a direct answer to a question — equivalent to search with include_answer=advanced and compact output, no extra flags needed.

  _Use this in agent tool calls when you need a direct factual answer rather than a list of sources._

  ```bash
  tavily-pp-cli qna --query 'What is the current price of Claude API?'
  ```
- **`context`** — Output a ready-to-paste LLM context string from search results, formatted as numbered passages with source URLs.

  _Use this to inject web-sourced context into an LLM prompt without writing post-processing code._

  ```bash
  tavily-pp-cli context --query 'transformer architecture' --max-results 5 | pbcopy
  ```

## Command Reference

**crawl** — BFS website traversal with content extraction

- `tavily-pp-cli crawl` — Systematically crawl a website using BFS, extracting content from each page

**extract** — Full-page content extraction from URLs

- `tavily-pp-cli extract` — Extract full markdown or text content from 1 to 20 URLs

**map** — Website URL structure discovery without content extraction

- `tavily-pp-cli map` — Discover all URLs in a website structure without fetching page content

**research** — Async deep research with AI synthesis and citations

- `tavily-pp-cli research get` — Get research task status and results by request ID
- `tavily-pp-cli research run` — Submit an async deep research task that synthesizes web sources into a cited report

**usage** — Credit usage and account plan information

- `tavily-pp-cli usage` — Get credit usage broken down by endpoint plus account plan details

**web** — Real-time web search with AI-generated answers and ranked results

- `tavily-pp-cli web` — Search the web with semantic ranking, AI answers, and time/domain filtering


**Hand-written commands**

- `tavily-pp-cli qna` — Get a direct answer to a question (search with include_answer=advanced)
- `tavily-pp-cli context` — Build a ready-to-paste LLM context string from search results
- `tavily-pp-cli local-search` — Full-text search over locally cached content without any API credits
- `tavily-pp-cli corpus build` — Build a RAG corpus via fan-out search+extract+crawl with URL deduplication
- `tavily-pp-cli corpus gaps` — Find URLs in cached search results that have not been extracted yet
- `tavily-pp-cli drift-detect` — Compare current search results against a stored baseline for a query
- `tavily-pp-cli budget-watch` — Monitor credit burn rate against a daily limit with terminal alerts
- `tavily-pp-cli cost-report` — Credit spend breakdown by endpoint and time window
- `tavily-pp-cli batch-plan` — Pre-flight a query list against the cache, estimate cost, and run net-new within budget
- `tavily-pp-cli replay` — Replay all API calls from a named agent session using cached results
- `tavily-pp-cli freshness-check` — Find cached pages older than a threshold and output a re-fetch list (supports 7d, 24h)
- `tavily-pp-cli crawl-resume` — Resume an interrupted crawl from its last SQLite checkpoint
- `tavily-pp-cli research-track` — Background-poll a research task and persist result in SQLite
- `tavily-pp-cli sources-rank` — Rank domains by average relevance score across all stored searches


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tavily-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Build a RAG corpus for a topic

```bash
tavily-pp-cli corpus build --query 'vector databases' --max-results 20 --output vdb-corpus.jsonl
```

Orchestrates search + extract + dedup in one command to produce a JSONL corpus ready for embedding

### Get a fast answer with citations

```bash
tavily-pp-cli qna --query 'who acquired Tavily in 2026' --json
```

Returns an AI-generated answer plus cited source URLs via the `qna` shortcut (search with include_answer=advanced)

### Check what changed in search results

```bash
tavily-pp-cli drift-detect --query 'claude api pricing' --json
```

Compares current vs stored baseline search results and reports which URLs appeared, disappeared, or shifted score

### Crawl a docs site for RAG

```bash
tavily-pp-cli crawl --url https://docs.tavily.com --max-depth 3
```

Crawls the Tavily docs site 3 levels deep, extracting content from each page

### Run deep research and track it

```bash
tavily-pp-cli research run --input 'competitive landscape for AI search APIs 2026' --model pro
tavily-pp-cli research-track --list
```

Submits a research task and tracks completion via polling, persisting the result in SQLite

### Refresh stale cached pages

```bash
tavily-pp-cli freshness-check --older-than 7d --output stale-urls.txt
tavily-pp-cli extract --urls "$(cat stale-urls.txt | head -5 | jq -R -s -c 'split("\n")[:-1]')"
```

Lists pages not refreshed in 7 days, then re-extracts the top candidates. Supports `7d`, `24h`, `168h` duration formats.

## Auth Setup

Set TAVILY_API_KEY in your environment or run `tavily-pp-cli auth set-token` to store it at ~/.tavily-pp-cli/config.json. API keys follow the tvly- prefix format and are sent as Authorization: Bearer headers.

Run `tavily-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tavily-pp-cli research get --request-id 550e8400-e29b-41d4-a716-446655440000 --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
tavily-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
tavily-pp-cli feedback --stdin < notes.txt
tavily-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.tavily-pp-cli/feedback.jsonl`. They are never POSTed unless `TAVILY_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `TAVILY_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
tavily-pp-cli profile save briefing --json
tavily-pp-cli --profile briefing research get --request-id 550e8400-e29b-41d4-a716-446655440000
tavily-pp-cli profile list --json
tavily-pp-cli profile show briefing
tavily-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `tavily-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/ai-tools/tavily/cmd/tavily-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add tavily-pp-mcp -- tavily-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tavily-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tavily-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tavily-pp-cli <command> --help`.
