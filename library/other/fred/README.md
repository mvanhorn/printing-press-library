# FRED CLI

**Every FRED endpoint, plus a local SQLite store, offline search, and macro commands no other FRED tool has.**

Search and pull U.S. and global economic time series from the St. Louis Fed's FRED API. Beyond mirroring every endpoint, it adds a macro dashboard, multi-series compare, a latest-value shortcut, a persistent watchlist that reports what changed, and a release calendar — all agent-native with --json and --select.

## Install

The recommended path installs both the `fred-pp-cli` binary and the `pp-fred` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install fred
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install fred --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install fred --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install fred --agent claude-code
npx -y @mvanhorn/printing-press-library install fred --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/fred/cmd/fred-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fred-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install fred --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-fred --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-fred --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install fred --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/fred-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `FRED_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/fred/cmd/fred-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "fred": {
      "command": "fred-pp-mcp",
      "env": {
        "FRED_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Check the API key and reachability before anything else.
fred-pp-cli doctor --dry-run

# Find a series ID by text.
fred-pp-cli series search 'unemployment rate'

# Pull the last 12 monthly observations.
fred-pp-cli series observations UNRATE --sort-order desc --limit 12

# Snapshot the headline macro indicators in one call.
fred-pp-cli dashboard --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Aggregation only we do
- **`dashboard`** — Latest value of a curated set of headline U.S. indicators (unemployment, CPI, GDP, fed funds, 10Y, payrolls) in one call.

  _Reach for this when an agent needs a quick read on the U.S. economy without choosing and fetching individual series IDs._

  ```bash
  fred-pp-cli dashboard --json
  ```
- **`series compare`** — Pull observations for multiple series and align them by date into one table or JSON for correlation.

  _Use when comparing or correlating two or more indicators over the same window._

  ```bash
  fred-pp-cli series compare UNRATE CPIAUCSL --start 2020-01-01 --json
  ```

### Agent-native shortcuts
- **`series latest`** — The single most recent observation (date + value) for a series, as a one-liner.

  _Use when you only need the current print of an indicator, not its history._

  ```bash
  fred-pp-cli series latest UNRATE --json
  ```

### Local state that compounds
- **`watchlist sync`** — Persist a set of series locally, sync their latest observations into SQLite, and report which ones moved since the last sync.

  _Use to track a personal set of indicators and surface only the ones that changed._

  ```bash
  fred-pp-cli watchlist sync --json
  ```
- **`release calendar`** — Recent and upcoming economic data release dates within a day window, aggregated across all releases.

  _Use to see what economic data is dropping soon without scanning hundreds of releases._

  ```bash
  fred-pp-cli release calendar --days 7 --json
  ```

## Recipes


### Current unemployment rate

```bash
fred-pp-cli series latest UNRATE --json
```

One-liner for the most recent print of any indicator.

### Year-over-year CPI inflation

```bash
fred-pp-cli series observations CPIAUCSL --units pc1 --sort-order desc --limit 12 --json --select observations.date,observations.value
```

Uses FRED's pc1 transform for YoY percent change and --select to trim the payload.

### Compare unemployment and inflation

```bash
fred-pp-cli series compare UNRATE CPIAUCSL --start 2020-01-01 --json
```

Aligns multiple series by date into one structure for correlation.

### This week's data releases

```bash
fred-pp-cli release calendar --days 7 --json
```

Windowed view of upcoming and recent economic releases.

## Usage

Run `fred-pp-cli --help` for the full command reference and flag list.

## Commands

### category

Browse the FRED category tree

- **`fred-pp-cli category children`** - List child categories under a category
- **`fred-pp-cli category get`** - Get a category by ID (root category is 0)
- **`fred-pp-cli category related`** - List categories related to a category
- **`fred-pp-cli category series`** - List the series within a category
- **`fred-pp-cli category tags`** - List tags for the series in a category

### release

Data releases and their schedules

- **`fred-pp-cli release dates`** - List release dates for all releases (the economic release calendar)
- **`fred-pp-cli release get`** - Get a single release by ID
- **`fred-pp-cli release list`** - List all data releases on FRED
- **`fred-pp-cli release release-dates`** - List release dates for a single release
- **`fred-pp-cli release series`** - List the series in a release
- **`fred-pp-cli release sources`** - List the sources for a release

### series

Economic data series — search, metadata, and observations

- **`fred-pp-cli series categories`** - List the categories a series belongs to
- **`fred-pp-cli series get`** - Get metadata for a single series by ID (e.g. UNRATE, GDP, CPIAUCSL)
- **`fred-pp-cli series observations`** - Pull the observation values (the actual time series) for a series
- **`fred-pp-cli series release`** - Get the release that a series belongs to
- **`fred-pp-cli series search`** - Search for series by full-text query (e.g. 'unemployment rate')
- **`fred-pp-cli series tags`** - List FRED tags attached to a series
- **`fred-pp-cli series updates`** - List series recently updated on FRED
- **`fred-pp-cli series vintagedates`** - List real-time vintage dates for a series (when data was revised)

### source

Sources of economic data

- **`fred-pp-cli source get`** - Get a single source by ID
- **`fred-pp-cli source list`** - List all sources of economic data on FRED
- **`fred-pp-cli source releases`** - List the releases for a source

### tags

Discover series via FRED tags

- **`fred-pp-cli tags list`** - List FRED tags, optionally filtered
- **`fred-pp-cli tags related`** - List tags related to a set of tags
- **`fred-pp-cli tags series`** - List series matching a set of tags


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
fred-pp-cli category get 0

# JSON for scripting and agents
fred-pp-cli category get 0 --json

# Filter to specific fields
fred-pp-cli category get 0 --json --select id,name,status

# Dry run — show the request without sending
fred-pp-cli category get 0 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
fred-pp-cli category get 0 --agent
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
fred-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/fred-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `FRED_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `fred-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `fred-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $FRED_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **400 Bad Request: api_key required** — Set FRED_API_KEY (free key at https://fredaccount.stlouisfed.org/apikeys).
- **Response is XML instead of JSON** — Pass --file-type json (it is the default; only an explicit override changes it).
- **Series not found** — Use 'series search <text>' to find the exact series ID; IDs are case-sensitive (e.g. UNRATE, CPIAUCSL).

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**fredapi**](https://github.com/mortada/fredapi) — Python (1000 stars)
- [**fredr**](https://github.com/sboysel/fredr) — R (140 stars)
- [**pyfredapi**](https://github.com/gw-moore/pyfredapi) — Python (60 stars)
- [**node-fred**](https://github.com/ScottRFrost/node-fred) — JavaScript (40 stars)
- [**fred-mcp-server**](https://github.com/stefanoamorelli/fred-mcp-server) — TypeScript (30 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
