# Airport Limousine CLI

**Plan Tokyo airport buses with exact terminals, dated fares and explicit travel-time uncertainty.**

Find routes and stops, inspect a JST service date, compare current duration evidence and plan party fares. Every result retains the provider source and observation time.

Learn more at [Airport Limousine](https://www.limousinebus.co.jp).

Created by [@zjsng](https://github.com/zjsng) (zjsng).

## Install

The recommended path installs both the `airport-limousine-pp-cli` binary and the `pp-airport-limousine` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install airport-limousine
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install airport-limousine --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install airport-limousine --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install airport-limousine --agent claude-code
npx -y @mvanhorn/printing-press-library install airport-limousine --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/cmd/airport-limousine-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airport-limousine-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install airport-limousine --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-airport-limousine --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-airport-limousine --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install airport-limousine --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/airport-limousine-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/airport-limousine/cmd/airport-limousine-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "airport-limousine": {
      "command": "airport-limousine-pp-mcp"
    }
  }
}
```

</details>

## Authentication

Public read-only sources; no login, key or reservation action. Chrome-compatible HTTP runs without a resident browser.

## Airport bus source contract

`routes`, `stops`, `timetable`, `travel-times`, `transfers`, `fare`, `conditions` and `handoff` use fresh public provider reads. Use `--data-source auto` or `--data-source live`; these commands reject local mode before making requests. Each invocation permits at most eight provider requests, reads at most 2 MiB per response, uses adaptive pacing, refuses redirects, and follows the configured `--timeout` for the whole command. Rate limiting is a typed error, rather than an empty result.

Select exact stop IDs: Haneda and Narita terminals, Shinjuku Station and Shinjuku Expressway Bus Terminal remain distinct. Dates default to today in Asia/Tokyo. Timetable output retains raw provider times and absolute `+09:00` times; `day_offset` and `rollover_inferred` expose midnight handling. Route context and station columns appear under `meta.context` and `meta.stations`.

Current travel times retain the source JST clock, which has no verified date, and explicitly preserve adjusting, retrieving and unknown values. They are route estimates and do not identify a terminal or guarantee arrival. Schedules and published fares do not establish seat availability. `fare` uses actual published adult/child units for a served stop pair; if units differ across trips, select `--trip-id` from `timetable`. A missing unit remains unknown. Passenger category eligibility and operator-specific rules remain the user's responsibility.

`conditions` retains guide content update dates and links to current baggage notices. Its structured bag values and units survive agent output. `handoff` validates the dated route and prints its canonical timetable page; the user follows reservation links on that page. No command books, pays or changes an account.

## Quick Start

```bash
# Inspect the configured public source safely.
airport-limousine-pp-cli doctor --dry-run

# Resolve a route identifier.
airport-limousine-pp-cli routes --query Shinjuku --agent

# Read today in JST with exact airport terminals.
airport-limousine-pp-cli timetable Haneda-Narita --agent

# Observe duration estimates and unavailable states.
airport-limousine-pp-cli travel-times --airport haneda --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Airport bus planning
- **`timetable`** — Plan a service date at exact stops with JST times and midnight rollover.

  _Inspect exact terminal rows and omit seat inventory._

  ```bash
  airport-limousine-pp-cli timetable --agent
  ```
- **`travel-times`** — Compare current and standard duration estimates with explicit unavailable states.

  _Preserve source clock and estimates without guaranteeing arrival._

  ```bash
  airport-limousine-pp-cli travel-times --agent
  ```
- **`transfers`** — Read both airport transfer directions for the same service date.

  _Keep terminal identity and failed fetches distinct._

  ```bash
  airport-limousine-pp-cli transfers --agent
  ```
- **`fare`** — Calculate an adult/child total from the operator fare for a served pair.

  _State passenger-category assumptions and unknown fares._

  ```bash
  airport-limousine-pp-cli fare --agent
  ```
- **`conditions`** — Read concise current provider conditions with source timestamps and notices.

  _Check luggage and child seating before booking._

  ```bash
  airport-limousine-pp-cli conditions --agent
  ```

## Recipes

### Find a stop

```bash
airport-limousine-pp-cli stops find --query Shinjuku --agent --select results.id,results.name,meta.observed_at
```

Retain exact stop IDs and source freshness.

### Inspect a terminal

```bash
airport-limousine-pp-cli stops get HanedaAirportTerminal3 --agent
```

Read boarding map and provider route handoff.

### Compare transfers

```bash
airport-limousine-pp-cli transfers --agent
```

Read both directions for today in JST.

### Read luggage conditions

```bash
airport-limousine-pp-cli conditions --topic baggage --agent
```

Check current operator limits and linked notices.

## Usage

Run `airport-limousine-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `AIRPORT_LIMOUSINE_CONFIG_DIR`, `AIRPORT_LIMOUSINE_DATA_DIR`, `AIRPORT_LIMOUSINE_STATE_DIR`, or `AIRPORT_LIMOUSINE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `AIRPORT_LIMOUSINE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export AIRPORT_LIMOUSINE_HOME=/srv/airport-limousine
airport-limousine-pp-cli doctor
```

Under `AIRPORT_LIMOUSINE_HOME=/srv/airport-limousine`, the four dirs resolve to `/srv/airport-limousine/config`, `/srv/airport-limousine/data`, `/srv/airport-limousine/state`, and `/srv/airport-limousine/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "airport-limousine": {
      "command": "airport-limousine-pp-mcp",
      "env": {
        "AIRPORT_LIMOUSINE_HOME": "/srv/airport-limousine"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `AIRPORT_LIMOUSINE_DATA_DIR` overrides an explicit `--home` for that kind. Use `AIRPORT_LIMOUSINE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `AIRPORT_LIMOUSINE_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `airport-limousine-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### pages

Read canonical operator page links

- **`airport-limousine-pp-cli pages guide`** - Read the provider baggage and boarding guide links
- **`airport-limousine-pp-cli pages routes`** - Read canonical route and timetable links
- **`airport-limousine-pp-cli pages stop`** - Read a known operator stop page and links
- **`airport-limousine-pp-cli pages timetable`** - Read canonical date timetable handoff links


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`airport-limousine-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`airport-limousine-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`airport-limousine-pp-cli learnings list`** - Inspect taught rows
- **`airport-limousine-pp-cli learnings forget <query>`** - Undo a teach
- **`airport-limousine-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`airport-limousine-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`airport-limousine-pp-cli teach-pattern`** - Install a query/resource template up front
- **`airport-limousine-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `AIRPORT_LIMOUSINE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `airport-limousine-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
airport-limousine-pp-cli pages guide

# JSON for scripting and agents
airport-limousine-pp-cli pages guide --json
# Filter to specific fields by name
airport-limousine-pp-cli pages guide --json --select <field>[,<field>...]

# Dry run — show the request without sending
airport-limousine-pp-cli pages guide --dry-run

# Agent mode — JSON + compact + no prompts in one flag
airport-limousine-pp-cli pages guide --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit confirmation** - `--agent` does not imply `--yes`; pass `--yes` separately only after the target, arguments, and side effects are clear
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
airport-limousine-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `airport-limousine-pp-cli doctor` to see the resolved config, data, state, and cache directories. Use `--home`, `AIRPORT_LIMOUSINE_HOME`, or per-kind environment variables to relocate those directories.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run `routes` for route IDs or `stops find --query Shinjuku` for stop IDs.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `AIRPORT_LIMOUSINE_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://www.limousinebus.co.jp/en/
- Capture coverage: 1 API entries from 5 total network entries
- Reachability: browser_http (85% confidence)
- Protocols: rest_json (75% confidence), html_scrape (55% confidence)
- Auth signals: none
- Candidate command ideas: list___data.json — Derived from observed GET /en/timetable/detail/Haneda-Narita/__data.json traffic.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)

The four page tools in MCP return bounded canonical titles and links. Embedded page application data, reservation state and seat inventory are omitted. Structured airport planning uses the same read-only commands as the CLI.

Baggage quantities have explicit scope: the English general guide lists two checked pieces per person, while the Japanese guide currently limits the Shibuya–Narita LCB route to one. The conditions command reads both guides and returns the route exception first; partner routes can differ.
