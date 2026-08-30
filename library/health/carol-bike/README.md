# CAROL Bike CLI

**Read and locally mirror personal CAROL Bike workout data.**

An unofficial read-only CLI for a rider's own CAROL workout history, with typed commands and a local SQLite mirror.

Learn more at [CAROL Bike](https://i.carolbike.com).

Created by [@bricenice17](https://github.com/bricenice17) (bricenice17).

## Install

The recommended path installs both the `carol-bike-pp-cli` binary and the `pp-carol-bike` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install carol-bike
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install carol-bike --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install carol-bike --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install carol-bike --agent claude-code
npx -y @mvanhorn/printing-press-library install carol-bike --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/health/carol-bike/cmd/carol-bike-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/carol-bike-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install carol-bike --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-carol-bike --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-carol-bike --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install carol-bike --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/carol-bike-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CAROL_BIKE_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/health/carol-bike/cmd/carol-bike-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "carol-bike": {
      "command": "carol-bike-pp-mcp",
      "env": {
        "CAROL_BIKE_RIDER_ID": "<riderId>",
        "CAROL_BIKE_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Set CAROL_BIKE_TOKEN to a bearer token from your own authenticated dashboard session and CAROL_BIKE_RIDER_ID to your rider identifier. Never commit either value.

## Quick Start

```bash
# Check credential configuration and API reachability.
carol-bike-pp-cli doctor --json

# Preview the latest-ride request without sending it.
carol-bike-pp-cli ride get-latest --dry-run --json

# Preview a bounded full sync of all four ride families into the local store.
carol-bike-pp-cli sync --full --max-pages 1 --dry-run --json

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local data
- **`sync`** — Mirror REHIT, FAT BURN, free/zones/custom, and fitness-test ride history into one local SQLite store for dependable offline search and analysis.

  _Agents can query a durable personal ride history without keeping a browser running._

  ```bash
  carol-bike-pp-cli sync --full --max-pages 1 --json
  ```

## Recipes

### Preview current weekly frequency

```bash
carol-bike-pp-cli stats get-rides-per-week --dry-run --json
```

Inspect the read-only request for weekly ride rate and target.

### Preview the recent ride calendar

```bash
carol-bike-pp-cli stats get-ride-calendar --dry-run --json
```

Inspect the read-only calendar request before making a live call.

### Preview a bounded local mirror

```bash
carol-bike-pp-cli sync --full --max-pages 1 --dry-run --json
```

Verify sync wiring without sending API requests.

## Usage

Run `carol-bike-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `CAROL_BIKE_CONFIG_DIR`, `CAROL_BIKE_DATA_DIR`, `CAROL_BIKE_STATE_DIR`, or `CAROL_BIKE_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `CAROL_BIKE_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export CAROL_BIKE_HOME=/srv/carol-bike
carol-bike-pp-cli doctor
```

Under `CAROL_BIKE_HOME=/srv/carol-bike`, the four dirs resolve to `/srv/carol-bike/config`, `/srv/carol-bike/data`, `/srv/carol-bike/state`, and `/srv/carol-bike/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "carol-bike": {
      "command": "carol-bike-pp-mcp",
      "env": {
        "CAROL_BIKE_HOME": "/srv/carol-bike"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `CAROL_BIKE_DATA_DIR` overrides an explicit `--home` for that kind. Use `CAROL_BIKE_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `CAROL_BIKE_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `carol-bike-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### ride

Read personal CAROL Bike ride history.

- **`carol-bike-pp-cli ride get-latest`** - Get the latest ride
- **`carol-bike-pp-cli ride list-fat-burn`** - List FAT BURN rides
- **`carol-bike-pp-cli ride list-fitness-tests`** - List fitness-test rides
- **`carol-bike-pp-cli ride list-free-custom-zones`** - List free, zones, and custom rides
- **`carol-bike-pp-cli ride list-rehit`** - List REHIT rides

### stats

Read aggregate CAROL Bike rider statistics.

- **`carol-bike-pp-cli stats get-ride-calendar`** - Get recent ride-calendar data
- **`carol-bike-pp-cli stats get-ride-count`** - Get total ride count
- **`carol-bike-pp-cli stats get-rider`** - Get aggregate rider statistics
- **`carol-bike-pp-cli stats get-rides-per-week`** - Get current weekly ride rate and target

### trends

Read CAROL Bike rider trend series. This trend surface remains REHIT-only; the all-family ride-list and sync support does not generalize trends to other workout families.

- **`carol-bike-pp-cli trends`** - Get rider trend series


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`carol-bike-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`carol-bike-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`carol-bike-pp-cli learnings list`** - Inspect taught rows
- **`carol-bike-pp-cli learnings forget <query>`** - Undo a teach
- **`carol-bike-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`carol-bike-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`carol-bike-pp-cli teach-pattern`** - Install a query/resource template up front
- **`carol-bike-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `CAROL_BIKE_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `carol-bike-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
carol-bike-pp-cli ride get-latest

# JSON for scripting and agents
carol-bike-pp-cli ride get-latest --json
# Filter to specific fields
carol-bike-pp-cli ride get-latest --json --select id,type,start

# Dry run — show the request without sending
carol-bike-pp-cli ride get-latest --dry-run

# Agent mode — JSON + compact + no prompts in one flag
carol-bike-pp-cli ride get-latest --agent
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

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `CAROL_BIKE_RIDER_ID` resolves `{riderId}`

Base URL: `https://i.carolbike.com/rider-api`

## Health Check

```bash
carol-bike-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `carol-bike-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is ``; `--home`, `CAROL_BIKE_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CAROL_BIKE_RIDER_ID` | endpoint | Yes | Rider identifier from your authenticated CAROL Bike account. |
| `CAROL_BIKE_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `carol-bike-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `carol-bike-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CAROL_BIKE_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **CAROL returns 401 or 403** — Obtain a current bearer token from your own authenticated CAROL dashboard session; this private API is unsupported and may change.
- **A command reports a missing riderId** — Set CAROL_BIKE_RIDER_ID to the rider identifier from your own CAROL account.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://i.carolbike.com/dashboard/main
- Capture coverage: 7 API entries from 7 total network entries
- Reachability: standard_http (95% confidence)
- Protocols: rest_json (99% confidence)
- Auth signals: bearer_token — headers: Authorization

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
