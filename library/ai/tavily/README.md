# Tavily CLI

**Every Tavily endpoint plus search caching, RAG corpus building, and budget management that no other Tavily tool has.**

tavily-pp-cli is a single compiled binary that covers all five Tavily API endpoints with full parameter support. Unlike the official Python CLI, it adds a local SQLite layer that caches results to save credits, tracks usage over time, and enables novel workflows like offline full-text search, corpus gap analysis, and crawl resume — all without re-spending credits.

Learn more at [Tavily](https://docs.tavily.com).

## Install

The recommended path installs both the `tavily-pp-cli` binary and the `pp-tavily` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install tavily
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install tavily --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/ai-tools/tavily/cmd/tavily-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tavily-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tavily --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tavily --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-tavily skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-tavily. The skill defines how its required CLI can be installed.
```

## Authentication

Set TAVILY_API_KEY in your environment or run `tavily-pp-cli auth set-token` to store it at ~/.tavily-pp-cli/config.json. API keys follow the tvly- prefix format and are sent as Authorization: Bearer headers.

## Quick Start

```bash
# Verify your API key and connection
tavily-pp-cli doctor


# Run a search and get an AI-generated answer
tavily-pp-cli search 'openai gpt-4o pricing' --include-answer basic --json


# Extract full page content from a URL
tavily-pp-cli extract https://openai.com/pricing --query 'API pricing' --format markdown


# Build a RAG corpus from search+extract with dedup
tavily-pp-cli corpus build --topic 'retrieval augmented generation' --pages 50 --out rag-corpus.jsonl


# Query cached content offline with no API credits
tavily-pp-cli local search 'attention mechanism' --highlight


# Check credit balance and per-endpoint spend
tavily-pp-cli usage

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### RAG corpus pipeline
- **`corpus build`** — Fan out search, extract, and crawl calls with automatic URL deduplication to build a JSONL corpus ready for embedding.

  _Use this to bootstrap a RAG knowledge base — the most common Tavily production use case — without writing orchestration code._

  ```bash
  tavily-pp-cli corpus build --topic 'retrieval augmented generation' --pages 200 --out corpus.jsonl
  ```
- **`corpus gaps`** — Find URLs mentioned in cached search results that have not yet been extracted, sorted by mention frequency.

  _Use this to identify which high-value pages your corpus is missing before running the next extraction batch._

  ```bash
  tavily-pp-cli corpus gaps --topic 'llm fine-tuning' --min-mentions 3 --out gaps.json
  ```

### Agent debugging
- **`replay`** — Replay every search and extract call from a named agent session, substituting cached results so no credits are spent.

  _Use this to debug an agent's information-gathering trace offline without re-running expensive API calls._

  ```bash
  tavily-pp-cli replay --session abc123 --dry-run --format markdown
  ```
- **`qna`** — Get a direct answer to a question — equivalent to search with include_answer=advanced and compact output, no extra flags needed.

  _Use this in agent tool calls when you need a direct factual answer rather than a list of sources._

  ```bash
  tavily-pp-cli qna 'What is the current price of Claude API?'
  ```
- **`context`** — Output a ready-to-paste LLM context string from search results, formatted as numbered passages with source URLs.

  _Use this to inject web-sourced context into an LLM prompt without writing post-processing code._

  ```bash
  tavily-pp-cli context 'transformer architecture' --max-tokens 4000 | pbcopy
  ```

## Usage

Run `tavily-pp-cli --help` for the full command reference and flag list.

## Commands

### crawl

BFS website traversal with content extraction

- **`tavily-pp-cli crawl crawl`** - Systematically crawl a website using BFS, extracting content from each page

### extract

Full-page content extraction from URLs

- **`tavily-pp-cli extract extract`** - Extract full markdown or text content from 1 to 20 URLs

### map

Website URL structure discovery without content extraction

- **`tavily-pp-cli map map`** - Discover all URLs in a website structure without fetching page content

### research

Async deep research with AI synthesis and citations

- **`tavily-pp-cli research get`** - Get research task status and results by request ID
- **`tavily-pp-cli research run`** - Submit an async deep research task that synthesizes web sources into a cited report

### usage

Credit usage and account plan information

- **`tavily-pp-cli usage get`** - Get credit usage broken down by endpoint plus account plan details

### web

Real-time web search with AI-generated answers and ranked results

- **`tavily-pp-cli web search`** - Search the web with semantic ranking, AI answers, and time/domain filtering


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tavily-pp-cli research get --request-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
tavily-pp-cli research get --request-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
tavily-pp-cli research get --request-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
tavily-pp-cli research get --request-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tavily-pp-cli research get --request-id 550e8400-e29b-41d4-a716-446655440000 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-tavily -g
```

Then invoke `/pp-tavily <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/ai-tools/tavily/cmd/tavily-pp-mcp@latest
```

Then register it:

```bash
claude mcp add tavily tavily-pp-mcp -e TAVILY_API_KEY=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tavily-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TAVILY_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/ai-tools/tavily/cmd/tavily-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "tavily": {
      "command": "tavily-pp-mcp",
      "env": {
        "TAVILY_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
tavily-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.tavily-pp-cli/config.json`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TAVILY_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `tavily-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TAVILY_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **429 Too Many Requests** — Use --cache-ttl to avoid re-querying recent searches; or add --sleep 1s between batch calls
- **502 Bad Gateway on search** — Tavily occasionally has transient 502s; retry with tavily-pp-cli search --retry 3
- **Research task stuck in pending** — Run tavily-pp-cli research track --id <id> to poll in background; pro model can take 2+ minutes
- **TAVILY_API_KEY not found** — Export TAVILY_API_KEY=tvly-... or run tavily-pp-cli auth set-token

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**tavily-ai/tavily-mcp**](https://github.com/tavily-ai/tavily-mcp) — TypeScript (2000 stars)
- [**tavily-ai/tavily-python**](https://github.com/tavily-ai/tavily-python) — Python (1200 stars)
- [**langchain-tavily**](https://github.com/tavily-ai/langchain-tavily) — Python (500 stars)
- [**tavily-ai/tavily-js**](https://github.com/tavily-ai/tavily-js) — TypeScript (89 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
