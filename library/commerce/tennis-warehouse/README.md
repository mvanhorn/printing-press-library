# Tennis Warehouse CLI

**Every racquet Tennis Warehouse sells — new and used — searchable offline with spec compare, substitute finder, price-drop tracking, and grip-size availability no browser tab gives you.**

Tennis Warehouse has the deepest racquet catalog and used inventory in the U.S., but the web UI is browse-only and the data is locked behind page navigation. This CLI mirrors the entire catalog into a local SQLite store, then exposes the spec-driven and price-driven queries the website cannot answer — `racquets similar <sku>`, `racquets compare <sku> <sku>`, `used deals --min-discount-pct 40`, `used drops --since 7d`, `used new --since 7d`, `used depth --grade A`, `used watch <pcode>`, `used grip-availability --size 4_3/8`.

Learn more at [Tennis Warehouse](https://www.tennis-warehouse.com).

Printed by [@blake41](https://github.com/blake41) (blake johnson).

## Install

The recommended path installs both the `tennis-warehouse-pp-cli` binary and the `pp-tennis-warehouse` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install tennis-warehouse
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install tennis-warehouse --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install tennis-warehouse --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install tennis-warehouse --agent claude-code
npx -y @mvanhorn/printing-press install tennis-warehouse --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tennis-warehouse-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tennis-warehouse --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tennis-warehouse --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-tennis-warehouse skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-tennis-warehouse. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tennis-warehouse-current).
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
    "tennis-warehouse": {
      "command": "tennis-warehouse-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No authentication required — Tennis Warehouse's catalog and used inventory are publicly browseable. Run `crawl` to populate the local store; everything else queries that store.

## Quick Start

```bash
# Populate the local SQLite store for Wilson — both new catalog and used inventory.
tennis-warehouse-pp-cli crawl --brand wilson


# Filter the new catalog and emit only the fields an agent needs.
tennis-warehouse-pp-cli racquets list --brand wilson --string-pattern 16x19 --json --select sku,model,strung_weight,swingweight


# Diff three Wilson Blade 98 versions side-by-side.
tennis-warehouse-pp-cli racquets compare WB9810 WB9818 WB9816 --json


# Surface Grade A used listings selling 40%+ below new MSRP.
tennis-warehouse-pp-cli used deals --grade A --min-discount-pct 40 --json


# Watch a model and check for price drops since last sync.
tennis-warehouse-pp-cli used watch WB9810 && tennis-warehouse-pp-cli used drops --since 7d --watchlist-only --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Spec-driven discovery
- **`racquets similar`** — Find current racquets whose specs match a target SKU within a tolerance band — head size, strung weight, swingweight, and exact string pattern.

  _When a player's racquet cracks or is discontinued, picking a replacement requires triangulating across 9 spec fields. This command does it in one call._

  ```bash
  tennis-warehouse-pp-cli racquets similar WB9810 --tolerance tight --json
  ```
- **`racquets compare`** — Render an aligned spec-by-spec table for 2–5 racquets with diff highlighting; --json emits a row-per-spec matrix.

  _Replaces opening 2–5 browser tabs and copy-pasting specs into a spreadsheet._

  ```bash
  tennis-warehouse-pp-cli racquets compare WB9810 WB9818 --json
  ```
- **`used grip-availability`** — Find used units in a specific grip size grouped by model and grade — wrong grip = unplayable, so this is a hard filter most shoppers care about.

  _Saves the shopper from clicking through every model to see if their grip size is even in stock._

  ```bash
  tennis-warehouse-pp-cli used grip-availability --size 4_3/8 --grade A --brand wilson --json
  ```

### Buying signals
- **`used deals`** — Find used listings whose price is a steep discount versus the new-racquet MSRP — joins used inventory against the new catalog.

  _Surfaces the actual bargain hunt — "40% off new for a Grade A unit" is the buy signal, not "$150 vs unknown."_

  ```bash
  tennis-warehouse-pp-cli used deals --min-discount-pct 40 --grade A --brand wilson --json
  ```
- **`used drops`** — List used listings whose latest price snapshot dropped below a prior snapshot beyond a threshold within a time window.

  _Catches deals the website cannot expose because it stores no historical pricing._

  ```bash
  tennis-warehouse-pp-cli used drops --since 7d --min-drop-pct 10 --json
  ```
- **`used new`** — Used listings whose first_seen_at falls within a recent time window — "what is new since I last looked."

  _Used inventory churns fast and good Grade A units sell in hours — the new-arrival feed is how bargain hunters actually shop._

  ```bash
  tennis-warehouse-pp-cli used new --since 7d --brand babolat --json
  ```
- **`used depth`** — Aggregate per-physical-unit counts grouped by model and condition grade — answers "how many Grade A Blade 98s are in stock right now?"

  _Depth is a buying-confidence signal. One unit at Grade A is a gamble; twelve units is a healthy market._

  ```bash
  tennis-warehouse-pp-cli used depth --min-units 3 --grade A --json
  ```
- **`used watch`** — Save SKUs to a local watchlist; view current state; combine with drops to alert on watched-only items.

  _Lets a player track a small set of candidates without manually refreshing pages._

  ```bash
  tennis-warehouse-pp-cli used watchlist drops --since 30d --json
  ```

## Usage

Run `tennis-warehouse-pp-cli --help` for the full command reference and flag list.

## Commands

### racquets

Current (new) racquet catalog across all stocked brands

- **`tennis-warehouse-pp-cli racquets`** - Fetch the all-racquets landing page (featured + best-sellers across every brand)

### used

Used racquet inventory — Grade A/B/C and Unused units across all stocked brands

- **`tennis-warehouse-pp-cli used get`** - Fetch the detail page for a used model (full spec sheet + individual unit listings)
- **`tennis-warehouse-pp-cli used list`** - List used-racquet models stocked for a brand


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tennis-warehouse-pp-cli used list --ccode example-value

# JSON for scripting and agents
tennis-warehouse-pp-cli used list --ccode example-value --json

# Filter to specific fields
tennis-warehouse-pp-cli used list --ccode example-value --json --select id,name,status

# Dry run — show the request without sending
tennis-warehouse-pp-cli used list --ccode example-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tennis-warehouse-pp-cli used list --ccode example-value --agent
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
tennis-warehouse-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/tennis-warehouse-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **`crawl` returns 0 racquets for a brand** — The brand catalog path is `/{Brand}racquets.html` with capitalized brand. Verify the brand slug matches (wilson → Wilson, prokennex → ProKennex, head → Head).
- **`used drops` returns empty** — Price drop detection requires at least two `crawl` runs separated by enough time for the site to update prices. Run `crawl` once, wait a day, crawl again.
- **HTML parsing fails on a specific SKU** — Tennis Warehouse occasionally redesigns individual product pages. Run `crawl <pcode> --debug` to dump the raw HTML and report the SKU in an issue.
- **Rate limited (HTTP 429)** — Crawl is rate-limited to 1 req/sec by default with adaptive backoff. If you still see 429s, run `crawl --rate 0.5` to halve the rate.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
