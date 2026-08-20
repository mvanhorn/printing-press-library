# Copper CLI — Novel Features Brainstorm (audit trail)

Customer model: Dana (RevOps analyst), Marcus (sales manager), Priya (account manager, quarter-close), Sam (integration engineer). See brief Top Workflows.

7 survivors (all hand-code), ~half cut. Killed: forecast-by-owner (→flag), stale-by-rep (→flag), bulk-reassign-on-stale (→flag), bulk-activity-log (→flag), stage-velocity (verifiability: no stage-entry timestamp), pipeline-health-snapshot (convenience aggregate), date-range-sharded-export (belongs in sync), untouched-leads-triage (→stale --type lead).

## Survivors
1. forecast (9/10) — weighted pipeline forecast: SUM(monetary_value × win_probability) over open opps grouped by stage/assignee/close-month.
2. stale (9/10) — stale-deal sweep: open opps with date_last_contacted older than N days, sorted by staleness×value.
3. bulk (9/10) — bulk write engine: single-record PUTs across many opps with bounded concurrency + heuristic 429 backoff (solves no-bulk-endpoint gap).
4. upsert (8/10) — idempotent upsert: fetch-by-match then create-or-update; normalizes people.emails[] vs leads.email.
5. dedupe (7/10) — duplicate finder: local SQLite self-join on email/name/company.
6. log (7/10) — activity log + correct: name-resolved activity create; `log fix` delete+recreate for immutable activities.
7. who (7/10) — contact-graph lookup: local join opp→company→people→recent activities into one view.
