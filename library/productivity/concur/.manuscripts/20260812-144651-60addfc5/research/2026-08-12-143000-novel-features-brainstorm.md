# Concur Novel Features Brainstorm (audit trail)

## Customer model

**1. Marcus, the Road-Warrior Consultant**
Marcus spends 60% of his time on-site, juggling client site visits, car
rentals, and hotel stays across three regions. Today, he keeps a physical
folder of receipts and manually transcribes them into Concur web forms on
Sundays.
His weekly ritual involves sitting down at 8:00 PM on Sunday, fighting the
slow Concur web UI to link receipts to card charges, applying specific
client-project codes, and ensuring no expense exceeds the regional policy.
His frustration is the "lost Sunday" cycle: manual data entry, clicking
through endless modals to fix category mismatches, and fearing he missed
a receipt attachment, resulting in rejected reports.

**2. Elena, the Project Lead**
Elena manages a team of 10 and approves reports weekly. Today, she relies
on notification emails, which often get buried. She has to log into
Concur, search for the reports, and manually cross-reference the business
purpose of each expense to ensure it aligns with project budget
allocations.
Her weekly ritual is a Thursday morning purge of her approval queue,
where she manually checks line items for "reasonable business purpose"
and ensures the project code is correct.
Her frustration is the lack of summary visibility. She can't see the "big
picture" of a report's budget impact without clicking into each
individual expense, making the approval process feel like a blind audit.

**3. Sarah, the High-Frequency Traveler**
Sarah does short, high-frequency trips (3-4 per month). She doesn't have
"reports" in the traditional long-term sense; she generates one report
per trip. She constantly forgets to link her "Available Expenses" to the
correct report, leading to a massive backlog in her queue.
Her weekly ritual is fighting the "Available Expenses" list to
drag-and-drop charges into new reports, then trying to remember which
expense belongs to which trip.
Her frustration is the UI friction. It's hard to reconcile her calendar
(where she sees the trip) with the Concur "Available Expenses" list
(where she sees a mess of charges).

## Candidates (pre-cut)

| # | Feature | Command | Description | Persona | Source |
|---|---|---|---|---|---|
| 1 | Report Dashboard | `reports summary --id` | Shows report budget vs actuals, status, and missing receipts. | Elena | (c) |
| 2 | Auto-link Expenses | `available-expenses link-to-trip` | Uses travel calendar/trips to auto-map available expenses to report IDs. | Sarah | (e) |
| 3 | Policy Checker | `reports validate <id>` | Local dry-run of policy/business rules before submission. | Marcus | (f) |
| 4 | Expense Bulk Tagging | `expenses tag --category --project` | Batch apply client-project codes to unfiled expenses. | Marcus | (a) |
| 5 | Receipt Inbox Sync | `receipts scan <path>` | Local OCR/upload bridge for faster attachment to expense rows. | Marcus | (a) |
| 6 | Approval Digest | `approvals list --summary` | Aggregated report list with budget flags for quick approval. | Elena | (a) |
| 7 | Trip-to-Expense Bridge | `trips reconcile --trip <id>` | Lists available expenses that align with specific trip dates. | Sarah | (c) |
| 8 | Rule-set Audit | `expense-types rules --verbose` | Dumps business logic and cap limits for all types in a policy. | Marcus | (b) |
| 9 | Duplicate Detector | `expenses scan-duplicates` | Finds potential double-entry of charges in large reports. | Marcus | (f) |
| 10 | Delegate Session Audit | `delegates session-audit` | Lists actions taken on-behalf-of others to prevent misuse. | Elena | (b) |

Kill/Keep checks applied inline: (5) killed — external OCR dependency: too
much maintenance overhead for a generated CLI. (10) killed — security
theater; Concur's own audit trail is the authoritative source, a local
CLI log would be incomplete and misleading.

## Survivors and kills

### Survivors

| Feature | Weekly Use | Leverage | Transcendence | Score | Buildability (subagent) | Buildability (corrected) |
|---|---|---|---|---|---|---|
| reports validate | High | Wrapper | Local dry-run catching errors before remote submission | 9/10 | spec-emits | **hand-code** — cross-references expense-type rules + form-field requirements + report state, same shape as prior-art's `apply_expense_type_rules` |
| trips reconcile | High | Transcendence | Bridges calendar/trip dates with unfiled receipts | 8/10 | hand-code | hand-code |
| available-expenses link-to-trip | High | Transcendence | Automated mapping reduces manual drag-and-drop | 8/10 | hand-code | hand-code |
| approvals list --summary | Medium | Wrapper | Quick-scan budget status for managers | 7/10 | spec-emits | **hand-code** — aggregation/flagging across a report's expenses, not a single generated endpoint |
| expenses scan-duplicates | Medium | Transcendence | Cross-report duplicate check impossible in standard UI | 7/10 | hand-code | hand-code |
| expenses tag | Medium | Wrapper | Bulk category/project tagging across unfiled expenses | 6/10 | spec-emits | **hand-code** — loops over multiple expense IDs in one invocation |

**Correction note:** the subagent tagged three survivors `spec-emits`; the
orchestrating agent (this run) reclassified all three to `hand-code` after
review — each requires either a local cross-table join, business-rule
evaluation, or a batch loop, none of which a single generated
endpoint-mirror command produces. All 6 survivors are hand-code. This
changes the Phase Gate 1.5 commitment from "3 hand-code, 3 spec-emits" to
"6 hand-code, 0 spec-emits" among the transcendence features. Absorbed
(table-stakes) features remain spec-emits/generated per Step 1.5b.

### Killed candidates

| Candidate | Reason for Kill |
|---|---|
| Receipt Inbox Sync | External OCR dependency creates too much maintenance overhead for a generated CLI. |
| Delegate Session Audit | High false-signal risk; Concur's own audit logs are the authoritative source, not a local CLI log. |
| Report Dashboard | Scope creep — largely covered by `reports get` plus `--select`/`--json` client-side parsing. |
| Rule-set Audit | Low utility relative to `reports validate`, which surfaces the same info actionably. |
