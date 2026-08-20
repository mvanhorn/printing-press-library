# Sprocket Sports — Novel Features Brainstorm (subagent audit trail)

## Customer model

**Dana — the travel-team logistics parent (primary).**
- Today: one kid on a JFC travel team, another in a younger rec program. Logs in weekly, reads the team calendar game-by-game, juggles two tabs to see both kids, reconciles in her head/Notes. Can't answer "what's our family's full schedule Saturday?" without hand-merging two calendars; can't see what changed since last look.
- Weekly ritual: Sunday night — read the coming week per child, copy away addresses to maps, text carpool parents.
- Frustration: merging two children's schedules into one chronological agenda and spotting collisions is entirely manual; double-bookings get discovered Saturday morning.

**Marcus — the carpool/away-game coordinator parent.**
- Today: owns away-game driving; scrolls for the awayGame flag, opens each away event for the field address, copies into Maps one at a time. No away-only/location-first view; re-does it whenever the schedule changes.
- Weekly ritual: Monday — pull next two weeks of away games, build a drive list (date, opponent, field), post to the group chat.
- Frustration: no away-only location-first export; rebuilds the same drive list weekly; changes are invisible without re-reading everything.

**Priya — the registration-and-dues account manager parent.**
- Today: manages money for two kids; open-registration windows are buried; dues live in a separate view from registration history, so she cross-references balances vs overdue invoices herself; no single outstanding-total number.
- Weekly ritual: during season transitions, check for newly-open programs and any overdue invoice, then pay/flag.
- Frustration: outstanding money is split across two screens with no combined total and no proactive deadline signal; she finds out late.

## Survivors (transcendence set — all hand-code, all >= 5/10)

| # | Feature | Command | Score | Build | Persona | How It Works |
|---|---------|---------|-------|-------|---------|--------------|
| 1 | This-week schedule | `week` | 9/10 | hand-code | Dana | current Mon-Sun window across all my-team events, ordered by start |
| 2 | Next event | `next` | 9/10 | hand-code | Dana | min(start) over future events across all my teams |
| 3 | Family agenda (merged) | `agenda --days <n>` | 8/10 | hand-code | Dana | merge every player's schedule into one chronological N-day list |
| 4 | Conflict finder | `conflicts --days <n>` | 8/10 | hand-code | Dana | pairwise time-overlap + tight time/location-gap detection across players |
| 5 | Away-game drive list | `away --weeks <n>` | 8/10 | hand-code | Marcus | away games only, location-first, opponent + field + date |
| 6 | iCal export | `ical [--days <n>]` | 7/10 | hand-code | Dana | serialize merged events to RFC-5545 .ics |
| 7 | Changed-since-sync | `since` | 7/10 | hand-code | Marcus | diff current vs prior snapshot → added/moved/cancelled |
| 8 | Outstanding balance roll-up | `owed` | 7/10 | hand-code | Priya | join registration remainingBalance + overdue invoices → one total |
| 9 | Registration deadline radar | `deadlines` | 6/10 | hand-code | Priya | sort/filter open programs by close-date, flag closing within N days |

## Killed candidates

| Feature | Kill reason | Sibling |
|---------|-------------|---------|
| `roster <player>` | thin rename of `teams mine`; one endpoint, no join/date-math | `away` |
| `record` (standings) | standings NOT in confirmed spec surface; would fabricate an endpoint | `week` |
| `messages` (announcements) | announcements NOT in confirmed spec surface; would fabricate an endpoint | `since` |
| `practices` | single clubEventTypeID flag on `week`/`agenda`; insufficient leverage | `week` |
| `search "<opp>"` | already covered by generated framework `search` | `agenda` |
| `fields` directory | slowly-changing reference list, not weekly; value absorbed into `away` | `away` |
| `next --quiet --select` cron line | not a distinct command; just `next` + global flags | `next` |

## Implementation note (added by orchestrator)
The calendar is a POST endpoint with a required {start,end} window that returns
events already scoped to the user's teams. So the schedule transcendence commands
(`week`/`next`/`agenda`/`away`/`conflicts`/`ical`) are implemented **`pp:data-source live`**:
fetch the right date window via the calendar POST, then date-math / merge / filter
in Go. `since` is hybrid (live fetch + a stored prior snapshot to diff). `owed`
joins two live GET calls (registrations + payments). `deadlines` sorts live
`programs open`. None require auto-syncing the POST calendar into SQLite.
