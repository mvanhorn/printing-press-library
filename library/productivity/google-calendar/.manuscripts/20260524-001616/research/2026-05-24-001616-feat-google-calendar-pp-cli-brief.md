# Google Calendar CLI Brief

## API Identity
- Domain: Calendar/scheduling. Google Calendar API v3 — calendars, events, ACLs, free/busy, colors, settings, push channels.
- Base URL: `https://www.googleapis.com/calendar/v3`
- Users: developers, power users living in the terminal, scheduling automations, AI agents booking/reading time.
- Data profile: events (recurring + instances), calendars, calendarList (subscriptions), ACL rules, settings, colors, free/busy intervals.
- Spec: official Google-published OpenAPI (via apis-guru), 22 endpoints — the canonical complete surface.

## Reachability Risk
- None. Official Google API. OAuth2 required for all meaningful operations (no plain API key for user data). Live smoke testing skipped this run (no OAuth creds provided) — CLI still ships full OAuth2 support.

## Top Workflows
1. **See my day/week** — agenda for a time range across one or many calendars (gcalcli `agenda`/`calw`).
2. **Quick-add an event** — natural language ("Lunch with Sam tomorrow 1pm") via `events.quickAdd`.
3. **Find free time** — `freeBusy.query` across multiple calendars in one call to find an open slot.
4. **Create/edit structured events** — attendees, reminders, conference (Meet), recurrence.
5. **Manage recurring events** — edit "this / following / all" instances correctly.

## Table Stakes (must match the incumbents)
- gcalcli surface: `agenda`, `calw` (N-week grid), `calm` (month grid), `quick`, `add`, `edit`, `delete`, `import` (ICS), `search`, `list` (calendars), `updates` (changes since), `remind`.
- MCP (nspady) surface: list-calendars, list/get/search/create/update/delete events, respond-to-event (RSVP), get-freebusy, list-colors, get-current-time, multi-account, conflict detection.
- Full CRUD on every spec resource: calendars, calendarList, events (+ import/quickAdd/move/instances), acl, settings, colors, freeBusy, channels.

## Data Layer
- Primary entities: events, calendars, calendarList, acl_rules, settings, colors.
- Sync cursor: **`nextSyncToken`** per-calendar incremental sync (events.list). Expired token → 410 GONE → clear + full resync. Deleted entries always included in incremental results.
- FTS/search: full-text over event summary/description/location/attendees; offline once synced.

## Codebase Intelligence
- Auth: OAuth2 (implicit + authorizationCode flows). Scopes: `calendar`, `calendar.events`, `calendar.events.readonly`, `calendar.readonly`, `calendar.settings.readonly`. Generated CLI will use authorization-code flow with local token storage + refresh.
- Quota: per-user + per-calendar caps; `rateLimitExceeded` returns 403 or 429 → exponential backoff. `patch` costs more quota than `update`.
- Notable endpoints: `quickAdd` (NL single-event), `freeBusy` (batch busy query), `events.move` (between calendars), `events.instances` (expand recurrence), `watch`/`channels.stop` (push notifications).

## User Vision
- (none provided — user chose "Let's go")

## Product Thesis
- Name: **gcal-pp** (Google Calendar CLI) — "Every gcalcli/MCP feature, plus a local SQLite mirror that makes free-time search, conflict detection, and 'what changed' instant and offline."
- Why it should exist: existing CLIs are live-only (every query hits the API, slow, quota-bound, no history). Putting events in SQLite with incremental sync tokens unlocks offline agenda, cross-calendar conflict detection, free-slot search, and time-windowed "what changed" — things no live-only tool can do. Agent-native output (`--json`, `--select`, typed exit codes, `--dry-run`) makes it the right tool for AI scheduling agents.

## Build Priorities
1. **P0 Data layer** — events/calendars/calendarList/acl/settings/colors tables, FTS over events, incremental sync via `nextSyncToken` (410 → full resync), `sync` + `search` + `sql`.
2. **P1 Absorb** — full CRUD on all 22 endpoints; gcalcli's agenda/week-grid/month-grid/quick/add/edit/delete/import/search/updates; MCP's RSVP/freebusy/colors.
3. **P2 Transcend** — local-join features impossible for live-only tools: free-slot finder, cross-calendar conflict detection, agenda from local store, "what changed since", busiest-day/load analytics, recurring-instance expander offline.
