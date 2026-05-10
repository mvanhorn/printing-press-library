# Outlook Calendar CLI

**The first Outlook calendar CLI built for AI agents on personal Microsoft 365 accounts — with offline conflict detection, free-time math, and recurring-drift insights no Graph endpoint can give you.**

Drives your personal Microsoft 365 calendar from scripts and agents. OAuth 2.0 device-code flow against the /common tenant, so personal MSAs work alongside work accounts. A local SQLite store synced through events/delta unlocks `conflicts`, `freetime`, `review`, `pending`, `recurring-drift`, and `prep` — workflows no other Outlook CLI exposes because they require persisted state.

## Install

The recommended path installs both the `outlook-calendar-pp-cli` binary and the `pp-outlook-calendar` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install outlook-calendar
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install outlook-calendar --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/outlook-calendar-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-outlook-calendar --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-outlook-calendar --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-outlook-calendar skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-outlook-calendar. The skill defines how its required CLI can be installed.
```

## Authentication

Authentication uses OAuth 2.0 device-code flow against `https://login.microsoftonline.com/common`. Run `outlook-calendar-pp-cli auth login --device-code` once; visit the displayed URL on any device, enter the code, and you're done. Tokens (access + refresh) are cached at `~/.config/outlook-calendar-pp-cli/config.toml` (mode 0600). Personal Microsoft accounts (Outlook.com, Hotmail, Live, MSA) are first-class and tested.

By default this CLI uses the Microsoft-published Graph PowerShell client id (`14d82eec-204b-4c2f-b7e8-296a70dab67e`), which is configured for `AzureADandPersonalMicrosoftAccount` and works with personal accounts out of the box. For production use, register your own Azure app (Public client; **Accounts in any organizational directory and personal Microsoft accounts**; scopes `Calendars.ReadWrite User.Read offline_access`) and pass it via `--client-id` or the `OUTLOOK_CALENDAR_CLIENT_ID` env var.

Reachability check: `outlook-calendar-pp-cli doctor --json` confirms the access token is valid and `/me` is reachable. To force-rotate the access token use `outlook-calendar-pp-cli auth refresh`.

## Quick Start

```bash
# One-time interactive login; subsequent commands are non-interactive and refresh-token driven.
outlook-calendar-pp-cli auth login --device-code


# Pull events into the local SQLite store via /me/events/delta. Run periodically to keep transcendence features fresh.
outlook-calendar-pp-cli delta events


# Today's week as JSON, ready for an agent.
outlook-calendar-pp-cli events range --from today --to +7d --json


# Find any double-bookings across every calendar you own.
outlook-calendar-pp-cli conflicts --from today --to +14d --json


# Hour-long open slots in working hours over the next week.
outlook-calendar-pp-cli freetime --duration 60m --within 'Mon-Fri 9-17' --next 7d --json


# Agent-shaped dossier for the next four hours of meetings.
outlook-calendar-pp-cli prep --next 4h --json --select subject,location,attendees,body_preview

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`conflicts`** — Find overlapping events across all your Outlook calendars in one pass — the double-bookings Outlook's own UI never shows.

  _When the agent needs to know whether a proposed time blocks the user, this is the source of truth across every calendar the user owns._

  ```bash
  outlook-calendar-pp-cli conflicts --from today --to +7d --json --select pair_id,a.subject,b.subject,overlap_minutes
  ```
- **`freetime`** — Compute N-minute gaps in your working hours over the next K days, honoring all calendars and optional OOF/tentative exclusion.

  _When the agent needs to propose a meeting time, this gives a deterministic answer that respects the user's actual working hours._

  ```bash
  outlook-calendar-pp-cli freetime --duration 60m --within 'Mon-Fri 9-17' --next 7d --exclude-oof --json
  ```
- **`review`** — Diff against the last delta-sync snapshot: what was added, rescheduled, cancelled, or had its RSVP change.

  _Lets the agent answer "what changed since I last looked?" without scanning the whole week._

  ```bash
  outlook-calendar-pp-cli review --since last-sync --json
  ```
- **`pending`** — List events whose RSVP is still pending and whose start time is in the future, ordered by start.

  _Agent task: "what invites do I still need to answer?" — answered in one query._

  ```bash
  outlook-calendar-pp-cli pending --json
  ```
- **`recurring-drift`** — For each recurring-series master, list instances whose start/end/subject/location diverged from the master pattern.

  _Catches the silent organizer-side reschedules that cause people to join Teams calls at the wrong hour._

  ```bash
  outlook-calendar-pp-cli recurring-drift --json
  ```
- **`with`** — How often have I met with this person, and when did I see them last? Counts and recent N events from local store.

  _Agent task: "how often do I meet with X?" without reading the whole calendar._

  ```bash
  outlook-calendar-pp-cli with alice@example.com --since 90d --json
  ```
- **`tz-audit`** — Surface events whose start time-zone differs from the calendar's default or from their own end-time TZ — likely-broken displays on other devices.

  _Agent task: "are any of my events about to render at the wrong hour for someone?" — yes/no with the offenders._

  ```bash
  outlook-calendar-pp-cli tz-audit --json
  ```

### Agent-native plumbing
- **`prep`** — For upcoming events in the next N hours, return a dossier: subject, location, attendee emails, organizer, body excerpt, attachments list, recurrence/online-meeting flags.

  _Single tool call that gives an agent everything needed to brief the user on what's coming up._

  ```bash
  outlook-calendar-pp-cli prep --next 4h --json
  ```

## Usage

Run `outlook-calendar-pp-cli --help` for the full command reference and flag list.

## Commands

### attachments

Manage event attachments

- **`outlook-calendar-pp-cli attachments delete`** - Delete an attachment
- **`outlook-calendar-pp-cli attachments get`** - Get a specific attachment by id
- **`outlook-calendar-pp-cli attachments list`** - List attachments on an event

### availability

Free/busy and meeting-time intelligence (degraded on personal Microsoft accounts; prefer freetime for self-only queries)

- **`outlook-calendar-pp-cli availability find`** - Suggest meeting times based on attendee availability and constraints
- **`outlook-calendar-pp-cli availability schedule`** - Get free/busy schedule for a list of users (limited on personal Microsoft accounts)

### calendars

Manage Outlook calendars on your account

- **`outlook-calendar-pp-cli calendars create`** - Create a new calendar
- **`outlook-calendar-pp-cli calendars default`** - Get the user's default calendar
- **`outlook-calendar-pp-cli calendars delete`** - Delete a calendar
- **`outlook-calendar-pp-cli calendars get`** - Get a calendar by id
- **`outlook-calendar-pp-cli calendars list`** - List all calendars on the account
- **`outlook-calendar-pp-cli calendars update`** - Update a calendar

### categories

Manage Outlook master categories used to tag events

- **`outlook-calendar-pp-cli categories create`** - Create a new master category
- **`outlook-calendar-pp-cli categories delete`** - Delete a master category
- **`outlook-calendar-pp-cli categories list`** - List master categories

### delta

Incremental delta-sync of events into the local SQLite store

- **`outlook-calendar-pp-cli delta events`** - Pull incremental event changes since the last delta token
- **`outlook-calendar-pp-cli delta view`** - Pull incremental calendar-view changes within a window

### events

Outlook calendar events on your default or named calendar

- **`outlook-calendar-pp-cli events accept`** - Accept a meeting invite
- **`outlook-calendar-pp-cli events cancel`** - Cancel an event you organized (notifies attendees)
- **`outlook-calendar-pp-cli events create`** - Create a new event on the default calendar
- **`outlook-calendar-pp-cli events decline`** - Decline a meeting invite
- **`outlook-calendar-pp-cli events delete`** - Delete an event by id
- **`outlook-calendar-pp-cli events dismiss`** - Dismiss the reminder for an event
- **`outlook-calendar-pp-cli events forward`** - Forward an event to additional attendees
- **`outlook-calendar-pp-cli events get`** - Get a single event by id
- **`outlook-calendar-pp-cli events instances`** - List occurrences of a recurring event in a date range
- **`outlook-calendar-pp-cli events list`** - List events on the default calendar
- **`outlook-calendar-pp-cli events range`** - List events occurring within a date range (calendarView; expands recurring instances)
- **`outlook-calendar-pp-cli events search`** - Server-side search across events ($search query)
- **`outlook-calendar-pp-cli events snooze`** - Snooze the reminder for an event until a specific time
- **`outlook-calendar-pp-cli events tentative`** - Tentatively accept a meeting invite
- **`outlook-calendar-pp-cli events update`** - Update fields on an existing event (subject, body, time, location, attendees)


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
outlook-calendar-pp-cli attachments list mock-value

# JSON for scripting and agents
outlook-calendar-pp-cli attachments list mock-value --json

# Filter to specific fields
outlook-calendar-pp-cli attachments list mock-value --json --select id,name,status

# Dry run — show the request without sending
outlook-calendar-pp-cli attachments list mock-value --dry-run

# Agent mode — JSON + compact + no prompts in one flag
outlook-calendar-pp-cli attachments list mock-value --agent
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

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-outlook-calendar -g
```

Then invoke `/pp-outlook-calendar <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add outlook-calendar outlook-calendar-pp-mcp -e OUTLOOK_CALENDAR_TOKEN=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/outlook-calendar-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `OUTLOOK_CALENDAR_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "outlook-calendar": {
      "command": "outlook-calendar-pp-mcp",
      "env": {
        "OUTLOOK_CALENDAR_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
outlook-calendar-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/outlook-calendar-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `OUTLOOK_CALENDAR_TOKEN` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `outlook-calendar-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $OUTLOOK_CALENDAR_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **auth login fails with AADSTS50059 'No tenant-identifying information found'** — Confirm your Azure app's Supported account types is `AzureADandPersonalMicrosoftAccount`. Re-register if it was set to single-tenant.
- **auth login fails with AADSTS70016 polling timeout** — Re-run `auth login --device-code`; the polling window is ~15 minutes.
- **401 on /me/events after a long idle** — Run `outlook-calendar-pp-cli auth refresh` to rotate the access token, or just re-run the failing command (the client refreshes automatically).
- **getSchedule returns degraded data on a personal MSA** — Use `freetime` (offline, your own calendars only) instead of `availability schedule` — getSchedule is documented as degraded on consumer accounts.
- **Events show in the wrong time zone in --json output** — Pass `--prefer-tz America/New_York` (or another IANA zone). The CLI sets the `Prefer: outlook.timezone="<tz>"` header so all times return in that zone.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**msgcli**](https://github.com/skylarbpayne/msgcli) — Go
- [**ms-365-mcp-server**](https://github.com/softeria/ms-365-mcp-server) — TypeScript
- [**microsoft-mcp**](https://github.com/elyxlz/microsoft-mcp) — Python
- [**outlook-mcp**](https://github.com/sajadghawami/outlook-mcp) — TypeScript
- [**outlook-meetings-scheduler-mcp**](https://github.com/anoopt/outlook-meetings-scheduler-mcp-server) — TypeScript
- [**cli-microsoft365**](https://github.com/pnp/cli-microsoft365) — TypeScript
- [**CalendarSync**](https://github.com/inovex/CalendarSync) — Go

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
