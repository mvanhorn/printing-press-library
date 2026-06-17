# World Bank CLI

**Every World Bank indicator, queryable offline, with cross-country SQL and rankings no wrapper has.**

A single-binary CLI for the World Bank Open Data API. Mirrors the ~16,000-indicator catalog and observations into local SQLite for offline search, then adds cross-country compare, rankings, trend stats, and pipeline exports the Python wrappers and MCP shims don't offer. No API key — World Bank is fully public.

## Install

The recommended path installs both the `world-bank-pp-cli` binary and the `pp-world-bank` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install world-bank
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install world-bank --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install world-bank --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install world-bank --agent claude-code
npx -y @mvanhorn/printing-press-library install world-bank --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/world-bank/cmd/world-bank-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/world-bank-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install world-bank --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-world-bank --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-world-bank --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install world-bank --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/world-bank-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/world-bank/cmd/world-bank-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "world-bank": {
      "command": "world-bank-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# confirm the API is reachable (no key needed)
world-bank-pp-cli doctor --dry-run

# discover the indicator code you want
world-bank-pp-cli indicators find "gdp"

# pull the time-series
world-bank-pp-cli data USA NY.GDP.MKTP.CD --date 2010:2024

# compare across countries
world-bank-pp-cli compare NY.GDP.MKTP.CD USA,CHN,IND --date 2024

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Offline catalog
- **`indicators find`** — Search the full ~16,000-indicator catalog by keyword in a single call.

  _Agents discover the exact indicator code before fetching values, in one local call._

  ```bash
  world-bank-pp-cli indicators find "co2 emissions" --json
  ```

### Local joins
- **`compare`** — Line up one indicator across many countries in a single aligned table with deltas vs a baseline.

  _One call answers 'how do these economies compare' without N separate fetches._

  ```bash
  world-bank-pp-cli compare NY.GDP.MKTP.CD USA,CHN,IND --date 2024 --agent
  ```
- **`trend`** — CAGR, year-over-year change, min/max/latest for one country+indicator series.

  _Agents get the trajectory summary directly instead of parsing decades of rows._

  ```bash
  world-bank-pp-cli trend USA NY.GDP.MKTP.CD --window 10 --agent
  ```
- **`rank`** — Rank all economies by an indicator for a year, filtered by region or income level.

  _Find leaders/laggards on any indicator in one call._

  ```bash
  world-bank-pp-cli rank NY.GDP.PCAP.CD --year 2024 --top 10 --income HIC --agent
  ```

### Data pipeline
- **`export`** — Bulk country x indicator pull emitted as pipeline-ready wide or long CSV.

  _Drops straight into notebooks, sheets, and ETL without reshaping._

  ```bash
  world-bank-pp-cli export USA,CHN NY.GDP.MKTP.CD,SP.POP.TOTL --wide --csv
  ```
- **`data diff`** — Compare a fresh pull against the last synced snapshot to surface revised observations.

  _Detect World Bank data revisions between vintages automatically._

  ```bash
  world-bank-pp-cli data diff USA NY.GDP.MKTP.CD --agent
  ```

## Recipes


### Find then fetch

```bash
world-bank-pp-cli indicators find "life expectancy" --json --select id,name
```

Discover the code, then pass it to data fetch.

### Compact cross-country GDP

```bash
world-bank-pp-cli compare NY.GDP.MKTP.CD USA,CHN,IND,DEU --date 2024 --agent --select country.value,date,value
```

Narrow a deeply nested response to just the fields that matter.

### Top economies by GDP per capita

```bash
world-bank-pp-cli rank NY.GDP.PCAP.CD --year 2024 --top 10 --income HIC
```

Rank and filter in one call.

### Pipeline export

```bash
world-bank-pp-cli export USA,CHN NY.GDP.MKTP.CD,SP.POP.TOTL --wide --csv
```

Wide CSV ready for a notebook.

## Usage

Run `world-bank-pp-cli --help` for the full command reference and flag list.

## Commands

### countries

Economies (countries and aggregates) with region, income level, lending type

- **`world-bank-pp-cli countries get`** - Get one economy by ISO code (e.g. USA, BRA)
- **`world-bank-pp-cli countries list`** - List economies

### data

Indicator observations (the time-series). Polished output via hand-authored fetch.

- **`world-bank-pp-cli data <country> <indicator>`** - Fetch indicator observations for one or more countries (raw envelope)

### income_levels

Income level classifications

- **`world-bank-pp-cli income-levels`** - List income levels

### indicators

The ~16,000 indicator catalog (series definitions)

- **`world-bank-pp-cli indicators get`** - Get indicator metadata by code (e.g. NY.GDP.MKTP.CD)
- **`world-bank-pp-cli indicators list`** - List indicators

### lending_types

Lending type classifications

- **`world-bank-pp-cli lending-types`** - List lending types

### regions

Regions

- **`world-bank-pp-cli regions`** - List regions

### sources

Data sources (e.g. World Development Indicators)

- **`world-bank-pp-cli sources`** - List data sources

### topics

Topics (e.g. Health, Education) and their indicators

- **`world-bank-pp-cli topics indicators`** - List indicators under a topic
- **`world-bank-pp-cli topics list`** - List topics


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
world-bank-pp-cli countries list

# JSON for scripting and agents
world-bank-pp-cli countries list --json

# Filter to specific fields
world-bank-pp-cli countries list --json --select id,name,status

# Dry run — show the request without sending
world-bank-pp-cli countries list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
world-bank-pp-cli countries list --agent
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
world-bank-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/world-bank-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **Empty data array for a country+indicator** — Not all indicators cover all economies; try --mrv 5 or a wider --date range.
- **Aggregate rows mixed with countries** — Use --income or --region filters, or filter by countryiso3code in --select.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**wbdata**](https://github.com/OliverSherouse/wbdata) — Python (280 stars)
- [**wbgapi**](https://github.com/tgherzog/wbgapi) — Python (230 stars)
- [**world_bank_data**](https://github.com/mwouts/world_bank_data) — Python (110 stars)
- [**world_bank_mcp_server**](https://github.com/anshumax/world_bank_mcp_server) — Python (30 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
