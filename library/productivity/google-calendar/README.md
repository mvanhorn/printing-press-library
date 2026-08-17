# Google Calendar CLI

**The multi-account Google Calendar layer for agents that must never surprise a human.**

Role-scoped OAuth profiles (a read-only account literally cannot write, by token), a conflicts engine whose all-clear carries coverage evidence, and mutations that are idempotent, etag-guarded, undo-bearing, and structurally incapable of emailing attendees. Built as the calendar substrate for an always-on personal-assistant agent; useful to anyone juggling multiple Google accounts from scripts.

Learn more at [Google Calendar API](https://developers.google.com/calendar).

## Install

The recommended path installs both the `google-calendar-pp-cli` binary and the `pp-google-calendar` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install google-calendar
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install google-calendar --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install google-calendar --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install google-calendar --agent claude-code
npx -y @mvanhorn/printing-press-library install google-calendar --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-calendar-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install google-calendar --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-google-calendar --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-google-calendar --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install google-calendar --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-calendar-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. If Claude Desktop prompts for `CALENDAR_OAUTH2C`, you can leave it blank for the normal gauth-profile flow — first run `google-calendar-pp-cli accounts auth --account <name>` in a terminal per account (see Authentication). `CALENDAR_OAUTH2C` is only a raw bearer-token fallback for the generated passthrough commands.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "google-calendar": {
      "command": "google-calendar-pp-mcp",
      "env": {
        "GCAL_CONFIG_DIR": "<path-to-your-gauth-config-dir>"
      }
    }
  }
}
```

Auth comes from the gauth profile dir (run `google-calendar-pp-cli accounts auth --account <name>` per account first); omit `GCAL_CONFIG_DIR` to use the default `~/.config/google-calendar-pp-cli/gauth`. Optional extras: `GOOGLE_CALENDAR_CALENDAR_ID` (fallback `{calendarId}` path placeholder) and `CALENDAR_OAUTH2C` (raw bearer-token fallback for generated passthrough commands).

</details>

## Authentication

Auth is per-profile installed-app OAuth: you supply a client.json from your own Google Cloud project, then run auth --account <name> once per account. Each profile declares a role — readonly profiles request calendar.readonly only, so write scopes never exist on that token. Tokens live in the configured token dir, never in the repo.

## Quick Start

```bash
# One browser round per account; the profile's role decides which scopes are requested
google-calendar-pp-cli accounts auth --account personal

# Confirm every profile authenticated and show each token's role
google-calendar-pp-cli accounts

# Merged calendar list across all accounts — the raw material for your manifest
google-calendar-pp-cli calendars --json

# Tomorrow across every account, with per-source freshness stamps
google-calendar-pp-cli agenda --from 2026-08-18 --to 2026-08-19 --agent

# The verdict: double-bookings, suspected mirrors, and coverage ('checked N of M')
google-calendar-pp-cli conflicts --from 2026-08-18 --to 2026-08-25 --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Verdict contracts
- **`conflicts`** — One command answers 'am I double-booked?' across every account, and refuses to give a confident all-clear over partial or stale data.

  _Reach for this whenever asserting schedule truth to a human — its downgrade semantics make the answer citable._

  ```bash
  google-calendar-pp-cli conflicts --from 2026-08-18 --to 2026-08-25 --agent
  ```
- **`slots`** — Ranked open windows of a requested duration across all accounts, computed from live freebusy under the same busy semantics as conflicts.

  _Use when proposing meeting times — 'open' carries coverage evidence, not guesswork._

  ```bash
  google-calendar-pp-cli slots --duration 90m --from 2026-08-18 --to 2026-08-22 --agent
  ```

### Safe writes
- **`events update`** — Writes take an etag precondition and return the pre-image plus a structured inverse operation, so every change is cleanly reversible and race-safe.

  _Use for any mutation an agent must be able to echo-with-undo to its principal._

  ```bash
  google-calendar-pp-cli events update primary evt123 --account ads --start 2026-08-19T15:00:00-06:00 --if-etag "abc" --json
  ```

### Governance and drift
- **`changes`** — Everything that moved, appeared, or was cancelled across all accounts since a caller-held watermark — deletions included.

  _Run between full sweeps to catch mid-day schedule movement cheaply._

  ```bash
  google-calendar-pp-cli changes --since 2026-08-17T07:00:00Z --agent
  ```
- **`manifest check`** — Diffs live calendar reality across all accounts against the approved calendars.yaml — new, missing, or permission-drifted calendars become findings.

  _Schedule this as hygiene; run it before trusting any multi-account verdict after account changes._

  ```bash
  google-calendar-pp-cli manifest check --json
  ```
- **`events exceptions`** — Instances of recurring events that moved or were cancelled in a window — the routine-deviation surprises, isolated.

  _Use in look-ahead sweeps: deviations from routine are the highest-yield surprise class._

  ```bash
  google-calendar-pp-cli events exceptions --from 2026-08-18 --to 2026-08-25 --agent
  ```

## Safety guarantees

Every event mutation — generated passthroughs (`calendars events insert|update|patch|delete|move`) and the novel `events update` alike — passes a structural barrier at the HTTP client choke point. None of these are opt-outable by flag:

1. **`sendUpdates=none` is forced** (and legacy `sendNotifications` stripped): no mutation performed by this binary can ever email attendees.
2. **Attendee-bearing events are never mutated**: payloads carrying attendees are refused, and mutations of existing events are preceded by a live check that refuses events that have attendees.
3. **Etag preconditions by default**: when the caller supplies no `If-Match`, the pre-check's etag is attached, so a mid-flight human edit fails the write cleanly (HTTP 412) instead of being clobbered.

Additionally, calendars declared `role: read` in the manifest are refused as write targets, and readonly gauth profiles hold tokens without write scopes at all.

## Recipes

### Morning conflict sweep

```bash
google-calendar-pp-cli conflicts --from today --to +7d --agent
```

The daily verdict: overlaps, mirrors flagged separately, and an explicit coverage line.

### Narrow a verbose agenda for context-poor consumers

```bash
google-calendar-pp-cli agenda --from today --to +2d --agent --select events.summary,events.start,events.account
```

Dotted-path narrowing keeps multi-account agenda payloads small enough for tight agent contexts.

### Find 90 minutes this week

```bash
google-calendar-pp-cli slots --duration 90m --from today --to +5d --between 09:00-17:00 --agent
```

Ranked open windows under the same busy semantics as conflicts — no false frees.

### What moved since 7am

```bash
google-calendar-pp-cli changes --since 2026-08-17T07:00:00Z --agent
```

Stateless mid-day sweep; cancellations included.

### Reversible reschedule

```bash
google-calendar-pp-cli events update primary evt123 --account ads --start 2026-08-19T15:00:00-06:00 --if-etag "etag-from-read" --json
```

Fails cleanly if the event changed since you read it; response carries the pre-image and inverse op.

## Usage

Run `google-calendar-pp-cli --help` for the full command reference and flag list.

## Paths & environment variables

This CLI separates local files into four path kinds:

| Kind | Contents |
|------|----------|
| `config` | User-editable settings such as `config.toml` and saved profiles |
| `data` | Durable local data: `credentials.toml`, `data.db`, cookies, browser-session proof files, and other auth sidecars |
| `state` | Runtime state such as persisted queries, jobs, and `teach.log` |
| `cache` | Regenerable HTTP/cache files |

Each kind resolves independently. The ladder is:

1. Per-kind env var: `GOOGLE_CALENDAR_CONFIG_DIR`, `GOOGLE_CALENDAR_DATA_DIR`, `GOOGLE_CALENDAR_STATE_DIR`, or `GOOGLE_CALENDAR_CACHE_DIR`
2. `--home <dir>` for this invocation
3. `GOOGLE_CALENDAR_HOME` for a flat relocated root
4. XDG env vars: `XDG_CONFIG_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`, `XDG_CACHE_HOME`
5. Platform defaults matching existing installs

For containers and agent sandboxes, prefer a single relocated root:

```bash
export GOOGLE_CALENDAR_HOME=/srv/google-calendar
google-calendar-pp-cli doctor
```

Under `GOOGLE_CALENDAR_HOME=/srv/google-calendar`, the four dirs resolve to `/srv/google-calendar/config`, `/srv/google-calendar/data`, `/srv/google-calendar/state`, and `/srv/google-calendar/cache`.

MCP servers do not receive CLI flags from the host. Put relocation in the host `env` block:

```json
{
  "mcpServers": {
    "google-calendar": {
      "command": "google-calendar-pp-mcp",
      "env": {
        "GOOGLE_CALENDAR_HOME": "/srv/google-calendar"
      }
    }
  }
}
```

Precedence matters in fleets: an ambient per-kind variable such as `GOOGLE_CALENDAR_DATA_DIR` overrides an explicit `--home` for that kind. Use `GOOGLE_CALENDAR_HOME` or the per-kind variables for durable fleet relocation; treat `--home` as the weaker per-invocation lever.

Relocation is one-way. Unsetting `GOOGLE_CALENDAR_HOME` does not move files back to platform defaults, and `doctor` cannot find credentials left under a former root. Move the files manually before unsetting relocation variables.

Existing installs keep working because the platform-default rung matches the legacy layout. On the first auth write, stored secrets leave `config.toml` and are consolidated into `credentials.toml` under the data directory. Run `google-calendar-pp-cli doctor --fail-on warn` to check path and credential-location warnings in automation.

## Commands

### accounts

gauth profiles

- **`google-calendar-pp-cli accounts`** - List gauth profiles and each token's status (roles decide OAuth scopes).
- **`google-calendar-pp-cli accounts auth --account <name>`** (or `--all`) - Run the browser OAuth flow for one profile (or every profile), requesting only the profile role's scopes.

### agenda / calendars

Merged reads across all accounts

- **`google-calendar-pp-cli agenda --from <when> --to <when>`** - Merged agenda across every manifest calendar, sorted by start, with per-source freshness and a coverage block.
- **`google-calendar-pp-cli calendars`** - Merged calendar list across all accounts (account, summary, access role, id).
- **`google-calendar-pp-cli calendars <calendarId> --account <name>`** - Metadata for one calendar.

### calendars events

Per-calendar event CRUD. Every mutation passes the safety barrier (see Safety guarantees).

- **`google-calendar-pp-cli calendars events list <calendarId>`** - Events on one calendar (`--q`, `--time-min`, `--time-max`, `--single-events`, `--show-deleted`, ...).
- **`google-calendar-pp-cli calendars events get <calendarId> <eventId>`** - One event.
- **`google-calendar-pp-cli calendars events instances <calendarId> <eventId>`** - Instances of a recurring event.
- **`google-calendar-pp-cli calendars events insert <calendarId>`** (alias `create`) - Creates an event.
- **`google-calendar-pp-cli calendars events update|patch <calendarId> <eventId>`** - Full/partial update (prefer top-level `events update` for the undo-bearing, etag-preconditioned contract).
- **`google-calendar-pp-cli calendars events delete <calendarId> <eventId>`** - Deletes an event.
- **`google-calendar-pp-cli calendars events move <calendarId> <eventId> --destination <calendarId>`** - Moves an event to another calendar.

### free-busy

- **`google-calendar-pp-cli free-busy --items <list> --time-min <rfc3339> --time-max <rfc3339>`** - Free/busy for a set of calendars on one account. Prefer `slots`/`conflicts` for cross-account availability verdicts.


### Self-learning loop

This CLI caches per-question discovery so repeat queries skip the walk and structurally similar queries get answered via entity substitution. The loop also self-captures: every invocation is journaled locally, and failed-flag corrections plus fresh teaches surface as candidates on the next `recall` for confirm/reject judgment. Agents call `recall` before discovery and fire `teach &` after answering. See the `## Automatic learning` section in `SKILL.md` for the full protocol.

- **`google-calendar-pp-cli recall <query>`** - Look up cached resources for a query before running discovery
- **`google-calendar-pp-cli teach`** - Record a query -> resource mapping (silent on success, safe to background with `&`)
- **`google-calendar-pp-cli learnings list`** - Inspect taught rows
- **`google-calendar-pp-cli learnings forget <query>`** - Undo a teach
- **`google-calendar-pp-cli learnings candidates`** - List auto-captured candidates awaiting confirm/reject
- **`google-calendar-pp-cli learnings stats`** - Local loop metrics: recall hit rate, teach-to-reuse, playbook resolution, candidate counts
- **`google-calendar-pp-cli teach-pattern`** - Install a query/resource template up front
- **`google-calendar-pp-cli teach-lookup`** - Add an entity mapping (e.g. country code, team alias) for pattern substitution

Pass `--no-learn` or set `GOOGLE_CALENDAR_NO_LEARN=true` to disable the loop for deterministic flows.

The local store's schema version stamp is one-way: once this version of `google-calendar-pp-cli` opens the database, older binaries refuse it with a version error — upgrade the binary rather than downgrading.

## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
google-calendar-pp-cli calendars primary --account personal

# JSON for scripting and agents
google-calendar-pp-cli calendars primary --account personal --json

# Filter to specific fields
google-calendar-pp-cli calendars primary --account personal --json --select id,summary,accessRole

# Dry run — show the request without sending
google-calendar-pp-cli calendars primary --account personal --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-calendar-pp-cli calendars primary --account personal --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and add `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

Verdict commands carry typed exit codes that override the generic meanings of `3` and `4`: `conflicts` — 0 no conflicts with complete coverage, 3 conflicts found, 4 no conflicts but coverage incomplete; `manifest check` — 0 clean, 3 findings, 4 no findings but an account unreadable; `slots`/`changes`/`events exceptions`/`agenda` — 0 complete coverage, 4 one or more sources unreadable.

## Runtime Endpoint

This CLI resolves endpoint placeholders at runtime, so one installed binary can target different tenants or API versions without regeneration.

Endpoint environment variables:
- `GOOGLE_CALENDAR_CALENDAR_ID` resolves `{calendarId}`

Base URL: `https://www.googleapis.com/calendar/v3`

## Health Check

```bash
google-calendar-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Run `google-calendar-pp-cli doctor` to see the resolved config, data, state, and cache directories. The platform-default config path is `~/.config/google-calendar-pp-cli/config.toml`; `--home`, `GOOGLE_CALENDAR_HOME`, and per-kind env vars can relocate it. The gauth auth dir (client.json, profiles.yaml, calendars.yaml, tokens) resolves separately via `--auth-dir` / `GCAL_CONFIG_DIR` (default `~/.config/google-calendar-pp-cli/gauth`).

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GCAL_CONFIG_DIR` | auth | No | gauth config dir (client.json, profiles.yaml, calendars.yaml, tokens); default `~/.config/google-calendar-pp-cli/gauth` |
| `GOOGLE_CALENDAR_CALENDAR_ID` | endpoint | No | Fallback for the `{calendarId}` path placeholder when a command does not receive it as an argument |
| `CALENDAR_OAUTH2C` | per_call | No | Raw bearer-token fallback for generated passthrough commands only. Normal auth is gauth profiles (`accounts auth`); leave this unset. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `google-calendar-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `google-calendar-pp-cli doctor` to check credentials
- Run `google-calendar-pp-cli accounts` to see each profile's token status; re-run `accounts auth --account <name>` for any expired or missing token
- Confirm the gauth config dir (`--auth-dir` / `GCAL_CONFIG_DIR`) holds client.json, profiles.yaml, and the tokens
- Note: from `conflicts`/`slots`/`changes`/`agenda`/`events exceptions`, exit 4 means incomplete coverage, not an auth failure — check the output's coverage block
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items
- Note: from `conflicts` and `manifest check`, exit 3 means conflicts/findings present, not "not found"

### API-specific
- **invalid_grant or token expired shortly after setup** — The OAuth app is still in Testing mode — publish it to Production in the Google Cloud console, then re-run auth for each account
- **403 accessNotConfigured on first call** — Enable the Google Calendar API on the Cloud project that issued your client.json
- **conflicts reports 'checked N of M sources'** — A source failed or served stale evidence — run doctor and manifest check before trusting an all-clear
- **update/delete refused with attendee-guard error** — By design: attendee-bearing events are never mutated by this CLI; change them in the Calendar UI

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**gcalcli**](https://github.com/insanum/gcalcli) — Python
- [**google-calendar-mcp**](https://github.com/nspady/google-calendar-mcp) — TypeScript
- [**mcp-google-multi**](https://github.com/bakissation/mcp-google-multi) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
