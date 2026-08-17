# Tiimo CLI

**Your Tiimo plan, finally exportable, queryable, and yours.**

Tiimo has no public API and declines to let you export your data or sync it back to a calendar. This CLI mirrors your own planner into local SQLite and gives you the surfaces the app will not: export to JSON, CSV, or iCalendar, a subscribable calendar feed, and analysis of what you planned versus what actually happened.

Learn more at [Tiimo](https://api.tiimoapp.com).

Created by [@vcolombo](https://github.com/vcolombo) (Vincent Colombo).

## Install

The recommended path installs both the `tiimo-pp-cli` binary and the `pp-tiimo` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install tiimo
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install tiimo --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install tiimo --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install tiimo --agent claude-code
npx -y @mvanhorn/printing-press-library install tiimo --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.6 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/tiimo/cmd/tiimo-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tiimo-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install tiimo --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-tiimo --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-tiimo --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install tiimo --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/tiimo-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `TIIMO_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/tiimo/cmd/tiimo-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "tiimo": {
      "command": "tiimo-pp-mcp",
      "env": {
        "TIIMO_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Tiimo authenticates through OpenID Connect at auth.tiimoapp.com. The simplest path is to set TIIMO_TOKEN to a bearer token for the tiimo_webapi scope; run `tiimo-pp-cli doctor` to confirm the token is accepted. Tokens are short-lived, so store a refresh token when available and let the CLI renew it.

## Quick Start

```bash
# confirm the token is set and the API is reachable before anything else
tiimo-pp-cli doctor

# mirror the last 90 days of activities into local SQLite so reads are offline and fast
tiimo-pp-cli sync --since 90d

# the plan for today, grouped the way the app groups it
tiimo-pp-cli today

# the headline: your plan as a calendar file, which Tiimo itself will not give you
tiimo-pp-cli feed --out tiimo.ics --days 30

# what actually happened versus what you planned
tiimo-pp-cli drift --days 30

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Your data, out of the app
- **`export`** — Stream a paginated resource straight to JSON or JSONL. Use backup for a whole-planner snapshot and feed for calendar or spreadsheet formats.

  _Reach for this to pull one resource as a stream; for everything at once use backup, and for a calendar file use feed._

  ```bash
  tiimo-pp-cli export profiles --format json
  ```
- **`feed`** — Generate a subscribable read-only iCalendar file of your Tiimo activities.

  _Use this to make Tiimo activities visible inside any calendar client without giving that client write access._

  ```bash
  tiimo-pp-cli feed --out ~/tiimo.ics --days 30
  ```
- **`backup`** — Write every mirrored record - activities, to-dos, tags, routines, calendars - to one portable JSON snapshot. This is the whole-planner export Tiimo does not offer.

  _This is the command to reach for when the user says they want their Tiimo data out. Use it before any bulk change, or on a schedule._

  ```bash
  tiimo-pp-cli backup --out ~/tiimo-backups
  ```

### Plan versus reality
- **`drift`** — Show which activities consistently start late, overrun, or spend the most time paused.

  _Reach for this when the user asks why their schedule never holds, or which task always blows its estimate._

  ```bash
  tiimo-pp-cli drift --days 30 --agent
  ```
- **`stalls`** — Find the exact checklist step where a multi-step routine tends to break down.

  _Use when a routine keeps failing and the user does not know which step is the blocker._

  ```bash
  tiimo-pp-cli stalls --weeks 8
  ```
- **`adherence`** — Completion rate for each recurring activity over a window.

  _Use to answer how reliably a habit is actually being kept, rather than whether it exists on the calendar._

  ```bash
  tiimo-pp-cli adherence --weeks 4 --agent
  ```

### Timeline intelligence
- **`overlaps`** — List activities that are double-booked against each other.

  _Use before committing to a plan, or when the user suspects their day is over-scheduled._

  ```bash
  tiimo-pp-cli overlaps --from 2026-08-14 --to 2026-08-21
  ```
- **`gaps`** — Find the unscheduled windows in a day that are at least a given length.

  _Use when fitting something new into an existing day without opening the app._

  ```bash
  tiimo-pp-cli gaps --min 30m --from 2026-08-14
  ```
- **`rolling`** — A true rolling multi-day view starting from today.

  _Use for a forward-looking view that does not reset at an arbitrary week boundary._

  ```bash
  tiimo-pp-cli rolling --days 7
  ```
- **`capacity`** — Committed versus free minutes per day, broken down by time-of-day bucket.

  _Use to judge whether a day has real room left before agreeing to add something to it._

  ```bash
  tiimo-pp-cli capacity --from 2026-08-14 --to 2026-08-21 --agent
  ```

## Recipes

### Capture a task, or a whole brain dump

```bash
tiimo-pp-cli todo add "book dentist"
```

Capture is the whole point for the target user, so the CLI takes one task per line on stdin. Pipe into it (printf 'call pharmacy\nbook dentist\n' | tiimo-pp-cli todo add --stdin) or redirect a file with < tasks.txt; blank lines are skipped and each line becomes its own task.

### Narrow a deeply nested activity payload for an agent

```bash
tiimo-pp-cli agenda --from 2026-08-14 --to 2026-08-21 --agent --select title,startTime,duration,completedAt,checklist.isCompleted
```

Activities carry dozens of fields including nested checklists and recurrence; selecting a handful keeps the response small enough to reason over.

### Find a free hour this week

```bash
tiimo-pp-cli gaps --min 60m --from 2026-08-14 --to 2026-08-21
```

Returns the unscheduled windows long enough to actually use, without opening the app.

### Publish a calendar feed your other tools can subscribe to

```bash
tiimo-pp-cli feed --out ~/Public/tiimo.ics --days 60
```

Produces the read-only calendar view Tiimo users asked for and were told would not be built.

### See which routine step keeps breaking

```bash
tiimo-pp-cli stalls --weeks 8 --agent
```

Aggregates per-step checklist completion so the failure point is a fact rather than a guess.

## Usage

Run `tiimo-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `TIIMO_CONFIG_DIR`, `TIIMO_DATA_DIR`, `TIIMO_STATE_DIR`, or `TIIMO_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `TIIMO_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export TIIMO_HOME=/srv/tiimo
tiimo-pp-cli doctor
```

Under `TIIMO_HOME=/srv/tiimo`, the four dirs resolve to `/srv/tiimo/config`, `/srv/tiimo/data`, `/srv/tiimo/state`, and `/srv/tiimo/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "tiimo": {
      "command": "tiimo-pp-mcp",
      "env": {
        "TIIMO_HOME": "/srv/tiimo"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `TIIMO_DATA_DIR` overrides an explicit `--home` for that kind. Use `TIIMO_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `TIIMO_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `tiimo-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### activities

Timeline activities - the scheduled items that make up a day

- **`tiimo-pp-cli activities create`** - Create an activity on the timeline
- **`tiimo-pp-cli activities delete`** - Delete an activity
- **`tiimo-pp-cli activities get`** - Get a single activity by ID
- **`tiimo-pp-cli activities list`** - List activities in a date window, keyed by day
- **`tiimo-pp-cli activities update`** - Replace an activity. The API rejects PATCH with 405, so send the full object.

### calendars

External calendars linked into Tiimo

- **`tiimo-pp-cli calendars activities`** - List imported activities from one linked calendar. These are read-only in Tiimo.
- **`tiimo-pp-cli calendars list`** - List linked external calendars

### configuration

Server-side feature configuration

- **`tiimo-pp-cli configuration`** - Feature flags, locked features, and client configuration

### profiles

Tiimo profiles (an account can hold several shared profiles)

- **`tiimo-pp-cli profiles get`** - Get a single profile
- **`tiimo-pp-cli profiles list`** - List every profile on the account

### routines

Saved routines

- **`tiimo-pp-cli routines <profile_id>`** - List routines in a date window

### tags

User-defined tags applied to activities and tasks

- **`tiimo-pp-cli tags <profile_id>`** - List tags

### todo_lists

To-do lists that group tasks

- **`tiimo-pp-cli todo-lists <profile_id>`** - List to-do lists with their tasks

### todo_tasks

To-do tasks - the priority-bucketed list, separate from the timeline

- **`tiimo-pp-cli todo-tasks create`** - Create a to-do task. Returns 200 with the created task.
- **`tiimo-pp-cli todo-tasks delete`** - Delete a to-do task
- **`tiimo-pp-cli todo-tasks get`** - Get a single to-do task
- **`tiimo-pp-cli todo-tasks list`** - List to-do tasks
- **`tiimo-pp-cli todo-tasks update`** - Update a to-do task. Note the API updates via the COLLECTION path; item-level PUT returns 405.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`tiimo-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`tiimo-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`tiimo-pp-cli learnings list`** - Inspect taught rows
- **`tiimo-pp-cli learnings forget <query>`** - Undo a teach
- **`tiimo-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`tiimo-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`tiimo-pp-cli teach-pattern`** - Install a query/resource template up front
- **`tiimo-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `TIIMO_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `tiimo-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
tiimo-pp-cli activities list mock-value --from-date 2026-01-15

# JSON for scripting and agents
tiimo-pp-cli activities list mock-value --from-date 2026-01-15 --json
# Filter to specific fields
tiimo-pp-cli activities list mock-value --from-date 2026-01-15 --json --select date,activities

# Dry run — show the request without sending
tiimo-pp-cli activities list mock-value --from-date 2026-01-15 --dry-run

# Agent mode — JSON + compact + no prompts in one flag
tiimo-pp-cli activities list mock-value --from-date 2026-01-15 --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select <field>[,<field>...]` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Freshness

This CLI owns bounded freshness for registered store-backed read command paths. In `--data-source auto` mode, covered commands check the local SQLite store before serving results; stale or missing resources trigger a bounded refresh, and refresh failures fall back to the existing local data with a warning. `--data-source local` never refreshes, and `--data-source live` reads the API without mutating the local store.

Set `TIIMO_NO_AUTO_REFRESH=1` to disable the pre-read freshness hook while preserving the selected data source.

Covered command paths:
- `tiimo-pp-cli profiles`
- `tiimo-pp-cli profiles get`
- `tiimo-pp-cli profiles list`
- `tiimo-pp-cli profiles search`

JSON outputs that use the generated provenance envelope include freshness metadata at `meta.freshness`. This metadata describes the freshness decision for the covered command path; it does not claim full historical backfill or API-specific enrichment.

## Health Check

```bash
tiimo-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `tiimo-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/tiimo-pp-cli/config.toml`; `--home`, `TIIMO_HOME`, and per-kind env vars can relocate it.

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `TIIMO_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `tiimo-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `tiimo-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $TIIMO_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **doctor reports 401 Unauthorized** — The bearer token has expired. Tiimo access tokens are short-lived; refresh it and re-export TIIMO_TOKEN.
- **Commands return no rows even though the app shows activities** — The local mirror is empty or stale. Run `tiimo-pp-cli sync --since 90d`.
- **400 with a fromDate validation error** — The activities endpoint requires a date window. Pass --from and --to, or let sync supply defaults.
- **An activity will not update and reports read-only** — It came from a linked external calendar. Edit it in the source calendar; Tiimo treats imported events as read-only.

## HTTP Transport

This CLI uses Chrome-compatible HTTP transport for browser-facing endpoints. It does not require a resident browser process for normal API calls.

TLS certificates are verified by default. For a trusted development or self-signed endpoint only, pass `--insecure` for one invocation, set `TIIMO_SKIP_TLS_VERIFY=true` for the current environment, or set `skip_tls_verify = true` in the config file for a persistent override.

## Discovery Signals

This CLI was generated with browser-captured traffic analysis.
- Target observed: https://webapp.tiimoapp.com/home
- Capture coverage: 15 API entries from 15 total network entries
- Reachability: standard_http (65% confidence)
- Protocols: rest_json (75% confidence)
- Auth signals: oauth2_bearer
- Candidate command ideas: create_activities — Derived from observed POST /api/profiles/{profile_id}/activities traffic.; delete_activities — Derived from observed DELETE /api/profiles/{profile_id}/activities/{activity_id} traffic.; get_activities — Derived from observed GET /api/externalCalendar/profiles/{profile_id}/externalCalendars/{externalcalendar_id}/activities traffic.; get_linkedCalendars — Derived from observed GET /api/externalCalendar/profiles/{profile_id}/linkedCalendars traffic.; get_profiles — Derived from observed GET /api/profiles/{profile_id} traffic.; get_routines — Derived from observed GET /api/profiles/{profile_id}/routines traffic.; get_tags — Derived from observed GET /api/profiles/{profile_id}/tags traffic.; get_todo_task_lists — Derived from observed GET /api/profiles/{profile_id}/todo-task-lists traffic.

Warnings from discovery:
- empty_payload: API-looking request returned an empty or null payload; schema confidence is weak.

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
