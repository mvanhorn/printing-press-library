# Tavily CLI

**Every Tavily feature, plus a local research database that compounds across searches, extracts, crawls, and deep research**

The official Tavily CLI is a thin API wrapper with no memory. This CLI stores every search result, extracted page, crawl output, and research report in a local SQLite database. Search offline across all your past results, diff sitemaps across crawl runs, track credit burn over time, and pipe search directly into extraction.

## Install

The recommended path installs both the `tavily-pp-cli` binary and the `pp-tavily` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install tavily
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install tavily --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

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

## Known Gaps

- **No automatic cache freshness tracking.** The CLI stores search results, extracts, and crawl data locally but does not automatically flag when cached content may be stale relative to the API source. Use `tavily-pp-cli stale --days N` to manually check content age.
- **`crawl`, `extract`, `map`, `research`, and `web-search` require a live API key** to execute. Dry-run mode shows the request shape but cannot produce sample output without valid credentials.

## Quick Start

```bash
# Run your first search and cache results locally
tavily-pp-cli web-search --query "AI agent frameworks 2026" --max-results 10


# Re-run and extract full content from the top 3 results
tavily-pp-cli web-search --query "AI agent frameworks 2026" --extract-top 3


# Query cached results with field selection
tavily-pp-cli web-search --query "AI agents" --json --select results.title,results.url,results.score


# See credit burn over the past week
tavily-pp-cli usage history --days 7


# Deep research with streaming progress
tavily-pp-cli research --input "Compare LangChain vs CrewAI" --model pro --stream

```

## Unique Features

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

## Usage

Run `tavily-pp-cli --help` for the full command reference and flag list.

## Commands

### crawl

Systematic website traversal and content extraction

- **`tavily-pp-cli crawl crawl`** - Crawl a website starting from a root URL, extracting content from discovered pages

### extract

Content extraction from web pages

- **`tavily-pp-cli extract extract`** - Extract clean content from one or more URLs

### keys

Enterprise API key management

- **`tavily-pp-cli keys deactivate`** - Deactivate API keys (Enterprise only)
- **`tavily-pp-cli keys generate`** - Generate new API keys (Enterprise only)
- **`tavily-pp-cli keys info`** - Get information about the current API key (Enterprise only)

### map

Website structure discovery and URL mapping

- **`tavily-pp-cli map map`** - Discover the URL structure of a website from a root URL

### research

Comprehensive multi-step research with citations

- **`tavily-pp-cli research create`** - Start a research task that investigates a topic across multiple sources

### usage

Credit usage and account information

- **`tavily-pp-cli usage get`** - Get credit usage for the current billing cycle

### web-search

Real-time web search optimized for AI and RAG applications

- **`tavily-pp-cli web-search search`** - Execute a web search query with configurable depth, filtering, and AI-generated answers


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tavily-pp-cli usage

# JSON for scripting and agents
tavily-pp-cli usage --json

# Filter to specific fields
tavily-pp-cli usage --json --select id,name,status

# Dry run — show the request without sending
tavily-pp-cli usage --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tavily-pp-cli usage --agent
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


Install the MCP binary from this CLI's published public-library entry or pre-built release.

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


Install the MCP binary from this CLI's published public-library entry or pre-built release.

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

Config file: `~/.config/tavily-pp-cli/config.json`

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

- **401 Unauthorized** — Set TAVILY_API_KEY or run tavily-pp-cli config set api_key tvly-...
- **429 Too Many Requests** — Check rate limits: 100 RPM dev, 1000 RPM prod. Use --time-range to narrow queries.
- **432 Plan limit exceeded** — Run tavily-pp-cli usage to check remaining credits. Upgrade plan at app.tavily.com.
- **Empty search results** — Try broader query terms, remove domain filters, or switch topic from news to general.
- **Research hangs** — Research endpoint is async. Use --stream for progress. Pro model takes up to 15 minutes.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**tavily-python**](https://github.com/tavily-ai/tavily-python) — Python (1200 stars)
- [**tavily-cli**](https://github.com/tavily-ai/skills) — Python
- [**tavily-mcp**](https://github.com/tavily-ai/tavily-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
