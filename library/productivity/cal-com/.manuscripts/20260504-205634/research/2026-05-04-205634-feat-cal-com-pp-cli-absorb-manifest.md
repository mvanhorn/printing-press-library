# Cal.com CLI — Absorb Manifest

## Summary

- **Endpoint mirror:** All 186 Cal.com v2 endpoints across 21 resource buckets become typed Cobra commands and MCP tools at generation time. Every feature any prior tool ships is matched.
- **Best competing tools:** `calcom/cal-mcp` (167 cmds, official MCP), `bcharleson/calcom-cli` (61), `dsddet/booking_chest` (70 Python), three smaller MCPs (12/12/8/4).
- **Transcendence:** 10 features the local store + composed-intent layer unlock, none of which any prior tool ships.

## Absorbed (match or beat everything that exists)

The Printing Press emits an endpoint-mirror surface for every spec endpoint, so the absorb table is bucket-level rather than row-per-endpoint:

| # | Resource bucket | # endpoints | Best Source | Our Implementation | Added Value |
|---|-----------------|-------------|-------------|-------------------|-------------|
| 1 | bookings (list / get / create / cancel / reschedule / mark-no-show / attendees / guests) | 20 | calcom/cal-mcp, bcharleson/calcom-cli | Endpoint-mirror Cobra+MCP; cached to local SQLite via `sync` | `--json --select`, FTS5 search, typed exit codes, dry-run on mutations |
| 2 | event-types (list / get / create / update / delete + ownership filters) | 6 | calcom/cal-mcp | Endpoint-mirror; cached to store | offline `event-types stale`, `--json --select` |
| 3 | slots (list-available / reserve / release) | 3 | calcom/cal-mcp | Endpoint-mirror + composed `slots find` | cross-event-type fan-out |
| 4 | schedules (list / get / create / update / delete) | 3 | calcom/cal-mcp | Endpoint-mirror; cached to store | powers `gaps` |
| 5 | calendars (cal-unified / destination / selected / busy-times / connect) | 11 | calcom/cal-mcp | Endpoint-mirror | powers `conflicts` |
| 6 | webhooks (event-type-scoped + global) | 2 | calcom/cal-mcp, bcharleson | Endpoint-mirror | powers `webhooks coverage` |
| 7 | conferencing (Zoom / Google / Office365 connect + default-app) | 7 | calcom/cal-mcp | Endpoint-mirror | uniform `--json` |
| 8 | api-keys (list / create / refresh / delete) | 4 | calcom/cal-mcp | Endpoint-mirror | `--dry-run` on creates |
| 9 | oauth-clients + OAuth2 + managed-users | 7 | calcom/cal-mcp | Endpoint-mirror | typed exit codes for token flows |
| 10 | verified-resources (email / phone) | 8 | calcom/cal-mcp | Endpoint-mirror | uniform output |
| 11 | teams (list / get / create / update / delete + memberships + event-types) | 22 | calcom/cal-mcp, bcharleson | Endpoint-mirror; team_memberships cached | powers `workload` |
| 12 | organizations (orgs/attributes, orgs/teams, orgs/memberships, orgs/roles, orgs/routing-forms, orgs/schedules, orgs/delegation-credentials, orgs/bookings) | 80 | calcom/cal-mcp | Endpoint-mirror | full org admin coverage |
| 13 | me (get / update + secondary emails) | 4 | every prior tool | Endpoint-mirror | first-run identity for `doctor` |
| 14 | misc (auth / oauth / credits / stripe / notifications / routing-forms / destination-calendars / selected-calendars) | 9 | calcom/cal-mcp | Endpoint-mirror | uniform output |

**Framework features the generator emits for every CLI** (matching/exceeding incumbents):
- Local SQLite store with FTS5 indexes; `sync` / `search` / `stale` / `sql` / `reconcile` / `orphans` / `load` / `export` / `import`
- `--json` / `--select` (dotted paths) / `--csv` / `--quiet` / `--compact` on every command
- `--dry-run` on mutations
- Typed exit codes (0/2/3/4/5/7/10)
- `doctor` health check
- `auth set-token` / `auth status` / `auth logout`
- MCP server (stdio, optional http)
- Workflow scenarios + `workflow run` / `workflow analytics`

## Transcendence (only possible with our approach)

| # | Feature | Command | Persona | Score | Why Only We Can Do This |
|---|---------|---------|---------|-------|--------------------------|
| 1 | One-shot booking | `book` | operator, agent | 9/10 | Cal.com forces a 4-call dance (slots/available → reservations → bookings → confirm). We compose the chain transactionally with `--dry-run` and roll back on partial failure. `// pp:client-call` |
| 2 | Unified agenda | `agenda --window today\|week\|<dur>` | operator, agent | 8/10 | Local SQLite join on `bookings` + `event_types` + attendees; no API call after sync. Default `--window today`; agents stay cheap on context |
| 3 | Cross-event-type slot search | `slots find` | operator, agent | 8/10 | Cal.com `/slots/available` accepts only one event-type per call. We fan out per event-type ID, merge, dedup, rank by start. `// pp:client-call` |
| 4 | Booking analytics | `analytics` | team lead | 9/10 | Cal.com Insights is paid-tier. We compute volume / density / no-show rate / cancel rate from local SQLite; group by event-type / attendee / weekday / hour |
| 5 | Conflict detection | `conflicts` | operator, team lead | 7/10 | Joins local `bookings` with `/v2/calendars/busy-times` over a window. The data exists in two API surfaces but no endpoint joins them |
| 6 | Availability gap finder | `gaps` | operator | 7/10 | Joins local `schedules` (availability windows) with `bookings`; reports open intervals ≥ `--min-minutes`. No API filter exists |
| 7 | Team workload balance | `workload` | team lead | 7/10 | Joins `bookings.hostId` → `team_memberships` locally; per-host count + duration distribution. No team-workload endpoint |
| 8 | Webhook coverage | `webhooks coverage` | integrator | 8/10 | Compares `/v2/webhooks` registered triggers vs a curated canonical-trigger table; reports lifecycle stages with no subscriber. The constant table ships inline (`// pp:novel-static-reference`) |
| 9 | Stale event types | `event-types stale` | operator | 6/10 | Left-join local `event_types` against `bookings` filtered by `--days`; returns zero-booking event types. No analogous API filter |
| 10 | Reschedule to next open slot | `reschedule next` | operator | 7/10 | Composed `/v2/bookings/{uid}` → `/v2/slots/available` → `/v2/bookings/{uid}/reschedule` to the first match after `--after`. Always supports `--dry-run`. `// pp:client-call` |

### Reprint reconciliation (audit trail)

The 12 prior novel features (printing-press 2.3.9) were re-evaluated against current personas:
- **9 kept verbatim:** `book`, `slots find`, `analytics`, `conflicts`, `gaps`, `workload`, `webhooks coverage`, `event-types stale`, plus the keep on `today`+`week` (now reframed)
- **2 reframed:** `today` and `week` collapsed into `agenda --window` (one parameterized command instead of two parallel handlers)
- **2 dropped:** `bookings pending` (reimplementation of `bookings list --status=pending` — fold `--max-age`+age-sort flags onto the endpoint command instead) and `webhooks triggers` (static reference inlined into `webhooks coverage`)
- **1 added:** `reschedule next` (third composed intent beside `book` and `slots find`; addresses operator's last-minute-bump scenario)

## MCP Surface Strategy

Total tool count at startup: ~196 (186 endpoints + 10 transcendence + framework). Above the 50-tool threshold. Default endpoint-mirror MCP burns agent context.

**Recommended spec enrichment before generation (Cloudflare pattern):**

```yaml
mcp:
  transport: [stdio, http]    # remote-capable; reaches hosted agents
  orchestration: code         # thin <api>_search + <api>_execute pair
  endpoint_tools: hidden      # suppress raw per-endpoint mirrors
  intents:
    - name: book
      description: "Find an open slot and book it in one transactional flow with --dry-run support"
    - name: agenda
      description: "Show upcoming bookings within a time window from the local store"
    - name: slots_find
      description: "Cross-event-type slot search ranked by start time"
    - name: reschedule_next
      description: "Move an existing booking to the next available slot"
```

The spec enrichment lives at the source level so all generated artifacts (server, manifest, README) emit the right surface from the start.

## Follow-up additions (post-publish, on top of PR #237)

After the initial 10-feature transcendence pack shipped, a product-shape review surfaced that `book` was misaligned with the host persona that actually holds the API key — an attendee schedules themselves through the booking link; the host doesn't normally script others into their calendar. The absorb manifest's persona analysis was correct but missed pressure-testing whether the included features actually serve those personas day-to-day. Four additions land the host's primary creative + scheduling-control surface, and `book` was reframed to make its scripting-an-attendee-onto-my-calendar use case explicit:

| # | Feature | Command | Persona | Why we added it |
|---|---------|---------|---------|-----------------|
| 11 | Bookable link create | `link create` | host (operator, team lead) | The host's primary creative act on Cal.com. "How do I create a booking link" is the natural host question; it should resolve to a command literally named `link create`. Wraps POST /v2/event-types with sensible defaults + URL render via /v2/me. |
| 12 | Bookable link list | `link list` | host | Companion to create: see what links you've published with full URLs pre-rendered for copy-share. |
| 13 | Out-of-office set | `ooo set` | host | Endpoint mirror exposes this as `me user-ooocontroller-create-my-ooo` — unergonomic for what is a primary host scheduling-control workflow. Wraps POST /v2/me/ooo with natural-language times and reason enum. |
| 14 | Out-of-office list | `ooo list` | host | Confirm OOO entries are recorded; find the IDs to cancel. (`ooo delete` exists too but isn't promoted as a novel feature.) |

`book` was also reframed (Short + Long) to clarify it's the host scripting an attendee onto their calendar (admin onboarding, recruiter pre-fill, test fixtures) — not the create-a-link workflow that `link create` now serves.

## Stubs

None. Every transcendence row is shipping-scope.
