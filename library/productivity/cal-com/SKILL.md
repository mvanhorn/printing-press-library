---
name: pp-cal-com
description: "Every Cal.com feature, plus offline agendas, composed booking flows, and analytics no other Cal.com tool ships. Trigger phrases: `book a meeting on cal.com`, `what's on my calendar today`, `find an open slot`, `reschedule my next booking`, `audit my cal.com webhooks`, `use cal-com`, `run cal-com-pp-cli`."
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - cal-com-pp-cli
    install:
      - kind: go
        bins: [cal-com-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/productivity/cal-com/cmd/cal-com-pp-cli
---

# Cal.com — Printing Press CLI

cal-com-pp-cli wraps the entire Cal.com v2 API and adds a local SQLite store of your bookings, event types, schedules, and team data. Composed intents like `book` and `reschedule next` collapse multi-call dances into one transactional command. Local-store analytics, conflict detection, and team workload land in milliseconds with no API call.

## When to Use This CLI

Reach for cal-com-pp-cli whenever an agent needs to read or mutate a Cal.com calendar without burning context on the multi-call dance the API requires for booking and rescheduling. The local store makes 'what's on my calendar', 'when am I free', 'who is overloaded', and 'where are my conflicts' near-instant offline answers. The composed `book` and `reschedule next` commands are the right tools for transactional bookings; the endpoint-mirror coverage is there for everything else.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Composed booking flows
- **`book`** — Schedule an attendee onto one of your event types in a single composed call — slot check, optional reservation, create, optional confirm.

  _For the host scripting an attendee onto their calendar (admin onboarding, recruiter pre-fill, test fixtures). For the normal flow where the attendee picks their own time, share a URL from `link list` instead._

  ```bash
  cal-com-pp-cli book --event-type-id 96531 --start 2026-05-06T17:00:00Z --attendee-name Guest --attendee-email guest@example.com --dry-run
  ```
- **`slots find`** — Find first available slots across multiple event-type IDs in one call, ranked by start time.

  _Use this when you don't know which event type fits — let the caller pick from a ranked merged list._

  ```bash
  cal-com-pp-cli slots find --event-type-ids 96531 --start tomorrow --end "tomorrow 23:59" --json
  ```
- **`reschedule next`** — Move an existing booking to the next available slot for the same event type, after a cutoff.

  _Use this for last-minute bumps — one command instead of three, with dry-run safety._

  ```bash
  cal-com-pp-cli reschedule next --uid <booking-uid> --after tomorrow --dry-run
  ```

### Local state that compounds
- **`agenda`** — Upcoming bookings in a window — today, this week, or any duration — read from the local store.

  _Use this whenever an agent needs 'what's on my calendar'; single command across any time window._

  ```bash
  cal-com-pp-cli agenda --window today --json --select id,start,title,attendees
  ```
- **`analytics no-show`** — No-show, cancellation, volume, and density metrics over a window. Sister subcommands under analytics: bookings (volume), cancellations, no-show, density. --by accepts event-type, attendee, or weekday on the rate commands; analytics density --unit hour adds hourly heatmaps.

  _Use this for capacity planning, no-show trend analysis, or attendee follow-up — answers no single API call provides._

  ```bash
  cal-com-pp-cli analytics no-show --window 90d --by attendee --json
  ```
- **`conflicts`** — Detects overlapping bookings within a time window — pairs whose time ranges intersect get reported. Reads the local store, no API call.

  _Run before sending confirmations or after a bulk reschedule — surfaces double-bookings the API silently allows._

  ```bash
  cal-com-pp-cli conflicts --window 7d --json
  ```
- **`gaps`** — Finds open windows in your schedule that are available but unbooked, filtered by minimum block size.

  _Use this for capacity planning — answers 'when can I take a meeting' rather than 'what's on my plate'._

  ```bash
  cal-com-pp-cli gaps --window 7d --min-minutes 60 --json
  ```
- **`workload`** — Booking distribution across team members over a window — surfaces overloaded vs underutilized hosts.

  _Use this for round-robin tuning or to spot host burnout before it shows up as no-shows._

  ```bash
  cal-com-pp-cli workload --team-id 42 --window 30d --json
  ```
- **`event-types stale`** — Event types with zero bookings in the last N days — candidates for removal.

  _Use this for quarterly cleanup — keeps your bookable surface from drifting._

  ```bash
  cal-com-pp-cli event-types stale --days 90 --json
  ```

### Host control surface
- **`link create`** — Create a new bookable link (event type) on your Cal.com account; prints the cal.com/<your-username>/<slug> URL ready to share.

  _The host's primary creative act. Bookable links are how attendees book time; this is the command to make one._

  ```bash
  cal-com-pp-cli link create --slug 30min --length 30 --title "30 Min Meeting"
  ```
- **`link list`** — List every bookable link you own with the full URL pre-rendered for copy-share.

  _Use this to see what links you have and grab their URLs without hand-composing cal.com/<user>/<slug>._

  ```bash
  cal-com-pp-cli link list --json
  ```
- **`ooo set`** — Mark yourself out-of-office for a date range so Cal.com excludes the period from slot search.

  _Going on vacation? Sick? Run this once and stop getting booked. Optional --redirect-to-user forwards bookings to a teammate (round-robin only)._

  ```bash
  cal-com-pp-cli ooo set --start 2026-05-12 --end 2026-05-18 --reason vacation --notes "Hawaii trip"
  ```
- **`ooo list`** — List your active and upcoming OOO entries.

  ```bash
  cal-com-pp-cli ooo list --json
  ```

### Agent-native plumbing
- **`webhooks coverage`** — Audits registered webhook triggers against the canonical set and reports lifecycle events with no subscriber.

  _Run this whenever you add a new automation — surfaces missed triggers like BOOKING_NO_SHOW_UPDATED before they bite._

  ```bash
  cal-com-pp-cli webhooks coverage --json
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

- `cal-com-pp-cli cal-com-auth` — RFC 6749-compliant token endpoint. Pass client_id in the request body (Section 2.3.1). Use grant_type...

**cal-com-auth-2** — Manage cal com auth 2

- `cal-com-pp-cli cal-com-auth-2 <clientId>` — Returns the OAuth2 client information for the given client ID

**calendars** — Manage calendars

- `cal-com-pp-cli calendars check-ics-feed` — If accessed using an OAuth access token, the `APPS_READ` scope is required.
- `cal-com-pp-cli calendars create-ics-feed` — If accessed using an OAuth access token, the `APPS_WRITE` scope is required.
- `cal-com-pp-cli calendars get` — If accessed using an OAuth access token, the `APPS_READ` scope is required.
- `cal-com-pp-cli calendars get-busy-times` — Get busy times from a calendar. Example request URL is `https://api.cal.com/v2/calendars/busy-times?timeZone=Europe%2...

**conferencing** — Manage conferencing

- `cal-com-pp-cli conferencing get-default` — If accessed using an OAuth access token, the `APPS_READ` scope is required.
- `cal-com-pp-cli conferencing list-installed-apps` — If accessed using an OAuth access token, the `APPS_READ` scope is required.

**credits** — Manage credits

- `cal-com-pp-cli credits charge` — Charge credits for a completed AI agent interaction. Uses externalRef for idempotency to prevent double-charging.
- `cal-com-pp-cli credits get-available` — Check if the authenticated user (or their org/team) has available credits and return the current balance.

**destination-calendars** — Manage destination calendars

- `cal-com-pp-cli destination-calendars` — If accessed using an OAuth access token, the `APPS_WRITE` scope is required.

**event-types** — Manage event types

- `cal-com-pp-cli event-types create` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...
- `cal-com-pp-cli event-types delete` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...
- `cal-com-pp-cli event-types get` — Hidden event types are returned only if authentication is provided and it belongs to the event type owner. Use the...
- `cal-com-pp-cli event-types get-by-id` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...
- `cal-com-pp-cli event-types update` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...

**me** — Manage me

- `cal-com-pp-cli me clear-my-booking-limits` — Removes all of the authenticated user's global booking limits. Only available to organization members — non-org...
- `cal-com-pp-cli me get` — If accessed using an OAuth access token, the `PROFILE_READ` scope is required.
- `cal-com-pp-cli me get-my-booking-limits` — Returns the authenticated user's global booking limits. Unset bounds are returned as null. Only available to...
- `cal-com-pp-cli me update` — Updates the authenticated user's profile. Email changes require verification and the primary email stays unchanged...
- `cal-com-pp-cli me update-my-booking-limits` — Partially updates the authenticated user's global booking limits. Only fields present in the request body are...
- `cal-com-pp-cli me user-ooocontroller-create-my-ooo` — If accessed using an OAuth access token, the `SCHEDULE_WRITE` scope is required.
- `cal-com-pp-cli me user-ooocontroller-delete-my-ooo` — If accessed using an OAuth access token, the `SCHEDULE_WRITE` scope is required.
- `cal-com-pp-cli me user-ooocontroller-get-my-ooo` — If accessed using an OAuth access token, the `SCHEDULE_READ` scope is required.
- `cal-com-pp-cli me user-ooocontroller-update-my-ooo` — If accessed using an OAuth access token, the `SCHEDULE_WRITE` scope is required.

**notifications** — Manage notifications

- `cal-com-pp-cli notifications subscriptions-register` — Register an app push subscription
- `cal-com-pp-cli notifications subscriptions-remove` — Remove an app push subscription

**oauth** — Manage oauth


**oauth-clients** — Manage oauth clients

- `cal-com-pp-cli oauth-clients create` — <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- `cal-com-pp-cli oauth-clients delete` — <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- `cal-com-pp-cli oauth-clients get` — <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- `cal-com-pp-cli oauth-clients get-by-id` — <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- `cal-com-pp-cli oauth-clients update` — <Warning>These endpoints are deprecated and will be removed in the future.</Warning>

**organizations** — Manage organizations


**routing-forms** — Manage routing forms


**schedules** — Manage schedules

- `cal-com-pp-cli schedules create` — Create a schedule for the authenticated user. The point of creating schedules is for event types to be available at...
- `cal-com-pp-cli schedules delete` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...
- `cal-com-pp-cli schedules get` — Get all schedules of the authenticated user. <Note>Please make sure to pass in the cal-api-version header value as...
- `cal-com-pp-cli schedules get-default` — Get the default schedule of the authenticated user. <Note>Please make sure to pass in the cal-api-version header...
- `cal-com-pp-cli schedules get-scheduleid` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...
- `cal-com-pp-cli schedules update` — <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing...

**selected-calendars** — Manage selected calendars

- `cal-com-pp-cli selected-calendars add` — If accessed using an OAuth access token, the `APPS_WRITE` scope is required.
- `cal-com-pp-cli selected-calendars delete` — If accessed using an OAuth access token, the `APPS_WRITE` scope is required.

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

**teams** — Manage teams

- `cal-com-pp-cli teams create` — If accessed using an OAuth access token, the `TEAM_PROFILE_WRITE` scope is required.
- `cal-com-pp-cli teams delete` — If accessed using an OAuth access token, the `TEAM_PROFILE_WRITE` scope is required.
- `cal-com-pp-cli teams get` — If accessed using an OAuth access token, the `TEAM_PROFILE_READ` scope is required.
- `cal-com-pp-cli teams get-teamid` — If accessed using an OAuth access token, the `TEAM_PROFILE_READ` scope is required.
- `cal-com-pp-cli teams update` — If accessed using an OAuth access token, the `TEAM_PROFILE_WRITE` scope is required.

**verified-resources** — Manage verified resources

- `cal-com-pp-cli verified-resources user-get-verified-email-by-id` — If accessed using an OAuth access token, the `VERIFIED_RESOURCES_READ` scope is required.
- `cal-com-pp-cli verified-resources user-get-verified-emails` — If accessed using an OAuth access token, the `VERIFIED_RESOURCES_READ` scope is required.
- `cal-com-pp-cli verified-resources user-get-verified-phone-by-id` — If accessed using an OAuth access token, the `VERIFIED_RESOURCES_READ` scope is required.
- `cal-com-pp-cli verified-resources user-get-verified-phone-numbers` — If accessed using an OAuth access token, the `VERIFIED_RESOURCES_READ` scope is required.
- `cal-com-pp-cli verified-resources user-request-email-verification-code` — Sends a verification code to the email. If accessed using an OAuth access token, the `VERIFIED_RESOURCES_WRITE`...
- `cal-com-pp-cli verified-resources user-request-phone-verification-code` — Sends a verification code to the phone number. If accessed using an OAuth access token, the...
- `cal-com-pp-cli verified-resources user-verify-email` — Use code to verify an email. If accessed using an OAuth access token, the `VERIFIED_RESOURCES_WRITE` scope is required.
- `cal-com-pp-cli verified-resources user-verify-phone-number` — Use code to verify a phone number. If accessed using an OAuth access token, the `VERIFIED_RESOURCES_WRITE` scope is...

**webhooks** — Manage webhooks

- `cal-com-pp-cli webhooks create` — If accessed using an OAuth access token, the `WEBHOOK_WRITE` scope is required.
- `cal-com-pp-cli webhooks delete` — If accessed using an OAuth access token, the `WEBHOOK_WRITE` scope is required.
- `cal-com-pp-cli webhooks get` — Gets a paginated list of webhooks for the authenticated user. If accessed using an OAuth access token, the...
- `cal-com-pp-cli webhooks get-webhookid` — If accessed using an OAuth access token, the `WEBHOOK_READ` scope is required.
- `cal-com-pp-cli webhooks update` — If accessed using an OAuth access token, the `WEBHOOK_WRITE` scope is required.


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
cal-com-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Create a bookable link and share its URL

```bash
cal-com-pp-cli link create --slug 30min --length 30 --title "30 Min Meeting"
cal-com-pp-cli link list --json --select links.slug,links.bookable_url
```

`link create` returns the bookable URL pre-rendered (cal.com/<your-username>/<slug>); `link list` is the host's view of every link they've published.

### Mark yourself out-of-office

```bash
cal-com-pp-cli ooo set --start 2026-05-12 --end 2026-05-18 --reason vacation --notes "Hawaii trip"
cal-com-pp-cli ooo list --json
```

While the OOO entry is active, Cal.com excludes the range from slot search so you don't get booked.

### Today's agenda from the local store

```bash
cal-com-pp-cli agenda --window today --json --select bookings.uid,bookings.title,bookings.start,bookings.attendees
```

Returns just the four fields an agent needs from the agenda envelope — keeps context tight against deeply-nested booking payloads.

### Cross-event-type slot search ranked by start

```bash
cal-com-pp-cli slots find --event-type-ids 96531 --start tomorrow --end "tomorrow 23:59" --json --first-only
```

Fans out /v2/slots per event-type ID; returns only the earliest slot.

### No-show rate by attendee for capacity planning

```bash
cal-com-pp-cli analytics no-show --window 90d --by attendee --json
```

Local SQL aggregation over synced bookings; no API call.

### Audit webhook coverage before adding automation

```bash
cal-com-pp-cli webhooks coverage --json
```

Compares your registered triggers against the canonical Cal.com lifecycle set and surfaces missing subscribers.

### Reschedule a booking to the next free slot

```bash
cal-com-pp-cli reschedule next --uid <booking-uid> --after tomorrow --dry-run
```

One composed command replaces three; --dry-run prints the planned move without committing.

## Auth Setup

Cal.com uses bearer tokens prefixed with `cal_live_` (live) or `cal_test_` (test). Set `CAL_COM_TOKEN` in your environment, or run `auth set-token` once. The CLI also accepts managed-user access tokens and OAuth access tokens through the same Authorization header. Per-resource API-version pinning via `cal-api-version` is handled automatically by the client.

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
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → CLI installation
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## CLI Installation

1. Check Go is installed: `go version` (requires Go 1.23+)
2. Install:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/cal-com/cmd/cal-com-pp-cli@latest
   ```
3. Verify: `cal-com-pp-cli --version`
4. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/productivity/cal-com/cmd/cal-com-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add cal-com-pp-mcp -- cal-com-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which cal-com-pp-cli`
   If not found, offer to install (see CLI Installation above).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   cal-com-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `cal-com-pp-cli <command> --help`.
