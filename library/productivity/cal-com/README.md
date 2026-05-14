# Cal.com CLI

**Every Cal.com command bcharleson ships, plus a local SQLite layer that answers questions the API can't: conflict scans across linked calendars, attendee history, no-show risk, schedule gap finder.**

121 generated commands across Bookings, Event Types, Schedules, Slots, Calendars, Webhooks, Teams, OOO, and Conferencing — every endpoint in the Cal.com Platform API v2 OpenAPI spec, agent-native with --json / --select / --csv / --dry-run / typed exit codes. On top, eight novel commands that join your local synced state to surface conflict detection, attendee history, no-show ranking, schedule gaps, reschedule chains, and host load reports — none of which exist as Cal.com API endpoints.

## Install

The recommended path installs both the `cal-com-pp-cli` binary and the `pp-cal-com` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install cal-com
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install cal-com --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cal-com-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cal-com --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cal-com --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-cal-com skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-cal-com. The skill defines how its required CLI can be installed.
```

## Authentication

Set CAL_COM_API_KEY (or CAL_API_KEY for bcharleson compatibility) to your cal_live_* key from Settings → Security → API keys. The CLI sends Authorization: Bearer <key>. cal-com-pp-cli doctor verifies auth + reachability before anything else.

## Quick Start

```bash
# Verify auth + API reachability
cal-com-pp-cli doctor


# Pull bookings, event types, schedules, teams, OOO into local SQLite
cal-com-pp-cli sync --full


# Today's confirmed bookings, agent-shaped JSON
cal-com-pp-cli bookings list --status accepted --after-start 2026-05-14 --json


# Cross-provider conflict scan — novel
cal-com-pp-cli conflicts --window 7d --json


# Rank attendees by historical no-show rate — novel
cal-com-pp-cli no-show-risk --since 90d --json --select email,no_show_rate,total_count

```

## Unique Features

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

## Usage

Run `cal-com-pp-cli --help` for the full command reference and flag list.

## Commands

### api-keys

Manage api keys

- **`cal-com-pp-cli api-keys keys-refresh`** - Generate a new API key and delete the current one. Provide API key to refresh as a Bearer token in the Authorization header (e.g. "Authorization: Bearer <apiKey>").

### bookings

Manage bookings

- **`cal-com-pp-cli bookings create`** - POST /v2/bookings is used to create regular bookings, recurring bookings and instant bookings. The request bodies for all 3 are almost the same except:
      If eventTypeId in the request body is id of a regular event, then regular booking is created.

      If it is an id of a recurring event type, then recurring booking is created.

      Meaning that the request bodies are equal but the outcome depends on what kind of event type it is with the goal of making it as seamless for developers as possible.

      The start needs to be in UTC aka if the timezone is GMT+2 in Rome and meeting should start at 11, then UTC time should have hours 09:00 aka without time zone.

      Finally, there are 2 ways to book an event type belonging to an individual user:
      1. Provide `eventTypeId` in the request body.
      2. Provide `eventTypeSlug` and `username` and optionally `organizationSlug` if the user with the username is within an organization.

      And 2 ways to book and event type belonging to a team:
      1. Provide `eventTypeId` in the request body.
      2. Provide `eventTypeSlug` and `teamSlug` and optionally `organizationSlug` if the team with the teamSlug is within an organization.

      If you are creating a seated booking for an event type with 'show attendees' disabled, then to retrieve attendees in the response either set 'show attendees' to true on event type level or
      you have to provide an authentication method of event type owner, host, team admin or owner or org admin or owner.

      For event types that have SMS reminders enabled, you need to pass the attendee's phone number in the request body via `attendee.phoneNumber` (e.g., "+19876543210" in international format). This is an optional field, but becomes required when SMS reminders are enabled for the event type. For the complete attendee object structure, see the attendee schema in the `/docs` Swagger endpoint.

      <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli bookings get`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli bookings get-bookinguid`** - `:bookingUid` can be

      1. uid of a normal booking

      2. uid of one of the recurring booking recurrences

      3. uid of recurring booking which will return an array of all recurring booking recurrences (stored as recurringBookingUid on one of the individual recurrences).

      If you are fetching a seated booking for an event type with 'show attendees' disabled, then to retrieve attendees in the response either set 'show attendees' to true on event type level or
      you have to provide an authentication method of event type owner, host, team admin or owner or org admin or owner.

      <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli bookings get-by-seat-uid`** - Get a seated booking by its seat reference UID. This is useful when you have a seatUid from a seated booking and want to retrieve the full booking details.

      If you are fetching a seated booking for an event type with 'show attendees' disabled, then to retrieve attendees in the response either set 'show attendees' to true on event type level or
      you have to provide an authentication method of event type owner, host, team admin or owner or org admin or owner.

      <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>

### cal-com-auth

Manage cal com auth

- **`cal-com-pp-cli cal-com-auth oauth2-get-client`** - Returns the OAuth2 client information for the given client ID
- **`cal-com-pp-cli cal-com-auth oauth2-token`** - RFC 6749-compliant token endpoint. Pass client_id in the request body (Section 2.3.1). Use grant_type 'authorization_code' to exchange an auth code for tokens, or 'refresh_token' to refresh an access token. Accepts both application/x-www-form-urlencoded (standard per RFC 6749 Section 4.1.3) and application/json content types.

### calendars

Manage calendars

- **`cal-com-pp-cli calendars cal-unified-create-connection-event`** - Create a new event on the specified calendar connection. Only supported for Google Calendar connections; other connection types return 400.
- **`cal-com-pp-cli calendars cal-unified-delete-connection-event`** - Delete/cancel an event on the specified calendar connection. Only supported for Google Calendar connections; other connection types return 400.
- **`cal-com-pp-cli calendars cal-unified-get-connection-event`** - Get a single event by ID for the specified calendar connection. Only supported for Google Calendar connections; other connection types return 400.
- **`cal-com-pp-cli calendars cal-unified-get-connection-free-busy`** - Get busy time slots for the specified calendar connection.
- **`cal-com-pp-cli calendars cal-unified-list-connection-events`** - List events in a date range for a specific calendar connection. Only supported for Google Calendar connections; other connection types return 400.
- **`cal-com-pp-cli calendars cal-unified-list-connections`** - Returns all calendar connections for the authenticated user (Google, Office 365, Apple). Use connectionId in connection-scoped endpoints. Note: Event CRUD (list/create/get/update/delete events) is currently only supported for Google Calendar connections; other types will return 400.
- **`cal-com-pp-cli calendars cal-unified-update-connection-event`** - Update an event on the specified calendar connection. Only supported for Google Calendar connections; other connection types return 400.
- **`cal-com-pp-cli calendars check-ics-feed`** - Check an ICS feed
- **`cal-com-pp-cli calendars create-ics-feed`** - Save an ICS feed
- **`cal-com-pp-cli calendars get`** - Get all calendars
- **`cal-com-pp-cli calendars get-busy-times`** - Get busy times from a calendar. Example request URL is `https://api.cal.com/v2/calendars/busy-times?timeZone=Europe%2FMadrid&dateFrom=2024-12-18&dateTo=2024-12-18&calendarsToLoad[0][credentialId]=135&calendarsToLoad[0][externalId]=skrauciz%40gmail.com`. Note: loggedInUsersTz is deprecated, use timeZone instead.

### conferencing

Manage conferencing

- **`cal-com-pp-cli conferencing get-default`** - Get your default conferencing application
- **`cal-com-pp-cli conferencing list-installed-apps`** - List your conferencing applications

### destination-calendars

Manage destination calendars

- **`cal-com-pp-cli destination-calendars update`** - Update destination calendars

### event-types

Manage event types

- **`cal-com-pp-cli event-types create`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli event-types delete`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli event-types get`** - Hidden event types are returned only if authentication is provided and it belongs to the event type owner.
      
      Use the optional `sortCreatedAt` query parameter to order results by creation date (by ID). Accepts "asc" (oldest first) or "desc" (newest first). When not provided, no explicit ordering is applied.
      
      <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli event-types get-by-id`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
    
    Access control: This endpoint fetches an event type by ID and returns it only if the authenticated user is authorized. Authorization is granted to:
    - System admins
    - The event type owner
    - Hosts of the event type or users assigned to the event type
    - Team admins/owners of the team that owns the team event type
    - Organization admins/owners of the event type owner's organization
    - Organization admins/owners of the team's parent organization

    Note: Update and delete endpoints remain restricted to the event type owner only.
- **`cal-com-pp-cli event-types update`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>

### me

Manage me

- **`cal-com-pp-cli me get`** - Get my profile
- **`cal-com-pp-cli me update`** - Updates the authenticated user's profile. Email changes require verification and the primary email stays unchanged until verification completes, unless the new email is already a verified secondary email or the user is platform-managed.

### oauth

Manage oauth


### oauth-clients

Manage oauth clients

- **`cal-com-pp-cli oauth-clients create`** - <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- **`cal-com-pp-cli oauth-clients delete`** - <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- **`cal-com-pp-cli oauth-clients get`** - <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- **`cal-com-pp-cli oauth-clients get-by-id`** - <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- **`cal-com-pp-cli oauth-clients update`** - <Warning>These endpoints are deprecated and will be removed in the future.</Warning>

### schedules

Manage schedules

- **`cal-com-pp-cli schedules create`** - Create a schedule for the authenticated user.

      The point of creating schedules is for event types to be available at specific times.

      The first goal of schedules is to have a default schedule. If you are platform customer and created managed users, then it is important to note that each managed user should have a default schedule.
      1. If you passed `timeZone` when creating managed user, then the default schedule from Monday to Friday from 9AM to 5PM will be created with that timezone. The managed user can then change the default schedule via the `AvailabilitySettings` atom.
      2. If you did not, then we assume you want the user to have this specific schedule right away. You should create a default schedule by specifying
      `"isDefault": true` in the request body. Until the user has a default schedule the user can't be booked nor manage their schedule via the AvailabilitySettings atom.

      The second goal of schedules is to create another schedule that event types can point to. This is useful for when an event is booked because availability is not checked against the default schedule but instead against that specific schedule.
      After creating a non-default schedule, you can update an event type to point to that schedule via the PATCH `event-types/{eventTypeId}` endpoint.

      When specifying start time and end time for each day use the 24 hour format e.g. 08:00, 15:00 etc.

      <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli schedules delete`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli schedules get`** - Get all schedules of the authenticated user.
    
     <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli schedules get-default`** - Get the default schedule of the authenticated user.
    
    <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli schedules get-scheduleid`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli schedules update`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>

### selected-calendars

Manage selected calendars

- **`cal-com-pp-cli selected-calendars add`** - Add a selected calendar
- **`cal-com-pp-cli selected-calendars delete`** - Delete a selected calendar

### slots

Manage slots

- **`cal-com-pp-cli slots delete-reserved`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli slots get-available`** - There are 4 ways to get available slots for event type of an individual user:

      1. By event type id. Example '/v2/slots?eventTypeId=10&start=2050-09-05&end=2050-09-06&timeZone=Europe/Rome'

      2. By event type slug + username. Example '/v2/slots?eventTypeSlug=intro&username=bob&start=2050-09-05&end=2050-09-06'

      3. By event type slug + username + organization slug when searching within an organization. Example '/v2/slots?organizationSlug=org-slug&eventTypeSlug=intro&username=bob&start=2050-09-05&end=2050-09-06'

      4. By usernames only (used for dynamic event type - there is no specific event but you want to know when 2 or more people are available). Example '/v2/slots?usernames=alice,bob&username=bob&organizationSlug=org-slug&start=2050-09-05&end=2050-09-06'. As you see you also need to provide the slug of the organization to which each user in the 'usernames' array belongs.

      And 3 ways to get available slots for team event type:

      1. By team event type id. Example '/v2/slots?eventTypeId=10&start=2050-09-05&end=2050-09-06&timeZone=Europe/Rome'.
         **Note for managed event types**: Managed event types are templates that create individual child event types for each team member. You cannot fetch slots for the parent managed event type directly. Instead, you must:
         - Find the child event type IDs (the ones assigned to specific users)
         - Use those child event type IDs to fetch slots as individual user event types using as described in the individual user section above.

      2. By team event type slug + team slug. Example '/v2/slots?eventTypeSlug=intro&teamSlug=team-slug&start=2050-09-05&end=2050-09-06'

      3. By team event type slug + team slug + organization slug when searching within an organization. Example '/v2/slots?organizationSlug=org-slug&eventTypeSlug=intro&teamSlug=team-slug&start=2050-09-05&end=2050-09-06'

      All of them require "start" and "end" query parameters which define the time range for which available slots should be checked.
      Optional parameters are:
      - timeZone: Time zone in which the available slots should be returned. Defaults to UTC.
      - duration: Only use for event types that allow multiple durations or for dynamic event types. If not passed for multiple duration event types defaults to default duration. For dynamic event types defaults to 30 aka each returned slot is 30 minutes long. So duration=60 means that returned slots will be each 60 minutes long.
      - format: Format of the slots. By default return is an object where each key is date and value is array of slots as string. If you want to get start and end of each slot use "range" as value.
      - bookingUidToReschedule: When rescheduling an existing booking, provide the booking's unique identifier to exclude its time slot from busy time calculations. This ensures the original booking time appears as available for rescheduling.

       <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli slots get-reserved`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli slots reserve`** - Make a slot not available for others to book for a certain period of time. If you authenticate using oAuth credentials, api key or access token
    then you can also specify custom duration for how long the slot should be reserved for (defaults to 5 minutes).
    
    <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli slots update-reserved`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>

### stripe

Manage stripe

- **`cal-com-pp-cli stripe check`** - Check Stripe connection
- **`cal-com-pp-cli stripe redirect`** - Get Stripe connect URL
- **`cal-com-pp-cli stripe save`** - Save Stripe credentials

### verified-resources

Manage verified resources

- **`cal-com-pp-cli verified-resources user-get-verified-email-by-id`** - Get verified email by id
- **`cal-com-pp-cli verified-resources user-get-verified-emails`** - Get list of verified emails
- **`cal-com-pp-cli verified-resources user-get-verified-phone-by-id`** - Get verified phone number by id
- **`cal-com-pp-cli verified-resources user-get-verified-phone-numbers`** - Get list of verified phone numbers
- **`cal-com-pp-cli verified-resources user-request-email-verification-code`** - Sends a verification code to the email
- **`cal-com-pp-cli verified-resources user-request-phone-verification-code`** - Sends a verification code to the phone number
- **`cal-com-pp-cli verified-resources user-verify-email`** - Use code to verify an email
- **`cal-com-pp-cli verified-resources user-verify-phone-number`** - Use code to verify a phone number

### webhooks

Manage webhooks

- **`cal-com-pp-cli webhooks create`** - Create a webhook
- **`cal-com-pp-cli webhooks delete`** - Delete a webhook
- **`cal-com-pp-cli webhooks get`** - Gets a paginated list of webhooks for the authenticated user.
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
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-cal-com -g
```

Then invoke `/pp-cal-com <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add cal-com cal-com-pp-mcp -e CAL_COM_API_KEY=<your-token>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cal-com-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CAL_COM_API_KEY` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "cal-com": {
      "command": "cal-com-pp-mcp",
      "env": {
        "CAL_COM_API_KEY": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
cal-com-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/cal-com-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `CAL_COM_API_KEY` | per_call | Yes | Set to your API credential. |
| `CAL_API_KEY` | per_call | Yes | Set to your API credential. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `cal-com-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $CAL_COM_API_KEY`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

### API-specific

- **401 UnauthorizedException** — Run cal-com-pp-cli auth status; ensure CAL_COM_API_KEY is set and starts with cal_live_.
- **429 rate-limited mid-sync** — The built-in AdaptiveLimiter honors X-RateLimit-Reset; rerun sync — it resumes from the last cursor.
- **conflict scan returns empty** — Run sync --full first; conflict scan reads local calendars/busy and bookings tables only.
- **rescheduled-from chain breaks** — Some legacy bookings missing rescheduledFromUid; cal-com-pp-cli bookings get <uid> --json to inspect.

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**bcharleson/calcom-cli**](https://github.com/bcharleson/calcom-cli) — TypeScript
- [**aditzel/caldotcom-api-v2-sdk**](https://github.com/aditzel/caldotcom-api-v2-sdk) — TypeScript
- [**@calcom/cal-mcp**](https://www.npmjs.com/package/@calcom/cal-mcp) — TypeScript

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
