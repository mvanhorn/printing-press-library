# Flighty Airports CLI

**Every Flighty Airports meltdown map feature, plus cross-airport intelligence no web page can answer.**

Live status, METAR weather, performance, and flight boards for 156 tracked airports — parsed straight from Flighty's SSR payload with no app install, no API key, and no login. Then it goes further than the site: network-wide rankings, airline footprints, route checks, healthy alternates, and change diffs, all from a local SQLite mirror that keeps working offline.

Learn more at [Flighty Airports](https://flighty.com).

Created by [@SomSamantray](https://github.com/SomSamantray) (Som Samantray).

## Install

The recommended path installs both the `flighty-pp-cli` binary and the `pp-flighty` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install flighty
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install flighty --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install flighty --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install flighty --agent claude-code
npx -y @mvanhorn/printing-press-library install flighty --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/flighty/cmd/flighty-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/flighty-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install flighty --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-flighty --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-flighty --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install flighty --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/flighty-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/flighty/cmd/flighty-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "flighty": {
      "command": "flighty-pp-mcp"
    }
  }
}
```

</details>

## Quick Start

```bash
# See which airports are melting down right now
flighty-pp-cli airports list --status MAJOR_ISSUES --json

# Full status, weather (raw METAR), and today's performance for one airport
flighty-pp-cli airports show den --json

# Rank the network — the web map never ranks
flighty-pp-cli airports worst --region Europe --limit 5 --json

# Find one flight across arrivals and departures boards
flighty-pp-cli airports find-flight den UA5072 --json

# Mirror the catalog locally for offline search and cross-airport commands
flighty-pp-cli sync --resources airports --full

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Cross-airport intelligence
- **`airports worst`** — Rank the network's airports right now by cumulative delay and cancellations — the web map color-codes but never ranks.

  _When an agent needs 'which airports are worst right now', this answers in one command instead of scraping the map._

  ```bash
  flighty-pp-cli airports worst --region Europe --limit 5 --json
  ```
- **`airports airline`** — One airline's delay/cancel/divert footprint aggregated across every synced airport — impossible per-airport on the site.

  _Travel agents and ops folks ask 'which airline is the worst offender across my hubs today' — this answers it network-wide._

  ```bash
  flighty-pp-cli airports airline UA --json
  ```
- **`airports compare`** — Side-by-side status, delays, warnings, and flight rules for two airports.

  _Choosing between alternates (SFO vs OAK) is a two-entity question the site cannot express._

  ```bash
  flighty-pp-cli airports compare sfo oak --json
  ```
- **`airports route`** — Both directions of one origin-destination pair's delay/cancel/divert stats, joined from each side's disrupted routes.

  _Route data exists only from one airport's perspective; this shows both ends of the same route._

  ```bash
  flighty-pp-cli airports route sfo den --json
  ```
- **`airports nearby`** — Distance-ranked nearby airports, flagging which have normal operations right now.

  _When your airport is melting down, 'where else can I fly from' is the immediate follow-up._

  ```bash
  flighty-pp-cli airports nearby sfo --healthy-only --limit 3 --json
  ```

### Flight board intelligence
- **`airports find-flight`** — Find one flight by number across arrivals and departures boards — status, original vs actual time, gate, belt, terminal.

  _The 'when is my flight actually leaving' question answered in one deterministic command._

  ```bash
  flighty-pp-cli airports find-flight den UA5072 --json
  ```

### Local state that compounds
- **`airports diff`** — What changed since the last sync: status transitions, new/cleared warnings, delay deltas.

  _Monitors and writers need 'what changed since this morning' — the numbers reset upstream, local history persists._

  ```bash
  flighty-pp-cli airports diff --json
  ```

## Recipes

### Morning meltdown sweep

```bash
flighty-pp-cli airports list --status MAJOR_ISSUES --json --select iata,city,status,warnings
```

One line of JSON naming every airport in serious trouble right now.

### Pre-flight check

```bash
flighty-pp-cli airports show den --json --select airportWeather.flightRulesTitle,today.departurePerformance.onTime.percentage,warnings
```

Flight rules, on-time rate, and active warnings for your departure airport before you leave.

### Find your flight's real departure

```bash
flighty-pp-cli airports find-flight den UA5072 --json
```

Status, original vs actual time, and gate for one flight number across both boards.

### Rank Europe's worst hubs

```bash
flighty-pp-cli airports worst --region Europe --limit 5 --json
```

Magnitude-ranked by cumulative delay and cancellations — the web map never ranks.

### Where else can I fly from

```bash
flighty-pp-cli airports nearby sfo --healthy-only --limit 3 --json
```

Nearest airports with normal operations when your home airport is melting down.

## Usage

Run `flighty-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `FLIGHTY_CONFIG_DIR`, `FLIGHTY_DATA_DIR`, `FLIGHTY_STATE_DIR`, or `FLIGHTY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `FLIGHTY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export FLIGHTY_HOME=/srv/flighty
flighty-pp-cli doctor
```

Under `FLIGHTY_HOME=/srv/flighty`, the four dirs resolve to `/srv/flighty/config`, `/srv/flighty/data`, `/srv/flighty/state`, and `/srv/flighty/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "flighty": {
      "command": "flighty-pp-mcp",
      "env": {
        "FLIGHTY_HOME": "/srv/flighty"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `FLIGHTY_DATA_DIR` overrides an explicit `--home` for that kind. Use `FLIGHTY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `FLIGHTY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `flighty-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### airports

Airport status, delays, weather, and flight boards

- **`flighty-pp-cli airports arrivals`** - Show the live arrivals board for an airport
- **`flighty-pp-cli airports departures`** - Show the live departures board for an airport
- **`flighty-pp-cli airports list`** - List the tracked airports with live delay status (meltdown map catalog)
- **`flighty-pp-cli airports show`** - Show live status, weather, and performance for one airport
- **`flighty-pp-cli airports tv`** - List disrupted airports from the TV dashboard (same catalog, status-sorted)


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`flighty-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`flighty-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`flighty-pp-cli learnings list`** - Inspect taught rows
- **`flighty-pp-cli learnings forget <query>`** - Undo a teach
- **`flighty-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`flighty-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`flighty-pp-cli teach-pattern`** - Install a query/resource template up front
- **`flighty-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `FLIGHTY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `flighty-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
flighty-pp-cli airports list

# JSON for scripting and agents
flighty-pp-cli airports list --json
# Filter to specific fields
flighty-pp-cli airports list --json --select id,slug,name

# Dry run — show the request without sending
flighty-pp-cli airports list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
flighty-pp-cli airports list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Read-only by default** - this CLI does not create, update, delete, publish, send, or mutate remote resources
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
flighty-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `flighty-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/flighty-pp-cli/config.toml`; `--home`, `FLIGHTY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **airports show says airport not found** — Use the IATA code (den) or exact slug (denver-intl-den); run flighty-pp-cli airports list to see tracked airports
- **Cross-airport commands (worst, nearby, airline, diff) return empty** — Run flighty-pp-cli sync --resources airports --full first — those commands read the local mirror
- **Data looks stale** — Re-sync or use --data-source live on commands that support it; Flighty updates the site continuously

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://flighty.com/airports
- Capture coverage: 0 API entries from 6 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: html_scrape (55% confidence)
- Auth signals: none

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**flighty-mcp-server (CPLX)**](https://github.com/CPLX/flighty-mcp-server) — TypeScript
- [**flighty-mcp (LukasHaas)**](https://github.com/LukasHaas/flighty-mcp) — Python
- [**flighty-mcp (sailingnaturali)**](https://github.com/sailingnaturali/flighty-mcp) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
