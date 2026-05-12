# BLS CLI

**The first feature-rich CLI for the BLS Public Data API — with offline series search, a release calendar, footnote decoding, and a one-command macro snapshot that no Python or R wrapper offers.**

BLS publishes hundreds of thousands of labor and economic time series behind packed structural IDs like CUUR0000SA0 and LNS14000000, and the live API has no way to search for them. bls-pp-cli ships a locally-synced series catalog with FTS5 search, a curated U.S. macro snapshot, the release calendar, and footnote decoding — the things every analyst, reporter, and agent needs but no existing wrapper provides. Every command is also an MCP tool, so LLM agents can resolve series IDs and fetch values without a hand-curated dictionary.

Learn more at [BLS](https://www.bls.gov/developers/).

Printed by [@amandahuarng](https://github.com/amandahuarng) (Amanda Huang).

## Install

The recommended path installs both the `bls-pp-cli` binary and the `pp-bls` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install bls
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install bls --cli-only
```


### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/bls/cmd/bls-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bls-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bls --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bls --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-bls skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-bls. The skill defines how its required CLI can be installed.
```

## Optional: API Key

**All core commands work without setup.** The API key below is only needed to unlock additional features.

BLS API works without a key (25 queries/day, 3 series per request, 10 years of history). A free registration key from https://data.bls.gov/registrationEngine/ unlocks 500 queries/day, 50 series per request, 20 years, and the calculations / catalog / annual-average flags. Set with `bls-pp-cli auth set-token <key>` or `BLS_API_KEY=<key>` in your environment.

## Quick Start

```bash
# One-time: stash your free BLS registration key in the local config so every command can use it.
bls-pp-cli auth set-token <your-bls-key>


# Bulk-imports the BLS flat-file series catalog, surveys, areas, items, and footnotes into a local SQLite store. Takes a few minutes; run monthly.
bls-pp-cli sync


# Find the canonical BLS series ID by plain English instead of bouncing through bls.gov/cpi/tables.
bls-pp-cli series search "Los Angeles CPI all items" --json


# Fetch the L.A. CPI-U with server-side YoY/MoM changes.
bls-pp-cli series get CUURA421SA0 --start 2020 --end 2025 --calc


# Pull the current U.S. macro dashboard (CPI, core CPI, U3, payrolls, JOLTS openings, PPI, ECI, productivity) in one call.
bls-pp-cli snapshot macro --csv


# See which BLS releases are coming up so you know when to refresh.
bls-pp-cli releases next --within 14d

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Discovery the live API can't do

- **`series search`** — Find the right BLS series ID by plain-English title, survey, or area without leaving the terminal.

  _Reach for this when you need the canonical BLS series ID for a concept (any indicator, any area, any survey) before fetching values. Without it, agents either hallucinate IDs or rely on a hand-curated dictionary._

  ```bash
  bls-pp-cli series search "Los Angeles CPI all items" --json
  ```
- **`footnotes decode`** — Decode BLS footnote codes (P, R, C, ...) into plain-English explanations.

  _Reach for this when an observation comes back with footnotes you don't recognize — preliminary vs revised vs corrected matters for the analyst writeup._

  ```bash
  bls-pp-cli footnotes decode P R --json
  ```

### Workflow shortcuts

- **`snapshot macro`** — One command returns the current state of the U.S. macro economy: headline + core CPI, U3, payrolls, JOLTS openings, PPI, ECI, productivity, with YoY and MoM changes.

  _Use when you want a single read of the U.S. labor and price-stability picture without composing a 15-series request yourself._

  ```bash
  bls-pp-cli snapshot macro --csv > macro.csv
  ```
- **`releases next`** — List upcoming BLS releases (CPI, employment situation, JOLTS, PPI...) with date, time, and news-release URL.

  _Use to plan around BLS data drops; combine with --watch to poll until the next print lands._

  ```bash
  bls-pp-cli releases next --within 14d --json
  ```

### Local-cache queries

- **`series extremum`** — Compute max, min, and percentile rank of a series's latest observation across a configurable window from the local cache.

  _Use for release-day writeups, agent tool-calls that need historical context, and quickly answering "have we been here before" without scrolling FRED charts._

  ```bash
  bls-pp-cli series extremum LNS14000000 --since 2005 --json
  ```
- **`series compare-sa`** — Show seasonally-adjusted and not-seasonally-adjusted variants of a series side-by-side.

  _Use when you need to disambiguate whether a trend you're seeing is genuine or a seasonal artifact._

  ```bash
  bls-pp-cli series compare-sa CUUR0000SA0 --json
  ```

## Usage

Run `bls-pp-cli --help` for the full command reference and flag list.

## Commands

### series

Fetch BLS time-series observations (CPI, employment, unemployment, JOLTS, PPI, ECI, productivity, and more).

- **`bls-pp-cli series batch`** - Fetch up to 50 BLS series in one call. Pass --ids comma-separated; the CLI partitions IDs >50 across multiple requests.
- **`bls-pp-cli series get`** - Fetch a single BLS time-series by ID.
- **`bls-pp-cli series popular`** - List BLS's most-popular series, optionally filtered by survey.

### surveys

BLS survey directory (CPI, CES, CPS, JOLTS, PPI, ECI, productivity, and more).

- **`bls-pp-cli surveys get`** - Show detail for a single BLS survey (allowed calculations, annual averages, etc.).
- **`bls-pp-cli surveys list`** - List every BLS survey by abbreviation and name.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bls-pp-cli series get mock-value

# JSON for scripting and agents
bls-pp-cli series get mock-value --json

# Filter to specific fields
bls-pp-cli series get mock-value --json --select id,name,status

# Dry run — show the request without sending
bls-pp-cli series get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bls-pp-cli series get mock-value --agent
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
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-bls -g
```

Then invoke `/pp-bls <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/bls/cmd/bls-pp-mcp@latest
```

Then register it:

```bash
claude mcp add bls bls-pp-mcp -e BLS_API_KEY=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bls-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BLS_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/developer-tools/bls/cmd/bls-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "bls": {
      "command": "bls-pp-mcp",
      "env": {
        "BLS_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
bls-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/bls-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BLS_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `bls-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BLS_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **REQUEST_NOT_PROCESSED: daily threshold reached** — Free unauthenticated tier is 25 queries/day. Register at https://data.bls.gov/registrationEngine/ and run `bls-pp-cli auth set-token <key>` for 500/day.
- **Series ID not found by `series search`** — The local catalog may be stale or missing the survey. Run `bls-pp-cli sync` to refresh from BLS flat files.
- **`series get` returns empty data array** — BLS returns empty for IDs that don't exist (no 404). Verify the ID with `bls-pp-cli series search <plain-english>` or `bls-pp-cli sql 'SELECT id FROM series WHERE id = ?'`.
- **Server-side calculations are missing from the response** — `calculations:true` requires a registered key. Set BLS_API_KEY and pass --calc.
- **`sync` fails downloading flat files** — download.bls.gov is bot-walled. The CLI ships a browser-realistic user-agent and a polite rate limit; if you still get 403, run with `--source api-only` to skip flat-file enrichment.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**blscrapeR**](https://github.com/keberwein/blscrapeR) — R (117 stars)
- [**us-gov-open-data-mcp**](https://github.com/lzinga/us-gov-open-data-mcp) — TypeScript (98 stars)
- [**bls (OliverSherouse)**](https://github.com/OliverSherouse/bls) — Python (84 stars)
- [**blsAPI**](https://github.com/mikeasilva/blsAPI) — R (14 stars)
- [**pyBLS**](https://github.com/addisonlynch/pyBLS) — Python (7 stars)
- [**tap-bls**](https://github.com/frasermarlow/tap-bls) — Python (6 stars)
- [**bls-data**](https://github.com/a-finocchiaro/bls-data) — Python (5 stars)
- [**go-bls**](https://github.com/cridenour/go-bls) — Go (3 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
