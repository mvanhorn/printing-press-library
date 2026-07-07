# Intervals.icu CLI

**The first intervals.icu CLI with a local training database — offline search, SQL, and fitness/form analytics no other tool has.**

Sync your activities, wellness, calendar, workouts and gear into local SQLite, then search, run SQL, and compute fitness/form trends offline. Wraps the full intervals.icu REST API for live reads and writes, and adds analytics every existing MCP server and wrapper leaves on the table.

## Install

The recommended path installs both the `intervals-icu-pp-cli` binary and the `pp-intervals-icu` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install intervals-icu
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install intervals-icu --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install intervals-icu --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install intervals-icu --agent claude-code
npx -y @mvanhorn/printing-press-library install intervals-icu --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/intervals-icu/cmd/intervals-icu-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/intervals-icu-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-intervals-icu --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-intervals-icu --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-intervals-icu skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-intervals-icu. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/intervals-icu-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `INTERVALS_ICU_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/other/intervals-icu/cmd/intervals-icu-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "intervals-icu": {
      "command": "intervals-icu-pp-mcp",
      "env": {
        "INTERVALS_ICU_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Quick Start

```bash
# Check auth and API reachability first.
intervals-icu-pp-cli doctor --dry-run

# Populate the local store so offline commands work.
intervals-icu-pp-cli sync --since 365d

# Find activities by name/tag offline.
intervals-icu-pp-cli search "threshold"

# See your fitness/fatigue/form trend.
intervals-icu-pp-cli form --days 90

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`sync`** — Sync your full activity, wellness, event, workout and gear history into a local SQLite database for offline search and SQL.

  _Reach for this first so search/sql/form work offline without hammering the API on every query._

  ```bash
  intervals-icu-pp-cli sync --since 365d
  ```
- **`form`** — Compute CTL (fitness), ATL (fatigue) and TSB/form from your synced activity load and show the trend.

  _Use to answer 'am I fresh enough to race / overreaching?' without opening the web UI._

  ```bash
  intervals-icu-pp-cli form --days 90 --json
  ```
- **`since`** — Show planned workouts, completed activities, and what was missed within a recent time window.

  _Use for a quick 'what happened / what's coming' digest after time away._

  ```bash
  intervals-icu-pp-cli since 7d --json
  ```

### Offline analysis
- **`curve compare`** — Compare best power/pace/HR curves between two date ranges from the local store.

  _Use to quantify season-over-season fitness change for a given duration._

  ```bash
  intervals-icu-pp-cli curve compare --metric power --this 90d --vs 365d --json
  ```
- **`wellness trends`** — Correlate HRV / resting HR / sleep against training load over a window from the local store.

  _Use to spot whether HRV/resting-HR are tracking accumulated fatigue._

  ```bash
  intervals-icu-pp-cli wellness trends --days 60 --json
  ```
- **`gear status`** — Roll up distance/time per gear component against reminders to flag what needs service or replacement.

  _Use to catch chain/tyre/shoe replacement thresholds before they are overdue._

  ```bash
  intervals-icu-pp-cli gear status --json
  ```

## Recipes


### Season-over-season power

```bash
intervals-icu-pp-cli curve compare --metric power --this 90d --vs 365d --json --select this.peaks,vs.peaks
```

Compare recent best power against a year ago, selecting just the peak arrays.

### Fresh enough to race?

```bash
intervals-icu-pp-cli form --days 42 --json
```

Show six weeks of fitness/fatigue/form to judge taper readiness.

### Catch up after a break

```bash
intervals-icu-pp-cli since 14d
```

Digest of planned, completed and missed sessions over two weeks.

## Usage

Run `intervals-icu-pp-cli --help` for the full command reference and flag list.

## Commands

### activity

Manage activity

- **`intervals-icu-pp-cli activity delete`** - Delete an activity
- **`intervals-icu-pp-cli activity get`** - An empty stub object is returned for Strava activities
- **`intervals-icu-pp-cli activity update`** - Strava activities cannot be updated

### athlete

Manage athlete

- **`intervals-icu-pp-cli athlete get`** - Get the athlete with sportSettings and custom_items
- **`intervals-icu-pp-cli athlete update`** - Update an athlete

### athlete-plans

Manage athlete plans

- **`intervals-icu-pp-cli athlete-plans`** - Change training plans for a list of athletes

### chats

Manage chats

- **`intervals-icu-pp-cli chats send-message`** - Returns the new message id. If a new chat was created then it is also returned.
- **`intervals-icu-pp-cli chats show`** - Get a chat by id

### disconnect-app

Manage disconnect app

- **`intervals-icu-pp-cli disconnect-app`** - Disconnect the athlete from the app matching the bearer token

### download-workout-ext

Manage download workout ext

- **`intervals-icu-pp-cli download-workout-ext <ext>`** - The athlete to use is extracted from the bearer token and used to resolve power targets etc.. Note that the create workout endpoint can convert workouts and might be more convenient.

### pace-distances

Manage pace distances

- **`intervals-icu-pp-cli pace-distances`** - List pace curve distances

### shared-event

Manage shared event

- **`intervals-icu-pp-cli shared-event <id>`** - Get a shared event (e.g. race)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
intervals-icu-pp-cli activity get mock-value

# JSON for scripting and agents
intervals-icu-pp-cli activity get mock-value --json

# Filter to specific fields
intervals-icu-pp-cli activity get mock-value --json --select id,name,status

# Dry run — show the request without sending
intervals-icu-pp-cli activity get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
intervals-icu-pp-cli activity get mock-value --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Health Check

```bash
intervals-icu-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/intervals-icu-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `INTERVALS_ICU_API_KEY` | per_call | Yes |  |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `intervals-icu-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `intervals-icu-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $INTERVALS_ICU_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized** — Set INTERVALS_ICU_API_KEY to your key from intervals.icu Settings - Developer; username is the literal API_KEY.
- **search/sql return nothing** — Run sync first; those commands read the local store, not the live API.

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**hhopke/intervals-icu-mcp**](https://github.com/hhopke/intervals-icu-mcp) — Python
- [**eddmann/intervals-icu-mcp**](https://github.com/eddmann/intervals-icu-mcp) — Python
- [**mvilanova/intervals-mcp-server**](https://github.com/mvilanova/intervals-mcp-server) — Python
- [**q050cr/intervals-icu**](https://github.com/q050cr/intervals-icu) — Python
- [**rday/py-intervalsicu**](https://github.com/rday/py-intervalsicu) — Python
- [**freekode/tp2intervals**](https://github.com/freekode/tp2intervals) — Kotlin

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
