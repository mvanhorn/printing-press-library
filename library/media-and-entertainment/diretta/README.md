# Diretta.it CLI

**The only Go CLI for Diretta.it — live scores, standings, and H2H with offline SQLite cache, MCP server, and 30+ sports in one binary.**

diretta-cli wraps the FlashScore Ninja API with a custom delimiter parser, stores results in SQLite for offline access, and exposes all data via --json for scripting and via an MCP server for AI agents. No Python, no browser, no API key — just a single static binary that covers calcio, tennis, basketball, and 30+ other sports.

Learn more at [Diretta.it](https://local-global.flashscore.ninja).

## Install

The recommended path installs both the `diretta-pp-cli` binary and the `pp-diretta` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install diretta
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install diretta --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/diretta-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-diretta --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-diretta --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-diretta skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-diretta. The skill defines how its required CLI can be installed.
```

## Authentication

No authentication required. The API uses a public static header (x-fsign: SW9D1eZo) shared across the FlashScore community. Set once in config; never expires.

## Quick Start

```bash
# Pull today's matches and standings into local SQLite cache
diretta sync


# Stream today's football matches as JSON
diretta matches today --sport calcio --json


# Roma's last 10 home results from cache
diretta form "Roma" --last 10 --home


# Full pregame H2H+odds report for tomorrow's Serie A round
diretta pregame serie-a --date domani

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`form`** — Shows a team's last N results with goals, cards, and venue split — home vs. away — from local cache.

  _AI agents can pull a team's recent home/away form to contextualize predictions or generate pre-match briefs without scraping individual pages._

  ```bash
  diretta form "Roma" --last 10 --home
  ```
- **`standings-trend`** — Shows how each team's league position changed over the last N weeks using timestamped SQLite snapshots.

  _Lets agents detect rising/falling teams for momentum-based analysis without re-scraping historical data._

  ```bash
  diretta standings-trend serie-a --weeks 4
  ```
- **`team-competitions`** — Lists every active competition a team is playing in, with the next fixture date for each — league, cup, and European.

  _Agents can determine a team's full schedule density across all competitions to reason about fixture congestion or rotation risk._

  ```bash
  diretta team-competitions "Napoli"
  ```

### Cross-source pivots
- **`h2h`** — Shows historical head-to-head results alongside live odds for the next scheduled match between two teams.

  _Lets agents compare bookmaker odds against historical H2H outcomes in a single tool call — the core scouting workflow for sports AI._

  ```bash
  diretta h2h "Inter" "Milan" --odds
  ```
- **`pregame`** — For every match in a league on a given date, shows last-3 H2H and current odds in a single table.

  _An AI agent can call this once to get a complete pre-match intelligence package for an entire round of fixtures._

  ```bash
  diretta pregame serie-a --date domani
  ```

### Bulk data access
- **`export`** — Exports every match, statistic, and event for a full season of a competition as CSV or JSON.

  _AI agents can load a full season dataset for training, analysis, or article generation with one command instead of scraping hundreds of match pages._

  ```bash
  diretta export serie-a --season 2024-25 --format csv
  ```

### Data hygiene
- **`sync-status`** — Reports the age, row counts, and freshness of every SQLite table so automation scripts can detect stale data before acting.

  _Agents that depend on cached sports data can call this as a precondition check before making predictions or reports._

  ```bash
  diretta sync-status
  ```

### Developer tooling
- **`raw`** — Dumps the raw FlashScore field-value pairs for any match ID, bypassing the normalizer — for debugging and protocol discovery.

  _Developers extending the CLI can inspect undocumented fields to build new commands without reverse-engineering the wire format manually._

  ```bash
  diretta raw Abc123Xyz --fields AA,DE,DF,CX,GL
  ```

## Usage

Run `diretta-pp-cli --help` for the full command reference and flag list.

## Commands

### match

Individual match details

- **`diretta-pp-cli match detail`** - Match general info, score, and basic data. FlashScore delimiter format.
- **`diretta-pp-cli match events`** - Match events: goals, cards, substitutions. FlashScore delimiter format.
- **`diretta-pp-cli match h2h`** - Head-to-head history between the two teams. FlashScore delimiter format.
- **`diretta-pp-cli match lineups`** - Team lineups and formations. FlashScore delimiter format.
- **`diretta-pp-cli match stats`** - Match statistics: possession, shots, corners, xG. FlashScore delimiter format.

### matches

Football matches and results

- **`diretta-pp-cli matches live`** - Live matches filtered from today's feed by status. FlashScore delimiter format.
- **`diretta-pp-cli matches today`** - Today's football matches. Response: FlashScore custom delimiter format (parsed in Phase 3).
- **`diretta-pp-cli matches tomorrow`** - Tomorrow's football fixtures. FlashScore delimiter format.
- **`diretta-pp-cli matches yesterday`** - Yesterday's football results. FlashScore delimiter format.

### odds

Match betting odds via GraphQL

- **`diretta-pp-cli odds match_odds`** - Betting odds for a match: 1X2, Under/Over, Asian handicap.

### sports

Multi-sport feeds covering 30+ sports

- **`diretta-pp-cli sports all_today`** - All sports today: calcio, tennis, basket, hockey, etc. FlashScore delimiter format.

### standings

Tournament standings and league tables

- **`diretta-pp-cli standings table`** - League table for a tournament. FlashScore delimiter format.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
diretta-pp-cli match detail mock-value

# JSON for scripting and agents
diretta-pp-cli match detail mock-value --json

# Filter to specific fields
diretta-pp-cli match detail mock-value --json --select id,name,status

# Dry run — show the request without sending
diretta-pp-cli match detail mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
diretta-pp-cli match detail mock-value --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-diretta -g
```

Then invoke `/pp-diretta <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add diretta diretta-pp-mcp
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/diretta-current).
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
    "diretta": {
      "command": "diretta-pp-mcp"
    }
  }
}
```

</details>

## Health Check

```bash
diretta-pp-cli doctor
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

- **x-fsign token rejected (403)** — Re-extract the token from diretta.it JS source: grep 'x-fsign' in browser DevTools → Network → any ninja request header
- **Empty feed for a sport or league** — Check the locale code: Italian-market feeds use locale 'it'; run diretta raw with --fields AA,AB to inspect record structure
- **Standings shows no data** — Standings requires tournament ID and season; run diretta sync first, then diretta standings-trend to confirm snapshots are populated

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Capture coverage: 0 API entries from 0 total network entries
- Reachability: standard_http (95% confidence)
- Protocols: custom_delimited (99% confidence), rest_json (0% confidence)
- Generation hints: custom_delimiter_parser_required, no_auth_needed, static_fsign_token

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**livescore**](https://github.com/nicholasgasior/livescore) — JavaScript (200 stars)
- [**flashscore.py**](https://github.com/yagop/flashscore.py) — Python (50 stars)
- [**livescore-api**](https://github.com/sindicuab/livescore-api) — JavaScript (15 stars)
- [**diretta-it-scraper**](https://github.com/GabMar/diretta-it-scraper) — Go (3 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
