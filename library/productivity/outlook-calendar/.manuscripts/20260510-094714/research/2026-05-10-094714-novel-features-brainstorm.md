# Outlook Calendar — Novel Features Brainstorm

## Customer model

**Persona 1 — Maya, indie consultant juggling two personal Microsoft 365 accounts**
- **Today (without this CLI):** Maya keeps Outlook web open in two browser profiles, one for each MSA. She manually scans the week each Sunday night to spot double-bookings between the personal-life calendar and the consulting calendar that share her body. When an agent (Claude) helps her plan, she copy-pastes screenshots of her week into chat because no CLI talks to personal Microsoft 365.
- **Weekly ritual:** Sunday-night triage — open both calendars, eyeball overlaps, accept/decline the week's invites, block focus time, send a recap to herself.
- **Frustration:** Cross-calendar conflict detection. Outlook web shows one calendar at a time on the relevant view; overlaps between her two calendars are invisible until the morning-of collision.

**Persona 2 — Devin, agent-driven knowledge worker on a personal MSA**
- **Today (without this CLI):** Devin runs Claude Code as a daily driver and wants the agent to "look at my calendar, prep me for my 2pm, draft an agenda." Today the agent has no read access to his Outlook calendar (every existing CLI/MCP demands a work tenant), so he pastes a screenshot or types the agenda from memory.
- **Weekly ritual:** Each morning he asks an agent to brief him on the day; each Friday he asks it to find two-hour deep-work blocks for next week.
- **Frustration:** Personal MSA lockout — every "Outlook MCP" he's tried fails on `AzureADandPersonalMicrosoftAccount` and falls back to Google Calendar examples. The agent simply cannot see his calendar.

**Persona 3 — Priya, recovering meeting-overloaded PM who runs her side projects on a personal MSA**
- **Today (without this CLI):** Priya gets dozens of recurring-meeting series invites that mutate (single-instance reschedules, organizer-side time shifts). She finds out about drift only when she joins a Teams call at the wrong hour. Outlook desktop's recurring view doesn't show which instances drifted from the master.
- **Weekly ritual:** Friday afternoon "what changed" pass — scroll the week, look for re-times, look for cancellations she missed, mark un-responded invites.
- **Frustration:** Silent change-set. Outlook surfaces no "what changed since I last looked" view, no list of invites she still hasn't RSVP'd, and no diff of recurring instances against their master.

**Persona 4 — Tomás, founder using an agent to schedule across his team's free/busy without a work tenant**
- **Today (without this CLI):** Tomás's three contractors share availability over email screenshots because his MSA can't `getSchedule` against their work tenants and there's no shared calendar. He hand-rolls a "free for all" hour by reading three weekly threads.
- **Weekly ritual:** Monday morning find-a-time across self + a small set of self-only views; daily, pull the next 24h with attendee context for his standup.
- **Frustration:** Self-only `getSchedule` on personal MSA is degraded; he needs a richer local-data substitute (his own free time across his own calendars, with working-hours awareness).

## Candidates (pre-cut)

(see brainstorm output — preserved verbatim above)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Cross-calendar conflicts | `conflicts --since today --until +7d --json` | 9/10 | SQL self-join on synced `events` (start_utc/end_utc) across all user calendars where intervals overlap and the row pair is not the same id; outputs collision pairs as JSON | Brief Build Priorities #4 explicitly lists `conflicts`; Maya persona; absorb manifest has no overlap detection |
| 2 | Free-time finder | `freetime --duration 60m --within "Mon-Fri 9-17" --next 7d --exclude-oof` | 9/10 | Walks merged busy-intervals from local `events` (incl. recurrence instances cached during sync), subtracts from working-hours window, returns gaps ≥ duration; honors `showAs` to exclude tentative/oof on demand | Brief Build Priorities #4; Tomás + Maya personas; `getSchedule` is degraded on personal MSA (brief codebase intel) — local-data is the only viable substitute |
| 3 | Weekly review (change set) | `review --since last-sync --json` | 8/10 | Compares pre-sync row snapshots (subject/start/end/responseStatus/cancelled flag) against post-`events/delta` rows; emits added / rescheduled / cancelled / rsvp-changed buckets | Delta-sync is cited in brief Codebase Intelligence + Sync cursor; Priya persona; absorb manifest #30/#31 (delta) supplies the cursor but no diffing UX |
| 4 | Stale invites | `stale --json` | 7/10 | Local SELECT where `responseStatus.response = 'none'` and `start_utc > now()` ordered by start; one row per organizer-sent meeting awaiting RSVP | Brief Build Priorities #4 names `stale`; Priya persona; no Graph endpoint surfaces this filter directly |
| 5 | Recurring drift | `recurring-drift --json` | 7/10 | For each series master, fetch instances via cached `/me/events/{id}/instances` and compare each instance's start/end/subject/location to the master pattern projection; emit divergent instances | Brief Build Priorities #4 names `recurring-drift`; Priya persona; absorb manifest #19 lists raw instance list but not drift detection |
| 6 | Meeting prep dossier | `prep --next 4h --json` | 7/10 | Local SELECT for events in `[now, now+4h]` joined to attendees + attachments-meta + body_preview, with recurrence/online-meeting flags added | Brief Build Priorities #4 names `prep`; Devin persona; agent-shaped output is the brief's stated thesis |
| 7 | Attendee co-occurrence | `with --who alice@example.com --since 90d --json` | 6/10 | Local FTS / attendee-table SELECT where `attendees.email = ?` AND `start_utc >= now()-interval` returning count, last_seen, recent N | Devin + Maya personas; brief Data Layer mentions FTS over attendee emails/names; no Graph endpoint aggregates per-attendee history |
| 8 | Time-zone audit | `tz-audit --json` | 5/10 | Local SELECT where `start_tz != calendar.default_tz` OR `start_tz != end_tz`, surfacing rows likely to render wrong on devices | Brief Common quirks (TZ Prefer header); Maya/Priya frustrations; absorb manifest does not cover this |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C7 Daily brief | Subset of `prep` once `--day today` is allowed; would duplicate the dossier query | C6 `prep` |
| C8 Auto-tag from rules | Write-side rule engine is scope creep, brittle to verify in dogfood | (none — out of scope) |
| C11 Focus-time enforcer | Creates events; collapses to absorbed `events create` plus a `freetime` query the user already has | C2 `freetime` |
| C12 Teams URL extractor | Thin renaming of `events get --select onlineMeeting` — fails wrapper-vs-leverage check | absorb #3 (`events get`) |
| C13 OOF window detector | Standalone command duplicates a `freetime` filter; folded into C2 as `--exclude-oof` | C2 `freetime` |
| C14 ICS export | Reimplementation risk (hand-rolled RFC 5545 serializer); user can pipe `--json` to a converter | absorb #1 (`events list --json`) |
| C15 Self-only findtime | Overlaps `freetime`; raw `findMeetingTimes` endpoint is already in the absorb manifest | C2 `freetime` (+ absorb #21) |
| C16 RSVP digest | Useful but a one-line aggregation over `stale` + accepted/tentative buckets — folds into `stale --group-by status` | C4 `stale` |
