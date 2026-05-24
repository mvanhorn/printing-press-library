---
name: pp-google-calendar
description: "Every gcalcli and MCP feature, plus a local SQLite mirror that makes free-time search, conflict detection, and 'what... Trigger phrases: `what's on my calendar today`, `find a free slot`, `am I double-booked this week`, `what changed on my calendar since`, `book a meeting on google calendar`, `use google-calendar`, `run google-calendar`."
author: "Rúben Gaspar"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - google-calendar-pp-cli
---

# Google Calendar — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `google-calendar-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install google-calendar --cli-only
   ```
2. Verify: `google-calendar-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/google-calendar/cmd/google-calendar-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

A complete Google Calendar v3 CLI with full CRUD on events, calendars, ACLs, free/busy, and settings — plus a local event store that caches events as you query them. That store unlocks commands no live-only tool can offer: free-slot finding (free), cross-calendar conflict detection (conflicts), change windows (changes), and meeting-load analytics (load). Agent-native throughout: --json, --select, --dry-run, and typed exit codes.

## When to Use This CLI

Use this CLI when an agent or a terminal user needs to read, search, or book Google Calendar events and wants offline-fast queries over a local mirror. It is the right choice for scheduling automations that must find free time, avoid double-bookings, or report what changed — all with structured output and typed exit codes. It is not the right tool for rendering a graphical calendar UI.

## Unique Capabilities

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

## Command Reference

**calendars** — Manage calendars

- `google-calendar-pp-cli calendars delete` — Deletes a secondary calendar. Use calendars.clear for clearing all events on primary calendars.
- `google-calendar-pp-cli calendars get` — Returns metadata for a calendar.
- `google-calendar-pp-cli calendars insert` — Creates a secondary calendar.
- `google-calendar-pp-cli calendars patch` — Updates metadata for a calendar. This method supports patch semantics.
- `google-calendar-pp-cli calendars update` — Updates metadata for a calendar.

**channels** — Manage channels

- `google-calendar-pp-cli channels` — Stop watching resources through this channel

**colors** — Manage colors

- `google-calendar-pp-cli colors` — Returns the color definitions for calendars and events.

**free-busy** — Manage free busy

- `google-calendar-pp-cli free-busy` — Returns free/busy information for a set of calendars.

**users** — Manage users

- `google-calendar-pp-cli users calendar-calendar-list-delete` — Removes a calendar from the user's calendar list.
- `google-calendar-pp-cli users calendar-calendar-list-get` — Returns a calendar from the user's calendar list.
- `google-calendar-pp-cli users calendar-calendar-list-insert` — Inserts an existing calendar into the user's calendar list.
- `google-calendar-pp-cli users calendar-calendar-list-list` — Returns the calendars on the user's calendar list.
- `google-calendar-pp-cli users calendar-calendar-list-patch` — Updates an existing calendar on the user's calendar list. This method supports patch semantics.
- `google-calendar-pp-cli users calendar-calendar-list-update` — Updates an existing calendar on the user's calendar list.
- `google-calendar-pp-cli users calendar-calendar-list-watch` — Watch for changes to CalendarList resources.
- `google-calendar-pp-cli users calendar-settings-get` — Returns a single user setting.
- `google-calendar-pp-cli users calendar-settings-list` — Returns all user settings for the authenticated user.
- `google-calendar-pp-cli users calendar-settings-watch` — Watch for changes to Settings resources.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
google-calendar-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Book without double-booking

```bash
google-calendar-pp-cli book --summary 'Design review' --start 2026-05-25T15:00:00Z --end 2026-05-25T16:00:00Z --on-conflict abort
```

Aborts with a typed exit code if the slot overlaps an existing event in the local store.

### Find the next free 90 minutes

```bash
google-calendar-pp-cli free --calendars primary,work --window 'next 5 days' --duration 90m --agent
```

Returns open intervals long enough for the meeting as JSON, ready for an agent to pick from.

### Weekly change review

```bash
google-calendar-pp-cli changes --since 2026-05-17 --agent --select id,summary,status,updated
```

Lists created/updated/cancelled events since a date, narrowing the deeply-nested event payload to four fields so agents don't burn context on full event bodies.

### Who has access where

```bash
google-calendar-pp-cli acl-audit --role writer
```

Flattens sharing rules across all calendars into one table before onboarding a contractor.

### Quantify a busy month

```bash
google-calendar-pp-cli load --window 'this month' --group-by week
```

Counts meetings and total booked hours per week.

## Auth Setup

Google Calendar uses OAuth2 (authorization-code). Run `google-calendar-pp-cli auth login` once to authorize in your browser; tokens are stored locally and refreshed automatically. Scopes follow least-privilege: read-only commands request calendar.readonly.

Run `google-calendar-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  google-calendar-pp-cli calendars get mock-value --agent --select id,name,status
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

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
google-calendar-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
google-calendar-pp-cli feedback --stdin < notes.txt
google-calendar-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.google-calendar-pp-cli/feedback.jsonl`. They are never POSTed unless `GOOGLE_CALENDAR_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `GOOGLE_CALENDAR_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
google-calendar-pp-cli profile save briefing --json
google-calendar-pp-cli --profile briefing calendars get mock-value
google-calendar-pp-cli profile list --json
google-calendar-pp-cli profile show briefing
google-calendar-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `google-calendar-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add google-calendar-pp-mcp -- google-calendar-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which google-calendar-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   google-calendar-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `google-calendar-pp-cli <command> --help`.
