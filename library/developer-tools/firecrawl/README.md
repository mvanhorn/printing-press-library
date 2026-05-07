# Firecrawl CLI

API for interacting with Firecrawl services to perform web scraping and crawling tasks.

Learn more at [Firecrawl](https://firecrawl.dev/support).

## Install

The recommended path installs both the `firecrawl-pp-cli` binary and the `pp-firecrawl` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install firecrawl
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install firecrawl --cli-only
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.23+):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/firecrawl/cmd/firecrawl-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/firecrawl-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-firecrawl --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-firecrawl --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-firecrawl skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-firecrawl. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your access token from your API provider's developer portal, then store it:

```bash
firecrawl-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set it via environment variable:

```bash
export FIRECRAWL_BEARER_AUTH="your-token-here"
```

### 3. Verify Setup

```bash
firecrawl-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
firecrawl-pp-cli batch cancel-scrape mock-value
```

## Usage

Run `firecrawl-pp-cli --help` for the full command reference and flag list.

## Commands

### batch

Manage batch

- **`firecrawl-pp-cli batch cancel-scrape`** - Cancel a batch scrape job
- **`firecrawl-pp-cli batch get-scrape-errors`** - Get the errors of a batch scrape job
- **`firecrawl-pp-cli batch get-scrape-status`** - Get the status of a batch scrape job
- **`firecrawl-pp-cli batch scrape-and-extract-from-urls`** - Scrape multiple URLs and optionally extract information using an LLM

### crawl

Manage crawl

- **`firecrawl-pp-cli crawl cancel`** - Cancel a crawl job
- **`firecrawl-pp-cli crawl get-active`** - Get all active crawls for the authenticated team
- **`firecrawl-pp-cli crawl get-status`** - Get the status of a crawl job
- **`firecrawl-pp-cli crawl urls`** - Crawl multiple URLs based on options

### deep-research

Manage deep research

- **`firecrawl-pp-cli deep-research get-status`** - Get the status and results of a deep research operation
- **`firecrawl-pp-cli deep-research start`** - Start a deep research operation on a query

### extract

Manage extract

- **`firecrawl-pp-cli extract data`** - Extract structured data from pages using LLMs
- **`firecrawl-pp-cli extract get-status`** - Get the status of an extract job

### firecrawl-search

Manage firecrawl search

- **`firecrawl-pp-cli firecrawl-search search-and-scrape`** - Search and optionally scrape search results

### llmstxt

Manage llmstxt

- **`firecrawl-pp-cli llmstxt generate-llms-txt`** - Generate LLMs.txt for a website
- **`firecrawl-pp-cli llmstxt get-llms-txt-status`** - Get the status and results of an LLMs.txt generation job

### map

Manage map

- **`firecrawl-pp-cli map urls`** - Map multiple URLs based on options

### scrape

Manage scrape

- **`firecrawl-pp-cli scrape and-extract-from-url`** - Scrape a single URL and optionally extract information using an LLM

### team

Manage team

- **`firecrawl-pp-cli team get-credit-usage`** - Get remaining credits for the authenticated team
- **`firecrawl-pp-cli team get-token-usage`** - Get remaining tokens for the authenticated team (Extract only)

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
firecrawl-pp-cli batch cancel-scrape mock-value

# JSON for scripting and agents
firecrawl-pp-cli batch cancel-scrape mock-value --json

# Filter to specific fields
firecrawl-pp-cli batch cancel-scrape mock-value --json --select id,name,status

# Dry run — show the request without sending
firecrawl-pp-cli batch cancel-scrape mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
firecrawl-pp-cli batch cancel-scrape mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-firecrawl -g
```

Then invoke `/pp-firecrawl <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:

```bash
go install github.com/mvanhorn/printing-press-library/library/other/firecrawl/cmd/firecrawl-pp-mcp@latest
```

Then register it:

```bash
claude mcp add firecrawl firecrawl-pp-mcp -e FIRECRAWL_BEARER_AUTH=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/firecrawl-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FIRECRAWL_BEARER_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/firecrawl/cmd/firecrawl-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "firecrawl": {
      "command": "firecrawl-pp-mcp",
      "env": {
        "FIRECRAWL_BEARER_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
firecrawl-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/firecrawl-pp-cli/config.toml`

Environment variables:
- `FIRECRAWL_BEARER_AUTH`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `firecrawl-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FIRECRAWL_BEARER_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
