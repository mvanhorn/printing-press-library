# Cal.com CLI — Novel Features (Reprint)

## Customer model

The CLI serves four overlapping personas, all of whom either own a Cal.com account or act on one through an agent:

1. **The operator** — sales rep, recruiter, founder, or consultant who lives in their booking inbox. Wakes up wanting today's agenda, sweeps pending confirmations, reschedules under pressure, and runs end-of-quarter cleanups on dead event types.
2. **The team lead** — manager of a round-robin or collective event type. Cares about workload distribution, no-show rates by attendee, cancellation patterns, and whether webhook automation actually fires for every lifecycle stage.
3. **The integrator/developer** — wires Cal.com into a CRM, Slack bot, or internal tool. Needs reliable webhook coverage, correct trigger constants, and the ability to dry-run a booking flow before pushing to production.
4. **The agent** — Claude or another model acting on the user's calendar. Wants one-shot composed intents (`book`, `reschedule`), offline agenda reads to keep context cheap, and predictable JSON.

Across all four, the unifying need is **answers the API can't give in a single call** — agendas, conflicts, gaps, workload, analytics — plus **safe composed mutations** (`book`, `reschedule`) the API forces you to choreograph by hand.

## Candidates (pre-cut)

| # | Candidate | Source | Notes |
|---|-----------|--------|-------|
| 1 | One-shot booking (`book`) | prior | Slots-available -> reserve -> create -> confirm in one call with `--dry-run` |
| 2 | Today's agenda (`today`) | prior | Local-store agenda for today |
| 3 | Week view (`week`) | prior | 7-day rollup with conflict highlighting |
| 4 | Cross-event-type slot search (`slots find`) | prior | Fan out across multiple event types, ranked by start |
| 5 | Booking analytics (`analytics`) | prior | Volume, density, no-show rate, cancel rate, grouped by event-type/attendee/weekday/hour |
| 6 | Conflict detection (`conflicts`) | prior | Bookings vs external calendar busy-times |
| 7 | Availability gap finder (`gaps`) | prior | Open windows in your schedule, filtered by min block size |
| 8 | Team workload balance (`workload`) | prior | Booking distribution across team members |
| 9 | Webhook coverage (`webhooks coverage`) | prior | Registered triggers vs canonical set |
| 10 | Stale event types (`event-types stale`) | prior | Event types with zero bookings in N days |
| 11 | Pending review (`bookings pending`) | prior | Pending-confirmation bookings sorted by age |
| 12 | Webhook trigger catalog (`webhooks triggers`) | prior | Static reference of all valid trigger constants |
| 13 | Unified agenda (`agenda --window`) | new | Collapses `today` + `week` into one window-parameterized command |
| 14 | Reschedule to next open slot (`reschedule next`) | new | Composed: cancel-or-reschedule current booking + slots find + book new time |
| 15 | Booking heatmap (`heatmap`) | new | Day-of-week x hour density grid from the local store |
| 16 | Calendar coverage audit (`calendars audit`) | new | Selected vs destination calendars vs event-type defaults — find disconnected wiring |
| 17 | No-show sweep (`bookings noshow-sweep`) | new | Find and mark every past booking with no attendance signal |
| 18 | Webhook replay-ready dump (`webhooks dump`) | new | Latest delivery payloads from local store, keyed by trigger, for offline handler dev |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Persona | Buildability proof |
|---|---------|---------|-------|---------|---------------------|
| 1 | One-shot booking | `book` | 9/10 | operator, agent | Composes `/v2/slots/available` → `/v2/slots/reservations` → `/v2/bookings` → `/v2/bookings/{uid}/confirm`; `// pp:client-call` |
| 2 | Unified agenda | `agenda --window today\|week\|<dur>` | 8/10 | operator, agent | Local SQLite join on `bookings` + `event_types` + attendees; default `--window today`; no API call after sync |
| 3 | Cross-event-type slot search | `slots find` | 8/10 | operator, agent | Fan-out `/v2/slots/available` per event-type ID, merge + dedup + rank; `// pp:client-call` |
| 4 | Booking analytics | `analytics` | 9/10 | team lead | SQL aggregation over local `bookings`; volume/density/no-show/cancel grouped by event-type / attendee / weekday / hour |
| 5 | Conflict detection | `conflicts` | 7/10 | operator, team lead | Joins active `bookings` (Cal.com) with `/v2/calendars/busy-times` for overlap rows |
| 6 | Availability gap finder | `gaps` | 7/10 | operator | Joins local `schedules` with `bookings` to compute open intervals ≥ `--min-minutes` |
| 7 | Team workload balance | `workload` | 7/10 | team lead | Joins `bookings.hostId` → `team_memberships` locally; per-host count + duration distribution over `--window` |
| 8 | Webhook coverage | `webhooks coverage` | 8/10 | integrator | Compares `/v2/webhooks` triggers vs canonical curated trigger constants; `// pp:novel-static-reference` for the constants |
| 9 | Stale event types | `event-types stale` | 6/10 | operator | Left-join local `event_types` against `bookings` filtered by `--days`; zero-booking event types |
| 10 | Reschedule to next open slot | `reschedule next` | 7/10 | operator | Composed `/v2/bookings/{uid}` + `/v2/slots/available` + `/v2/bookings/{uid}/reschedule`; `// pp:client-call` |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| `today` (separate) | Collapsed into `agenda --window today` (parallel-helpers drift) | `agenda` |
| `week` (separate) | Collapsed into `agenda --window 7d` | `agenda` |
| `bookings pending` | Reimplementation of native `bookings list --status=pending`; fold `--max-age` + age-sort onto the endpoint-mirror command | `bookings list` |
| `webhooks triggers` | Static constant table with no agent-decision value standalone; inline constants into `webhooks coverage` | `webhooks coverage` |
| Booking heatmap (`heatmap`) | Strict subset of `analytics --by weekday-hour` | `analytics` |
| Calendars audit (`calendars audit`) | Implementation requires four API surfaces + per-account heuristic; verifiability poor | — |
| No-show sweep (`bookings noshow-sweep`) | Thin wrapper over `/v2/bookings/{uid}/no-show`; discovery half folded into `analytics --metric no-show` | `analytics` |
| Webhooks dump (`webhooks dump`) | Cal.com API does not expose delivery payloads; would require local receiver — scope creep | — |

## Reprint verdicts

| # | Prior feature | Prior command | Verdict | Note |
|---|---------------|---------------|---------|------|
| 1 | One-shot booking | `book` | **Keep** | Strongest single feature; prior command reused |
| 2 | Today's agenda | `today` | **Reframe** | Folded into `agenda --window today`; eliminates parallel handler with `week` |
| 3 | Week view | `week` | **Reframe** | Folded into `agenda --window 7d`; same data lens at a different window |
| 4 | Cross-event-type slot search | `slots find` | **Keep** | Distinct from `book`; agents need slot discovery without committing |
| 5 | Booking analytics | `analytics` | **Keep** | Headline local-store feature; no Cal.com equivalent without paid Insights |
| 6 | Conflict detection | `conflicts` | **Keep** | Two-source join nothing else does |
| 7 | Availability gap finder | `gaps` | **Keep** | Distinct lens from agenda — answers "when can I take a meeting" not "what is on my plate" |
| 8 | Team workload balance | `workload` | **Keep** | Team-lead persona's primary command |
| 9 | Webhook coverage | `webhooks coverage` | **Keep** | Integrator persona's primary command; absorbs the trigger-catalog data inline |
| 10 | Stale event types | `event-types stale` | **Keep** | Quarterly-cleanup workflow; cheap to compute, high signal |
| 11 | Pending review | `bookings pending` | **Drop** | Reimplementation of `bookings list --status=pending` |
| 12 | Webhook trigger catalog | `webhooks triggers` | **Drop** | Static constant table; inline into `webhooks coverage` |

**One new feature:** `reschedule next` — composed reschedule-to-next-open-slot, third "composed intent" alongside `book` and `slots find`. Targets the operator persona's most common under-pressure scenario (last-minute bump).

---

## Post-publish additions (on top of PR #237)

A product-shape review after the initial 10 features shipped surfaced that the brainstorm's persona analysis was correct (host-shaped: operator, team lead, integrator, agent) but missed pressure-testing whether each included feature actually serves the host day-to-day. Specifically:

- **`book` was misaligned.** The bearer token authenticates the HOST. The host doesn't normally script attendees onto their own calendar — attendees schedule themselves through the bookable link. `book`'s real use cases (admin onboarding, recruiter pre-fill, test fixtures) are real but rare. Action: keep `book` for the rare scripting use case, but reframe its Short/Long to set expectations correctly, and add the headline-host commands that were missing.

- **The host's primary creative act was missing.** Creating bookable links (event types) IS the host's main creative move on Cal.com. Endpoint mirror exposed this as `event-types create`, which leaks the API's noun-vocabulary into the user's surface. Action: add `link create` and `link list` as the host-shaped alias with the bookable URL pre-rendered.

- **OOO is a primary host scheduling-control workflow** that the endpoint mirror exposes only via `me user-ooocontroller-create-my-ooo` (auto-derived from Cal.com's NestJS controller class names — not human-friendly). Action: add `ooo set` / `ooo list` / `ooo delete` as ergonomic wrappers.

### Final survivor count after follow-up: 14

| # | Feature | Command | Persona | Source |
|---|---------|---------|---------|--------|
| 11 | Bookable link create | `link create` | host | new (post-publish) |
| 12 | Bookable link list | `link list` | host | new (post-publish) |
| 13 | Out-of-office set | `ooo set` | host | new (post-publish, ergonomic alias for verbose endpoint mirror) |
| 14 | Out-of-office list | `ooo list` | host | new (post-publish, ergonomic alias) |

`ooo delete` ships as a child of the `ooo` parent but isn't promoted as a novel feature in its own right (follows obvious pattern from `set`).

### Lesson for next reprint

The persona-driven brainstorm correctly identified host as the dominant persona, but didn't ask the secondary question: "for each feature in the survivor list, does the named persona actually run this weekly?" `book`'s answer is "rarely" once you separate scripting from the natural attendee-driven booking flow. The pre-publish triage should add a "weekly use" sanity check that's specifically about whoever holds the API key — not an abstract "is this a real use case."
