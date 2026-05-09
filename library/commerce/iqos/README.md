# IQOS CLI

**Every IQOS product, store, and flavor across 30+ global markets — synced locally, searchable offline.**

iqos.com spans 30+ country markets with different product lineups, store lists, and flavor availability. This CLI syncs the full catalog locally so you can diff markets, track changes over time, find flavors by profile, and locate stores — all without a browser.

## Install

The recommended path installs both the `iqos-pp-cli` binary and the `pp-iqos` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install iqos
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install iqos --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/iqos-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-iqos --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-iqos --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-iqos skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-iqos. The skill defines how its required CLI can be installed.
```

## Authentication

No authentication required. The entire product catalog, store list, and support content is publicly accessible. Auth exists on iqos.com for orders and loyalty, but no features here require it.

## Quick Start

```bash
# Pull the full US product and store catalog into local SQLite
iqos-pp-cli sync --country us


# Browse all US products
iqos-pp-cli products list --country us --json


# See all HEETS variants available in the UK
iqos-pp-cli flavors list --type heets --country gb --json


# Find nearest stores to Austin, TX
iqos-pp-cli stores nearest --lat 30.27 --lon -97.74 --limit 3


# Products in UK but not US
iqos-pp-cli diff --from us --to gb --json


# What changed in the UK catalog this week
iqos-pp-cli changes --since 7d --country gb

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`diff`** — See which products exist in one country but not another — find market-exclusive devices, flavors, or accessories instantly.

  _Use this when you need to know if a product is available in a specific market before traveling or ordering internationally._

  ```bash
  iqos-pp-cli diff --from us --to gb --json
  ```
- **`changes`** — Detect products added, removed, or changed in price since your last sync — track IQOS catalog evolution over time.

  _Use this to monitor for new product launches or discontinuations in a specific market._

  ```bash
  iqos-pp-cli changes --since 7d --country gb --json
  ```

### Agent-native plumbing
- **`flavors find`** — Search HEETS and TEREA sticks by flavor profile across all markets — find menthol, tobacco, or fruity variants available near you.

  _Use this when helping a user find a specific flavor profile available in their country._

  ```bash
  iqos-pp-cli flavors find --profile menthol --json
  ```
- **`products export`** — Export the complete product catalog for one or all markets as CSV or JSON — aggregate 30+ sitemaps in one command.

  _Use this to build a complete reference dataset of IQOS products across all global markets._

  ```bash
  iqos-pp-cli products export --country all --format csv > iqos-catalog.csv
  ```
- **`stores nearest`** — Find the closest IQOS stores to any GPS coordinate — offline, using geocoordinates extracted from store pages.

  _Use this when an agent needs to recommend an IQOS store without triggering a browser._

  ```bash
  iqos-pp-cli stores nearest --lat 30.27 --lon -97.74 --limit 3 --json
  ```

## Usage

Run `iqos-pp-cli --help` for the full command reference and flag list.

## Commands

### flavors

HEETS, TEREA, and other tobacco stick flavors

- **`iqos-pp-cli flavors list`** - List consumable sticks filtered by type (heets, terea, levia)

### products

IQOS product catalog — devices, sticks, accessories, and vaping products

- **`iqos-pp-cli products get`** - Get product details from its shop page, extracting schema.org JSON-LD
- **`iqos-pp-cli products list`** - List all products for a market by parsing the product sitemap

### stores

Physical IQOS retail locations

- **`iqos-pp-cli stores get`** - Get store details including address, hours, and coordinates from schema.org/Store JSON-LD
- **`iqos-pp-cli stores list`** - List all stores for a market from the stores sitemap

### support

IQOS support articles and FAQs

- **`iqos-pp-cli support faqs`** - Get support FAQ content for a market
- **`iqos-pp-cli support troubleshooting`** - Get device troubleshooting guides


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
iqos-pp-cli flavors --country example-value --lang example-value

# JSON for scripting and agents
iqos-pp-cli flavors --country example-value --lang example-value --json

# Filter to specific fields
iqos-pp-cli flavors --country example-value --lang example-value --json --select id,name,status

# Dry run — show the request without sending
iqos-pp-cli flavors --country example-value --lang example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
iqos-pp-cli flavors --country example-value --lang example-value --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-iqos -g
```

Then invoke `/pp-iqos <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add iqos iqos-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/iqos-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "iqos": {
      "command": "iqos-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
iqos-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: ``

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **Empty product list after sync** — Run iqos-pp-cli sync --country <code> --full to force a full re-fetch
- **Store nearest returns no results** — Run iqos-pp-cli sync --country us first to populate store coordinates
- **diff shows nothing** — You need at least two synced markets: iqos-pp-cli sync --country us && iqos-pp-cli sync --country gb

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
