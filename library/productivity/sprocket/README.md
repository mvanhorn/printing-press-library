# Sprocket Sports CLI

**Your youth-soccer club's schedule, teams, and dues from the terminal — with merged multi-kid agendas, conflict detection, and iCal export no dashboard gives you.**

Sprocket Sports CLI wraps your club's Sprocket dashboard API so you can answer the questions a parent actually asks: 'when is the next game' (next), 'what's on this week' (week), 'plan the next two weeks for both kids' (agenda), 'any double-bookings' (conflicts), and 'what do we owe' (owed). It merges every one of your players' teams into a single view the per-team web dashboard can't produce, and exports everything to your phone calendar with ical.

Learn more at [Sprocket Sports](https://jfcsoccer.sprocketsports.com).

Created by [@stellato](https://github.com/stellato) (Greg Stellato).

## Install

The recommended path installs both the `sprocket-pp-cli` binary and the `pp-sprocket` agent skill (Claude Code, Codex, Cursor, Gemini CLI, GitHub Copilot, and other agents supported by the upstream [`skills`](https://github.com/vercel-labs/skills) CLI) in one shot:

```bash
npx -y @mvanhorn/printing-press-library install sprocket
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press-library install sprocket --cli-only
```

For skill only — installs the skill into the same agents as the default command above, but skips the CLI binary (use this to update or reinstall just the skill):

```bash
npx -y @mvanhorn/printing-press-library install sprocket --skill-only
```

To constrain the skill install to one or more specific agents (repeatable — agent names match the [`skills`](https://github.com/vercel-labs/skills) CLI):

```bash
npx -y @mvanhorn/printing-press-library install sprocket --agent claude-code
npx -y @mvanhorn/printing-press-library install sprocket --agent claude-code --agent codex
```

### Without Node (Go fallback)

If `npx` isn't available (no Node, offline), install the CLI directly via Go (requires Go 1.26.4 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/sprocket/cmd/sprocket-pp-cli@latest
```

This installs the CLI only — no skill.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sprocket-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

Install the CLI binary first. The installer writes binaries to a per-user managed bin directory by default: `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows.

```bash
npx -y @mvanhorn/printing-press-library install sprocket --cli-only
```

Then install the focused Hermes skill.

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-sprocket --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-sprocket --force
```

Restart the Hermes session or gateway if the newly installed skill is not visible immediately.

## Install for OpenClaw
Install both the CLI binary and the focused OpenClaw skill. The installer defaults binaries to a per-user bin directory (`$HOME/.local/bin` on macOS/Linux, `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows):

```bash
npx -y @mvanhorn/printing-press-library install sprocket --agent openclaw
```

Restart the OpenClaw session or gateway if the newly installed skill is not visible immediately.

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/sprocket-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `SPROCKET_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/sprocket/cmd/sprocket-pp-mcp@latest
```

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "sprocket": {
      "command": "sprocket-pp-mcp",
      "env": {
        "SPROCKET_TOKEN": "<your-key>"
      }
    }
  }
}
```

</details>

## Authentication

Sprocket Sports uses OAuth2/OIDC (Duende IdentityServer at login.sprocketsports.com) and the API takes a Bearer access token. This CLI authenticates with that token: set SPROCKET_TOKEN to the access_token from your logged-in dashboard session. To find it: open your club dashboard in Chrome, open DevTools, go to Application > Session Storage, find the 'oidc.user:...sprocket-sports' entry, and copy its access_token value. Tokens are short-lived (about an hour), so re-copy when commands start returning 401.

## Choose your club

Sprocket Sports is multi-tenant — every club lives at `https://<club>.sprocketsports.com`. This CLI works for **any** Sprocket club, not just one. It defaults to `jfcsoccer`; point it at your own club with either of:

```bash
# Shorthand — just your club's subdomain:
export SPROCKET_CLUB=myclub                              # -> https://myclub.sprocketsports.com

# Or a full base URL (wins if both are set):
export SPROCKET_BASE_URL=https://myclub.sprocketsports.com
```

Or persist it in `~/.config/sprocket-pp-cli/config.toml`:

```toml
base_url = "https://myclub.sprocketsports.com"
```

Your `SPROCKET_TOKEN` must come from **that** club's dashboard session (tokens are club-scoped). Run `sprocket-pp-cli doctor` to confirm the base URL and token line up.

## Quick Start

```bash
# check that your token works and the API is reachable
sprocket doctor

# your next game or practice across all your kids' teams
sprocket next

# this week's full schedule, merged
sprocket week

# the next two weeks as one family agenda
sprocket agenda --days 14

# away games with fields for the carpool chat
sprocket away --weeks 4

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Schedule, the way a parent asks
- **`week`** — Every game and practice for the current week across all your players' teams, in one view.

  _Reach for this when asked 'what's on this week' for a family with kids on different teams._

  ```bash
  sprocket week
  ```
- **`next`** — The single next upcoming game or practice across all your players' teams: what, when, where, home/away, opponent.

  _Reach for this for 'when is my kid's next game' or a one-line reminder._

  ```bash
  sprocket next --agent --select clubCalendarEvent.title,clubCalendarEvent.startDate,clubCalendarEvent.opponent,clubCalendarEvent.awayGame
  ```
- **`agenda`** — Both kids' separate team schedules merged into one chronological agenda over an N-day window.

  _Reach for this to plan the family's next two weeks in a single sorted list._

  ```bash
  sprocket agenda --days 14
  ```
- **`conflicts`** — Flags time overlaps and impossible tight time-plus-location gaps between events across all your players.

  _Reach for this to catch a double-booked Saturday before Saturday morning._

  ```bash
  sprocket conflicts --days 14
  ```
- **`away`** — Away games only, location-first, with field, opponent, and date — built for the carpool group chat.

  _Reach for this when planning away-game driving or posting a travel list._

  ```bash
  sprocket away --weeks 4
  ```
- **`ical`** — Emit the merged schedule as an RFC-5545 .ics file for phone-calendar subscription.

  _Reach for this to get soccer events into Apple/Google Calendar._

  ```bash
  sprocket ical --days 60
  ```
- **`since`** — What schedule items were added, moved, or cancelled since the last time you ran it.

  _Reach for this to catch a moved practice or a newly-added game._

  ```bash
  sprocket since
  ```

### Money and deadlines
- **`owed`** — Total money owed across all your players: registration balances plus overdue invoices, with one number.

  _Reach for this for 'what do we still owe the club' across multiple kids._

  ```bash
  sprocket owed
  ```
- **`deadlines`** — Open registration programs sorted by how soon they close, flagging ones closing within N days.

  _Reach for this to avoid missing a registration window._

  ```bash
  sprocket deadlines --days 14
  ```

## Recipes


### Next game as a one-line reminder

```bash
sprocket next --agent --select clubCalendarEvent.title,clubCalendarEvent.startDate,clubCalendarEvent.opponent,clubCalendarEvent.awayGame
```

Narrow the deeply-nested event to just the fields a reminder needs; pipe to a text/cron job.

### Two-week family agenda

```bash
sprocket agenda --days 14
```

Merge both kids' team schedules into one sorted list for the next two weeks.

### Away-game drive list for the group chat

```bash
sprocket away --weeks 4
```

Just the away games, location-first, ready to paste to carpool parents.

### What do we owe

```bash
sprocket owed
```

One combined outstanding total across registrations and overdue invoices for all players.

### Subscribe in your phone calendar

```bash
sprocket ical --days 60
```

Write 60 days of merged events to an .ics file you can import or host for subscription.

## Usage

Run `sprocket-pp-cli --help` for the full command reference and flag list.

## Commands

### account

Your account and roles

- **`sprocket-pp-cli account clubs`** - List clubs this account belongs to
- **`sprocket-pp-cli account me`** - Get your club-user account profile
- **`sprocket-pp-cli account roles`** - List your account roles

### club

Club information

- **`sprocket-pp-cli club get`** - Get club metadata
- **`sprocket-pp-cli club settings`** - Get public club settings

### family

Your family group

- **`sprocket-pp-cli family`** - Get your family (parents/guardians and players)

### payments

Dues and outstanding payments

- **`sprocket-pp-cli payments failed`** - List failed payments
- **`sprocket-pp-cli payments overdue-invoices`** - List overdue invoice payments (dues owed)
- **`sprocket-pp-cli payments overdue-tickets`** - List overdue ticket payments

### players

Your players

- **`sprocket-pp-cli players`** - List your players (children) on this account

### programs

Club programs and registration

- **`sprocket-pp-cli programs list`** - List all club programs/seasons
- **`sprocket-pp-cli programs open`** - List programs currently open for registration

### registrations

Registration and payment history

- **`sprocket-pp-cli registrations completed`** - List completed program registrations (with balances)
- **`sprocket-pp-cli registrations team`** - List completed team registrations

### schedule

Club calendar — games, practices, and events

- **`sprocket-pp-cli schedule event-types`** - List calendar event types (game, practice, etc.)
- **`sprocket-pp-cli schedule list`** - List calendar events in a date window (max ~31 days). Dates are YYYY-MM-DD.
- **`sprocket-pp-cli schedule settings`** - Calendar display settings for the club

### teams

Club teams

- **`sprocket-pp-cli teams list`** - List every team in the club
- **`sprocket-pp-cli teams mine`** - List the teams your players are assigned to


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
sprocket-pp-cli family

# JSON for scripting and agents
sprocket-pp-cli family --json

# Filter to specific fields
sprocket-pp-cli family --json --select id,name,status

# Dry run — show the request without sending
sprocket-pp-cli family --dry-run

# Agent mode — JSON + compact + no prompts in one flag
sprocket-pp-cli family --agent
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

## Health Check

```bash
sprocket-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/sprocket-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `SPROCKET_TOKEN` | per_call | Yes | Set to your API credential. |

### agentcookie (optional)

If you use agentcookie to sync secrets across machines, this CLI auto-adopts agentcookie-managed credentials with no extra setup. When the daemon writes to this CLI's config, `sprocket-pp-cli doctor` reports `agentcookie: detected` and `auth-status` labels the source as `agentcookie`. Skip this section if you don't use agentcookie - the CLI works the same as any other.

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `sprocket-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $SPROCKET_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific
- **401 Unauthorized on every command** — Your SPROCKET_TOKEN expired (tokens last about an hour). Re-copy the access_token from your dashboard session — see Authentication.
- **'Start date is out of valid range'** — The calendar API caps a query at ~31 days. Use a smaller --days/--weeks value; next/week/agenda already stay inside the limit.
- **Empty schedule even though events exist** — Confirm the token is for the right club tenant and that you have players assigned to teams: run 'sprocket teams mine'.
- **Wrong club's data** — The base URL defaults to the Jacksonville FC tenant. Set the base URL in the config file to your own club's <club>.sprocketsports.com host.
