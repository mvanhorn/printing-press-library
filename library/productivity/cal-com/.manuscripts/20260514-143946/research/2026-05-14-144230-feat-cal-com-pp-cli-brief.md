# Cal.com CLI Brief

## API Identity
- Domain: Open-source scheduling infrastructure (Calendly alternative). Self-serve booking links, team round-robin, OAuth integrations.
- Users: SaaS founders embedding bookings, freelancers/consultants, engineering teams running 1:1s and customer interviews, RevOps embedding scheduling into their stack.
- Data profile: Bookings (status, attendees, time, location), Event Types (template + availability rules), Schedules (availability windows), Slots (computed availability), Webhooks, Teams, OOO entries, connected calendars (Google/Outlook/iCloud/ICS), verified emails/phones.

## Reachability Risk
- None. Public Platform API v2 with stable OpenAPI 3.0.0 spec (82 paths, 121 operations) at `raw.githubusercontent.com/calcom/cal.com/main/docs/api-reference/v2/openapi.json`. Bearer `cal_live_*` key auth, 120 req/min rate limit.

## Top Workflows
1. **"Show me today's bookings"** - `bookings list` with status filters; the headline read command. Power users want filterable, sorted, JSON-piped output.
2. **"Cancel/reschedule a specific booking"** - find a booking by attendee email or UID, then cancel/reschedule with reason. Most common mutation.
3. **"What slots are free for my X event-type next week?"** - query availability slots, optionally reserve, then book.
4. **"Sync my upcoming week locally"** - pull all bookings + event types + schedules into local store; query offline with SQL/jq/grep.
5. **"Create + share a new event type"** - author a new bookable resource (length, schedule, buffer, slot interval) and surface the share URL.

## Table Stakes (every competitor has these - we must too)
From bcharleson/calcom-cli (61 tools, 14 resource groups; the GOAT bar):
- Full CRUD: bookings, event-types, schedules, webhooks, teams, OOO, slots/reservations
- Auth: env var + Bearer header, interactive login, status, logout
- Calendar list/busy/check, conferencing connect/default/disconnect, Stripe connect/check
- Profile read/update
- Selected + destination calendars
- Verified email/phone resources
- JSON output, structured errors, exponential-backoff retry
- MCP server mode (single codebase serves CLI + MCP)

## Data Layer
- Primary entities: bookings, event_types, schedules, webhooks, teams, ooo, calendars, verified_emails, verified_phones, me.
- Sync cursor: bookings ordered by start time (afterStart/beforeEnd natural pagination); event-types and schedules are small enough to full-sync.
- FTS/search: full-text over booking attendee names+emails+notes+title, event-type title+description+slug.
- Foreign keys: bookings -> event_types, bookings -> attendees, event_types -> schedules.

## Codebase Intelligence
- Source: bcharleson/calcom-cli README + Cal.com Platform API v2 docs + OpenAPI raw.
- Auth: Authorization: Bearer cal_live_*. Env var CAL_COM_API_KEY (slug-prefix default); also recognize CAL_API_KEY as alias for compat with bcharleson.
- Data model: REST-only, JSON. Body shapes change between date-versions (e.g. 2024_08_13 for bookings) - version pinned via cal-api-version header.
- Rate limiting: 120 req/min standard, 200 on request. Headers X-RateLimit-Remaining/Reset. Use AdaptiveLimiter for sync flows.
- Architecture: Each path is a NestJS controller; date-versioned modules. Spec title is "Cal.diy" - same surface, OSS variant.

## Product Thesis
- Name: cal-com-pp-cli (slug cal-com)
- Why it should exist: Today the only credible CLI is bcharleson/calcom-cli - a TypeScript wrapper that's good but stateless. Every read hits the API; rate limits bite; there's no offline mode, no SQL, no time-window aggregation across bookings, no "who's about to no-show," no conflict detection across multiple calendars. Our CLI absorbs every command bcharleson ships and adds a local SQLite layer that unlocks queries the API can't answer at all (or can only answer with N+1 spam).

## Build Priorities
1. Generate from official OpenAPI (82 paths, 121 ops) - gives us 1:1 parity with bcharleson at zero hand-build cost.
2. Local SQLite + FTS for bookings, event_types, schedules, webhooks, teams, OOO, verified resources. Sync + reconcile.
3. Novel commands: load-by-day, conflict detection across linked calendars, no-show risk, attendee follow-up, schedule gap finder, reschedule replay, cancel sweep, slot fanout, attendee history.
4. MCP server (auto-mirrored from Cobra tree by the generator).
