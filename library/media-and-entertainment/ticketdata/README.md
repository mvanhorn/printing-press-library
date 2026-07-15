# TicketData CLI

**Every TicketData price signal for any event, performer, or venue, plus a local price-history store for trend math, drift alerts, and cross-event comparisons no ticket tool ships.**

TicketData tracks each event's get-in price (the lowest all-in resale price across marketplaces), its full price history, a forecast, and 3/7/14/30-day change. This CLI exposes all of it as scriptable, agent-native JSON and keeps the raw price series in local SQLite, so `stats` can compute historical lows and volatility, `drift` can alert when a floor drops to your target, `board` can rank your whole watchlist, and `compare` can pick the cheapest date, none of which the website surfaces.

## Install

The recommended path installs both the `ticketdata-pp-cli` binary and the `pp-ticketdata` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install ticketdata
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install ticketdata --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install ticketdata --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install ticketdata --agent claude-code
npx -y @mvanhorn/printing-press-library install ticketdata --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketdata/cmd/ticketdata-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ticketdata-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install ticketdata --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-ticketdata --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-ticketdata --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install ticketdata --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/ticketdata-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketdata/cmd/ticketdata-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "ticketdata": {
      "command": "ticketdata-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No account or API key. TicketData's public data API is open; this CLI reads it directly.

## Quick Start

```bash
# Confirm the CLI can reach the TicketData data API.
ticketdata-pp-cli doctor --dry-run

# Resolve a name to its canonical performer and stats.
ticketdata-pp-cli performers search --query "ariana grande"

# Get an event's current get-in price, forecast, and N-day change.
ticketdata-pp-cli events get 22323960 --agent

# Track the event so sync starts building its local history.
ticketdata-pp-cli watch add 22323960

# See the historical low, percentile, and best day to buy.
ticketdata-pp-cli stats 22323960 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local price store that compounds
- **`watch`** — Track a set of events locally so one sync re-fetches all their current prices.

  _Reach for this first: everything downstream (board, drift, stats, compare) reads the watchlist the CLI syncs._

  ```bash
  ticketdata-pp-cli watch add 22323960
  ```
- **`board`** — One sortable table of every watched event: get-in price, N-day change, forecast direction, and where today sits in its own history.

  _Use it for a whole-watchlist snapshot; for what changed since last sync use drift, for one event's distribution use stats._

  ```bash
  ticketdata-pp-cli board --sort change --agent
  ```
- **`drift`** — Diff the two most recent snapshots per watched event, flag floors that moved past a threshold, and fire price-target hits.

  _Use it for what moved since the last sync and for buy-when-it-hits-my-price alerts._

  ```bash
  ticketdata-pp-cli drift --threshold 10 --target 22323960=150 --agent
  ```

### Price intelligence the site chart hides
- **`stats`** — Historical low/high, median, current percentile, volatility, and the weekday the floor is typically lowest, from the local price series.

  _Use it to judge whether today's get-in price is actually low for one event and which day tends to be cheapest._

  ```bash
  ticketdata-pp-cli stats 22323960 --agent
  ```
- **`compare`** — Rank multiple watched events, or all of one performer's watched events, by get-in price and percent change.

  _Use it to pick the cheapest city or date; for a single event's own history use stats._

  ```bash
  ticketdata-pp-cli compare --performer ariana-grande --agent
  ```
- **`zones`** — Rank an event's zones by current get-in price and by each zone's drop versus its own history to surface the underpriced section.

  _Use it to pick a section by value; for the plain section-name catalog use events sections._

  ```bash
  ticketdata-pp-cli zones 22323960 --agent
  ```

### Agent-native plumbing
- **`search`** — Local full-text search across synced events, performers, and venues, returning multiple matches.

  _Use it to browse offline matches; for the single canonical resolve of a name use performers search or venues search._

  ```bash
  ticketdata-pp-cli search "ariana" --type performers --agent
  ```

## Recipes

### Is this ticket cheap right now?

```bash
ticketdata-pp-cli watch add 22323960 && ticketdata-pp-cli sync && ticketdata-pp-cli stats 22323960 --agent
```

Track the event, pull its history, and see where today's get-in price sits in its own distribution.

### Alert me when a floor drops to my price

```bash
ticketdata-pp-cli drift --target 22323960=150 --threshold 8 --agent
```

Flags target hits and any floor that moved more than 8% since the last sync.

### Which date is cheapest for this artist?

```bash
ticketdata-pp-cli compare --performer ariana-grande --agent
```

Ranks all of the performer's watched events by get-in price and percent change.

### Narrow a large event payload for an agent

```bash
ticketdata-pp-cli events get 22323960 --agent --select tickets.get_in_price,tickets.number_of_listings,forecast_value,price_trend.direction
```

Pulls only the decision-relevant fields from the verbose event response using dotted select paths.

### Find the best-value section

```bash
ticketdata-pp-cli zones 22323960 --agent
```

Ranks the event's zones by current get-in price and by each zone's drop versus its own history.

## Usage

Run `ticketdata-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TICKETDATA_CONFIG_DIR`, `TICKETDATA_DATA_DIR`, `TICKETDATA_STATE_DIR`, or `TICKETDATA_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TICKETDATA_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TICKETDATA_HOME=/srv/ticketdata
ticketdata-pp-cli doctor
```

Under `TICKETDATA_HOME=/srv/ticketdata`, the four dirs resolve to `/srv/ticketdata/config`, `/srv/ticketdata/data`, `/srv/ticketdata/state`, and `/srv/ticketdata/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "ticketdata": {
      "command": "ticketdata-pp-mcp",
      "env": {
        "TICKETDATA_HOME": "/srv/ticketdata"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TICKETDATA_DATA_DIR` overrides an explicit `--home` for that kind. Use `TICKETDATA_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TICKETDATA_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `ticketdata-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### events

Look up ticket events, their get-in price, price history, and section catalog

- **`ticketdata-pp-cli events get`** - Get an event's current get-in price, forecast, N-day change, marketplace links, and venue/performer detail
- **`ticketdata-pp-cli events history`** - Get the full get-in-price time series for an event (hundreds of points), plus per-zone series and on/presale dates
- **`ticketdata-pp-cli events sections`** - List the section names catalogued for an event

### performers

Look up performers (artists, teams) and resolve names to canonical performer pages

- **`ticketdata-pp-cli performers get`** - Get a performer's stats (upcoming events, avg/min/max get-in price) and resale/social links
- **`ticketdata-pp-cli performers search`** - Resolve a search query to the best-matching performer

### venues

Look up venues and resolve names to canonical venue pages

- **`ticketdata-pp-cli venues get`** - Get a venue's stats (upcoming events, location)
- **`ticketdata-pp-cli venues search`** - Resolve a search query to the best-matching venue


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`ticketdata-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`ticketdata-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`ticketdata-pp-cli learnings list`** - Inspect taught rows
- **`ticketdata-pp-cli learnings forget <query>`** - Undo a teach
- **`ticketdata-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`ticketdata-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`ticketdata-pp-cli teach-pattern`** - Install a query/resource template up front
- **`ticketdata-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `TICKETDATA_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `ticketdata-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
ticketdata-pp-cli events get mock-value

# JSON for scripting and agents
ticketdata-pp-cli events get mock-value --json

# Filter to specific fields
ticketdata-pp-cli events get mock-value --json --select id,name,status

# Dry run — show the request without sending
ticketdata-pp-cli events get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
ticketdata-pp-cli events get mock-value --agent
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
ticketdata-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `ticketdata-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/ticketdata/config.toml`; `--home`, `TICKETDATA_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **A command returns an empty board or 'no data' for stats/drift/compare** — Run `ticketdata-pp-cli sync` first; these read the local store, which is empty until you watch events and sync.
- **Intermittent HTTP 403 / Cloudflare challenge** — Retry; the CLI ships a Chrome-fingerprint HTTP transport that clears it. Persistent 403s usually mean a transient challenge, not a block.
- **events get returns found=false** — The event id no longer exists on TicketData. Re-resolve it with `performers search` then look up the performer's current events on the site.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
