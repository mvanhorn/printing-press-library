# Google Calendar CLI Brief (gcal-pp-cli)

## API Identity
- **Domain:** Google Calendar API v3 — official, OAuth2, spec at apis.guru (37 operations; calendars, events, freebusy, calendarList, ACL, settings, colors, channels).
- **Users:** (1) **the assistant** — the operator's always-on personal-assistant agent (launchd bridge + scheduled sweeps + desk sessions); invokes via JSON; requires freshness evidence, structural safety (must be *unable* to notify third parties), idempotent mutations. (2) **the operator** — the sole human; touches the CLI only at setup (OAuth consents) and via the assistant. (3) **the builder** — the agent-specialist who builds/tests it. This is an **internal-tool print**: correctness and safety outrank breadth.
- **Data profile:** 3 Google accounts with per-account roles (1 read-only by OAuth scope, 2 writable), governed by a operator-approved `calendars.yaml` manifest keyed by stable calendar IDs; events windows queried live.

## Reachability Risk
- None — official Google API. All endpoints require OAuth (401 without token is expected/correct).

## Top Workflows
1. **Cross-account conflict sweep** under the plan's verdict contract: every manifest source read fresh-or-downgrade ("checked N of M"), tz-normalized, busy-only (tentative=busy, declined/free excluded), API-side recurrence expansion, same-title/time mirror flagging, all-day handled separately.
2. **Freebusy check** ("is Thursday 2–4 open?") across all manifest calendars.
3. **Merged agenda** across accounts for a window.
4. **Create event** on the default write calendar (the assistant echoes-with-undo upstream).
5. **Move/delete attendee-less events** on writable calendars (the assistant confirm-gates upstream).
6. **Manifest reconciliation** — live discovery vs `calendars.yaml`: unmanifested calendar appearing, or manifested one missing/permission-lost → finding, not silent drift.

## Table Stakes (from gcalcli / khal / calcurse / nylas landscape)
- Absorb: calendars list, windowed events list, agenda view, event create/update/delete, freebusy, search within window, auth status/doctor.
- **Deliberately NOT absorbed** (internal-tool scope; the assistant is the NLP/notification layer): TUI month/week views, ICS import, reminder daemons, quick-add natural-language parsing, contacts integration.

## Data Layer — DELIBERATE PRESS-DEFAULT INVERSION
- **No synced event store as a source of truth.** The consuming plan (the assistant build plan §4, grill R5-C5) forbids verdicts from caches: a fetch-stamped stale copy recreates the family's stale-ledger failure. All event/freebusy reads are **live**, and output carries upstream evidence (`updated`, etag) + a `fetched_at` stamp per source.
- Local state limited to: OAuth token store per profile (configurable dir), profile/role config, and a client-generated-ID helper for idempotent creates. SQLite sync/FTS machinery: **omit**.

## Codebase Intelligence
- Prior art verified 2026-08-17 (planning session): nspady/google-calendar-mcp ships multi-account + cross-account conflict detection (validates demand); bakissation/mcp-google-multi does multi-account OAuth switching. Auth shape: installed-app OAuth, per-account token files, refresh via golang.org/x/oauth2/google.
- Google specifics: `events.insert` accepts **client-supplied event IDs** (base32hex) — the idempotency lever; `sendUpdates` param on insert/update/delete controls attendee email — **must be hard-coded `none`**; `singleEvents=true` + `timeMin/timeMax` gives API-side recurrence expansion; calendarList vs calendars distinction; `updated` + etag on responses = freshness evidence.

## User Vision
- From the ratified the assistant build plan (grilled 5 rounds): multi-account named profiles with per-role scopes (readonly account CANNOT write *by token*), forced `sendUpdates=none` (not a flag — a structural barrier), no Gmail surface in v1, freshness evidence on every read, agent-friendly JSON. Token/config default dir `~/.config/google-calendar-pp-cli/gauth/`.

## Product Thesis
- **Name:** gcal-pp-cli (slug `google-calendar`).
- **Why it should exist:** every existing Google Calendar tool assumes one account and a human at the keyboard. This is the calendar layer for an *agent* whose hard rule is "never surprise a human": role-scoped multi-account OAuth in one binary, a conflicts engine implementing a reviewed verdict contract, mutations that structurally cannot email attendees, and freshness you can cite.

## Build Priorities
1. Auth: profiles, per-role scopes, installed-app flow, token store, `auth`/`accounts`/`doctor`.
2. Reads: `calendars`, `events list` (windowed, singleEvents), `agenda` (merged), `freebusy`.
3. **`conflicts`** — the verdict-contract engine (the transcendence centerpiece).
4. Mutations: `events create/update/delete` — client IDs, forced `sendUpdates=none`, attendee-bearing guard (refuse to mutate events with attendees).
5. `manifest check` — calendars.yaml reconciliation.
