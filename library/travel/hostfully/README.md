# Hostfully CLI

**Hostfully reservations and guest messaging — plus caller-ID lookup, quotes, and multi-unit vacancy analysis.**

Reads current operational data (today's guests, availability, caller identity, quotes) live from Hostfully, and keeps a local SQLite archive for historical analysis such as occupancy, turnover and vacancy windows. Unit numbers are discovered from your agency's properties, or pinned with HOSTFULLY_UNITS.

## Install

The recommended path installs both the `hostfully-pp-cli` binary and the `pp-hostfully` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install hostfully
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install hostfully --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install hostfully --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install hostfully --agent claude-code
npx -y @mvanhorn/printing-press-library install hostfully --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.9 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hostfully/cmd/hostfully-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hostfully-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install hostfully --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-hostfully --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-hostfully --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install hostfully --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/hostfully-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `HOSTFULLY_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/travel/hostfully/cmd/hostfully-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "hostfully": {
      "command": "hostfully-pp-mcp",
      "env": {
        "HOSTFULLY_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set HOSTFULLY_API_KEY to an agency API key and HOSTFULLY_AGENCY_ID to the agency UID; every Hostfully request is agency-scoped. Unit numbers used by today, availability, whois, quote, vacancy and the other per-unit commands come from HOSTFULLY_UNITS ("1=<property-uid>,2=<property-uid>"), a units.json file, synced properties, or live discovery of the agency's properties; run `hostfully-pp-cli units` to see the mapping. The property time zone comes from HOSTFULLY_TIMEZONE or the properties themselves. `inbox reply` is the one command that sends a guest message: it requires --yes and supports --dry-run.

## Quick Start

```bash
# Check configuration without touching the network
hostfully-pp-cli doctor --dry-run

# See how unit numbers map to your Hostfully properties
hostfully-pp-cli units

# Check-ins, check-outs, in-house guests and vacant units
hostfully-pp-cli today

# Identify a caller from their phone number
hostfully-pp-cli whois "+1 555 010 0199"

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Live guest operations
- **`whois`** — Look up a guest by phone number or name and get every stay, payment status and reservation access PIN.

  _Reach for this first when a guest calls or texts and you need to know who they are and which stay they mean._

  ```bash
  hostfully-pp-cli whois "+1 555 010 0199" --agent
  ```
- **`inbox`** — Read recent guest messages and threads, and reply through the reservation thread with a dry-run guard.

  _Use to triage new guest messages and draft or send replies without opening the web app._

  ```bash
  hostfully-pp-cli inbox messages --since 24h --agent
  ```
- **`quote`** — Price a stay for a property and dates with rent, taxes and fees itemized.

  _Use when a prospective guest asks what a stay costs._

  ```bash
  hostfully-pp-cli quote --unit 1 --check-in 2026-11-06 --check-out 2026-11-08 --guests 2 --agent
  ```
- **`jobs`** — List cleaning and maintenance jobs with service and provider identity, including cancelled jobs.

  _Use to reconcile a cleaning or service vendor's invoice against scheduled jobs._

  ```bash
  hostfully-pp-cli jobs --from 2026-10-01 --to 2026-10-31 --agent
  ```

### Planning and analytics
- **`vacancy`** — Find consecutive vacant days across several units at once.

  _Use to plan work or owner stays that need several units free together._

  ```bash
  hostfully-pp-cli vacancy --units 1,2 --consecutive 2 --agent
  ```
- **`maintenance`** — Find maintenance slots across units around upcoming bookings.

  _Use when scheduling repairs that must not disturb guests._

  ```bash
  hostfully-pp-cli maintenance --units all --min-days 1 --agent
  ```
- **`turnover`** — See cleaning windows between each checkout and the next check-in.

  _Use to build the week's cleaning schedule._

  ```bash
  hostfully-pp-cli turnover --days 7 --agent
  ```
- **`occupancy`** — Per-unit and overall occupancy for any period.

  _Use for monthly performance reviews._

  ```bash
  hostfully-pp-cli occupancy --month 2026-09 --agent
  ```
- **`channels`** — Break down where bookings come from over a period.

  _Use to see which channels drive bookings._

  ```bash
  hostfully-pp-cli channels --period 90d --agent
  ```
- **`revenue`** — Estimate revenue per unit and month from booked nights.

  _Use for quick revenue snapshots._

  ```bash
  hostfully-pp-cli revenue --month 2026-09 --agent
  ```
- **`patterns`** — Spot weekday/weekend and seasonal booking trends.

  _Use for pricing and staffing decisions._

  ```bash
  hostfully-pp-cli patterns --weeks 12 --agent
  ```

## Recipes

### Who is calling

```bash
hostfully-pp-cli whois "+1 555 010 0199" --agent
```

Resolve a phone number to the guest's stays, payment status and access PIN.

### Today's operations

```bash
hostfully-pp-cli today --agent
```

Daily summary of check-ins, check-outs, in-house guests and vacant units.

### Narrow the unit map

```bash
hostfully-pp-cli units --agent --select units.unit,units.propertyUid
```

Pull only unit numbers and property UIDs when wiring another tool to this CLI.

### Maintenance window

```bash
hostfully-pp-cli vacancy --units 1,2 --consecutive 2 --agent
```

Find two consecutive days where units 1 and 2 are both empty.

## Usage

Run `hostfully-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `HOSTFULLY_CONFIG_DIR`, `HOSTFULLY_DATA_DIR`, `HOSTFULLY_STATE_DIR`, or `HOSTFULLY_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `HOSTFULLY_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export HOSTFULLY_HOME=/srv/hostfully
hostfully-pp-cli doctor
```

Under `HOSTFULLY_HOME=/srv/hostfully`, the four dirs resolve to `/srv/hostfully/config`, `/srv/hostfully/data`, `/srv/hostfully/state`, and `/srv/hostfully/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "hostfully": {
      "command": "hostfully-pp-mcp",
      "env": {
        "HOSTFULLY_HOME": "/srv/hostfully"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `HOSTFULLY_DATA_DIR` overrides an explicit `--home` for that kind. Use `HOSTFULLY_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `HOSTFULLY_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `hostfully-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### leads

Reservations and bookings across all properties

- **`hostfully-pp-cli leads`** - List leads (reservations) with date and property filters

### properties

Rental properties/units managed by the agency

- **`hostfully-pp-cli properties`** - List all properties for the agency


### channels


- **`hostfully-pp-cli channels`** - Break down where bookings come from over a period.


### inbox


- **`hostfully-pp-cli inbox`** - Read recent guest messages and threads, and reply through the reservation thread with a dry-run guard.


### jobs


- **`hostfully-pp-cli jobs`** - List cleaning and maintenance jobs with service and provider identity, including cancelled jobs.


### maintenance


- **`hostfully-pp-cli maintenance`** - Find maintenance slots across units around upcoming bookings.


### occupancy


- **`hostfully-pp-cli occupancy`** - Per-unit and overall occupancy for any period.


### patterns


- **`hostfully-pp-cli patterns`** - Spot weekday/weekend and seasonal booking trends.


### quote


- **`hostfully-pp-cli quote`** - Price a stay for a property and dates with rent, taxes and fees itemized.


### revenue


- **`hostfully-pp-cli revenue`** - Estimate revenue per unit and month from booked nights.


### turnover


- **`hostfully-pp-cli turnover`** - See cleaning windows between each checkout and the next check-in.


### vacancy


- **`hostfully-pp-cli vacancy`** - Find consecutive vacant days across several units at once.


### whois


- **`hostfully-pp-cli whois`** - Look up a guest by phone number or name and get every stay, payment status and reservation access PIN.




### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`hostfully-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`hostfully-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`hostfully-pp-cli learnings list`** - Inspect taught rows
- **`hostfully-pp-cli learnings forget <query>`** - Undo a teach
- **`hostfully-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`hostfully-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`hostfully-pp-cli teach-pattern`** - Install a query/resource template up front
- **`hostfully-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `HOSTFULLY_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `hostfully-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
hostfully-pp-cli leads --agency-uid 550e8400-e29b-41d4-a716-446655440000

# JSON for scripting and agents
hostfully-pp-cli leads --agency-uid 550e8400-e29b-41d4-a716-446655440000 --json
# Filter to specific fields by name
hostfully-pp-cli leads --agency-uid 550e8400-e29b-41d4-a716-446655440000 --json --select <field>[,<field>...]

# Dry run — show the request without sending
hostfully-pp-cli leads --agency-uid 550e8400-e29b-41d4-a716-446655440000 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
hostfully-pp-cli leads --agency-uid 550e8400-e29b-41d4-a716-446655440000 --agent
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

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
hostfully-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `hostfully-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `HOSTFULLY_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `HOSTFULLY_API_KEY` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `hostfully-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `hostfully-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $HOSTFULLY_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **403 or authentication error** — Set HOSTFULLY_API_KEY and HOSTFULLY_AGENCY_ID, then run hostfully-pp-cli doctor
- **Unit numbers don't match your property names** — Pin them with HOSTFULLY_UNITS="1=<property-uid>,2=<property-uid>" or a units map in the config file
- **HTTP 500 while fetching leads** — Use hostfully-pp-cli fetch, which fetches per property and recovers failed pages through GraphQL
