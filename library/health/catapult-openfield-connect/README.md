# Catapult OpenField CLI

**Every Catapult GPS metric — ACWR, velocity benchmarks, load reports — from one command, with a local SQLite mirror for offline analysis.**

catapult-openfield-connect-pp-cli syncs your OpenField data into a local SQLite store and unlocks computations the API cannot do alone: 28-day ACWR dashboards, personal velocity bests, session diffs, and positional benchmarks — all scriptable, pipeable, and agent-native. What used to require a custom R script now takes one command.

Created by [@erash11](https://github.com/erash11).

## Install

The recommended path installs both the `catapult-openfield-connect-pp-cli` binary and the `pp-catapult-openfield-connect` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install catapult-openfield-connect
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install catapult-openfield-connect --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install catapult-openfield-connect --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install catapult-openfield-connect --agent claude-code
npx -y @mvanhorn/printing-press-library install catapult-openfield-connect --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.5 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/health/catapult-openfield-connect/cmd/catapult-openfield-connect-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/catapult-openfield-connect-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install catapult-openfield-connect --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-catapult-openfield-connect --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-catapult-openfield-connect --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install catapult-openfield-connect --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/catapult-openfield-connect-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CATAPULT_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/health/catapult-openfield-connect/cmd/catapult-openfield-connect-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "catapult-openfield-connect": {
      "command": "catapult-openfield-connect-pp-mcp",
      "env": {
        "CATAPULT_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

OpenField Connect accepts a long-lived API token (create one in OpenField under API Tokens) sent as a Bearer header. Set CATAPULT_TOKEN in your environment or run auth set-token. Regional hosts differ: this CLI defaults to the Americas host (us.catapultsports.com); override base_url in config for APAC/EMEA tenants.

## Quick Start

```bash
# Verify config and connectivity without touching the API
catapult-openfield-connect-pp-cli doctor --dry-run


# Mirror the roster, metric catalog, and a month of sessions locally
catapult-openfield-connect-pp-cli sync --resources athletes,parameters,activities --since 30d


# The morning check: who is in the caution or danger zone
catapult-openfield-connect-pp-cli acwr --squad --metric total_player_load --flag-risk --agent


# Find exact metric slugs before building a stats query
catapult-openfield-connect-pp-cli search "velocity" --type parameters --limit 10


# Monday coach briefing in one command
catapult-openfield-connect-pp-cli report --week --squad --format markdown --export ./week_load.md

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Load intelligence that compounds

- **`acwr`** — Instant acute:chronic workload ratios for any athlete or the full squad, color-coded into safe (0.8-1.3), caution, and danger (>1.5) zones.

  _Reach for this when asked who is at injury risk or for the morning workload check._

  ```bash
  catapult-openfield-connect-pp-cli acwr --squad --metric total_player_load --flag-risk --agent
  ```
- **`report`** — One-command weekly summary — total Player Load, session count, ACWR, max velocity vs PB, monotony, strain, and risk flags — exportable as markdown or CSV.

  _Reach for this when a coach briefing or weekly load summary is requested._

  ```bash
  catapult-openfield-connect-pp-cli report --week --squad --format markdown --export ./week_load.md
  ```
- **`pb`** — Compares recent session values against each athlete's all-time or rolling personal best, with readiness % and 3-session trend arrows.

  _Use before deciding who trains at full intensity today._

  ```bash
  catapult-openfield-connect-pp-cli pb --metric max_vel --vs-peak --agent
  ```
- **`benchmark`** — Ranks any athlete against positional peers for a metric: percentile rank, squad median, and position median with outlier flags.

  _Use after match debriefs to answer how an athlete compares to peers._

  ```bash
  catapult-openfield-connect-pp-cli benchmark --metric total_distance --percentile --agent
  ```

### Session analysis

- **`diff`** — Side-by-side comparison of two sessions (by ID, date, or last/prev) showing absolute and percentage change per metric and per athlete.

  _Use to diagnose travel fatigue and fixture congestion between consecutive matches._

  ```bash
  catapult-openfield-connect-pp-cli diff last prev --squad --metrics total_player_load,max_vel --agent
  ```
- **`rtp`** — Tracks an injured athlete's load progression session-by-session as a percentage of squad average or a pre-injury baseline window.

  _Use when managing return-to-play decisions to give medical staff quantitative progression data._

  ```bash
  catapult-openfield-connect-pp-cli rtp --athlete top --target-metric total_player_load --vs-squad --threshold 85 --agent
  ```
- **`heatmap`** — Renders an athlete-by-period intensity matrix for a session — ANSI heatmap on a terminal, JSON matrix for agents — surfacing where fatigue compounded.

  _Use after matches to diagnose second-half performance drops._

  ```bash
  catapult-openfield-connect-pp-cli heatmap --activity last --metric total_player_load --json
  ```

## Recipes


### Morning ACWR check for the full squad

```bash
catapult-openfield-connect-pp-cli acwr --squad --flag-risk --agent --select athlete_name,acwr,risk_zone
```

Run before every training session to see who is in the caution or danger zone.

### Post-session stats grouped by athlete

```bash
catapult-openfield-connect-pp-cli stats --parameters total_player_load,total_distance,max_vel --group-by athlete --json
```

The primary data pull after every session.

### Compare the last two matches

```bash
catapult-openfield-connect-pp-cli diff last prev --squad --metrics total_player_load,max_vel --agent --select athlete_name,metric,delta_pct
```

Percentage change per athlete per metric between matches; --select keeps agent context tight.

### Find velocity metric slugs

```bash
catapult-openfield-connect-pp-cli search "velocity" --type parameters --limit 10
```

Discover exact slugs before building a stats query.

### Weekly report to markdown

```bash
catapult-openfield-connect-pp-cli report --week --squad --format markdown --export ./week12_load.md
```

Paste directly into the coach briefing doc.

## Usage

Run `catapult-openfield-connect-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CATAPULT_OPENFIELD_CONNECT_CONFIG_DIR`, `CATAPULT_OPENFIELD_CONNECT_DATA_DIR`, `CATAPULT_OPENFIELD_CONNECT_STATE_DIR`, or `CATAPULT_OPENFIELD_CONNECT_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CATAPULT_OPENFIELD_CONNECT_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CATAPULT_OPENFIELD_CONNECT_HOME=/srv/catapult-openfield-connect
catapult-openfield-connect-pp-cli doctor
```

Under `CATAPULT_OPENFIELD_CONNECT_HOME=/srv/catapult-openfield-connect`, the four dirs resolve to `/srv/catapult-openfield-connect/config`, `/srv/catapult-openfield-connect/data`, `/srv/catapult-openfield-connect/state`, and `/srv/catapult-openfield-connect/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "catapult-openfield-connect": {
      "command": "catapult-openfield-connect-pp-mcp",
      "env": {
        "CATAPULT_OPENFIELD_CONNECT_HOME": "/srv/catapult-openfield-connect"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CATAPULT_OPENFIELD_CONNECT_DATA_DIR` overrides an explicit `--home` for that kind. Use `CATAPULT_OPENFIELD_CONNECT_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CATAPULT_OPENFIELD_CONNECT_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `catapult-openfield-connect-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### activities

Training sessions and matches

- **`catapult-openfield-connect-pp-cli activities annotations`** - Annotations on an activity
- **`catapult-openfield-connect-pp-cli activities athletes`** - Athletes who participated in an activity
- **`catapult-openfield-connect-pp-cli activities devices`** - Devices used in an activity
- **`catapult-openfield-connect-pp-cli activities get`** - Get one activity; use --embed all for tag/athlete detail
- **`catapult-openfield-connect-pp-cli activities list`** - List activities (sessions and matches) with venue and timing
- **`catapult-openfield-connect-pp-cli activities periods`** - Periods (halves, quarters, drills) within an activity

### annotations

Annotation metadata

- **`catapult-openfield-connect-pp-cli annotations`** - List annotation categories

### athletes

Athletes registered in the OpenField account

- **`catapult-openfield-connect-pp-cli athletes annotations`** - Annotations recorded against an athlete
- **`catapult-openfield-connect-pp-cli athletes bands`** - Velocity/acceleration band thresholds configured for an athlete
- **`catapult-openfield-connect-pp-cli athletes list`** - List all athletes with physical attributes, jersey numbers, and positions

### customer

Account info and settings

- **`catapult-openfield-connect-pp-cli customer get`** - Customer account info
- **`catapult-openfield-connect-pp-cli customer settings`** - User settings including velocity and distance units

### parameters

The 200+ metric catalog (Player Load, banded velocity, heart rate...)

- **`catapult-openfield-connect-pp-cli parameters`** - List every parameter (metric) available in OpenField, with slugs used by stats queries

### periods

Periods across activities

- **`catapult-openfield-connect-pp-cli periods annotations`** - Annotations on a period
- **`catapult-openfield-connect-pp-cli periods athletes`** - Athletes in a period

### sensor

Raw 10Hz sensor data, IMA events, and velocity/acceleration efforts

- **`catapult-openfield-connect-pp-cli sensor data`** - 10Hz sensor stream for one athlete in one activity (large; requires sensor-read-only scope)
- **`catapult-openfield-connect-pp-cli sensor efforts`** - Generation-2 velocity or acceleration efforts for one athlete in one activity
- **`catapult-openfield-connect-pp-cli sensor events`** - IMA events (jumps, dives, sport-specific movements) for one athlete in one activity

### stats

The stats query engine — computed metrics with grouping and filters

- **`catapult-openfield-connect-pp-cli stats`** - Query computed statistics. Body: parameter slugs, group_by dims (athlete, activity, period, position, team), and filters (date, activity_id, athlete_id, lastActivities...)

### teams

Teams in the account

- **`catapult-openfield-connect-pp-cli teams`** - List teams

### venues

Venues

- **`catapult-openfield-connect-pp-cli venues get`** - Get one venue
- **`catapult-openfield-connect-pp-cli venues list`** - List venues


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`catapult-openfield-connect-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`catapult-openfield-connect-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`catapult-openfield-connect-pp-cli learnings list`** - Inspect taught rows
- **`catapult-openfield-connect-pp-cli learnings forget <query>`** - Undo a teach
- **`catapult-openfield-connect-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`catapult-openfield-connect-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`catapult-openfield-connect-pp-cli teach-pattern`** - Install a query/resource template up front
- **`catapult-openfield-connect-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CATAPULT_OPENFIELD_CONNECT_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `catapult-openfield-connect-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
catapult-openfield-connect-pp-cli activities list

# JSON for scripting and agents
catapult-openfield-connect-pp-cli activities list --json

# Filter to specific fields
catapult-openfield-connect-pp-cli activities list --json --select id,name,status

# Dry run — show the request without sending
catapult-openfield-connect-pp-cli activities list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
catapult-openfield-connect-pp-cli activities list --agent
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

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `CATAPULT_OPENFIELD_CONNECT_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `catapult-openfield-connect-pp-cli activities`
- `catapult-openfield-connect-pp-cli activities list`
- `catapult-openfield-connect-pp-cli annotations`
- `catapult-openfield-connect-pp-cli athletes`
- `catapult-openfield-connect-pp-cli athletes list`
- `catapult-openfield-connect-pp-cli parameters`
- `catapult-openfield-connect-pp-cli teams`
- `catapult-openfield-connect-pp-cli venues`
- `catapult-openfield-connect-pp-cli venues list`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
catapult-openfield-connect-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `catapult-openfield-connect-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/catapult-openfield-connect-pp-cli/config.toml`; `--home`, `CATAPULT_OPENFIELD_CONNECT_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CATAPULT_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `catapult-openfield-connect-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `catapult-openfield-connect-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CATAPULT_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 Unauthorized on every call** — Token expired or wrong region host — regenerate the API token in OpenField and confirm base_url matches your tenant region
- **stats query returns empty rows** — Check parameter slugs with 'search --type parameters' — slugs, not display names, are required
- **acwr/pb/report show no data** — These read the local mirror: run 'sync --resources activities,athletes,parameters --since 30d' first

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
