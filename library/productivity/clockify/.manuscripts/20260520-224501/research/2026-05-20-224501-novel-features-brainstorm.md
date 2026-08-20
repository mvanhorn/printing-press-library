# Clockify CLI — Novel Features Brainstorm (audit trail)

## Customer model

### Persona 1 — Dana, the agency timesheet-filler
- **Today:** UX designer at a 20-person agency. Every Friday opens `app.clockify.me/timesheet`, reconstructs the week from Slack/calendar/git. Cannot answer "did I log all 40 hours?" or "which days am I short?" without manually summing grid cells.
- **Weekly ritual:** Friday timesheet reconstruction — fill every cell, eyeball totals, Submit for Approval.
- **Frustration:** The grid shows what she entered, never what she missed. Gaps surface only when the total comes up short.

### Persona 2 — Marcus, the freelance consultant chasing billable hours
- **Today:** Independent backend consultant billing three clients. Month-end web report → CSV → hand-built invoice. Cannot answer "how much unbilled billable time is sitting in entries?" or "which billable entries have no project?"
- **Weekly ritual:** Friday billable-hours review — sum billable time per client, sanity-check tagging/projecting, decide what's invoice-ready.
- **Frustration:** Revenue leakage from misfiled entries — a billable entry with no project silently drops off the invoice.

### Persona 3 — Priya, the team lead who approves timesheets
- **Today:** Manages a 6-person delivery team. Weekly approvals queue, clicks into each person's timesheet. Cannot answer "whose timesheet is still unsubmitted?" without clicking through every member.
- **Weekly ritual:** Monday approval sweep — review every submitted week, approve/reject, chase non-submitters.
- **Frustration:** The queue shows only what was submitted; non-submitters are invisible.

## Killed candidates
| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C3 Timesheet submit-for-approval | Thin wrapper of approval-request POST; becomes a `--submit` flag on the gap-checked week flow | #1 `timesheet week --submit` |
| C8 Team week overview | Near-duplicate of team submission tracker; per-member columns fold in | #4 `team timesheets` |
| C9 Approval triage | Sanity flags are the same per-member annotations the tracker needs | #4 `team timesheets` |
| C10 Smart entry from last week | Whole-week clone duplicates absorbed templates (#26) and duplicate (#10) | #1 `timesheet week` |
| C11 Untracked-days streak | A zero-hour day is the degenerate case of a gap | #2 `timesheet gaps` |
| C14 Time-off balance check | Thin wrapper, no join; covered by absorbed typed commands (#28) | #1 `timesheet week` |
| C15 Rejected-timesheet rework list | Thin wrapper over a filtered approval-requests GET | #4 `team timesheets` |

(See absorb manifest for the surviving transcendence table.)
