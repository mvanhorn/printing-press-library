---
name: pp-cal-com
description: "Every Cal.com command bcharleson ships, plus a local SQLite layer that answers questions the API can't: conflict... Trigger phrases: `list my Cal.com bookings`, `find calendar conflicts`, `who's likely to no-show`, `sync Cal.com to local`, `use cal-com`, `run cal-com-pp-cli`."
author: "david-n"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - cal-com-pp-cli
---

# Cal.com — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `cal-com-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install cal-com --cli-only
   ```
2. Verify: `cal-com-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/productivity/cal-com/cmd/cal-com-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

121 generated commands across Bookings, Event Types, Schedules, Slots, Calendars, Webhooks, Teams, OOO, and Conferencing — every endpoint in the Cal.com Platform API v2 OpenAPI spec, agent-native with --json / --select / --csv / --dry-run / typed exit codes. On top, eight novel commands that join your local synced state to surface conflict detection, attendee history, no-show ranking, schedule gaps, reschedule chains, and host load reports — none of which exist as Cal.com API endpoints.

## When to Use This CLI

Use this CLI when an agent needs full Cal.com Platform API v2 coverage with local-state superpowers: not just listing/creating/cancelling bookings, but joining bookings × attendees × event_types × calendar busy data to answer questions the API itself can't (cross-provider conflicts, attendee no-show rates, contiguous schedule gaps, host load reports). Prefer it over generic HTTP clients when offline aggregation, cross-entity queries, or batched operations matter.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds

- **`conflicts`** — Find overlapping busy time across your Google, Outlook, iCloud, and ICS calendars during bookable windows — surfaces double-booking risk before it happens.

  _When an agent needs to know whether a Cal.com bookable slot has an external calendar conflict, this is the only command that joins the data instead of doing N round-trips._

  ```bash
  cal-com-pp-cli conflicts --window 7d --json
  ```
- **`no-show-risk`** — Rank attendees by historical no-show and cancellation rate from your local booking history.

  _Lets an agent decide whether to require a deposit or reminder for a specific attendee before confirming a new booking._

  ```bash
  cal-com-pp-cli no-show-risk --since 90d --json --select email,no_show_rate,total_count
  ```
- **`attendee`** — Every past booking for a single attendee email — event types, statuses, first-seen, last-seen — with --summary collapsing to one row.

  _Agents tagging 'repeat customer' or 'first-time prospect' use this instead of paginating /bookings._

  ```bash
  cal-com-pp-cli attendee jane@example.com --summary --json
  ```
- **`gaps`** — Contiguous free time of at least N minutes inside your availability windows.

  _Agents fitting an interview block or a focus window pick gaps directly instead of probing /slots per event-type._

  ```bash
  cal-com-pp-cli gaps --min 30m --window 7d --json
  ```
- **`reschedule-history`** — Full chain of reschedules for a booking — every prior UID, who/when, with the final state on top.

  _When a customer asks 'what happened to my booking?' the agent traces the full trail in one call._

  ```bash
  cal-com-pp-cli reschedule-history abc123 --json
  ```
- **`cancel-sweep`** — Find and (with --apply) cancel stale unconfirmed bookings older than a threshold. Dry-run by default.

  _Agents doing weekly hygiene get one command instead of paginating /bookings and looping cancel calls._

  ```bash
  cal-com-pp-cli cancel-sweep --status PENDING --older-than 48h --json
  ```
- **`host-load`** — Per-host booking counts, total hours, cancel rate, and no-show rate for a given ISO week.

  _RevOps agents building team-load reports stop writing Python pagination scripts._

  ```bash
  cal-com-pp-cli host-load --week 2026-W20 --json
  ```

### Sync verbs

- **`load-day`** — Sync one calendar day's bookings into local store and emit the delta (added/changed) vs the prior sync.

  _Targeted incremental sync for debugging a specific day without re-syncing the whole history._

  ```bash
  cal-com-pp-cli load-day 2026-05-14 --json
  ```

## Command Reference

**api-keys** — Manage api keys

- `cal-com-pp-cli api-keys` — Generate a new API key and delete the current one. Provide API key to refresh as a Bearer token in the Authorization...

**bookings** — Manage bookings

- `cal-com-pp-cli bookings create` — POST /v2/bookings is used to create regular bookings, recurring bookings and instant bookings. The request bodies...
- `cal-com-pp-cli bookings get` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...
- `cal-com-pp-cli bookings get-bookinguid` — `:bookingUid` can be 1. uid of a normal booking 2. uid of one of the recurring booking recurrences 3. uid of...
- `cal-com-pp-cli bookings get-by-seat-uid` — Get a seated booking by its seat reference UID. This is useful when you have a seatUid from a seated booking and...

**cal-com-auth** — Manage cal com auth

- `cal-com-pp-cli cal-com-auth oauth2-get-client` — Returns the OAuth2 client information for the given client ID
- `cal-com-pp-cli cal-com-auth oauth2-token` — RFC 6749-compliant token endpoint. Pass client_id in the request body (Section 2.3.1). Use grant_type...

**calendars** — Manage calendars

- `cal-com-pp-cli calendars cal-unified-create-connection-event` — Create a new event on the specified calendar connection. Only supported for Google Calendar connections; other...
- `cal-com-pp-cli calendars cal-unified-delete-connection-event` — Delete/cancel an event on the specified calendar connection. Only supported for Google Calendar connections; other...
- `cal-com-pp-cli calendars cal-unified-get-connection-event` — Get a single event by ID for the specified calendar connection. Only supported for Google Calendar connections;...
- `cal-com-pp-cli calendars cal-unified-get-connection-free-busy` — Get busy time slots for the specified calendar connection.
- `cal-com-pp-cli calendars cal-unified-list-connection-events` — List events in a date range for a specific calendar connection. Only supported for Google Calendar connections;...
- `cal-com-pp-cli calendars cal-unified-list-connections` — Returns all calendar connections for the authenticated user (Google, Office 365, Apple). Use connectionId in...
- `cal-com-pp-cli calendars cal-unified-update-connection-event` — Update an event on the specified calendar connection. Only supported for Google Calendar connections; other...
- `cal-com-pp-cli calendars check-ics-feed` — Check an ICS feed
- `cal-com-pp-cli calendars create-ics-feed` — Save an ICS feed
- `cal-com-pp-cli calendars get` — Get all calendars
- `cal-com-pp-cli calendars get-busy-times` — Get busy times from a calendar. Example request URL is `https://api.cal.com/v2/calendars/busy-times?timeZone=Europe%2...

**conferencing** — Manage conferencing

- `cal-com-pp-cli conferencing get-default` — Get your default conferencing application
- `cal-com-pp-cli conferencing list-installed-apps` — List your conferencing applications

**destination-calendars** — Manage destination calendars

- `cal-com-pp-cli destination-calendars` — Update destination calendars

**event-types** — Manage event types

- `cal-com-pp-cli event-types create` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...
- `cal-com-pp-cli event-types delete` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...
- `cal-com-pp-cli event-types get` — Hidden event types are returned only if authentication is provided and it belongs to the event type owner. Use the...
- `cal-com-pp-cli event-types get-by-id` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...
- `cal-com-pp-cli event-types update` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...

**me** — Manage me

- `cal-com-pp-cli me get` — Get my profile
- `cal-com-pp-cli me update` — Updates the authenticated user's profile. Email changes require verification and the primary email stays unchanged...

**oauth** — Manage oauth


**oauth-clients** — Manage oauth clients

- `cal-com-pp-cli oauth-clients create` — <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- `cal-com-pp-cli oauth-clients delete` — <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- `cal-com-pp-cli oauth-clients get` — <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- `cal-com-pp-cli oauth-clients get-by-id` — <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- `cal-com-pp-cli oauth-clients update` — <Warning>These endpoints are deprecated and will be removed in the future.</Warning>

**schedules** — Manage schedules

- `cal-com-pp-cli schedules create` — Create a schedule for the authenticated user. The point of creating schedules is for event types to be available at...
- `cal-com-pp-cli schedules delete` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...
- `cal-com-pp-cli schedules get` — Get all schedules of the authenticated user. <Note>Please make sure to pass in the cal-api-version header value as...
- `cal-com-pp-cli schedules get-default` — Get the default schedule of the authenticated user. <Note>Please make sure to pass in the cal-api-version header...
- `cal-com-pp-cli schedules get-scheduleid` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...
- `cal-com-pp-cli schedules update` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...

**selected-calendars** — Manage selected calendars

- `cal-com-pp-cli selected-calendars add` — Add a selected calendar
- `cal-com-pp-cli selected-calendars delete` — Delete a selected calendar

**slots** — Manage slots

- `cal-com-pp-cli slots delete-reserved` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...
- `cal-com-pp-cli slots get-available` — There are 4 ways to get available slots for event type of an individual user: 1. By event type id. Example...
- `cal-com-pp-cli slots get-reserved` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...
- `cal-com-pp-cli slots reserve` — Make a slot not available for others to book for a certain period of time. If you authenticate using oAuth...
- `cal-com-pp-cli slots update-reserved` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...

**stripe** — Manage stripe

- `cal-com-pp-cli stripe check` — Check Stripe connection
- `cal-com-pp-cli stripe redirect` — Get Stripe connect URL
- `cal-com-pp-cli stripe save` — Save Stripe credentials

**verified-resources** — Manage verified resources

- `cal-com-pp-cli verified-resources user-get-verified-email-by-id` — Get verified email by id
- `cal-com-pp-cli verified-resources user-get-verified-emails` — Get list of verified emails
- `cal-com-pp-cli verified-resources user-get-verified-phone-by-id` — Get verified phone number by id
- `cal-com-pp-cli verified-resources user-get-verified-phone-numbers` — Get list of verified phone numbers
- `cal-com-pp-cli verified-resources user-request-email-verification-code` — Sends a verification code to the email
- `cal-com-pp-cli verified-resources user-request-phone-verification-code` — Sends a verification code to the phone number
- `cal-com-pp-cli verified-resources user-verify-email` — Use code to verify an email
- `cal-com-pp-cli verified-resources user-verify-phone-number` — Use code to verify a phone number

**webhooks** — Manage webhooks

- `cal-com-pp-cli webhooks create` — Create a webhook
- `cal-com-pp-cli webhooks delete` — Delete a webhook
- `cal-com-pp-cli webhooks get` — Gets a paginated list of webhooks for the authenticated user.
- `cal-com-pp-cli webhooks get-webhookid` — Get a webhook
- `cal-com-pp-cli webhooks update` — Update a webhook


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
cal-com-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Weekly cancellation hygiene

```bash
cal-com-pp-cli cancel-sweep --status PENDING --older-than 48h --json
```

Dry-run lists pending bookings older than 48h; add --apply to cancel.

### Conflict scan before sharing slots

```bash
cal-com-pp-cli conflicts --window 14d --json --select date,event_title,conflict_calendar,conflict_event
```

Shows every 14-day window where a connected calendar has a busy block overlapping a confirmed booking.

### Attendee follow-up at scale

```bash
cal-com-pp-cli attendee jane@example.com --summary --json
```

First booking, last booking, total count, cancel rate, last event type for one attendee.

### Find a 45-minute window this week

```bash
cal-com-pp-cli gaps --min 45m --window 7d --json --select start,end,minutes
```

Contiguous free time across your availability windows, sorted earliest first.

### VP-ready host load report

```bash
cal-com-pp-cli host-load --week 2026-W20 --json --select host_email,booking_count,total_hours,no_show_rate
```

Group bookings by host with derived rates, ready to paste into a slide or feed to claude.

## Auth Setup

Set CAL_COM_API_KEY (or CAL_API_KEY for bcharleson compatibility) to your cal_live_* key from Settings → Security → API keys. The CLI sends Authorization: Bearer <key>. cal-com-pp-cli doctor verifies auth + reachability before anything else.

Run `cal-com-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  cal-com-pp-cli bookings get --agent --select id,name,status
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
cal-com-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
cal-com-pp-cli feedback --stdin < notes.txt
cal-com-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.cal-com-pp-cli/feedback.jsonl`. They are never POSTed unless `CAL_COM_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `CAL_COM_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
cal-com-pp-cli profile save briefing --json
cal-com-pp-cli --profile briefing bookings get
cal-com-pp-cli profile list --json
cal-com-pp-cli profile show briefing
cal-com-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `cal-com-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add cal-com-pp-mcp -- cal-com-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which cal-com-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   cal-com-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `cal-com-pp-cli <command> --help`.
