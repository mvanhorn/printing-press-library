# Cal.com CLI Brief

## API Identity
- **Domain:** scheduling and bookings — calendar-driven invite links, availability windows, booking management, and team scheduling
- **Users:** sales reps, recruiters, founders, consultants, customer-success teams, and developers integrating booking flows; agents acting on a user's calendar
- **Data profile:** 186 OpenAPI v2 endpoints across 21 resource buckets, dominated by orgs (80), teams (22), bookings (20), calendars (11). 4 HTTP verbs, 124 GET / 81 POST / 45 DELETE / 38 PATCH / 3 PUT. Per-resource API-version pinning via `cal-api-version` header on 46 endpoints (e.g. `bookings` use `2024-08-13`, `event-types` use `2024-06-14`)

## Reachability Risk
- **None.** Live probe confirmed: /v2/me, /v2/event-types, /v2/bookings all 200. Test account has 16 bookings + 3 event types — usable fixtures across the matrix.

## Top Workflows
1. **Find an open slot and book it.** End-user flow: pick an event type, request slots for a window, book the chosen slot. Today this is a 3-call dance (GET /event-types, GET /slots, POST /bookings). One-shot booking is the headline.
2. **See what's on my calendar today / this week.** Cached view of upcoming bookings with attendees, status, meeting links — fastest answer is from a local store, not a live call.
3. **Manage event types.** Create/clone/edit bookable links, toggle visibility, change duration/price/locations. Power users iterate on these constantly.
4. **Manage availability schedules.** Tweak working hours, copy schedules between event types, audit overlap with external calendars.
5. **Reconcile bookings against external calendars.** Detect double-bookings, surface gaps, see team workload. The Cal.com API exposes the data; no incumbent puts it together.
6. **Webhooks lifecycle.** Discover registered triggers, audit which events have no subscriber, reconcile against the canonical trigger catalog.

## Table Stakes (the bar to match)
The Cal.com universe today: official `calcom/cal-mcp` (167 tools, 21 stars, TypeScript), `bcharleson/calcom-cli` (61 commands), `aditzel/caldotcom-api-v2-sdk` (12), `vinayh/calcom-mcp` (12), `dsddet/booking_chest` (Python, 70 cmds). Combined coverage:
- Full CRUD on event types, bookings, schedules, calendars, webhooks, teams
- OAuth client management + token refresh
- Slot search by event type / username / time window
- Booking status mutations (cancel, reschedule, mark no-show)
- Org/team admin (memberships, roles, attributes, delegated credentials)
- Verified resources (email, phone)
- Conferencing connections (Zoom, Google Meet, Office365)

The official MCP is the strongest direct competitor. Beating it requires (a) full feature parity (every endpoint reachable), (b) offline + agent-native output (`--json`, `--select`, typed exit codes), (c) novel features only possible with a local store.

## Data Layer
Sync these primary entities:
- **bookings** (id, status, start/end, eventTypeId, attendeeEmails, meetingUrl, hostId, source) — sync cursor: `updatedAt`. The hot table for analytics, conflict detection, agendas.
- **event_types** (id, slug, title, length, locations, hidden, schedulingType, hosts, price). Small, refreshed on demand.
- **schedules** (id, name, timeZone, availability windows). Powers gap-finder, copy-schedule.
- **users** + **team_memberships** (id, name, email, role, teamId). Powers team-workload.
- **webhooks** (id, subscriberUrl, eventTriggers, active). Powers webhook coverage audit.
- **slots_cache** (eventTypeId, start, end, capturedAt). Optional warm cache for slot search.

FTS5 indexes on bookings (title + attendees + notes), event_types (title + slug + description), users (name + email).

## Codebase Intelligence
- **Auth quirk worth remembering:** the OpenAPI spec has NO `components.securitySchemes` block. Every operation declares `Authorization: Bearer <token>` as a regular header parameter. The pre-generation auth-enrichment step (Phase 2) MUST inject bearer auth before generation, otherwise the parser will default to `type: none` and the generated client will skip bearer entirely.
- **Token format:** `cal_live_*` for live keys, `cal_test_*` for test, plus managed-user access tokens and OAuth tokens — all share the same `Bearer ` prefix.
- **API versioning:** The `cal-api-version` header is set per-endpoint (46 endpoints carry it). The generator's client should default the header from the spec's `parameters[].example` and let it be overridden via flag/env. `bookings` ⇒ `2024-08-13`; `event-types` ⇒ `2024-06-14`.
- **Pagination:** Cal.com returns a `pagination` envelope with `currentPage`, `totalPages`, `hasNextPage`. The generator's pagination-aware list emit should detect this shape.
- **Response envelope:** Every response is `{ status: "success" | "error", data: ... }` with the body of interest under `.data`. README/SKILL examples and `--select` should account for this.

## User Vision
- "fully use [the API] including clicking on links without worry of production data" — user wants the CLI exercised end-to-end against the live test account. Phase 5 should be Full Dogfood, not Quick.
- Carry-forward themes from prior 12 novel features (Pass 2(d) input to the novel-features subagent): one-shot booking, today's agenda, week view, cross-event-type slot search, booking analytics, conflict detection, availability gap finder, team workload, webhook coverage, stale event types, pending review, webhook trigger catalog. The subagent re-evaluates these against current rubrics.

## Product Thesis
- **Name:** `cal-com-pp-cli`
- **Why it exists:** The official Cal.com MCP and the open-source CLIs all wrap the API endpoint-by-endpoint. None of them combine the calendar into a queryable local store. That's the unlock — once bookings, event types, schedules, and team data live in SQLite together, you can answer questions no single API call answers: "what's on my plate this week", "where am I overbooked", "which event types are stale", "who on my team is overloaded", "which webhook triggers are subscribed". Plus offline searches/SQL/agent-native output the others lack.

## Build Priorities
1. **Generator's endpoint mirror** for all 186 paths with bearer auth wired correctly (auth-enrichment + per-endpoint API-version pinning).
2. **Local store** for bookings, event_types, schedules, users, webhooks; sync command that pulls everything; FTS5 search.
3. **Transcendence pack** — the novel-feature subagent picks the final list; carry-forwards include `today`, `week`, `book` (one-shot), `slots search` (cross-event-type), `analytics`, `conflict-check`, `gaps`, `team workload`, `webhook coverage`, `stale-event-types`, `pending`, `webhook trigger catalog`.
4. **MCP surface tuning** — 186 endpoints + ~12 novel commands ⇒ ~200 tools at startup. Default endpoint-mirror surface scores poorly here. Default to the Cloudflare pattern: `mcp.transport: [stdio, http]`, `mcp.orchestration: code`, `mcp.endpoint_tools: hidden`. Surface the high-leverage workflows as named `mcp.intents` (book, today, week, conflicts).
