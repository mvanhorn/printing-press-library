# Techmeme CLI

**Every Techmeme headline, searchable and cached locally — plus topic tracking, trending analysis, and catch-up workflows no other tool has.**

The Techmeme CLI puts the tech industry's most trusted news curation into your terminal. Sync headlines to a local SQLite store, then search, filter by time, track topics, and analyze which stories and sources are dominating. The 'since' command answers the question every tech professional asks: 'what did I miss?'

## Install

The recommended path installs both the `techmeme-pp-cli` binary and the `pp-techmeme` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install techmeme
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install techmeme --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install techmeme --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install techmeme --agent claude-code
npx -y @mvanhorn/printing-press-library install techmeme --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/techmeme/cmd/techmeme-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/techmeme-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-techmeme --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-techmeme --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-techmeme skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-techmeme. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/techmeme-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/techmeme/cmd/techmeme-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "techmeme": {
      "command": "techmeme-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# See today's top tech headlines
techmeme-pp-cli headlines

# Sync 5 days of headlines to local SQLite
techmeme-pp-cli sync

# What happened in the last 4 hours?
techmeme-pp-cli since 4h

# Search all cached headlines
techmeme-pp-cli search 'Apple AI'

# What topics are hot right now?
techmeme-pp-cli trending

# Which publications lead the coverage?
techmeme-pp-cli sources --top 10

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Time intelligence
- **`since`** — See every tech headline from the last N hours — the perfect catch-up when you've been away

  _When an agent needs to brief a user on what happened in tech while they were in meetings, this is the single command that answers it_

  ```bash
  techmeme-pp-cli since 4h --agent
  ```
- **`digest`** — Get a day's tech news grouped by topic — the briefing you'd write if you had time

  _When an agent needs to produce a tech news briefing for a specific date, this structures raw headlines into a readable summary_

  ```bash
  techmeme-pp-cli digest --date 2026-05-08 --agent
  ```

### Persistent monitoring
- **`track`** — Save topics and get alerts when they hit Techmeme — persistent monitoring without browser tabs

  _Agents monitoring specific companies or technologies can subscribe to exactly what matters without polling the full feed_

  ```bash
  techmeme-pp-cli track add 'OpenAI' && techmeme-pp-cli track check --agent
  ```

### News intelligence
- **`sources`** — See which publications dominate Techmeme and track source frequency over time

  _When analyzing media landscape or choosing which publications to prioritize, this gives hard data on source influence_

  ```bash
  techmeme-pp-cli sources --top 20 --agent
  ```
- **`trending`** — Extract the hottest topics from recent headlines using frequency analysis on cached data

  _When an agent needs to answer 'what's hot in tech right now' with data instead of vibes_

  ```bash
  techmeme-pp-cli trending --hours 24 --agent
  ```
- **`velocity`** — Find stories that are blowing up — multiple sources covering the same topic in a short window

  _When an agent needs to identify breaking news vs steady coverage, velocity shows what's accelerating now_

  ```bash
  techmeme-pp-cli velocity --agent
  ```
- **`author`** — Find all Techmeme headlines by a specific journalist across the cached archive

  _When tracking a specific journalist's coverage or building a media contact list, this surfaces their Techmeme footprint_

  ```bash
  techmeme-pp-cli author 'Kara Swisher' --agent
  ```

## Usage

Run `techmeme-pp-cli --help` for the full command reference and flag list.

## Commands

### feed-xml

Manage feed xml

- **`techmeme-pp-cli feed-xml get-main-feed`** - Top 15 headlines currently on Techmeme. RSS 2.0 format. Each item has title, link (to Techmeme permalink), description (HTML with source attribution, image, and excerpt), pubDate, guid.

### lb-opml

Manage lb opml

- **`techmeme-pp-cli lb-opml get-leaderboard-opml`** - OPML file listing Techmeme's top 51 sources with source name, website URL, and RSS feed URL. Updated regularly based on headline prominence over the past 180 days.

### river

Manage river

- **`techmeme-pp-cli river get`** - 5-day rolling archive of all Techmeme headlines in reverse chronological order. 150+ headlines with timestamp, author, source publication, headline text, and article link. HTML page.

### techmeme-search

Manage techmeme search

- **`techmeme-pp-cli techmeme-search headlines`** - Search Techmeme headlines. Supports quoted phrases, wildcards, +/-, AND/OR/NOT, parentheses. Can filter by url, author, date, sourcename. Default searches title+summary; can extend to full text. Results available as HTML or RSS.
- **`techmeme-pp-cli techmeme-search rss`** - RSS feed of search results. Same query syntax as /search/query. Subscribe in any RSS reader for alerts on specific topics.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
techmeme-pp-cli river

# JSON for scripting and agents
techmeme-pp-cli river --json

# Filter to specific fields
techmeme-pp-cli river --json --select id,name,status

# Dry run — show the request without sending
techmeme-pp-cli river --dry-run

# Agent mode — JSON + compact + no prompts in one flag
techmeme-pp-cli river --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Cookbook

```bash
# Morning catch-up: what happened overnight?
techmeme-pp-cli sync && techmeme-pp-cli since 12h

# Track a company and check for mentions
techmeme-pp-cli track add "OpenAI" && techmeme-pp-cli track check

# Find breaking stories (multiple sources covering the same topic)
techmeme-pp-cli velocity --hours 4

# Get a daily briefing grouped by source
techmeme-pp-cli digest --date 2026-05-08

# Who are the top 10 publications on Techmeme?
techmeme-pp-cli sources --top 10 --json

# Search for a topic across all cached headlines
techmeme-pp-cli search 'Apple AI'

# Find all headlines by a specific journalist
techmeme-pp-cli author 'Kara Swisher'

# What's trending in the last 24 hours?
techmeme-pp-cli trending --hours 24

# Export all synced data for analysis
techmeme-pp-cli export --format jsonl --output techmeme-backup.jsonl

# Agent pipeline: sync, then get compact JSON digest
techmeme-pp-cli sync && techmeme-pp-cli digest --agent

# Pipe trending topics to another tool
techmeme-pp-cli trending --json | jq '.[].topic'

# Full archive sync for offline access
techmeme-pp-cli workflow archive
```

## Health Check

```bash
$ techmeme-pp-cli doctor --json
{
  "api": "reachable",
  "auth": "not required",
  "base_url": "https://www.techmeme.com",
  "cache": {
    "status": "unknown",
    "hint": "sync_state is empty; run 'techmeme-pp-cli sync' to hydrate."
  },
  "config": "ok",
  "version": "1.0.0"
}
```

## Configuration

Config file: `~/.config/techmeme-pp-cli/config.toml`

| Variable | Description |
|----------|-------------|
| `TECHMEME_CONFIG` | Override config file path |
| `TECHMEME_BASE_URL` | Override API base URL (default: `https://www.techmeme.com`) |
| `TECHMEME_FEEDBACK_ENDPOINT` | URL for sending CLI feedback upstream |
| `TECHMEME_FEEDBACK_AUTO_SEND` | Set to `true` to auto-send feedback on submit |

No API key is required. Techmeme's public feeds are openly accessible.

## Troubleshooting

**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

**Empty results from `since` or `search` (exit code 0, no output)**
- Run `techmeme-pp-cli sync` first to populate the local cache from the 5-day river archive

**Only 15 headlines from `headlines`**
- The RSS feed carries only the top 15 items. Use `river` or `sync` for the full 5-day archive (150+ headlines)

**Search finds nothing for a recent topic**
- The search endpoint indexes Techmeme headlines only. Topics that haven't made Techmeme's curated feed won't appear

**API error (exit code 5)**
- Run `techmeme-pp-cli doctor` to verify connectivity
- Check that `TECHMEME_BASE_URL` is not set to an invalid URL

**Rate limited (exit code 7)**
- Use `--rate-limit 1` to throttle requests to 1 per second

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
