# iRail CLI

**Every Belgian rail lookup the existing tools offer, plus transfer-risk, delay history and accessibility data that none of them have.**

iRail exposes live NMBS/SNCB departures, journey planning and disruptions for free with no API key. This CLI adds what every other client throws away: it records observations locally so punctuality can answer whether a train is chronically late, joins the open stations dataset so transfer-risk knows the real minimum transfer time at each station, and surfaces station accessibility data the API never returns. The analysis commands emit typed JSON - delays as numbers, cancellations as booleans - while the raw endpoint commands pass iRail's payload through unchanged.

## Install

The recommended path installs both the `irail-pp-cli` binary and the `pp-irail` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install irail
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install irail --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install irail --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install irail --agent claude-code
npx -y @mvanhorn/printing-press-library install irail --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/irail/cmd/irail-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/irail-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install irail --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-irail --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-irail --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install irail --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/irail-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/irail/cmd/irail-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "irail": {
      "command": "irail-pp-mcp"
    }
  }
}
```

</details>

## Authentication

No credentials are required: the iRail API is open and unauthenticated, so every command works immediately after install. Two operational limits matter instead of a key. iRail allows 3 requests per second per IP with 5 burst, returning HTTP 429 beyond that, so this CLI ships an adaptive rate limiter. iRail also blocks source IPs that send no User-Agent without prior warning, so every request carries an identifying User-Agent automatically.

## Quick Start

```bash
# Confirm the binary works and the API is reachable; no credentials needed
irail-pp-cli doctor --dry-run

# Pull the 716-station reference set into local SQLite; board and route are live-only and take no sync
irail-pp-cli sync --resources stations,disruptions

# The core lookup: what is leaving right now, with delay and platform
irail-pp-cli board --station Brussels-Central

# Plan a journey with transfers and per-leg live delay
irail-pp-cli route --from Ghent-Sint-Pieters --to Brussels-Central

# Narrow the national disruption feed to just this journey
irail-pp-cli disruptions route --from Ghent-Sint-Pieters --to Brussels-Central

# Start recording observations so punctuality and changes have history to read
irail-pp-cli observe --station Brussels-Central

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Live data joined with open datasets
- **`transfer-risk`** — Tells you whether the transfers in a journey still hold once today's delays are applied.

  _Reach for this instead of a plain route lookup whenever a journey has a transfer and delays are already in play._

  ```bash
  irail-pp-cli transfer-risk --from Oostende --to Hasselt --agent
  ```
- **`disruptions route`** — Filters the national disruption feed down to the stations your journey actually passes through.

  _Use this when the national list is too noisy to answer whether one specific trip is affected._

  ```bash
  irail-pp-cli disruptions route --from Ghent-Sint-Pieters --to Brussels-Central --agent
  ```
- **`stations facilities`** — Reports step-free access, elevators, ramps, lockers, bike parking and ticket-desk hours for a station.

  _Use this for accessibility and amenity questions that the live rail API simply cannot answer._

  ```bash
  irail-pp-cli stations facilities --station Ghent-Sint-Pieters --agent
  ```

### Local state that compounds
- **`punctuality`** — Shows how reliable a train or route has actually been, from delay observations recorded on your machine.

  _Use this for questions about the past such as chronic lateness; it never calls the API._

  ```bash
  irail-pp-cli punctuality --from Ghent-Sint-Pieters --to Brussels-Central --board-type route --agent
  ```
- **`observe`** — Records what the board says right now into local SQLite, building the history other commands read.

  _Run this on a schedule; it is what makes punctuality and changes able to answer anything._

  ```bash
  irail-pp-cli observe --station Brussels-Central
  ```
- **`changes`** — Reports new delays, cancellations and platform changes since the last time you looked.

  _Use this for deltas during a commute rather than re-reading a whole board._

  ```bash
  irail-pp-cli changes --station Brussels-Central --agent
  ```

### Time reasoning done properly
- **`leave-by`** — Answers the last train you can take and still arrive before a deadline, accounting for current delays.

  _Use this when the arrival deadline is fixed and the departure time is the unknown._

  ```bash
  irail-pp-cli leave-by --from Leuven --to Brussels-Central --arrive-by 09:00 --agent
  ```

## Recipes

### Next departures, narrowed for an agent

```bash
irail-pp-cli board --station Brussels-Central --agent --select vehicle,delay,platform,canceled
```

A full board is roughly 34 KB of JSON; selecting four fields keeps the response small enough to reason over without burning context.

### Will my transfer survive today's delays

```bash
irail-pp-cli transfer-risk --from Oostende --to Hasselt --agent
```

Joins live per-leg delay against each station's official minimum transfer time and flags transfers that no longer hold.

### Only the disruptions that affect my commute

```bash
irail-pp-cli disruptions route --from Ghent-Sint-Pieters --to Brussels-Central --agent
```

Filters the national feed, which was 32 entries on a normal day, down to the stations this journey passes through.

### Last train that still gets me there by nine

```bash
irail-pp-cli leave-by --from Leuven --to Brussels-Central --arrive-by 09:00 --agent
```

Plans backwards from the deadline and applies current delays plus a safety margin.

### Is this station step-free

```bash
irail-pp-cli stations facilities --station Ghent-Sint-Pieters --agent
```

Reads the open facilities dataset for elevators, ramps and wheelchair access, none of which the rail API returns.

## Usage

Run `irail-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data such as `data.db` |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `IRAIL_CONFIG_DIR`, `IRAIL_DATA_DIR`, `IRAIL_STATE_DIR`, or `IRAIL_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `IRAIL_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export IRAIL_HOME=/srv/irail
irail-pp-cli doctor
```

Under `IRAIL_HOME=/srv/irail`, the four dirs resolve to `/srv/irail/config`, `/srv/irail/data`, `/srv/irail/state`, and `/srv/irail/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "irail": {
      "command": "irail-pp-mcp",
      "env": {
        "IRAIL_HOME": "/srv/irail"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `IRAIL_DATA_DIR` overrides an explicit `--home` for that kind. Use `IRAIL_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `IRAIL_HOME` does not move files back to platform defaults, and `doctor` cannot find files left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. Run `irail-pp-cli doctor --fail-on warn` to check path warnings in automation.

## Commands

### board

Live departure and arrival boards for a station

- **`irail-pp-cli board`** - Live departures (or arrivals) at a station, with delay, platform and occupancy

### disruptions

Network-wide disruptions and planned engineering works

- **`irail-pp-cli disruptions`** - Current disruptions and planned works across the whole network

### logs

Recent iRail API request log entries

- **`irail-pp-cli logs`** - Last request log entries. Note: upstream currently returns an empty list; bulk archives live at gtfs.irail.be/logs/

### route

Journey planning between two stations, with transfers and live delay

- **`irail-pp-cli route`** - Plan a journey between two stations, including transfers and per-leg delay

### stations

Belgian and cross-border stations served by NMBS/SNCB

- **`irail-pp-cli stations`** - List every station iRail knows about (716 incl. cross-border)

### train

Individual trains: full stop trace and physical composition

- **`irail-pp-cli train composition`** - Physical make-up of a train: segments, units, and per-carriage facilities
- **`irail-pp-cli train get`** - Every stop of one train with live delay. Note: iRail ignores the date parameter


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`irail-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`irail-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`irail-pp-cli learnings list`** - Inspect taught rows
- **`irail-pp-cli learnings forget <query>`** - Undo a teach
- **`irail-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`irail-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`irail-pp-cli teach-pattern`** - Install a query/resource template up front
- **`irail-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `IRAIL_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `irail-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
irail-pp-cli board --station Ghent-Sint-Pieters

# JSON for scripting and agents
irail-pp-cli board --station Ghent-Sint-Pieters --json

# Filter to specific fields
irail-pp-cli board --station Ghent-Sint-Pieters --json --select id,name,status

# Dry run — show the request without sending
irail-pp-cli board --station Ghent-Sint-Pieters --dry-run

# Agent mode — JSON + compact + no prompts in one flag
irail-pp-cli board --station Ghent-Sint-Pieters --agent
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
irail-pp-cli doctor
```

Verifies configuration and connectivity to the API.

## Configuration

Run `irail-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/irail-pp-cli/config.toml`; `--home`, `IRAIL_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

## Troubleshooting
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **HTTP 429 Too Many Requests** — iRail allows 3 requests/second per IP plus 5 burst. Reduce concurrency or add --limit to narrow the query; the built-in limiter backs off automatically.
- **Requests suddenly fail from your network** — iRail blocks IPs that send no User-Agent. Do not strip the User-Agent header; if already blocked, contact the iRail team to be unblocked.
- **Output is XML instead of JSON** — The API defaults to xml. This CLI always sends format=json; if you overrode it, drop --format or set --format json.
- **train get returns today even though you passed a date** — Upstream ignores the date parameter on the vehicle endpoint. Use board list with --date to inspect another day.
- **HTTP 500 on a date far in the past or future** — iRail only holds a window around the current date. Query a date close to today.
- **punctuality or changes returns nothing** — Both read local observations. Run irail-pp-cli observe at least twice before expecting results. Both also read one --board-type at a time (default departure), because departure, arrival and route captures are separate histories and are never mixed; history recorded by observe with --from and --to is route history, so read it back with --board-type route.
- **A station name is not recognised** — Run irail-pp-cli stations search with part of the name; it matches Dutch, French, German and English names plus telegraph codes such as FBMZ.
- **delay or canceled compares wrong in jq, e.g. .delay > 300 is always false** — The raw endpoint commands (board, route, stations, disruptions, train) return iRail's payload verbatim, and iRail encodes every scalar as a string. Cast with (.delay|tonumber), or use the analysis commands (transfer-risk, punctuality, leave-by, changes, observe) which emit typed numbers and booleans.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**iRail**](https://github.com/iRail/iRail) — PHP (134 stars)
- [**iRail stations**](https://github.com/iRail/stations) — PHP (32 stars)
- [**commandtrein**](https://github.com/Kaya-Sem/commandtrein) — Go (21 stars)
- [**irail-mcp**](https://github.com/HansF/irail-mcp) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
