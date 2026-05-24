# Google Calendar CLI — Absorb Manifest

Spec: official Google Calendar v3 OpenAPI (22 endpoints). Auth: OAuth2 (authorization-code). Competitors: gcalcli (incumbent), nspady google-calendar-mcp, gcal-cli, neocal, gcsa.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List events (time range) | gcalcli agenda / MCP list-events | `events list` + local-backed `agenda` | Offline via SQLite, `--json`/`--select`, typed exits |
| 2 | Get event | MCP get-event | `events get` | `--select` field narrowing |
| 3 | Create structured event | gcalcli add / MCP create-event | `events insert` (attendees, reminders, recurrence, conference) | `--dry-run`, `--stdin` batch |
| 4 | Quick-add (natural language) | gcalcli quick / MCP | `events quickAdd --text` | scriptable, JSON out |
| 5 | Update/patch event | gcalcli edit / MCP update-event | `events patch` / `events update` | `--dry-run`, idempotent patch |
| 6 | Delete event | gcalcli delete / MCP delete-event | `events delete` | `--dry-run` |
| 7 | RSVP / respond to invite | MCP respond-to-event | `events respond --status accepted` | patches attendee responseStatus for self |
| 8 | Import ICS/iCal | gcalcli import | `events import` | from file or `--stdin` |
| 9 | Move event between calendars | events.move endpoint | `events move` | — |
| 10 | Expand recurring instances | events.instances endpoint | `events instances` | offline expansion (transcend variant) |
| 11 | Search events (text) | gcalcli search / MCP search-events | FTS5 `search` | offline, regex, SQL-composable |
| 12 | List calendars | gcalcli list / MCP list-calendars | `calendar-list list` | offline |
| 13 | Get/insert/update/delete calendar | spec calendars.* | `calendars get/insert/patch/delete/clear` | full CRUD |
| 14 | CalendarList CRUD (subscriptions) | spec calendarList.* | `calendar-list get/insert/patch/delete` | full CRUD |
| 15 | ACL rules CRUD (sharing) | spec acl.* | `acl list/get/insert/patch/delete` | full CRUD |
| 16 | Free/busy query (batch) | MCP get-freebusy | `freebusy query` | one call across N calendars |
| 17 | Colors | MCP list-colors | `colors get` | — |
| 18 | Settings | spec settings.* | `settings list/get` | offline |
| 19 | Push channels stop | spec channels.stop | `channels stop` | — |
| 20 | Watch (push notifications) | spec *.watch | `events/acl/calendar-list/settings watch` | — |
| 21 | Incremental sync | (no competitor — API syncToken) | `sync` w/ nextSyncToken, 410→full resync | foundation for all offline features |
| 22 | Current time helper | MCP get-current-time | `now` (tz-aware) | agent convenience |
| 23 | Multi-account / multi-calendar | gcalcli / MCP multi-account | `--calendar` selectors, default-calendar config | — |
| 24 | SQL over local store | (none) | `sql` SELECT-only | composable analytics |

Notes: items 1-20 map to spec endpoints the generator auto-emits as typed commands; `agenda`, `respond`, `now`, FTS `search`, `sync`, and `sql` are framework/hand-built. No stubs planned — all shipping scope.

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Score | Why Only We Can Do This |
|---|---------|---------|--------------|-------|-------------------------|
| 1 | Free-slot finder | `free --calendars a,b,c --window <range> --duration 90m` | hand-code | 9 | Inverts busy intervals from the local events store into free gaps ≥ duration, offline across N calendars; live `freeBusy` only reports busy |
| 2 | Cross-calendar conflict detection | `conflicts --calendars a,b --window <range>` | hand-code | 9 | Self-join over local events on overlapping start/end across calendars; no Google endpoint returns cross-calendar overlaps |
| 3 | What-changed-since | `changes --since <date> [--calendar x]` | hand-code | 8 | Reads locally persisted sync deltas (created/updated/cancelled, deletions always included) for a historical window; impossible live-only |
| 4 | Meeting-load analytics | `load --window <range> [--group-by day\|week\|calendar] [--split allday,timed]` | hand-code | 7 | SQLite GROUP BY over local events: count + summed booked hours per bucket |
| 5 | ACL audit across calendars | `acl-audit [--role reader\|writer\|owner]` | hand-code | 6 | Joins local calendarList + acl_rules into one flat who-has-what-access table; Google UI has no cross-calendar sharing view |
| 6 | Conflict-guarded create | `events insert ... --on-conflict abort\|warn` | hand-code | 7 | Runs local overlap query before the insert API call; typed non-zero exit on conflict (agent-shaped write guard) |
| 7 | Attendee/RSVP rollup | `rsvp-status --window <range>` | hand-code | 6 | Counts accepted/declined/tentative per event from stored attendee arrays; mechanical, no LLM |

All 7 are `hand-code` (post-generate Go + root.go wiring). All shipping scope, no stubs.

Full brainstorm (personas, all 16 candidates, kills): see `2026-05-24-001616-novel-features-brainstorm.md`.
