## Customer model

**1. Priya — SaaS founder embedding Cal.com**
- Today: curl + jq debugging; web app for cancels. Rate-limits self mid-debug.
- Weekly: Mon scan last week's bookings for no-shows + reschedule rate; Fri audit cancellations by event-type.
- Frustration: No "show bookings where attendee rescheduled twice" query.

**2. Marco — Freelance consultant, 1-person ops**
- Today: Lives in web UI; manually scans for back-to-back bookings + paid-prospect no-shows.
- Weekly: Sunday night Notion copy of next week's bookings, manual repeat-vs-new annotation.
- Frustration: No attendee history across bookings.

**3. Devi — RevOps at 40-person SaaS, AE team scheduling**
- Today: bcharleson CLI + Python scripts paginating /bookings + CSV join.
- Weekly: Mon "team load + no-show risk" VP report; Wed sweep unconfirmed >48h.
- Frustration: No cross-team attendee-keyed query; hits 120 req/min ceiling.

**4. Sam — Eng lead running customer interviews**
- Today: 4 Google calendars + 1 iCloud connected; double-booked twice this quarter.
- Weekly: Fri export next week's interview slots to PMs.
- Frustration: No calendar-conflict-vs-booking query across providers.

## Candidates (pre-cut)

(See subagent output - 16 candidates, 8 cut, 8 survive)

## Survivors and kills

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| 1 | Conflict scan across linked calendars | `cal-com conflicts --window 7d` | 9/10 | Joins local calendars/busy multi-provider rows against bookings; emits overlap with offending calendar+title | Brief thesis names "conflict detection"; bcharleson lacks; Sam pain |
| 2 | No-show risk by attendee | `cal-com no-show-risk --since 30d` | 8/10 | Local aggregation over bookings grouped by attendee email; cancel_rate, no_show_rate, total_count | Brief names "who's about to no-show"; no API endpoint; Devi+Priya |
| 3 | Attendee history | `cal-com attendee <email> [--summary]` | 8/10 | Joins bookings × attendees × event_types filtered by email; --summary = GROUP BY | Brief Build Pri #3; Marco; no API endpoint |
| 4 | Schedule gap finder | `cal-com gaps --min 30m --window 7d` | 7/10 | Walks schedules availability windows minus bookings time-ranges; emits gaps >= min | Brief Build Pri #3; Marco/Sam ritual |
| 5 | Reschedule replay | `cal-com reschedule-history <uid>` | 7/10 | Recursive walk of rescheduledFromUid pointers; ordered timeline | Brief Build Pri #3; Priya debug |
| 6 | Cancel sweep | `cal-com cancel-sweep --status PENDING --older-than 48h [--apply]` | 7/10 | Local SQL pre-filter; --apply loops /bookings/{uid}/cancel with --dry-run default | Brief Build Pri #3; Devi Wed |
| 7 | Host load report | `cal-com host-load --week 2026-W20` | 6/10 | Local GROUP BY host with counts/hours/cancel-rate/no-show-rate | Devi VP report; no API aggregation |
| 8 | Load-by-day | `cal-com load-day <date>` | 5/10 | /bookings windowed call + upsert + delta line vs prior sync | Brief Build Pri #3; Priya debug |

### Killed candidates

| Feature | Kill reason | Sibling |
|---------|-------------|---------|
| Event-type funnel | Reimplementation risk; collapses to status counts | Host load (#7) |
| Booking search FTS | Duplicates framework auto-emit search | n/a |
| Webhook replay | Reconstructing signed payloads; fake-payload risk; no replay endpoint | n/a |
| Calendar-conflict-explain | Merged into conflict scan per-row | Conflict scan (#1) |
| Booking-export --by-attendee | Overlap | Attendee history (#3) --summary flag |
| Rate-limit-aware sync | Framework table stakes | n/a |
| Slot-fanout | Borderline wrapper; no clear ritual | Schedule gap finder (#4) |
| Availability-diff | Niche Sam-only | Conflict scan (#1) |
