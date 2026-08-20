# Sprocket Sports CLI — Absorb Manifest

## Landscape
No existing Sprocket Sports CLI/SDK/MCP server/plugin exists (confirmed via search;
all "Sprocket" repos are unrelated — esports bots, Rails asset pipeline, HubSpot's
framework). Nothing to absorb from competing *tools*. Parity is benchmarked against
general youth-sports apps (TeamSnap, SportsEngine, GameChanger, Spond) for table
stakes. Differentiation comes entirely from the transcendence set.

## Absorbed (table-stakes read surface — all generator-emitted from the spec)
| # | Feature | Peer Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Schedule/calendar by date window | TeamSnap/SportsEngine | (generated endpoint) schedule list | date-window POST, --json/--select, offline-able |
| 2 | Event detail (opponent, location, home/away, type) | TeamSnap event | (behavior in sprocket-pp-cli schedule list) clubCalendarEvent fields | structured, scriptable |
| 3 | List all club teams | SportsEngine directory | (generated endpoint) teams list | full ~120-team directory |
| 4 | My players' team assignments | TeamSnap my-teams | (generated endpoint) teams mine | per-player |
| 5 | List your players | GameChanger roster | (generated endpoint) players list | |
| 6 | Family/household | TeamSnap household | (generated endpoint) family get | |
| 7 | Programs/seasons | SportsEngine programs | (generated endpoint) programs list | |
| 8 | Open registration programs | SportsEngine registration | (generated endpoint) programs open | |
| 9 | Registration history (balances) | SportsEngine | (generated endpoint) registrations completed | |
| 10 | Team registration history | SportsEngine | (generated endpoint) registrations team | |
| 11 | Dues / overdue invoices | TeamSnap payments | (generated endpoint) payments overdue-invoices | |
| 12 | Overdue tickets / failed payments | TeamSnap payments | (generated endpoint) payments overdue-tickets, payments failed | |
| 13 | Account profile / roles / clubs | all | (generated endpoint) account me, account roles, account clubs | |
| 14 | Event-type lookup | calendar filter | (generated endpoint) schedule event-types | |
| 15 | Club info / settings | club site | (generated endpoint) club info, club settings | |

## Transcendence (only possible with our approach — all hand-code Phase 3)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | This-week schedule | week | hand-code | week-boundary date-math over the calendar window, merged across all my players' teams in one view (dashboard shows one team at a time) | Use this command for the current calendar week across all your players' teams. Do NOT use it for an arbitrary day range (use `agenda --days`) or just the next item (use `next`). |
| 2 | Next event | next | hand-code | min(start) over future events across all my teams — a single cross-team answer no one screen gives | Use this command for the one next event on the family calendar. Do NOT use it for a multi-event list; use `week` or `agenda`. |
| 3 | Family agenda (merged) | agenda --days N | hand-code | merges two children's separate team schedules into one chronological agenda — impossible in the per-team SPA | Use this command for the merged multi-child schedule over a date range. Do NOT use it for the single next event (`next`), the fixed current week (`week`), or conflict detection alone (`conflicts`). |
| 4 | Conflict finder | conflicts --days N | hand-code | pairwise time-overlap + tight time/location-gap detection across all players' events | Use this command to find double-bookings and impossible back-to-backs across players. Do NOT use it to list the schedule; use `agenda` or `week`. |
| 5 | Away-game drive list | away --weeks N | hand-code | filters awayGame + resolves location into a location-first drive list — the home/away content pattern, packaged for the carpool chat | Use this command for away games with their fields and opponents. Do NOT use it for home games or the full schedule; use `week` or `agenda`. |
| 6 | iCal export | ical [--days N] | hand-code | serializes merged events to RFC-5545 .ics for phone-calendar subscription — no remote endpoint emits calendar files | none |
| 7 | Changed-since-sync | since | hand-code | diffs a stored prior snapshot against a fresh fetch → added/moved/cancelled; the dashboard has no "what changed" view | Use this command for schedule deltas since your last sync. Do NOT use it for the current schedule; use `week` or `agenda`. |
| 8 | Outstanding balance roll-up | owed | hand-code | joins registration remainingBalance with overdue invoices into one cross-player total that exists in neither call | Use this command for total money owed across all players. Do NOT use it for registration history detail (`registrations completed`) or the raw invoice list (`payments overdue-invoices`). |
| 9 | Registration deadline radar | deadlines | hand-code | sorts/filters open programs by close-date math, flagging those closing within N days | Use this command for upcoming registration close dates. Do NOT use it to list all open programs unsorted; use `programs open`. |

### Dropped vs the brief (honest scope notes)
- **Standings/record** and **announcements/messages** were proposed and CUT: those
  endpoints are NOT in the parent-dashboard API surface we sniffed (standings is
  league-admin-level; the parent view doesn't expose it). Building them would mean
  fabricating endpoints. Out of scope for v1 — would require admin/league access.

## Data-source strategy
Schedule transcendence commands are `pp:data-source live` (fetch the calendar POST
window, compute in Go). `since` is hybrid (live fetch + stored prior snapshot).
`owed` joins two live GETs. `deadlines` sorts the live open-programs list. No
auto-sync of the POST calendar into SQLite is required.

## Stubs
None. All 9 transcendence rows are shipping scope (hand-code in Phase 3).
