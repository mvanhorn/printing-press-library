# Google Calendar — Novel Features Brainstorm (audit trail)

## Customer model

**Bruno — terminal-living founder/operator (gcalcli refugee).**
- Today: Runs `gcalcli agenda` and `calw 2` every morning; juggles work, personal, and two shared client calendars. Every command hits the API — flaky connection or quota spike stalls his morning.
- Weekly ritual: Monday planning — pulls the whole week across all four calendars, eyeballs for double-bookings, blocks focus time in gaps.
- Frustration: No tool tells him "you're booked twice at 2pm Thursday" or "where's my next 90-minute free block?" without manual scanning, offline.

**Mira — AI scheduling agent author.**
- Today: Builds an assistant that books meetings. Shells out to a CLI because raw OAuth + recurrence expansion is painful per project.
- Weekly ritual: Agent runs dozens of times/day: "find a free 30-min slot across 3 calendars," "create event, on conflict abort." Parses `--json`, branches on exit codes.
- Frustration: Existing CLIs return human text, no typed exits, no `--dry-run`. Conflict detection means pulling all events and diffing herself — burns quota and latency.

**Sofia — ops lead managing shared/team calendars.**
- Today: Owns ACL on shared client calendars; fields "who can see this?" and "what changed since Friday?"
- Weekly ritual: Friday review — reconciles what moved/cancelled/added across team calendars; audits sharing rules before onboarding a contractor.
- Frustration: Google UI has no "diff since last week" and no flat ACL audit across calendars.

## Candidates (pre-cut)
(16 candidates generated; see survivors + kills below for verdicts)

## Survivors and kills

### Survivors (transcendence)

| # | Feature | Command | Score | Buildability |
|---|---------|---------|-------|--------------|
| 1 | Free-slot finder | `free --calendars a,b,c --window <range> --duration 90m` | 9/10 | hand-code |
| 2 | Cross-calendar conflict detection | `conflicts --calendars a,b --window <range>` | 9/10 | hand-code |
| 3 | What-changed-since | `changes --since <date> [--calendar x]` | 8/10 | hand-code |
| 4 | Meeting-load analytics | `load --window <range> [--group-by day\|week\|calendar] [--split allday,timed]` | 7/10 | hand-code |
| 5 | ACL audit across calendars | `acl-audit [--role reader\|writer\|owner]` | 6/10 | hand-code |
| 6 | Conflict-guarded create | `events insert ... --on-conflict abort\|warn` | 7/10 | hand-code |
| 7 | Attendee/RSVP rollup | `rsvp-status --window <range>` | 6/10 | hand-code |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Offline agenda/week/month grid | Table-stakes absorb, not novel; offline property folded into survivors | Free-slot finder |
| Offline recurring-instance expander | Reimplementation kill — API endpoint already ships; hand-rolling RRULE risky | What-changed-since |
| Next free slot (single answer) | Strict subset of free-slot finder | Free-slot finder |
| Double-booking watch (poll loop) | Scope creep toward background process | Cross-calendar conflict detection |
| Travel-time gap checker | External-service kill — needs maps/distance API | Cross-calendar conflict detection |
| Color-coded calendar legend | Cosmetic, no user pain | (none) |
| All-day vs timed split report | Thin slice — folded into load as `--split` | Meeting-load analytics |
| Stale/never-synced calendar check | Niche operator-debug, low weekly use | What-changed-since |
