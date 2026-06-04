# Ahrefs CLI

SEO and competitive intelligence API for backlinks, keywords, rank tracking, site audit, and SERP data.

Learn more at [Ahrefs](https://docs.ahrefs.com/docs/api/reference/introduction).

Created by [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

## Install

The recommended path installs both the `ahrefs-pp-cli` binary and the `pp-ahrefs` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ahrefs
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ahrefs --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ahrefs --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ahrefs --agent claude-code
npx -y @mvanhorn/printing-press-library install ahrefs --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/ahrefs/cmd/ahrefs-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ahrefs-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ahrefs --force
```

Inside a Hermes chat session:

```text
/skills install mvanhorn/printing-press-library/cli-skills/pp-ahrefs --force
```

## Install for OpenClaw

Install both the CLI binary and the focused OpenClaw skill into runtime-visible locations:

```bash
npx -y @mvanhorn/printing-press-library install ahrefs --agent openclaw --bin-dir ~/.local/bin
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ahrefs-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `AHREFS_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle, install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/marketing/ahrefs/cmd/ahrefs-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ahrefs": {
      "command": "ahrefs-pp-mcp",
      "env": {
        "AHREFS_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export AHREFS_API_KEY="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/ahrefs-pp-cli/config.toml`.

### 3. Verify Setup

```bash
ahrefs-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
ahrefs-pp-cli keywords-explorer matching-terms
```

## Usage

Run `ahrefs-pp-cli --help` for the full command reference and flag list.

## Commands

### keywords-explorer

Keywords Explorer endpoints.

- **`ahrefs-pp-cli keywords-explorer matching-terms`** - Matching terms
- **`ahrefs-pp-cli keywords-explorer overview`** - Overview
- **`ahrefs-pp-cli keywords-explorer related-terms`** - Related terms
- **`ahrefs-pp-cli keywords-explorer search-suggestions`** - Search suggestions
- **`ahrefs-pp-cli keywords-explorer volume-by-country`** - Volume by country
- **`ahrefs-pp-cli keywords-explorer volume-history`** - Time-series. Volume history

### public

Public endpoints.

- **`ahrefs-pp-cli public crawler-ip-ranges`** - Crawler IP ranges
- **`ahrefs-pp-cli public crawler-ips`** - Crawler IP addresses

### rank-tracker

Rank Tracker endpoints.

- **`ahrefs-pp-cli rank-tracker competitors-overview`** - Competitors overview
- **`ahrefs-pp-cli rank-tracker overview`** - Overview
- **`ahrefs-pp-cli rank-tracker serp-overview`** - SERP Overview

### serp-overview

Serp Overview endpoints.

- **`ahrefs-pp-cli serp-overview`** - SERP Overview

### site-audit

Site Audit endpoints.

- **`ahrefs-pp-cli site-audit issues`** - Project Issues
- **`ahrefs-pp-cli site-audit page-content`** - Page content
- **`ahrefs-pp-cli site-audit page-explorer`** - Page explorer
- **`ahrefs-pp-cli site-audit projects`** - Project Health Scores

### site-explorer

Site Explorer endpoints.

- **`ahrefs-pp-cli site-explorer all-backlinks`** - Backlinks
- **`ahrefs-pp-cli site-explorer backlinks-stats`** - Backlinks stats
- **`ahrefs-pp-cli site-explorer broken-backlinks`** - Broken Backlinks
- **`ahrefs-pp-cli site-explorer domain-rating`** - Point-in-time snapshot. Domain rating
- **`ahrefs-pp-cli site-explorer domain-rating-history`** - Time-series. Domain Rating history
- **`ahrefs-pp-cli site-explorer metrics`** - Point-in-time snapshot. Metrics
- **`ahrefs-pp-cli site-explorer metrics-by-country`** - Metrics by country
- **`ahrefs-pp-cli site-explorer organic-competitors`** - Organic competitors
- **`ahrefs-pp-cli site-explorer organic-keywords`** - Organic keywords
- **`ahrefs-pp-cli site-explorer pages-by-traffic`** - Pages by traffic
- **`ahrefs-pp-cli site-explorer refdomains-history`** - Time-series. Refdomains history
- **`ahrefs-pp-cli site-explorer top-pages`** - Top pages

### subscription-info

Subscription Info endpoints.

- **`ahrefs-pp-cli subscription-info`** - Limits and usage

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ahrefs-pp-cli keywords-explorer matching-terms

# JSON for scripting and agents
ahrefs-pp-cli keywords-explorer matching-terms --json

# Filter to specific fields
ahrefs-pp-cli keywords-explorer matching-terms --json --select id,name,status

# Dry run — show the request without sending
ahrefs-pp-cli keywords-explorer matching-terms --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ahrefs-pp-cli keywords-explorer matching-terms --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
ahrefs-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/ahrefs-pp-cli/config.toml`

Environment variables:
- `AHREFS_API_KEY`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `ahrefs-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $AHREFS_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
