# Bandsintown CLI

**Turn Bandsintown's two read-only endpoints into a tour-routing brain: local cache, calendar-gap detection, lineup co-bill mining, and tracker-count trends.**

Every existing Bandsintown wrapper is a 10-year-old language binding for the same two API endpoints. This CLI absorbs all of them and adds the queries promoters and tour routers actually need: feasibility-ranked routing candidates for a target city and date, empty windows in an artist's calendar, co-bill patterns across past festivals, and tracker_count trends over time. Local SQLite store, FTS search, agent-native --json output, and MCP exposure are standard.

Learn more at [Bandsintown](https://bandsintown.com/).

## Install

The recommended path installs both the `bandsintown-pp-cli` binary and the `pp-bandsintown` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install bandsintown
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install bandsintown --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install bandsintown --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install bandsintown --agent claude-code
npx -y @mvanhorn/printing-press install bandsintown --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/bandsintown/cmd/bandsintown-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bandsintown-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bandsintown --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bandsintown --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-bandsintown skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-bandsintown. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bandsintown-current).
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
    "bandsintown": {
      "command": "bandsintown-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Bandsintown's API is partner-only as of 2025 — a free 'pick any app_id' string no longer works. Apply for partner access at help.bandsintown.com, then set BANDSINTOWN_APP_ID in your environment.

## Quick Start

```bash
# Bandsintown's API requires an approved partner key as of 2025
export BANDSINTOWN_APP_ID=your-partner-key

# Verify auth and reachability before doing anything else
bandsintown-pp-cli doctor

# Build a local watchlist; every routing query reads from it
bandsintown-pp-cli track add "Phoenix" "Beach House" "Tame Impala"

# Pull every tracked artist's events into the local store
bandsintown-pp-cli pull --tracked

# Find routing candidates for your target Jakarta date
bandsintown-pp-cli route --to "Jakarta,ID" --on 2026-08-15 --window 7d --tracked --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Routing intelligence
- **`route`** — Find which tracked artists already have shows near a target city within a date window — feasibility-ranked routing candidates for booking.

  _When booking an event in a specific city, the cheapest path is finding an artist already routing through the region. Reach for this before any cold outreach._

  ```bash
  bandsintown-pp-cli route --to "Jakarta,ID" --on 2026-08-15 --window 7d --tracked --json
  ```
- **`gaps`** — Find empty windows in an artist's tour calendar that match an event slot length, optionally constrained to a region.

  _Use this to identify artists with a touring gap that aligns with your event date — the most actionable booking lead._

  ```bash
  bandsintown-pp-cli gaps "Beach House" --min 5d --max 21d --in SEA --json
  ```
- **`sea-radar`** — One-shot Southeast Asia briefing: all upcoming shows in a date range across tracked artists, grouped by city, tagged with tracker tier.

  _Monday-morning briefing in one invocation: who is touring SEA in your event window, ranked by demand._

  ```bash
  bandsintown-pp-cli sea-radar --date 2026-08-01,2026-08-31 --tier mid --json
  ```

### Lineup intelligence
- **`lineup co-bill`** — Surface which artists frequently co-bill with a given artist by aggregating lineup arrays across many events.

  _When building festival lineups, find natural co-bill pairings backed by real shared-stage history._

  ```bash
  bandsintown-pp-cli lineup co-bill "Phoenix" --since 2024-01-01 --min-shared 2 --json
  ```

### Demand intelligence
- **`trend`** — Track tracker_count and upcoming_event_count over time per artist; surface rising and falling demand signals.

  _Rising tracker_count is a leading indicator of audience demand; use to decide who to book before peers notice._

  ```bash
  bandsintown-pp-cli trend --top 20 --period 30d --json
  ```

### Agent-native plumbing
- **`pull`** — Re-fetch every tracked artist's events with a staleness window; emit a structured diff (added / removed / changed events) for downstream agents.

  _Run this in a daily cron; pipe the diff into Slack or a project tracker to alert when tracked artists add new shows._

  ```bash
  bandsintown-pp-cli pull --tracked --since-stale 12 --json
  ```

### Local state that compounds
- **`track`** — Local watchlist of artists you care about. Drives sync, snapshot, route, and sea-radar.

  _Build a curated watchlist once; every other intelligence command reads from it._

  ```bash
  bandsintown-pp-cli track add "Phoenix" "Tame Impala" "Beach House"
  ```

## Usage

Run `bandsintown-pp-cli --help` for the full command reference and flag list.

## Commands

### artists

Manage artists

- **`bandsintown-pp-cli artists artist`** - Get artist information

### events

Manage events

- **`bandsintown-pp-cli events artist_events`** - Get upcoming, past, or all artist events, or events within a date range

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bandsintown-pp-cli artists mock-value --app-id 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
bandsintown-pp-cli artists mock-value --app-id 550e8400-e29b-41d4-a716-446655440000 --json

# Filter to specific fields
bandsintown-pp-cli artists mock-value --app-id 550e8400-e29b-41d4-a716-446655440000 --json --select id,name,status

# Dry run — show the request without sending
bandsintown-pp-cli artists mock-value --app-id 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bandsintown-pp-cli artists mock-value --app-id 550e8400-e29b-41d4-a716-446655440000 --agent
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
bandsintown-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Config file: `~/.config/bandsintown-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **403 "explicit deny" on every request** — Your BANDSINTOWN_APP_ID is missing, expired, or not a partner-issued key. Apply at help.bandsintown.com — the free app_id model was deprecated.
- **Artist name with special chars returns 404** — The CLI double-escapes /, ?, *, and " per the spec. If you're hitting 404, check the artist exists on bandsintown.com first.
- **`route --tracked` returns empty** — Run `bandsintown-pp-cli pull --tracked` first to populate the local store, and check `track list` is non-empty.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**subdigital/intown**](https://github.com/subdigital/intown) — Ruby
- [**aroscoe/bandsintown**](https://github.com/aroscoe/bandsintown) — Python
- [**chrisforrette/python-bandsintown**](https://github.com/chrisforrette/python-bandsintown) — Python
- [**bandsintown/api-gem**](https://github.com/bandsintown/api-gem) — Ruby
- [**TappNetwork/php-sdk-bands-in-town-api**](https://github.com/TappNetwork/php-sdk-bands-in-town-api) — PHP
- [**tobysimone/node-bandsintown**](https://github.com/tobysimone/node-bandsintown) — JavaScript
- [**adamcumiskey/BandsintownAPI**](https://github.com/adamcumiskey/BandsintownAPI) — Objective-C

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
