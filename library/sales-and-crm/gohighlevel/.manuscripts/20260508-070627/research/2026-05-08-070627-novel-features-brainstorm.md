# Novel Features Brainstorm — gohighlevel

(Preserved from Phase 1.5c.5 subagent output. Customer model + killed candidates kept here as audit trail; survivors are promoted into the absorb manifest.)

## Customer model

**Maya — Agency Operator (50-sub-account agency)**
- Today: opens 50 GHL tabs every morning to scan unread + new leads; maintains a hand-rolled "client health" Google Sheet on Mondays.
- Weekly ritual: Monday standup wants stale-opportunity counts, conversation response-time outliers, and unpaid-invoice totals across all 50 locations on one screen.
- Frustration: every "where is X across all my locations" question requires opening 50 tabs. MCP servers are tool-by-tool wrappers; they don't compose.

**Jordan — Sales Ops at a Single-Location SaaS**
- Today: lives in the GHL pipeline kanban; hand-counts opportunities stuck in stages >14 days every Monday and Slacks reps.
- Weekly ritual: Tuesday pipeline review wants stage-by-stage velocity, average time-in-stage, oldest opps per stage, and opps with no activity in N days.
- Frustration: pays $300/mo for an Airtable/Zapier mirror just to print this report.

**Priya — Conversation Triage Lead, 10 Locations**
- Today: 4-person SDR team, 30-minute first-response SLA, no tooling to flag breaches before clients complain.
- Weekly ritual: end-of-day SLA audit walking each location's conversation list to tally response-time breaches.
- Frustration: needs "which conversations across all 10 locations are unanswered for >30 minutes during business hours, assigned to my team" — a join across conversations + users + locations + business_hours that no tool today does.

**Sam — Reseller Migration Engineer**
- Today: writes throwaway Python per migration; hand-rolls rate-limit backoff every time.
- Weekly ritual: 2 migrations/week — bulk contact create, bulk tag apply, bulk custom-field set, then reconcile against source.
- Frustration: no tool ships idempotent batch ops + reconcile; wants `ghl contacts create --stdin --idempotency-key email --dry-run`.

## Killed candidates (audit trail)

| Feature | Kill reason | Closest survivor |
|---|---|---|
| conversation-fresh | Subset of sla-breach without business-hours flag | sla-breach |
| invoice-aging | Single-table aggregation; folds into roster metric | roster |
| calendar-day | Thin date-filter; absorbed-row level | absorbed `calendars events list --on today` |
| contact-graph | Resembles GHL UI 360 view; better as `--include` on `contacts get` | absorbed `contacts get --include ...` |
| rate-budget | GHL headers unreliable; verifiability weak | (dropped) |
| roster-diff | User-space `diff` of two `roster --json` runs is sufficient | roster |
| bulk-stage | Narrower than bulk-tag; high risk; defer | bulk-tag pattern |

