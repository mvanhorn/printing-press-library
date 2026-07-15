# AutoTempest CLI

**Every AutoTempest car-search source in your terminal, with a local store, cross-source VIN dedupe, and price-drop tracking no AutoTempest tool has.**

AutoTempest already unifies the major used-car marketplaces into one search, but only in a browser. This CLI hits the same aggregated search over plain HTTP, persists listings to a local SQLite store, dedupes the same VIN across sources, and tracks price drops over time. Commands like drops, dedupe, deal, and spread turn one-shot browsing into queryable, compounding car-search state for shoppers and agents.

Learn more at [AutoTempest](https://www.autotempest.com).

Created by [@richardadonnell](https://github.com/richardadonnell) (richardadonnell).

## Install

The recommended path installs both the `autotempest-pp-cli` binary and the `pp-autotempest` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install autotempest
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install autotempest --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install autotempest --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install autotempest --agent claude-code
npx -y @mvanhorn/printing-press-library install autotempest --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/autotempest/cmd/autotempest-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/autotempest-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install autotempest --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-autotempest --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-autotempest --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install autotempest --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/autotempest-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/commerce/autotempest/cmd/autotempest-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "autotempest": {
      "command": "autotempest-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# Health check; confirms the search endpoint is reachable before you search.
autotempest-pp-cli doctor --dry-run

# Aggregated live search across every source, structured output.
autotempest-pp-cli find "honda civic" --zip 33701 --radius 200 --json

# Filtered live search: clean title under twenty-five thousand.
autotempest-pp-cli find "honda civic" --zip 33701 --max-price 25000 --title clean --json

# Collapse the same VIN across sources, cheapest first.
autotempest-pp-cli dedupe --select vin,min_price,sources.source,sources.price --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`drops`** — Surface listings whose price fell since a prior find or watch run, biggest drop first.

  _Reach for this when an agent or shopper needs price movement over time, not a fresh listing dump._

  ```bash
  autotempest-pp-cli drops "civic-fl" --since 7d --min-drop 500 --agent
  ```
- **`watch`** — Register named searches with their filters, then replay them through run so drops and diff have snapshots to compare.

  _Reach for this to set up recurring searches an agent can poll; one-off queries use search instead._

  ```bash
  autotempest-pp-cli watch run --agent
  ```

### Cross-source intelligence
- **`dedupe`** — Collapse the same physical VIN listed on multiple marketplaces into one row with every source and price, cheapest first.

  _Use when the same car appears on eBay and Cars.com and a dealer feed and you need the cheapest source for that exact VIN._

  ```bash
  autotempest-pp-cli dedupe --select vin,min_price,sources.source,sources.price --agent
  ```
- **`deal`** — Rank listings by mechanical price delta from the median of comparable cars (same model, year, mileage band) in your local store.

  _Pick this to answer 'is this price actually good vs comparable cars' with a number, not an opinion._

  ```bash
  autotempest-pp-cli deal "Camry" --select title,price,deal_score --agent
  ```
- **`spread`** — Report min, median, and max price per marketplace for a model so you see which sources run cheap or expensive.

  _Use when deciding which marketplace to shop for a given model._

  ```bash
  autotempest-pp-cli spread "F-150" --agent
  ```
- **`auctions`** — Filter to eBay auction listings with live current bid and bid count, sortable by bid.

  _Use when hunting auction listings rather than fixed-price inventory._

  ```bash
  autotempest-pp-cli auctions --select title,current_bid,bids,url --agent
  ```

## Recipes


### Clean-title AWD under budget near me

```bash
autotempest-pp-cli find "subaru outback" --zip 33701 --radius 150 --max-price 28000 --title clean --drive awd --json --select title,price,mileage,location,sitecode
```

Narrows a deeply nested multi-source live result to the fields that matter using --select.

### Cheapest source for a specific VIN

```bash
autotempest-pp-cli dedupe --select vin,min_price,sources.source,sources.price --json
```

Shows each physical car once with every marketplace and price it appears at.

### What dropped in price this week

```bash
autotempest-pp-cli drops "my-search" --since 7d --min-drop 500 --json
```

Compares local price snapshots to surface motivated sellers.

### Is this price good

```bash
autotempest-pp-cli deal "tacoma" --select title,price,deal_score --json
```

Scores each listing against the median of comparable cars in your store.

## Usage

Run `autotempest-pp-cli --help` for the full command reference and flag list.

## Commands

### makes

List vehicle makes AutoTempest recognizes (slug + display name).

- **`autotempest-pp-cli makes`** - List makes. Use the returned slug (e.g. 'honda') as the --make value for find.

### models

List models for a given make (slug + display name + year range).

- **`autotempest-pp-cli models <make>`** - List models for a make. Pass the make slug (e.g. 'honda'); use the returned model slug as --model for find.

### sources

List the AutoTempest search sources and their kind.

- **`autotempest-pp-cli sources`** - **inline** sources (`te`, `hem`, `cs`, `cv`, `cm`, `eb`, `ot`) return parsed per-car listings; **link** sources (`fbm` = Facebook Marketplace, `st` = SearchTempest / Craigslist) are comparison-link-only because those sites block scraping or require login. `find` defaults to the 7 inline sources; pass `--sites fbm,st` to also get their comparison URLs in `comparison_links`.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
autotempest-pp-cli makes

# JSON for scripting and agents
autotempest-pp-cli makes --json

# Filter to specific fields
autotempest-pp-cli makes --json --select id,name,status

# Dry run — show the request without sending
autotempest-pp-cli makes --dry-run

# Agent mode — JSON + compact + no prompts in one flag
autotempest-pp-cli makes --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - find/sync commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
autotempest-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/autotempest-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the make/model slug is correct
- Run `makes` (and `models <make>`) to see the valid slugs

### API-specific
- **Search returns no results for a valid make/model** — Check the make/model slugs with 'autotempest-pp-cli makes' and 'autotempest-pp-cli models honda'; AutoTempest uses slugs like 'chevrolet' and 'civic'.
- **Invalid token or empty results from a source** — The per-request token is computed automatically; re-run the command. If it persists, run 'doctor' to confirm the endpoint contract has not changed.
- **drops or diff returns nothing** — Those read local price snapshots. Run 'autotempest-pp-cli watch run' (or re-run the same 'find') at least twice over time so there are two snapshots to compare.
