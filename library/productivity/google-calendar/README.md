# Google Calendar CLI

**Every gcalcli and MCP feature, plus a local SQLite mirror that makes free-time search, conflict detection, and 'what changed' instant and offline.**

A complete Google Calendar v3 CLI with full CRUD on events, calendars, ACLs, free/busy, and settings — plus a local event store that caches events as you query them. That store unlocks commands no live-only tool can offer: free-slot finding (free), cross-calendar conflict detection (conflicts), change windows (changes), and meeting-load analytics (load). Agent-native throughout: --json, --select, --dry-run, and typed exit codes.

Learn more at [Google Calendar](https://google.com).

Printed by [@rubenasgaspar](https://github.com/rubenasgaspar) (Rúben Gaspar).

## Install

The recommended path installs both the `google-calendar-pp-cli` binary and the `pp-google-calendar` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press install google-calendar
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install google-calendar --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press install google-calendar --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press install google-calendar --agent claude-code
npx -y @mvanhorn/printing-press install google-calendar --agent claude-code --agent codex
```

### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-calendar-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-google-calendar --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-google-calendar --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-google-calendar skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-google-calendar. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/google-calendar-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `GOOGLE_CALENDAR_CLIENT_ID` when Claude Desktop prompts you.

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
        "GOOGLE_CALENDAR_CLIENT_ID": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Google Calendar uses OAuth2 (authorization-code). Run `google-calendar-pp-cli auth login` once to authorize in your browser; tokens are stored locally and refreshed automatically. Scopes follow least-privilege: read-only commands request calendar.readonly.

## Quick Start

```bash
# One-time browser OAuth; stores and auto-refreshes tokens locally.
google-calendar-pp-cli auth login


# Mirror your calendars and events into the local SQLite store.
google-calendar-pp-cli sync --full


# See today across all synced calendars, offline.
google-calendar-pp-cli agenda --window today


# Find open one-hour slots this week.
google-calendar-pp-cli free --calendars primary --window 'next 7 days' --duration 60m


# Surface any double-bookings across calendars as JSON.
google-calendar-pp-cli conflicts --window 'this week' --agent

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`free`** — Find open time blocks of a given length across several calendars in one query.

  _When an agent needs to actually book something, reach for this instead of pulling every event and computing gaps yourself._

  ```bash
  google-calendar-pp-cli free --calendars primary,work --window 'next 7 days' --duration 90m --agent
  ```
- **`conflicts`** — List events that overlap across two or more calendars in a time window.

  _Use to detect double-bookings before they bite, without scanning calendars by hand._

  ```bash
  google-calendar-pp-cli conflicts --calendars primary,team --window 'this week' --agent
  ```
- **`changes`** — Show events created, updated, or cancelled since a date, by update timestamp (updatedMin), deletions included.

  _Reach for this to answer 'what moved on my calendar since Friday?' in one call._

  ```bash
  google-calendar-pp-cli changes --since 2026-05-17 --calendars primary --agent
  ```
- **`load`** — Count meetings and summed booked hours per day, week, or calendar over a window.

  _Use to quantify how booked a window is before committing to more meetings._

  ```bash
  google-calendar-pp-cli load --window 'this month' --group-by week --agent
  ```
- **`rsvp-status`** — Summarize accepted/declined/tentative counts per event across a window.

  _Reach for this to see which upcoming meetings still need responses._

  ```bash
  google-calendar-pp-cli rsvp-status --window 'next 14 days' --agent
  ```

### Agent-native plumbing
- **`acl-audit`** — Flatten every calendar's sharing rules into one who-has-what-access table.

  _Reach for this before onboarding or offboarding someone to audit access in one place._

  ```bash
  google-calendar-pp-cli acl-audit --role writer --agent
  ```
- **`book`** — Create an event but abort (or warn) with a typed exit code if it overlaps an existing event.

  _Use when an agent books on a user's behalf and must never double-book._

  ```bash
  google-calendar-pp-cli book --summary 'Sync' --start 2026-05-25T14:00:00Z --end 2026-05-25T15:00:00Z --on-conflict abort
  ```

## Usage

Run `google-calendar-pp-cli --help` for the full command reference and flag list.

## Commands

### calendars

Manage calendars

- **`google-calendar-pp-cli calendars delete`** - Deletes a secondary calendar. Use calendars.clear for clearing all events on primary calendars.
- **`google-calendar-pp-cli calendars get`** - Returns metadata for a calendar.
- **`google-calendar-pp-cli calendars insert`** - Creates a secondary calendar.
- **`google-calendar-pp-cli calendars patch`** - Updates metadata for a calendar. This method supports patch semantics.
- **`google-calendar-pp-cli calendars update`** - Updates metadata for a calendar.

### channels

Manage channels

- **`google-calendar-pp-cli channels`** - Stop watching resources through this channel

### colors

Manage colors

- **`google-calendar-pp-cli colors`** - Returns the color definitions for calendars and events.

### free-busy

Manage free busy

- **`google-calendar-pp-cli free-busy`** - Returns free/busy information for a set of calendars.

### users

Manage users

- **`google-calendar-pp-cli users calendar-calendar-list-delete`** - Removes a calendar from the user's calendar list.
- **`google-calendar-pp-cli users calendar-calendar-list-get`** - Returns a calendar from the user's calendar list.
- **`google-calendar-pp-cli users calendar-calendar-list-insert`** - Inserts an existing calendar into the user's calendar list.
- **`google-calendar-pp-cli users calendar-calendar-list-list`** - Returns the calendars on the user's calendar list.
- **`google-calendar-pp-cli users calendar-calendar-list-patch`** - Updates an existing calendar on the user's calendar list. This method supports patch semantics.
- **`google-calendar-pp-cli users calendar-calendar-list-update`** - Updates an existing calendar on the user's calendar list.
- **`google-calendar-pp-cli users calendar-calendar-list-watch`** - Watch for changes to CalendarList resources.
- **`google-calendar-pp-cli users calendar-settings-get`** - Returns a single user setting.
- **`google-calendar-pp-cli users calendar-settings-list`** - Returns all user settings for the authenticated user.
- **`google-calendar-pp-cli users calendar-settings-watch`** - Watch for changes to Settings resources.


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
google-calendar-pp-cli calendars get mock-value

# JSON for scripting and agents
google-calendar-pp-cli calendars get mock-value --json

# Filter to specific fields
google-calendar-pp-cli calendars get mock-value --json --select id,name,status

# Dry run — show the request without sending
google-calendar-pp-cli calendars get mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
google-calendar-pp-cli calendars get mock-value --agent
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
google-calendar-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/calendar-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `GOOGLE_CALENDAR_CLIENT_ID` | auth_flow_input | Yes | OAuth2 client ID from your Google Cloud project. Used by `auth login`. |
| `GOOGLE_CALENDAR_CLIENT_SECRET` | auth_flow_input | Yes | Set during initial auth setup. |
| `GOOGLE_CALENDAR_TOKEN` | harvested | No | Populated automatically by auth login. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `google-calendar-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $GOOGLE_CALENDAR_CLIENT_ID`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **410 GONE on sync** — The sync token expired; run `sync --full` to clear the per-calendar cursor and resync from scratch.
- **rateLimitExceeded (403/429)** — You hit a per-user/per-calendar quota; the client backs off exponentially — retry or narrow the window. Prefer `patch` over `update` to spend fewer quota units.
- **auth token invalid / expired** — Run `google-calendar-pp-cli auth login` again to re-authorize; check `auth status`.
- **free/busy returns 400 on timeZone** — Omit --timezone to use the calendar default, or pass an IANA name like Europe/Lisbon rather than an offset.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**gcalcli**](https://github.com/insanum/gcalcli) — Python (4200 stars)
- [**google-calendar-simple-api**](https://github.com/kuzmoyev/google-calendar-simple-api) — Python (1500 stars)
- [**google-calendar-mcp**](https://github.com/nspady/google-calendar-mcp) — TypeScript (1100 stars)
- [**gcal-cli**](https://github.com/toniov/gcal-cli) — JavaScript (80 stars)
- [**neocal**](https://github.com/oscarmcm/neocal) — Rust (40 stars)

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
