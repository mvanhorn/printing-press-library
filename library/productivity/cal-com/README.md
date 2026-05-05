# Cal.com CLI

**Every Cal.com feature, plus offline agendas, composed booking flows, and analytics no other Cal.com tool ships.**

A Go single-binary CLI for Cal.com v2 with a local SQLite mirror that turns scheduling data into something queryable. Composed intents like `book` collapse slot-find / reserve / create / confirm into one safe call with `--dry-run`. The store powers `today`, `week`, `analytics`, and `conflicts` — views Cal.com's own API has no equivalent for.

## What This CLI Is Not

This CLI talks to the Cal.com v2 REST API. It is not a replacement for any of the following — reach for the right tool:

- **Calendly bookings** — Calendly is a different scheduling service with a different API. Use a Calendly-specific tool.
- **Google Calendar / Outlook event creation** — those are separate calendar APIs. Use those vendors' SDKs/CLIs. (Cal.com can `connect` external calendars for availability, but this CLI does not write events to them.)
- **Cal.com v1 API workflows** — v1 was deprecated April 8, 2026. This CLI targets `/v2` only.
- **Cal.com web-app navigation** — there is no headless browser; everything goes through the public REST API. Dashboard customization and settings UI flows cannot be automated here.

## Install

### Go

```
go install github.com/mvanhorn/printing-press-library/library/productivity/cal-com/cmd/cal-com-pp-cli@latest
```

### Binary

Download from [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cal-com-current).

## Authentication

Cal.com uses Bearer API keys (the `cal_live_*` prefix from Settings → Developer → API Keys). Set `CAL_COM_TOKEN` in your environment, or run `cal-com-pp-cli auth set-token <token>` to save it to `~/.config/cal-com-pp-cli/config.toml`. `doctor` will tell you which source is active and whether the key reaches `/v2/me`. Per-endpoint `cal-api-version` headers (Bookings 2024-08-13, Slots 2024-09-04, etc.) are handled automatically.

## Quick Start

```bash
# Store your cal_live_ API key; or export CAL_COM_TOKEN
cal-com-pp-cli auth set-token cal_live_xxxxxxxxxxxx


# Confirm auth, reachability, and account match
cal-com-pp-cli doctor


# Mirror bookings, event types, schedules, teams, and webhooks into the local store
cal-com-pp-cli sync --full


# View today's bookings offline; no API call after sync
cal-com-pp-cli today --json


# Compose a full booking flow safely; drop --dry-run to execute
cal-com-pp-cli book --event-type-id <id> --start "tomorrow 2pm" --attendee-name "Jane" --attendee-email "jane@example.com" --dry-run

```

## Unique Features

These capabilities aren't available in any other tool for this API.

### Composed intents

- **`book`** — Find a slot and book it in a single command — no slot/reserve/create/confirm chain.

  _Reach for this when an agent or operator wants to book a meeting end-to-end without managing slot reservation state by hand._

  ```bash
  cal-com-pp-cli book --event-type-id 96531 --start "2026-05-01T14:00:00Z" --attendee-name "Jane Doe" --attendee-email "jane@example.com" --dry-run --json
  ```
- **`slots find`** — Find first available slot across multiple event types in one call, ranked by start time.

  _Pick this when the agent doesn't care which meeting type — only when the next available slot is._

  ```bash
  cal-com-pp-cli slots find --event-type-ids 96531,96532,96533 --start "2026-05-01" --end "2026-05-08" --first-only --json
  ```
- **`bookings pending`** — Pending-confirmation bookings sorted by age, with default 24h max-age cutoff.

  _Use when sweeping the pending queue to confirm/decline before the SLA window closes._

  ```bash
  cal-com-pp-cli bookings pending --max-age 24h --json
  ```

### Local state that compounds

- **`today`** — Today's bookings with status, attendees, and meeting links — read from the local store, no API call needed.

  _First-thing-in-the-morning view that works offline and stays cheap to call repeatedly._

  ```bash
  cal-com-pp-cli today --json
  ```
- **`week`** — 7-day calendar view of upcoming bookings, with conflict highlighting and per-day rollup counts.

  _Use when you need a one-look view of the upcoming week without paging through API responses._

  ```bash
  cal-com-pp-cli week --start monday --json
  ```
- **`analytics bookings|cancellations|no-show|density`** — Booking analytics over a time window. Each subcommand is a different metric; group with `--by event-type|attendee|weekday|hour|status` (or `--unit weekday|hour` for `density`).

  _Reach for these when scoring a workflow's health, planning team capacity, or hunting cancellation patterns._

  ```bash
  cal-com-pp-cli analytics bookings --window 30d --by event-type --json
  cal-com-pp-cli analytics no-show --window 90d --by attendee --json
  cal-com-pp-cli analytics density --unit hour --window 90d --json
  ```
- **`conflicts`** — Detects overlaps between active Cal.com bookings and external calendar busy-times.

  _Spot last-minute calendar additions that didn't propagate into Cal.com availability._

  ```bash
  cal-com-pp-cli conflicts --window 7d --json
  ```
- **`gaps`** — Finds open windows in your schedule that are available but unbooked, filtered by minimum block size.

  _Use when looking for capacity to take new meetings or to spot underused windows worth promoting._

  ```bash
  cal-com-pp-cli gaps --window 7d --min-minutes 30 --json
  ```
- **`workload`** — Booking distribution across team members over a window — surfaces overloaded vs underutilized hosts.

  _Reach for this when tuning round-robin weights or planning team capacity._

  ```bash
  cal-com-pp-cli workload --team-id 42 --window 30d --json
  ```
- **`event-types stale`** — Event types with zero bookings in the last N days — candidates for removal.

  _Run during quarterly cleanups to retire dead booking links._

  ```bash
  cal-com-pp-cli event-types stale --days 30 --json
  ```

### Reachability mitigation

- **`webhooks coverage`** — Audits registered webhook triggers against the canonical set and reports lifecycle events with no subscriber.

  _Run before relying on webhooks in production to confirm every lifecycle stage has a handler._

  ```bash
  cal-com-pp-cli webhooks coverage --json
  ```

### Agent-native plumbing

- **`webhooks triggers`** — Static reference of every valid Cal.com webhook trigger constant, grouped by lifecycle stage.

  _Reach for this before writing webhook scaffolding, so trigger strings are exact._

  ```bash
  cal-com-pp-cli webhooks triggers --json
  ```

## Usage

Run `cal-com-pp-cli --help` for the full command reference and flag list.

## Commands

### api-keys

Manage api keys

- **`cal-com-pp-cli api-keys keys-refresh`** - Refresh API Key

### auth

Manage auth

- **`cal-com-pp-cli auth oauth2-get-client`** - Get OAuth2 client
- **`cal-com-pp-cli auth oauth2-token`** - Exchange authorization code or refresh token for tokens

### bookings

Manage bookings

- **`cal-com-pp-cli bookings create`** - Create a booking
- **`cal-com-pp-cli bookings get`** - Get all bookings
- **`cal-com-pp-cli bookings get-bookinguid`** - Get a booking
- **`cal-com-pp-cli bookings get-by-seat-uid`** - Get a booking by seat UID

### calendars

Manage calendars

- **`cal-com-pp-cli calendars cal-unified-create-connection-event`** - Create event on a connection
- **`cal-com-pp-cli calendars cal-unified-delete-connection-event`** - Delete event for a connection
- **`cal-com-pp-cli calendars cal-unified-get-connection-event`** - Get event for a connection
- **`cal-com-pp-cli calendars cal-unified-get-connection-free-busy`** - Get free/busy for a connection
- **`cal-com-pp-cli calendars cal-unified-list-connection-events`** - List events for a connection
- **`cal-com-pp-cli calendars cal-unified-list-connections`** - List calendar connections
- **`cal-com-pp-cli calendars cal-unified-update-connection-event`** - Update event for a connection
- **`cal-com-pp-cli calendars check-ics-feed`** - Check an ICS feed
- **`cal-com-pp-cli calendars create-ics-feed`** - Save an ICS feed
- **`cal-com-pp-cli calendars get`** - Get all calendars
- **`cal-com-pp-cli calendars get-busy-times`** - Get busy times

### conferencing

Manage conferencing

- **`cal-com-pp-cli conferencing get-default`** - Get your default conferencing application
- **`cal-com-pp-cli conferencing list-installed-apps`** - List your conferencing applications

### destination-calendars

Manage destination calendars

- **`cal-com-pp-cli destination-calendars update`** - Update destination calendars

### event-types

Manage event types

- **`cal-com-pp-cli event-types create`** - Create an event type
- **`cal-com-pp-cli event-types delete`** - Delete an event type
- **`cal-com-pp-cli event-types get`** - Get all event types
- **`cal-com-pp-cli event-types get-by-id`** - Get an event type
- **`cal-com-pp-cli event-types update`** - Update an event type

### me

Manage me

- **`cal-com-pp-cli me get`** - Get my profile
- **`cal-com-pp-cli me update`** - Update my profile

### oauth

Refresh managed-user OAuth flow tokens.

- **`cal-com-pp-cli oauth refresh oauth-flow-tokens`** - Refresh managed user tokens

### oauth-clients

Manage oauth clients

- **`cal-com-pp-cli oauth-clients create`** - Create an OAuth client
- **`cal-com-pp-cli oauth-clients delete`** - Delete an OAuth client
- **`cal-com-pp-cli oauth-clients get`** - Get all OAuth clients
- **`cal-com-pp-cli oauth-clients get-by-id`** - Get an OAuth client
- **`cal-com-pp-cli oauth-clients update`** - Update an OAuth client

### organizations

Enterprise org admin: attributes, bookings, delegation credentials, memberships, OOO, roles, routing forms, schedules, teams, users, and webhooks. Each is a nested subcommand group — run `cal-com-pp-cli organizations <group> --help` to see leaves. Examples:

- **`cal-com-pp-cli organizations attributes ...`** - Manage org-level custom attributes (create, list, options, assign to users)
- **`cal-com-pp-cli organizations memberships ...`** - List/create/update/delete org memberships
- **`cal-com-pp-cli organizations teams ...`** - Org-scoped team admin
- **`cal-com-pp-cli organizations users ...`** - Org user admin (create, list, OOO, schedules)
- **`cal-com-pp-cli organizations webhooks ...`** - Org-level webhook subscriptions

### routing-forms

Routing-form helpers (calculate-slots subgroup).

- **`cal-com-pp-cli routing-forms calculate-slots ...`** - Compute available slots for a routing-form response

### schedules

Manage schedules

- **`cal-com-pp-cli schedules create`** - Create a schedule
- **`cal-com-pp-cli schedules delete`** - Delete a schedule
- **`cal-com-pp-cli schedules get`** - Get all schedules
- **`cal-com-pp-cli schedules get-default`** - Get default schedule
- **`cal-com-pp-cli schedules get-scheduleid`** - Get a schedule
- **`cal-com-pp-cli schedules update`** - Update a schedule

### selected-calendars

Manage selected calendars

- **`cal-com-pp-cli selected-calendars add`** - Add a selected calendar
- **`cal-com-pp-cli selected-calendars delete`** - Delete a selected calendar

### slots

Manage slots

- **`cal-com-pp-cli slots delete-reserved`** - Delete a reserved slot
- **`cal-com-pp-cli slots get-available`** - Get available time slots for an event type
- **`cal-com-pp-cli slots get-reserved`** - Get reserved slot
- **`cal-com-pp-cli slots reserve`** - Reserve a slot
- **`cal-com-pp-cli slots update-reserved`** - Update a reserved slot

### stripe

Manage stripe

- **`cal-com-pp-cli stripe check`** - Check Stripe connection
- **`cal-com-pp-cli stripe redirect`** - Get Stripe connect URL
- **`cal-com-pp-cli stripe save`** - Save Stripe credentials

### teams

Manage teams

- **`cal-com-pp-cli teams create`** - Create a team
- **`cal-com-pp-cli teams delete`** - Delete a team
- **`cal-com-pp-cli teams get`** - Get teams
- **`cal-com-pp-cli teams get-teamid`** - Get a team
- **`cal-com-pp-cli teams update`** - Update a team

### verified-resources

Manage verified resources

- **`cal-com-pp-cli verified-resources user-get-verified-email-by-id`** - Get verified email by id
- **`cal-com-pp-cli verified-resources user-get-verified-emails`** - Get list of verified emails
- **`cal-com-pp-cli verified-resources user-get-verified-phone-by-id`** - Get verified phone number by id
- **`cal-com-pp-cli verified-resources user-get-verified-phone-numbers`** - Get list of verified phone numbers
- **`cal-com-pp-cli verified-resources user-request-email-verification-code`** - Request email verification code
- **`cal-com-pp-cli verified-resources user-request-phone-verification-code`** - Request phone number verification code
- **`cal-com-pp-cli verified-resources user-verify-email`** - Verify an email
- **`cal-com-pp-cli verified-resources user-verify-phone-number`** - Verify a phone number

### webhooks

Manage webhooks

- **`cal-com-pp-cli webhooks create`** - Create a webhook
- **`cal-com-pp-cli webhooks delete`** - Delete a webhook
- **`cal-com-pp-cli webhooks get`** - Get all webhooks
- **`cal-com-pp-cli webhooks get-webhookid`** - Get a webhook
- **`cal-com-pp-cli webhooks update`** - Update a webhook


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
cal-com-pp-cli bookings get

# JSON for scripting and agents
cal-com-pp-cli bookings get --json

# Filter to specific fields
cal-com-pp-cli bookings get --json --select id,name,status

# Dry run — show the request without sending
cal-com-pp-cli bookings get --dry-run

# Agent mode — JSON + compact + no prompts in one flag
cal-com-pp-cli bookings get --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use as MCP Server

This CLI ships a companion MCP server for use with Claude Desktop, Cursor, and other MCP-compatible tools.

### Claude Code

```bash
claude mcp add cal-com cal-com-pp-mcp -e CAL_COM_TOKEN=<your-token>
```

### Claude Desktop

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "cal-com": {
      "command": "cal-com-pp-mcp",
      "env": {
        "CAL_COM_TOKEN": "<your-key>"
      }
    }
  }
}
```

## Health Check

```bash
cal-com-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/cal-com-pp-cli/config.toml`

Environment variables:
- `CAL_COM_TOKEN`

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `cal-com-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CAL_COM_TOKEN`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 from /v2/me** — Run `doctor` — the token may be missing the `cal_live_` prefix or have been revoked. Set `CAL_COM_TOKEN` or `auth set-token <token>`.
- **Slots return empty for a busy week** — Confirm the event type's host has a default schedule active (`schedules get-default`) and at least one connected calendar (`calendars get`).
- **`today` shows yesterday's data** — Run `sync` to refresh; the store is incremental and won't catch new bookings until you sync.
- **Webhook never fires** — Run `webhooks coverage` to see if the lifecycle event you expect is registered, then `webhooks triggers` for valid trigger names.
- **429 rate limited** — Cal.com allows 120 req/min on API keys. Use `--limit` and the local store to reduce repeat calls.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**calcom/cal-mcp**](https://github.com/calcom/cal-mcp) — TypeScript (21 stars)
- [**mumunha/cal_dot_com_mcpserver**](https://github.com/mumunha/cal_dot_com_mcpserver) — TypeScript (3 stars)
- [**bcharleson/calcom-cli**](https://github.com/bcharleson/calcom-cli) — TypeScript
- [**dsddet/booking_chest**](https://github.com/dsddet/booking_chest) — Python
- [**aditzel/caldotcom-api-v2-sdk**](https://github.com/aditzel/caldotcom-api-v2-sdk) — TypeScript
- [**vinayh/calcom-mcp**](https://github.com/vinayh/calcom-mcp) — TypeScript
- [**Danielpeter-99/calcom-mcp**](https://github.com/Danielpeter-99/calcom-mcp) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
