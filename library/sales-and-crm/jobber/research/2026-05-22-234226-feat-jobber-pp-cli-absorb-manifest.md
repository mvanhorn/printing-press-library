# Jobber CLI — Absorb Manifest

## Context

- API: Jobber GraphQL (single endpoint `POST /api/graphql`)
- 18 verified read-only root surfaces from 2026-05-15 introspection
- Read-only by user mandate (no mutations); discovery_plan.md guardrail
- Competitor MCPs: `flutchai/mcp-server-jobber` (10 tools), Zapier Jobber MCP (~8 actions), viaSocket (0 actions live), Pipedream `@pipedream/jobber`. No dedicated CLI exists.

## Absorbed (match or beat every existing read-only feature)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|--------------------|-------------|--------|
| 1 | List clients with filters | flutchai list_clients; Zapier Find Client | `clients list` mirrors `ClientFilterAttributes` (isCompany, isLead, isArchived, updatedAt, createdAt, tags) | Offline after sync, `--json`/`--csv`/`--select`, FTS5 across name/email/address | ship |
| 2 | Get client detail | flutchai get_client; Zapier Find Client | `clients get <id>` with `--expand` for properties/jobs/invoices/quotes/requests | Local store join; no N+1 API calls | ship |
| 3 | List properties | (no competitor) | `properties list` with `PropertiesFilterAttributes` (clientId, primary) | First offline view of property data | ship |
| 4 | List requests | (no competitor) | `requests list` with `RequestFilterAttributes` (clientId, propertyId, status, updatedAt, createdAt, assignedTo) | First offline view of pre-quote stage | ship |
| 5 | List quotes with filters | flutchai list_quotes | `quotes list` with `QuoteFilterAttributes` (clientId, quoteNumber, status, cost, sentAt, updatedAt, createdAt, salespersonId) | Offline + funnel state filters | ship |
| 6 | List jobs with filters | flutchai list_jobs | `jobs list` with `JobFilterAttributes` (jobType, status, startAt, completedAt, visitsScheduledBetween, visitsAssignedToUserId, includeUnscheduled, onlyInvoiceable) | Offline, ranged date filters | ship |
| 7 | Get job detail | flutchai get_job | `jobs get <id>` with `--expand` for visits/invoices/timeSheetEntries/expenses/lineItems | Local join across 5 child tables | ship |
| 8 | List visits | (no competitor) | `visits list` with `VisitFilterAttributes` (status, startAt, endAt, completedAt, invoiceStatus, assignedTo, productOrServiceId) | First offline view of dispatch data | ship |
| 9 | List invoices with filters | flutchai list_invoices | `invoices list` with `InvoiceFilterAttributes` (clientId, invoiceNumber, total, issuedDate, dueDate, status, excludeOrigin) | Offline, status filters, ranged dates | ship |
| 10 | List payment records | (no competitor) | `payment-records list` with `PaymentRecordFilterAttributes` (entryDate, adjustmentType, paymentType, refundable, clientId) | First offline view; required for AR analysis | ship |
| 11 | List payout records | (no competitor) | `payout-records list` with `PayoutFilterAttributes` (createdAt, updatedAt, arrivalDateRange, status, payoutMethod) | First offline view of Jobber Payments deposits | ship |
| 12 | List timesheet entries | (no competitor) | `timesheets list` with `TimeSheetEntriesFilterAttributes` (activeOnDate, assignedTo, isApproved, isPayable, ticking, startAt) | First offline labor data view | ship |
| 13 | List expenses | (no competitor) | `expenses list` with `ExpenseFilterAttributes` (createdAt, updatedAt, date, enteredById, reimbursableToId) | First offline cost view; job costing input | ship |
| 14 | List users | (no competitor) | `users list` with `UsersFilterAttributes` (status, permissions, userIds) | Required join key for timesheets/expenses | ship |
| 15 | List products/services | (no competitor) | `products list` with `ProductsFilterInput` (category, sort, ids) | Catalog visibility | ship |
| 16 | List tax rates | (no competitor) | `tax-rates list` with searchTerm | Tax detail for invoice analysis | ship |
| 17 | Sync to local SQLite | (none — competitors are RPC proxies) | `sync` with full + incremental modes, `updatedAt` cursors where supported, cost-aware throttle | THE differentiator: offline-first analysis | ship |
| 18 | FTS search | (no competitor) | `search "<query>"` (FTS5) over Client (name/email/address), Job (title/description), Invoice (number), Property (address) | Cross-entity text search offline | ship |
| 19 | SQL passthrough | (no competitor) | `sql "<SELECT ...>"` — read-only SELECT against local SQLite; blocks DML/DDL | Composable analytics | ship |
| 20 | Doctor | partial: flutchai connectivity | `doctor` — account query, version check, throttle budget check, sync state freshness | Pre-flight check before long sync | ship |

### Rejected per read-only guardrail (deliberately NOT in scope)

- `create_client`, `update_client` (flutchai)
- `create_job` (flutchai)
- `create_quote` (flutchai, Zapier)
- `Create Client`, `Add Tags to Client`, `Create Request`, `Add Note to Request` (Zapier)

These are introspectable mutations on the Jobber GraphQL surface. Per `schema/discovery_plan.md` guardrail "Do not run mutations," the CLI will not emit them as Cobra commands or MCP tools.

## Transcendence (only possible with offline SQLite + local joins)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This |
|---|---------|---------|-------|--------------|------------------------|
| 1 | AR aging by client | `ar aging [--tag] [--as-of <date>]` | 9/10 | hand-code | Local Invoice + PaymentRecord aggregation with age buckets; no API call exposes this — Jobber UI's AR report is unfilterable on the dimensions advisors need |
| 2 | Invoice payment trace | `invoices trace [--mismatched]` | 8/10 | hand-code | Local Invoice ⋈ PaymentRecord ⋈ PayoutRecord; one row per invoice with billed/paid/balance/payout-ref; flutchai/Zapier return invoices as flat objects |
| 3 | Payout reconciliation | `payouts reconcile` | 8/10 | hand-code | Local PayoutRecord ⋈ PaymentRecord ⋈ Invoice ⋈ Client; no Jobber MCP exposes payouts at all |
| 4 | Job profit & loss | `jobs pnl [--since <d>] [--bottom <n>]` | 9/10 | hand-code | Local Job ⋈ JobLineItem ⋈ TimeSheetEntry ⋈ Expense; no competitor performs this 4-table join |
| 5 | Stale jobs queue | `jobs stale --days <n> [--no-future-visits]` | 7/10 | hand-code | Local Job left-join future Visit with negative conditions; absorbs the unscheduled-work query |
| 6 | Sales funnel | `funnel [--stage stuck] [--since <d>]` | 7/10 | hand-code | Aggregates counts and per-stage transition times across Request → Quote → Job → Invoice; `--stage stuck` lists each cohort that stalled |
| 7 | Snapshot diff | `snapshot diff <a> <b> [--save <label>]` | 8/10 | hand-code | Labels and diffs SQLite snapshots: new clients, status transitions, open-AR deltas per client; week-over-week diligence view |
| 8 | Client 360 | `clients 360 <id>` | 8/10 | hand-code | Single-screen Client ⋈ Property ⋈ recent Job ⋈ open Invoice ⋈ recent PaymentRecord; replaces the 6-tab Jobber UI workflow |

**Hand-code commitment:** 8 of 8 transcendence features tagged `hand-code` (`spec-emits`: 0). Each is ~50-150 LoC plus `root.go` wiring. Total novel-feature code budget: ~600-1200 LoC.

## Stubs / deferred

- **Tag-scoped rollup** — Tag connection availability is `Unknown` in introspection. Killed for v1; reprint candidate once tag reachability is confirmed.
- **User labor utilization** — Depends on per-user billing rate that is not confirmed present on `User` type. Killed for v1; labor cost dimension partially covered by job P&L.
- **Audit export bundle** — Monthly cadence; composable from `sync` + `sql` + shell. Not promoted to a hand-coded command.
- **Heritage-specific transcendence** (AR vs QBO reconciliation, off-integration payment detector) — Explicitly deferred per user's Option A pick. Added post-generation via polish or hand-edits, not part of v1 manifest.

## Risk notes

- `paymentMethods`, `customFieldConfigurations`, `tags` are partially verified in the schema (per `endpoint_inventory.csv`). Sync skips these in v1; they can be added in a polish pass after follow-up probes.
- `Jobs` and `Visits` have no `updatedAt` filter at the schema level — incremental sync falls back to `createdAt` or `completedAt` ranges.
- Refresh-token rotation: every refresh MUST persist newest refresh token to Windows user env. Non-negotiable; without it the CLI breaks within 60 min of issuance.
