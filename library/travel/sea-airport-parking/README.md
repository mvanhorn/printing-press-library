# SEA Airport Parking CLI

**Check, track, and get alerted on SEA's official on-airport Reserved garage — with a local price/availability history and sold-out watch that no site or aggregator has.**

reservesea.portseattle.org is the only place to book SEA's Floor-4 Reserved garage, and it keeps no history and offers no alerts. This CLI quotes availability and price for any date range over plain HTTP (no login, no browser), persists every quote to a local SQLite store, and adds what the site can't: sweep flexible dates for the cheapest open stay, watch a sold-out range until it frees up, and see how a range's price has drifted over time.

## Install

The recommended path installs both the `sea-airport-parking-pp-cli` binary and the `pp-sea-airport-parking` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install sea-airport-parking
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install sea-airport-parking --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install sea-airport-parking --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install sea-airport-parking --agent claude-code
npx -y @mvanhorn/printing-press-library install sea-airport-parking --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/cmd/sea-airport-parking-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sea-airport-parking-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install sea-airport-parking --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sea-airport-parking --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sea-airport-parking --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install sea-airport-parking --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sea-airport-parking-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/sea-airport-parking/cmd/sea-airport-parking-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sea-airport-parking": {
      "command": "sea-airport-parking-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No account or API key is required. Availability, pricing, and even booking use guest checkout, so every read command works anonymously. The CLI bootstraps a fresh session cookie automatically on each quote.

## Quick Start

```bash
# confirm the site is reachable before quoting
sea-airport-parking-pp-cli doctor --dry-run

# quote a specific date range (price + availability)
sea-airport-parking-pp-cli quote --entry 2026-08-15T11:00 --exit 2026-08-18T11:00

# find the cheapest open 3-night stay in a flexible window
sea-airport-parking-pp-cli sweep --from 2026-08-10 --to 2026-08-20 --nights 3

# poll a sold-out Thanksgiving range until a space opens
sea-airport-parking-pp-cli watch --entry 2026-11-25T11:00 --exit 2026-11-30T11:00 --interval 30m --notify

# see how that range's price moved across your stored quotes
sea-airport-parking-pp-cli drift --entry 2026-08-15T11:00 --exit 2026-08-18T11:00

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Ranging over dates
- **`sweep`** — Search a flexible date window for the cheapest or any-available Reserved stay.

  _Reach for this when the trip dates are soft and the agent needs the single best-priced open option._

  ```bash
  sea-airport-parking-pp-cli sweep --from 2026-08-10 --to 2026-08-20 --nights 3 --agent
  ```
- **`watch`** — Poll a sold-out or target date range until Reserved opens up (or drops below a price), then notify.

  _Reach for this when a peak-date range is sold out and the user must grab a space the instant it frees up._

  ```bash
  sea-airport-parking-pp-cli watch --entry 2026-11-25T11:00 --exit 2026-11-30T11:00 --max-polls 1 --agent
  ```
- **`calendar`** — Render price and open/sold-out across many entry days for a fixed-length stay.

  _Reach for this to eyeball which entry days in a month are open and how price varies day to day._

  ```bash
  sea-airport-parking-pp-cli calendar --month 2026-11 --nights 3 --max-scan-days 6 --agent
  ```

### Ranging over time
- **`history`** — List every recorded quote snapshot for a date range: price, availability, promo, captured time.

  _Reach for this when the agent needs the raw recorded time series for a range before reasoning about it._

  ```bash
  sea-airport-parking-pp-cli history --entry 2026-08-15T11:00 --exit 2026-08-18T11:00 --agent
  ```
- **`drift`** — Summarize how a range's price and availability moved across stored snapshots: first vs latest, min/max, sold-out flips.

  _Reach for this to tell whether today's price for a range is high or low relative to its own history._

  ```bash
  sea-airport-parking-pp-cli drift --entry 2026-08-15T11:00 --exit 2026-08-18T11:00 --agent
  ```

## Recipes

### Quote a specific trip

```bash
sea-airport-parking-pp-cli quote --entry 2026-08-15T11:00 --exit 2026-08-18T11:00
```

Returns total price, the nightly breakdown, and whether Reserved is available for the exact range.

### Find the cheapest open stay (agent-narrowed)

```bash
sea-airport-parking-pp-cli sweep --from 2026-08-10 --to 2026-08-20 --nights 3 --agent --select entry,exit,total_price,available
```

Sweeps a flexible window and returns only the fields an agent needs from each candidate, avoiding the verbose per-date payload.

### Watch a sold-out holiday range

```bash
sea-airport-parking-pp-cli watch --entry 2026-11-25T11:00 --exit 2026-11-30T11:00 --interval 30m --notify
```

Polls the sold-out Thanksgiving range every 30 minutes and notifies the moment Reserved becomes available.

### Is today's price high or low for this range?

```bash
sea-airport-parking-pp-cli drift --entry 2026-08-15T11:00 --exit 2026-08-18T11:00
```

Summarizes first-vs-latest price and any sold-out flips from your stored snapshots for that range.

### Prepare a booking handoff

```bash
sea-airport-parking-pp-cli book --quote --entry 2026-08-15T11:00 --exit 2026-08-18T11:00
```

Builds the prefilled guest-checkout link/params and stops before payment so you finish the card step yourself.

## Usage

Run `sea-airport-parking-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `SEA_AIRPORT_PARKING_CONFIG_DIR`, `SEA_AIRPORT_PARKING_DATA_DIR`, `SEA_AIRPORT_PARKING_STATE_DIR`, or `SEA_AIRPORT_PARKING_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `SEA_AIRPORT_PARKING_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export SEA_AIRPORT_PARKING_HOME=/srv/sea-airport-parking
sea-airport-parking-pp-cli doctor
```

Under `SEA_AIRPORT_PARKING_HOME=/srv/sea-airport-parking`, the four dirs resolve to `/srv/sea-airport-parking/config`, `/srv/sea-airport-parking/data`, `/srv/sea-airport-parking/state`, and `/srv/sea-airport-parking/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "sea-airport-parking": {
      "command": "sea-airport-parking-pp-mcp",
      "env": {
        "SEA_AIRPORT_PARKING_HOME": "/srv/sea-airport-parking"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `SEA_AIRPORT_PARKING_DATA_DIR` overrides an explicit `--home` for that kind. Use `SEA_AIRPORT_PARKING_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `SEA_AIRPORT_PARKING_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `sea-airport-parking-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### parking

SEA on-airport Reserved garage parking (reservesea.portseattle.org)

- **`sea-airport-parking-pp-cli parking`** - Fetch the live SEA parking booking page (product card + date-entry form)


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`sea-airport-parking-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`sea-airport-parking-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`sea-airport-parking-pp-cli learnings list`** - Inspect taught rows
- **`sea-airport-parking-pp-cli learnings forget <query>`** - Undo a teach
- **`sea-airport-parking-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`sea-airport-parking-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`sea-airport-parking-pp-cli teach-pattern`** - Install a query/resource template up front
- **`sea-airport-parking-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `SEA_AIRPORT_PARKING_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `sea-airport-parking-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sea-airport-parking-pp-cli parking

# JSON for scripting and agents
sea-airport-parking-pp-cli parking --json

# Filter to specific fields
sea-airport-parking-pp-cli parking --json --select id,name,status

# Dry run — show the request without sending
sea-airport-parking-pp-cli parking --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sea-airport-parking-pp-cli parking --agent
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
sea-airport-parking-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `sea-airport-parking-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/sea-airport-parking-pp-cli/config.toml`; `--home`, `SEA_AIRPORT_PARKING_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **"minimum 2 day stay is required"** — SEA Reserved requires at least a 2-day stay; widen your exit date.
- **"must be made at least 6 hours in advance"** — Entry must be 6+ hours out; pick a later entry time.
- **No availability / SOLD OUT for your dates** — Reserved sells out before peak dates; use 'watch' to be alerted when it opens, or 'sweep' to find nearby open dates.
- **Dates more than 120 days out return an error** — SEA only sells Reserved up to 120 days ahead; quote once inside that window.
