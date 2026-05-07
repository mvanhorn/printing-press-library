# Cal.com CLI

**Every Cal.com feature, plus offline agendas, composed booking flows, and analytics no other Cal.com tool ships.**

cal-com-pp-cli wraps the entire Cal.com v2 API and adds a local SQLite store of your bookings, event types, schedules, and team data. Composed intents like `book` and `reschedule next` collapse multi-call dances into one transactional command. Local-store analytics, conflict detection, and team workload land in milliseconds with no API call.

Learn more at [Cal.com](https://cal.com).

## Install

### Binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cal-com-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

### Go

```
go install github.com/mvanhorn/printing-press-library/library/other/cal-com/cmd/cal-com-pp-cli@latest
```

## Authentication

Cal.com uses bearer tokens prefixed with `cal_live_` (live) or `cal_test_` (test). Set `CAL_COM_TOKEN` in your environment, or run `auth set-token` once. The CLI also accepts managed-user access tokens and OAuth access tokens through the same Authorization header. Per-resource API-version pinning via `cal-api-version` is handled automatically by the client.

## Quick Start

```bash
# Confirm CAL_COM_TOKEN is loaded; use auth set-token <token> to save one to the config
cal-com-pp-cli auth status


# Confirm the token works against /v2/me
cal-com-pp-cli doctor


# Pull bookings, event types, schedules, teams, and webhooks into the local store
cal-com-pp-cli sync


# See today's bookings without an API call
cal-com-pp-cli agenda --window today --json


# Fan out the slot search across event-type IDs
cal-com-pp-cli slots find --event-type-ids 96531 --start tomorrow --end "tomorrow 23:59" --json


# Compose slot check + reservation + create + confirm in one call
cal-com-pp-cli book --event-type-id 96531 --start 2026-05-06T17:00:00Z --attendee-name Guest --attendee-email guest@example.com --dry-run

```

## Unique Features

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

## Cookbook

Daily flows that combine the unique commands above with the underlying API surface. Every recipe uses verified flag names — copy and run.

### Create a bookable link and grab its URL

```bash
# Create a 30-minute meeting link, then list to confirm
cal-com-pp-cli link create --slug 30min --length 30 --title "30 Min Meeting"
cal-com-pp-cli link list --json --select links.slug,links.bookable_url
```

### Mark yourself out-of-office

```bash
# Vacation block, exclude from slot search
cal-com-pp-cli ooo set --start 2026-05-12 --end 2026-05-18 --reason vacation --notes "Hawaii trip"
cal-com-pp-cli ooo list --json
```

### Book a meeting end-to-end (dry run, then commit)

```bash
# Verify and preview
cal-com-pp-cli book \
  --event-type-id 96531 \
  --start 2026-05-06T17:00:00Z \
  --attendee-name "Jane Doe" \
  --attendee-email jane@example.com \
  --dry-run

# Drop --dry-run to actually create the booking
cal-com-pp-cli book \
  --event-type-id 96531 \
  --start 2026-05-06T17:00:00Z \
  --attendee-name "Jane Doe" \
  --attendee-email jane@example.com
```

### See today's calendar offline (no API call)

```bash
cal-com-pp-cli sync                                # one-time refresh
cal-com-pp-cli agenda --window today --json
cal-com-pp-cli agenda --window 7d --select id,start,title,attendees
```

### Find the next free slot across event types

```bash
cal-com-pp-cli slots find \
  --event-type-ids 96531,96532 \
  --start tomorrow \
  --end "tomorrow 23:59" \
  --json
```

### Reschedule a booking to the next available slot

```bash
cal-com-pp-cli reschedule next --uid <booking-uid> --after tomorrow --dry-run
```

### Detect double-bookings before they bite

```bash
cal-com-pp-cli conflicts --window 7d --json
```

### No-show analytics by attendee

```bash
cal-com-pp-cli analytics no-show --window 90d --by attendee --json
```

### Find unbooked windows (capacity planning)

```bash
cal-com-pp-cli gaps --window 7d --min-minutes 60 --json
```

### Audit team workload over the last 30 days

```bash
cal-com-pp-cli workload --team-id 42 --window 30d --json
```

### Audit webhook coverage against canonical events

```bash
cal-com-pp-cli webhooks coverage --json
```

### Cancel a booking (preview, then commit)

```bash
cal-com-pp-cli bookings cancel bookings-booking <bookingUid> --dry-run
cal-com-pp-cli bookings cancel bookings-booking <bookingUid>
```

### Search across synced data

```bash
cal-com-pp-cli search "pricing review" --json
```

### Browse the full API surface offline

```bash
cal-com-pp-cli api --help
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

      For team event types it is possible to create instant meeting. To do that just pass `"instant": true` to the request body.

      The start needs to be in UTC aka if the timezone is GMT+2 in Rome and meeting should start at 11, then UTC time should have hours 09:00 aka without time zone.

      Finally, there are 2 ways to book an event type belonging to an individual user:
      1. Provide `eventTypeId` in the request body.
      2. Provide `eventTypeSlug` and `username` and optionally `organizationSlug` if the user with the username is within an organization.

      And 2 ways to book and event type belonging to a team:
      1. Provide `eventTypeId` in the request body.
      2. Provide `eventTypeSlug` and `teamSlug` and optionally `organizationSlug` if the team with the teamSlug is within an organization.

      If you are creating a seated booking for an event type with 'show attendees' disabled, then to retrieve attendees in the response either set 'show attendees' to true on event type level or
      you have to provide an authentication method of event type owner, host, team admin or owner or org admin or owner.

      For event types that have SMS reminders workflow, you need to pass the attendee's phone number in the request body via `attendee.phoneNumber` (e.g., "+19876543210" in international format). This is an optional field, but becomes required when SMS reminders are enabled for the event type. For the complete attendee object structure, see the [attendee object](https://cal.com/docs/api-reference/v2/bookings/create-a-booking#body-attendee) documentation.

      <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>
- **`cal-com-pp-cli bookings get`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>

If accessed using an OAuth access token, the `BOOKING_READ` scope is required.
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

- **`cal-com-pp-cli cal-com-auth oauth2-token`** - RFC 6749-compliant token endpoint. Pass client_id in the request body (Section 2.3.1). Use grant_type 'authorization_code' to exchange an auth code for tokens, or 'refresh_token' to refresh an access token. Accepts both application/x-www-form-urlencoded (standard per RFC 6749 Section 4.1.3) and application/json content types.

### cal-com-auth-2

Manage cal com auth 2

- **`cal-com-pp-cli cal-com-auth-2 oauth2-get-client`** - Returns the OAuth2 client information for the given client ID

### calendars

Manage calendars

- **`cal-com-pp-cli calendars check-ics-feed`** - If accessed using an OAuth access token, the `APPS_READ` scope is required.
- **`cal-com-pp-cli calendars create-ics-feed`** - If accessed using an OAuth access token, the `APPS_WRITE` scope is required.
- **`cal-com-pp-cli calendars get`** - If accessed using an OAuth access token, the `APPS_READ` scope is required.
- **`cal-com-pp-cli calendars get-busy-times`** - Get busy times from a calendar. Example request URL is `https://api.cal.com/v2/calendars/busy-times?timeZone=Europe%2FMadrid&dateFrom=2024-12-18&dateTo=2024-12-18&calendarsToLoad[0][credentialId]=135&calendarsToLoad[0][externalId]=skrauciz%40gmail.com`. Note: loggedInUsersTz is deprecated, use timeZone instead. If accessed using an OAuth access token, the `APPS_READ` scope is required.

### conferencing

Manage conferencing

- **`cal-com-pp-cli conferencing get-default`** - If accessed using an OAuth access token, the `APPS_READ` scope is required.
- **`cal-com-pp-cli conferencing list-installed-apps`** - If accessed using an OAuth access token, the `APPS_READ` scope is required.

### credits

Manage credits

- **`cal-com-pp-cli credits charge`** - Charge credits for a completed AI agent interaction. Uses externalRef for idempotency to prevent double-charging.
- **`cal-com-pp-cli credits get-available`** - Check if the authenticated user (or their org/team) has available credits and return the current balance.

### destination-calendars

Manage destination calendars

- **`cal-com-pp-cli destination-calendars update`** - If accessed using an OAuth access token, the `APPS_WRITE` scope is required.

### event-types

Manage event types

- **`cal-com-pp-cli event-types create`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>

If accessed using an OAuth access token, the `EVENT_TYPE_WRITE` scope is required.
- **`cal-com-pp-cli event-types delete`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>

If accessed using an OAuth access token, the `EVENT_TYPE_WRITE` scope is required.
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

    Note: Update and delete endpoints remain restricted to the event type owner only. If accessed using an OAuth access token, the `EVENT_TYPE_READ` scope is required.
- **`cal-com-pp-cli event-types update`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>

If accessed using an OAuth access token, the `EVENT_TYPE_WRITE` scope is required.

### me

Manage me

- **`cal-com-pp-cli me clear-my-booking-limits`** - Removes all of the authenticated user's global booking limits. Only available to organization members — non-org accounts receive a 403. If accessed using an OAuth access token, the `PROFILE_WRITE` scope is required.
- **`cal-com-pp-cli me get`** - If accessed using an OAuth access token, the `PROFILE_READ` scope is required.
- **`cal-com-pp-cli me get-my-booking-limits`** - Returns the authenticated user's global booking limits. Unset bounds are returned as null. Only available to organization members — non-org accounts receive a 403. If accessed using an OAuth access token, the `PROFILE_READ` scope is required.
- **`cal-com-pp-cli me update`** - Updates the authenticated user's profile. Email changes require verification and the primary email stays unchanged until verification completes, unless the new email is already a verified secondary email or the user is platform-managed. If accessed using an OAuth access token, the `PROFILE_WRITE` scope is required.
- **`cal-com-pp-cli me update-my-booking-limits`** - Partially updates the authenticated user's global booking limits. Only fields present in the request body are changed; omit a field to leave it untouched, or set it to null to remove that limit. Only available to organization members — non-org accounts receive a 403. If accessed using an OAuth access token, the `PROFILE_WRITE` scope is required.
- **`cal-com-pp-cli me user-ooocontroller-create-my-ooo`** - If accessed using an OAuth access token, the `SCHEDULE_WRITE` scope is required.
- **`cal-com-pp-cli me user-ooocontroller-delete-my-ooo`** - If accessed using an OAuth access token, the `SCHEDULE_WRITE` scope is required.
- **`cal-com-pp-cli me user-ooocontroller-get-my-ooo`** - If accessed using an OAuth access token, the `SCHEDULE_READ` scope is required.
- **`cal-com-pp-cli me user-ooocontroller-update-my-ooo`** - If accessed using an OAuth access token, the `SCHEDULE_WRITE` scope is required.

### notifications

Manage notifications

- **`cal-com-pp-cli notifications subscriptions-register`** - Register an app push subscription
- **`cal-com-pp-cli notifications subscriptions-remove`** - Remove an app push subscription

### oauth

Manage oauth


### oauth-clients

Manage oauth clients

- **`cal-com-pp-cli oauth-clients create`** - <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- **`cal-com-pp-cli oauth-clients delete`** - <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- **`cal-com-pp-cli oauth-clients get`** - <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- **`cal-com-pp-cli oauth-clients get-by-id`** - <Warning>These endpoints are deprecated and will be removed in the future.</Warning>
- **`cal-com-pp-cli oauth-clients update`** - <Warning>These endpoints are deprecated and will be removed in the future.</Warning>

### organizations

Manage organizations


### routing-forms

Manage routing forms


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

If accessed using an OAuth access token, the `SCHEDULE_WRITE` scope is required.
- **`cal-com-pp-cli schedules delete`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>

If accessed using an OAuth access token, the `SCHEDULE_WRITE` scope is required.
- **`cal-com-pp-cli schedules get`** - Get all schedules of the authenticated user.
    
     <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>

If accessed using an OAuth access token, the `SCHEDULE_READ` scope is required.
- **`cal-com-pp-cli schedules get-default`** - Get the default schedule of the authenticated user.
    
    <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>

If accessed using an OAuth access token, the `SCHEDULE_READ` scope is required.
- **`cal-com-pp-cli schedules get-scheduleid`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>

If accessed using an OAuth access token, the `SCHEDULE_READ` scope is required.
- **`cal-com-pp-cli schedules update`** - <Note>Please make sure to pass in the cal-api-version header value as mentioned in the Headers section. Not passing the correct value will default to an older version of this endpoint.</Note>

If accessed using an OAuth access token, the `SCHEDULE_WRITE` scope is required.

### selected-calendars

Manage selected calendars

- **`cal-com-pp-cli selected-calendars add`** - If accessed using an OAuth access token, the `APPS_WRITE` scope is required.
- **`cal-com-pp-cli selected-calendars delete`** - If accessed using an OAuth access token, the `APPS_WRITE` scope is required.

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

### teams

Manage teams

- **`cal-com-pp-cli teams create`** - If accessed using an OAuth access token, the `TEAM_PROFILE_WRITE` scope is required.
- **`cal-com-pp-cli teams delete`** - If accessed using an OAuth access token, the `TEAM_PROFILE_WRITE` scope is required.
- **`cal-com-pp-cli teams get`** - If accessed using an OAuth access token, the `TEAM_PROFILE_READ` scope is required.
- **`cal-com-pp-cli teams get-teamid`** - If accessed using an OAuth access token, the `TEAM_PROFILE_READ` scope is required.
- **`cal-com-pp-cli teams update`** - If accessed using an OAuth access token, the `TEAM_PROFILE_WRITE` scope is required.

### verified-resources

Manage verified resources

- **`cal-com-pp-cli verified-resources user-get-verified-email-by-id`** - If accessed using an OAuth access token, the `VERIFIED_RESOURCES_READ` scope is required.
- **`cal-com-pp-cli verified-resources user-get-verified-emails`** - If accessed using an OAuth access token, the `VERIFIED_RESOURCES_READ` scope is required.
- **`cal-com-pp-cli verified-resources user-get-verified-phone-by-id`** - If accessed using an OAuth access token, the `VERIFIED_RESOURCES_READ` scope is required.
- **`cal-com-pp-cli verified-resources user-get-verified-phone-numbers`** - If accessed using an OAuth access token, the `VERIFIED_RESOURCES_READ` scope is required.
- **`cal-com-pp-cli verified-resources user-request-email-verification-code`** - Sends a verification code to the email. If accessed using an OAuth access token, the `VERIFIED_RESOURCES_WRITE` scope is required.
- **`cal-com-pp-cli verified-resources user-request-phone-verification-code`** - Sends a verification code to the phone number. If accessed using an OAuth access token, the `VERIFIED_RESOURCES_WRITE` scope is required.
- **`cal-com-pp-cli verified-resources user-verify-email`** - Use code to verify an email. If accessed using an OAuth access token, the `VERIFIED_RESOURCES_WRITE` scope is required.
- **`cal-com-pp-cli verified-resources user-verify-phone-number`** - Use code to verify a phone number. If accessed using an OAuth access token, the `VERIFIED_RESOURCES_WRITE` scope is required.

### webhooks

Manage webhooks

- **`cal-com-pp-cli webhooks create`** - If accessed using an OAuth access token, the `WEBHOOK_WRITE` scope is required.
- **`cal-com-pp-cli webhooks delete`** - If accessed using an OAuth access token, the `WEBHOOK_WRITE` scope is required.
- **`cal-com-pp-cli webhooks get`** - Gets a paginated list of webhooks for the authenticated user. If accessed using an OAuth access token, the `WEBHOOK_READ` scope is required.
- **`cal-com-pp-cli webhooks get-webhookid`** - If accessed using an OAuth access token, the `WEBHOOK_READ` scope is required.
- **`cal-com-pp-cli webhooks update`** - If accessed using an OAuth access token, the `WEBHOOK_WRITE` scope is required.


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
- **Retryable** - creates return "already exists" on retry, deletes return "already deleted"
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

```bash
go install github.com/mvanhorn/printing-press-library/library/other/cal-com/cmd/cal-com-pp-mcp@latest
```

Then register it:

```bash
claude mcp add cal-com cal-com-pp-mcp -e CAL_COM_TOKEN=<your-token>
```

</details>

<!-- pp-hermes-install-anchor -->
## Install via Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-cal-com --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-cal-com --force
```

## Install via OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-cal-com skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-cal-com. The skill defines how its required CLI can be installed.
```

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/cal-com-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `CAL_COM_TOKEN` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.

```bash
go install github.com/mvanhorn/printing-press-library/library/other/cal-com/cmd/cal-com-pp-mcp@latest
```

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

</details>

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

- **doctor reports 401 Unauthorized** — Re-run `auth set-token` with a `cal_live_` or `cal_test_` key from https://app.cal.com/settings/developer/api-keys
- **sync reports 0 bookings but you have bookings on Cal.com** — Cal.com's /v2/bookings defaults to upcoming only; pass `--include-past` or use a wider date range
- **slots find returns nothing for a known-bookable event type** — Slots respect the schedule's working hours and timezone — pass `--timezone America/Los_Angeles` (or your account TZ) explicitly
- **book fails with 'no available slot'** — Slot reservations expire after 5 minutes; re-run with a fresh `--after` window
- **agenda shows nothing after a fresh sync** — agenda reads from the local store; ensure sync completed (`sync --json | jq '.synced_resources'`) and pass `--window 7d` to widen the lens

---

## Sources & Inspiration

This CLI was built by studying these projects and resources:

- [**calcom/cal-mcp**](https://github.com/calcom/cal-mcp) — TypeScript (21 stars)
- [**mumunha/cal_dot_com_mcpserver**](https://github.com/mumunha/cal_dot_com_mcpserver) — TypeScript (3 stars)
- [**dsddet/booking_chest**](https://github.com/dsddet/booking_chest) — Python
- [**bcharleson/calcom-cli**](https://github.com/bcharleson/calcom-cli) — TypeScript
- [**aditzel/caldotcom-api-v2-sdk**](https://github.com/aditzel/caldotcom-api-v2-sdk) — TypeScript
- [**vinayh/calcom-mcp**](https://github.com/vinayh/calcom-mcp) — TypeScript
- [**Danielpeter-99/calcom-mcp**](https://github.com/Danielpeter-99/calcom-mcp) — Python

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
