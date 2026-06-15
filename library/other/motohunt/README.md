# MotoHunt CLI

**Search motorcycle and ATV listings from the terminal — with MotoHunt's MSRP/average-price/deal-rating data exposed as structured fields no other tool gives you.**

MotoHunt has no public API; this CLI scrapes its server-rendered HTML with one HTTP GET per page and returns clean JSON. It surfaces the price-research data (base MSRP, average listing price, deal rating) that makes 'is this a good deal?' answerable, ranks synced inventory by under-market gap with `deal`, watches saved searches for new listings and price drops, and covers the ATV sister site via `--site atv`.

## Install

The recommended path installs both the `motohunt-pp-cli` binary and the `pp-motohunt` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install motohunt
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install motohunt --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install motohunt --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install motohunt --agent claude-code
npx -y @mvanhorn/printing-press-library install motohunt --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/motohunt/cmd/motohunt-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/motohunt-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install motohunt --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-motohunt --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-motohunt --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install motohunt --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/motohunt-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/motohunt/cmd/motohunt-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "motohunt": {
      "command": "motohunt-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# health check + selector-health probe, no network needed
motohunt-pp-cli doctor --dry-run

# best-deal Harleys near a ZIP, parsed cards as JSON
motohunt-pp-cli search --make Harley-Davidson --location 33705 --sort c --limit 30 --agent

# one listing's price-research at a glance
motohunt-pp-cli get 13256655 --agent --select title,price,base_msrp,alp,deal_rating

# same search against the ATV marketplace
motohunt-pp-cli search --site atv --style Sport --location 33705 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Price intelligence
- **`get`** — See a listing's MSRP, average listing price, and deal rating as typed fields, not buried prose.

  _Reach for this to answer 'is this listing actually a good price?' without reading the page._

  ```bash
  motohunt-pp-cli get 13256655 --agent --select base_msrp,alp,deal_rating
  ```
- **`deal`** — Rank synced listings by how far the asking price sits below the average listing price.

  _Reach for this to surface the biggest under-market deals across a whole search._

  ```bash
  motohunt-pp-cli deal --make Harley-Davidson --location 33705 --limit 20 --agent
  ```

### Local state that compounds
- **`watch run`** — Re-run saved searches and report new listings and price drops since the last run.

  _Reach for this to monitor a hunt over time instead of re-searching by hand._

  ```bash
  motohunt-pp-cli watch run --agent
  ```

### Coverage
- **`search`** — Search motorcycles (motohunt.com) or ATV/UTV/SxS (atvhunt.com) from one binary via --site.

  _Reach for --site atv when the hunt is four-wheelers instead of bikes._

  ```bash
  motohunt-pp-cli search --site atv --location 33705 --agent
  ```

## Recipes


### Best-deal Harleys near me

```bash
motohunt-pp-cli search --make Harley-Davidson --location 33705 --sort c --limit 40 --agent --select title,price,deal_rating,location
```

Top under-market Harleys, slimmed to the fields that matter.

### Is this listing a good price?

```bash
motohunt-pp-cli get 13256655 --agent --select price,base_msrp,alp,deal_rating
```

Compare the ask against MSRP and the average listing price.

### Watch a hunt for price drops

```bash
motohunt-pp-cli watch add --name 'gs near me' --make BMW --model R-1250-GS --location 33705 && motohunt-pp-cli watch run --agent
```

Save a search, then diff it over time for new listings and price drops.

## Usage

Run `motohunt-pp-cli --help` for the full command reference and flag list.

## Commands

### listings

Search and inspect motorcycle/ATV listings

- **`motohunt-pp-cli listings get`** - Fetch a single listing detail page
- **`motohunt-pp-cli listings search`** - Search listings; returns links to listing detail pages (use the hand-built `search` command for parsed cards)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
motohunt-pp-cli listings get mock-value

# JSON for scripting and agents
motohunt-pp-cli listings get mock-value --json

# Filter to specific fields
motohunt-pp-cli listings get mock-value --json --select id,name,status

# Dry run — show the request without sending
motohunt-pp-cli listings get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
motohunt-pp-cli listings get mock-value --agent
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

## Health Check

```bash
motohunt-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/motohunt-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **search returns zero cards but the site shows results** — the page HTML changed; run `motohunt-pp-cli doctor` to check selector health and re-verify the card selectors
- **only 24 results come back** — raise --limit; the CLI auto-pages via ?start= in 24-row pages up to --max-pages
