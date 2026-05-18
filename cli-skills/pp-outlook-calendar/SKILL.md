---
name: pp-outlook-calendar
description: "The first Outlook calendar CLI built for AI agents on personal Microsoft 365 accounts — with offline conflict... Trigger phrases: `what's on my calendar today`, `find me an hour next week`, `do I have any conflicts`, `what meetings haven't I responded to`, `prep me for my next meeting`, `schedule a meeting on my Outlook calendar`, `use outlook-calendar`, `run outlook-calendar`."
author: "Paul Brennaman"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - outlook-calendar-pp-cli
---
<!-- GENERATED FILE — DO NOT EDIT.
     This file is a verbatim mirror of library/productivity/outlook-calendar/SKILL.md,
     regenerated post-merge by tools/generate-skills/. Hand-edits here are
     silently overwritten on the next regen. Edit the library/ source instead.
     See AGENTS.md "Generated artifacts: registry.json, cli-skills/". -->

# Outlook Calendar — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `outlook-calendar-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install outlook-calendar --cli-only
   ```
2. Verify: `outlook-calendar-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Drives your personal Microsoft 365 calendar from scripts and agents. OAuth 2.0 device-code flow against the /common tenant, so personal MSAs work alongside work accounts. A local SQLite store synced through events/delta unlocks `conflicts`, `freetime`, `review`, `pending`, `recurring-drift`, and `prep` — workflows no other Outlook CLI exposes because they require persisted state.

## When to Use This CLI

Reach for outlook-calendar-pp-cli when an agent needs to read or write a personal Microsoft 365 calendar non-interactively. It is the right choice for daily-brief, schedule-this-meeting, find-me-time, what-changed-this-week, and prep-me-for-my-next-meeting tasks. Prefer it over hitting Microsoft Graph directly when you want offline-shaped queries (conflicts, free-time math, recurring drift) that the Graph endpoints don't expose as single calls.

## Unique Capabilities

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

## Command Reference

**attachments** — Manage event attachments

- `outlook-calendar-pp-cli attachments delete` — Delete an attachment
- `outlook-calendar-pp-cli attachments get` — Get a specific attachment by id
- `outlook-calendar-pp-cli attachments list` — List attachments on an event

**availability** — Free/busy and meeting-time intelligence (degraded on personal Microsoft accounts; prefer freetime for self-only queries)

- `outlook-calendar-pp-cli availability find` — Suggest meeting times based on attendee availability and constraints
- `outlook-calendar-pp-cli availability schedule` — Get free/busy schedule for a list of users (limited on personal Microsoft accounts)

**calendars** — Manage Outlook calendars on your account

- `outlook-calendar-pp-cli calendars create` — Create a new calendar
- `outlook-calendar-pp-cli calendars default` — Get the user's default calendar
- `outlook-calendar-pp-cli calendars delete` — Delete a calendar
- `outlook-calendar-pp-cli calendars get` — Get a calendar by id
- `outlook-calendar-pp-cli calendars list` — List all calendars on the account
- `outlook-calendar-pp-cli calendars update` — Update a calendar

**categories** — Manage Outlook master categories used to tag events

- `outlook-calendar-pp-cli categories create` — Create a new master category
- `outlook-calendar-pp-cli categories delete` — Delete a master category
- `outlook-calendar-pp-cli categories list` — List master categories

**delta** — Incremental delta-sync of events into the local SQLite store

- `outlook-calendar-pp-cli delta events` — Pull incremental event changes since the last delta token
- `outlook-calendar-pp-cli delta view` — Pull incremental calendar-view changes within a window

**events** — Outlook calendar events on your default or named calendar

- `outlook-calendar-pp-cli events accept` — Accept a meeting invite
- `outlook-calendar-pp-cli events cancel` — Cancel an event you organized (notifies attendees)
- `outlook-calendar-pp-cli events create` — Create a new event on the default calendar
- `outlook-calendar-pp-cli events decline` — Decline a meeting invite
- `outlook-calendar-pp-cli events delete` — Delete an event by id
- `outlook-calendar-pp-cli events dismiss` — Dismiss the reminder for an event
- `outlook-calendar-pp-cli events forward` — Forward an event to additional attendees
- `outlook-calendar-pp-cli events get` — Get a single event by id
- `outlook-calendar-pp-cli events instances` — List occurrences of a recurring event in a date range
- `outlook-calendar-pp-cli events list` — List events on the default calendar
- `outlook-calendar-pp-cli events range` — List events occurring within a date range (calendarView; expands recurring instances)
- `outlook-calendar-pp-cli events search` — Server-side search across events ($search query)
- `outlook-calendar-pp-cli events snooze` — Snooze the reminder for an event until a specific time
- `outlook-calendar-pp-cli events tentative` — Tentatively accept a meeting invite
- `outlook-calendar-pp-cli events update` — Update fields on an existing event (subject, body, time, location, attendees)


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
outlook-calendar-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Daily brief

```bash
outlook-calendar-pp-cli prep --next 24h --json --select subject,start,end,location,attendees
```

Agent-friendly daily snapshot with all the fields a briefing needs and nothing else.

### Find an hour next week

```bash
outlook-calendar-pp-cli freetime --duration 60m --within 'Mon-Fri 9-17' --next 7d --exclude-oof --json
```

Returns gap windows in working hours, OOF excluded — pipe to `jq '.[0]'` for the first opening.

### What changed since Monday

```bash
outlook-calendar-pp-cli review --since 'Mon 09:00' --json
```

Diff buckets (added/rescheduled/cancelled/rsvp-changed) that surface organizer-side reschedules.

### Pending RSVPs

```bash
outlook-calendar-pp-cli pending --json --select subject,start,organizer.email
```

All future events whose RSVP is still pending — narrow output via `--select` so the agent only sees the fields it needs.

### Conflict scan with deep `--select`

```bash
outlook-calendar-pp-cli conflicts --from today --to +14d --json --select pair.a.subject,pair.b.subject,overlap_minutes,calendar.a,calendar.b
```

Agents on multi-calendar users (consultant + personal) hit `conflicts` first to know what's actually colliding; dotted `--select` keeps the payload tiny.

## Auth Setup

Authentication uses OAuth 2.0 device-code flow against `https://login.microsoftonline.com/common`. Run `outlook-calendar-pp-cli auth login --device-code` once; visit the displayed URL on any device, enter the code, and you're done. Tokens are cached at `~/.config/outlook-calendar-pp-cli/config.toml` (mode 0600). The CLI auto-refreshes the access token on expiry using the stored refresh token, so subsequent commands run non-interactively. The default client id is the Microsoft-published Graph PowerShell client (works with personal Microsoft accounts out of the box); pass `--client-id` to use your own Azure app registration. Personal Microsoft accounts (Outlook.com, Hotmail, Live, MSA) are first-class and tested.

Run `outlook-calendar-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  outlook-calendar-pp-cli attachments list mock-value --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
outlook-calendar-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
outlook-calendar-pp-cli feedback --stdin < notes.txt
outlook-calendar-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.outlook-calendar-pp-cli/feedback.jsonl`. They are never POSTed unless `OUTLOOK_CALENDAR_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `OUTLOOK_CALENDAR_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
outlook-calendar-pp-cli profile save briefing --json
outlook-calendar-pp-cli --profile briefing attachments list mock-value
outlook-calendar-pp-cli profile list --json
outlook-calendar-pp-cli profile show briefing
outlook-calendar-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `outlook-calendar-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add outlook-calendar-pp-mcp -- outlook-calendar-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which outlook-calendar-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   outlook-calendar-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `outlook-calendar-pp-cli <command> --help`.
