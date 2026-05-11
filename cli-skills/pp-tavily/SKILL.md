---
name: pp-tavily
description: "Every Tavily feature, plus a local research database that compounds across searches, extracts, crawls, and deep research Trigger phrases: `search the web for`, `extract content from`, `crawl this site`, `map this website`, `research this topic`, `check my Tavily credits`, `use tavily`, `run tavily`."
author: "user"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - tavily-pp-cli
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
go install github.com/mvanhorn/printing-press-library/library/ai/tavily/cmd/tavily-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

The official Tavily CLI is a thin API wrapper with no memory. This CLI stores every search result, extracted page, crawl output, and research report in a local SQLite database. Search offline across all your past results, diff sitemaps across crawl runs, track credit burn over time, and pipe search directly into extraction.

## When to Use This CLI

Use tavily-pp-cli when you need persistent web research across sessions. It excels at iterative search refinement (diff results across runs), building local content databases from crawled sites, and tracking research costs over time. Prefer it over raw API calls when you want offline access to past results or compound workflows like search-then-extract.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`usage history`** — Track credit consumption over time with daily snapshots, showing per-endpoint burn rate trends

  _Use when an agent needs to understand cost trends before deciding whether to use basic or advanced search depth_

  ```bash
  tavily-pp-cli usage history --days 30 --json --select date,search_usage,total
  ```
- **`map diff`** — Compare current sitemap against the last stored map for the same URL, showing added and removed pages

  _Use when monitoring competitor sites for new pages or structural changes between crawl runs_

  ```bash
  tavily-pp-cli map diff https://competitor.com --json --select added,removed
  ```
- **`web-search diff`** — Re-run a previous search and show which results are new, dropped, or changed rank position

  _Use when an agent needs to detect changes in search landscape for a monitored topic_

  ```bash
  tavily-pp-cli web-search diff "AI agent frameworks" --json
  ```
- **`research search`** — Full-text search across all stored research reports, returning matching excerpts with dates and queries

  _Use when an agent needs to find prior research on a topic before deciding whether to run a new research query_

  ```bash
  tavily-pp-cli research search "edge computing" --json --select query,date,excerpt
  ```
- **`stale`** — Show extracted pages and crawl results older than N days that may need re-extraction

  _Use when an agent needs to decide which cached content to refresh before answering a question_

  ```bash
  tavily-pp-cli stale --days 7 --json --select url,age_days,source_type
  ```

### Agent-native plumbing
- **`web-search --extract-top`** — Search for a topic and automatically extract full content from the top N result URLs in one operation

  _Use when building RAG context: one command searches and extracts instead of two separate calls_

  ```bash
  tavily-pp-cli web-search --query "Claude Code best practices" --extract-top 3 --json
  ```

## Command Reference

**crawl** — Systematic website traversal and content extraction

- `tavily-pp-cli crawl` — Crawl a website starting from a root URL, extracting content from discovered pages

**extract** — Content extraction from web pages

- `tavily-pp-cli extract` — Extract clean content from one or more URLs

**keys** — Enterprise API key management

- `tavily-pp-cli keys deactivate` — Deactivate API keys (Enterprise only)
- `tavily-pp-cli keys generate` — Generate new API keys (Enterprise only)
- `tavily-pp-cli keys info` — Get information about the current API key (Enterprise only)

**map** — Website structure discovery and URL mapping

- `tavily-pp-cli map` — Discover the URL structure of a website from a root URL

**research** — Comprehensive multi-step research with citations

- `tavily-pp-cli research` — Start a research task that investigates a topic across multiple sources

**usage** — Credit usage and account information

- `tavily-pp-cli usage` — Get credit usage for the current billing cycle

**web-search** — Real-time web search optimized for AI and RAG applications

- `tavily-pp-cli web-search` — Execute a web search query with configurable depth, filtering, and AI-generated answers


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
tavily-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Monitor competitor sites weekly

```bash
tavily-pp-cli map diff https://competitor.com --json --select added,removed
```

Shows new and removed pages since last map run

### Build RAG context with budget

```bash
tavily-pp-cli web-search --query "kubernetes best practices" --extract-top 5 --search-depth basic --agent --select results.url,results.content
```

Searches, extracts top 5, returns agent-optimized output with only URL and content fields

### Find stale cached content

```bash
tavily-pp-cli stale --days 14 --json --select url,age_days
```

Lists all cached extractions and crawls older than 14 days

### Research with structured output

```bash
tavily-pp-cli research --input "Compare React vs Vue in 2026" --model pro --citation-format apa --json
```

Deep research with APA citations and JSON output for downstream processing

### Track credit usage trends

```bash
tavily-pp-cli usage history --days 30 --agent --select date,search_usage,extract_usage,total
```

Shows daily credit consumption with per-endpoint breakdown

## Auth Setup

Run `tavily-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
tavily-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `TAVILY_API_KEY` as an environment variable.

Run `tavily-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  tavily-pp-cli usage --agent --select id,name,status
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
tavily-pp-cli --profile briefing usage
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

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add tavily-pp-mcp -- tavily-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which tavily-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   tavily-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `tavily-pp-cli <command> --help`.
