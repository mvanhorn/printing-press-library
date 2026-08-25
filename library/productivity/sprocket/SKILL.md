---
name: pp-sprocket
description: "Your youth-soccer club's schedule, teams, and dues from the terminal — with merged multi-kid agendas, conflict detection Trigger phrases: `when is my kid's next game`, `this week's soccer schedule`, `do we have any schedule conflicts`, `away games this month`, `what do we owe the soccer club`, `use sprocket`, `run sprocket`."
author: "Greg Stellato"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - sprocket-pp-cli
    install:
      - kind: go
        bins: [sprocket-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/sprocket/cmd/sprocket-pp-cli
---

# Sprocket Sports — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `sprocket-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer. It defaults binaries to `$HOME/.local/bin` on macOS/Linux and `%LOCALAPPDATA%\Programs\PrintingPress\bin` on Windows:
   ```bash
   npx -y @mvanhorn/printing-press-library install sprocket --cli-only
   ```
2. Verify: `sprocket-pp-cli --version`
3. Ensure the reported install directory is on `$PATH` for the agent/runtime that will invoke this skill.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.4 or newer). This installs into `$GOPATH/bin` (default `$HOME/go/bin`), so add that directory to `$PATH` instead:

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/sprocket/cmd/sprocket-pp-cli@latest
```

If `--version` reports "command not found" after install, the runtime cannot see the binary directory on `$PATH`. Do not proceed with skill commands until verification succeeds.

Sprocket Sports CLI wraps your club's Sprocket dashboard API so you can answer the questions a parent actually asks: 'when is the next game' (next), 'what's on this week' (week), 'plan the next two weeks for both kids' (agenda), 'any double-bookings' (conflicts), and 'what do we owe' (owed). It merges every one of your players' teams into a single view the per-team web dashboard can't produce, and exports everything to your phone calendar with ical.

## When to Use This CLI

Use this CLI when an agent or script needs a Sprocket Sports club's schedule, rosters, programs, registrations, or dues — especially merged across multiple children and teams, or as date-math (next game, this week, conflicts, away games) that the per-team web dashboard cannot answer in one view. It is ideal for reminders, carpool and away-game planning, iCal subscriptions, and 'what do we owe' roll-ups.

## Anti-triggers

Do not use this CLI for:
- Do not use this CLI for league scores or standings — the parent dashboard API does not expose them (they are league-admin level).
- Do not use it for coach or club announcements and messaging — that surface is not available here.
- Do not use it to register, pay, RSVP, or otherwise change anything — v1 is strictly read-only.
- Works for any Sprocket club, but only one at a time: it targets whatever `SPROCKET_CLUB` / `SPROCKET_BASE_URL` (or config `base_url`) is set to, defaulting to `jfcsoccer`. The token must match that club. Don't expect cross-club queries in a single invocation.

## Unique Capabilities

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

## Command Reference

**account** — Your account and roles

- `sprocket-pp-cli account clubs` — List clubs this account belongs to
- `sprocket-pp-cli account me` — Get your club-user account profile
- `sprocket-pp-cli account roles` — List your account roles

**club** — Club information

- `sprocket-pp-cli club get` — Get club metadata
- `sprocket-pp-cli club settings` — Get public club settings

**family** — Your family group

- `sprocket-pp-cli family` — Get your family (parents/guardians and players)

**payments** — Dues and outstanding payments

- `sprocket-pp-cli payments failed` — List failed payments
- `sprocket-pp-cli payments overdue-invoices` — List overdue invoice payments (dues owed)
- `sprocket-pp-cli payments overdue-tickets` — List overdue ticket payments

**players** — Your players

- `sprocket-pp-cli players` — List your players (children) on this account

**programs** — Club programs and registration

- `sprocket-pp-cli programs list` — List all club programs/seasons
- `sprocket-pp-cli programs open` — List programs currently open for registration

**registrations** — Registration and payment history

- `sprocket-pp-cli registrations completed` — List completed program registrations (with balances)
- `sprocket-pp-cli registrations team` — List completed team registrations

**schedule** — Club calendar — games, practices, and events

- `sprocket-pp-cli schedule event-types` — List calendar event types (game, practice, etc.)
- `sprocket-pp-cli schedule list` — List calendar events in a date window (max ~31 days). Dates are YYYY-MM-DD.
- `sprocket-pp-cli schedule settings` — Calendar display settings for the club

**teams** — Club teams

- `sprocket-pp-cli teams list` — List every team in the club
- `sprocket-pp-cli teams mine` — List the teams your players are assigned to


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
sprocket-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

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

## Auth Setup

Sprocket Sports uses OAuth2/OIDC (Duende IdentityServer at login.sprocketsports.com) and the API takes a Bearer access token. This CLI authenticates with that token: set SPROCKET_TOKEN to the access_token from your logged-in dashboard session. To find it: open your club dashboard in Chrome, open DevTools, go to Application > Session Storage, find the 'oidc.user:...sprocket-sports' entry, and copy its access_token value. Tokens are short-lived (about an hour), so re-copy when commands start returning 401.

**Any Sprocket club, not just one.** The CLI defaults to the `jfcsoccer` tenant but works for any `https://<club>.sprocketsports.com`. Point it at yours with `export SPROCKET_CLUB=<subdomain>` (shorthand) or `export SPROCKET_BASE_URL=https://<club>.sprocketsports.com`, or set `base_url` in `~/.config/sprocket-pp-cli/config.toml`. The token must come from that same club's session (tokens are club-scoped).

Run `sprocket-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  sprocket-pp-cli family --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
sprocket-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
sprocket-pp-cli feedback --stdin < notes.txt
sprocket-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.local/share/sprocket-pp-cli/feedback.jsonl`. They are never POSTed unless `SPROCKET_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `SPROCKET_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
sprocket-pp-cli profile save briefing --json
sprocket-pp-cli --profile briefing family
sprocket-pp-cli profile list --json
sprocket-pp-cli profile show briefing
sprocket-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `sprocket-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/sprocket/cmd/sprocket-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add sprocket-pp-mcp -- sprocket-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which sprocket-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   sprocket-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `sprocket-pp-cli <command> --help`.
