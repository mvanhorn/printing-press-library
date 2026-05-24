# Jobber CLI Brief

## API Identity
- Domain: Field service management CRM (residential/commercial home services: cleaning, lawn care, HVAC, plumbing, roofing). Customer master, scheduling, quotes-to-invoices, payments, time tracking, expenses.
- Users: Field service business owners, office managers, accountants, M&A/CFO advisors auditing a target's operational and AR data, integrators (Zapier/Pipedream/QBO).
- Data profile: Highly relational — clients own properties, properties anchor jobs/quotes/invoices, jobs roll up visits/timesheets/expenses/line-items, invoices carry payment records and tax details. 18 verified GraphQL root surfaces in this tenant's introspection.
- API style: **GraphQL only.** Single endpoint `POST https://api.getjobber.com/api/graphql`. Relay-style cursor pagination (`first`/`after`/`pageInfo.endCursor`/`hasNextPage`). `extensions.cost` carries query cost; `extensions.versioning` carries version metadata.

## Reachability Risk
- **None.** Live introspection probe at 2026-05-15 verified 18 root connections returning consistent shapes with usable cost/throttle metadata. All six OAuth env vars are set locally. No reports of access blocking on the Developer Center docs. Reachability gate (Phase 1.9) should pass on a single `account` query.

## Top Workflows
1. **Sync everything to local SQLite for offline analysis.** Read-only nightly pull of clients, properties, jobs, visits, quotes, invoices, payments, timesheets, expenses. Drives every other workflow.
2. **AR analysis & reconciliation.** "Where is my unpaid revenue?" — aged AR by client/job, invoices vs payments, paymentRecords vs allocations, off-integration payment detection. *This is the Heritage motivation per memory: Jobber AR is structurally inflated because off-integration payments don't relieve AR and contracts bill in full at signing.*
3. **Job costing & profitability.** Per-job revenue (invoices) vs labor (timesheets) vs expenses vs visit count. "Which jobs lost money this quarter?"
4. **Schedule / dispatch visibility.** Visits by date range, by assigned user, unscheduled queue, overdue work.
5. **Sales pipeline pulse.** Requests → quotes (sent/approved/converted) → jobs → invoices funnel with conversion rates and average time-in-stage.
6. **Tag / custom-field segmentation.** "Show me all clients tagged `commercial` with open AR > 90 days."

## Table Stakes (from community MCPs and Zapier integrations)
- List/get for clients, properties, jobs, visits, quotes, invoices, payments, timesheets, expenses, users, products, tax rates, payouts.
- Filter by client/property/status/date range (Jobber filter attributes vary by surface; see endpoint_inventory.csv).
- `updatedAt` cursor support on clients, requests, quotes, invoices, payouts, timesheets, expenses (per schema introspection); jobs/visits have date/status filters but no `updatedAt` filter.
- OAuth2 with refresh token rotation (newest refresh token must be persisted).
- Required `X-JOBBER-GRAPHQL-VERSION` header on every request.
- Bearer token in `Authorization` header.

## Data Layer
- **Primary entities (18 SQLite tables):** `Account`, `Client`, `Property`, `Request`, `Quote`, `Job`, `Visit`, `Invoice`, `PaymentRecord`, `PayoutRecord`, `TimeSheetEntry`, `User`, `TaxRate`, `Expense`, `ProductOrService`, plus nested but worth surfacing (`JobLineItem`, `QuoteLineItem`, `InvoiceLineItem`). `Tag`, `CustomFieldConfiguration`, `PaymentMethod` need follow-up probes per `discovery_plan.md`.
- **Sync cursor:** `updatedAt` filter where supported (clients, requests, quotes, invoices, payouts, timesheets, expenses). For jobs/visits, fall back to `createdAt` or `completedAt` ranges. Persist cursor per resource_type in a `sync_state` table.
- **FTS:** SQLite FTS5 across `Client` (name, email, address), `Job` (title, description), `Invoice` (number, client name), `Property` (address). High-gravity textual search surface.
- **Rate-limit aware sync:** Read `extensions.cost.throttleStatus.{requested,actual,max,restore}` from every response; back off when `actual/max > 0.5` and pause when restore is needed.
- **Token rotation hook:** Refresh handler MUST persist newest `refresh_token` to Windows user env (`JOBBER_REFRESH_TOKEN`). This is non-negotiable per Jobber's rotation policy.

## Codebase Intelligence
- Source: web research only (no DeepWiki query attempted yet for `flutchai/mcp-server-jobber`)
- Auth: OAuth2 authorization code flow, `Authorization: Bearer <ACCESS_TOKEN>`, mandatory `X-JOBBER-GRAPHQL-VERSION` header
- Data model: Heavily nested with deep relationship graphs; Relay-style pagination on every connection
- Rate limiting: Per-query cost model in `extensions.cost` (not request-count). Cost up to ~14 per medium query; max budget 10,000; restore 500/sec
- Architecture: Single GraphQL endpoint; version pinned per request; refresh token rotation enforced

## User Vision
Working context from briefing + local schema:
- **Read-only is non-negotiable.** `schema/discovery_plan.md` guardrails forbid mutations. CLI must not emit mutation tools by default (even though GraphQL introspection exposes them).
- **Heritage engagement context** (from auto-memory): Jobber is used as the operational CRM; AR is structurally inflated; off-integration payments don't relieve AR; contracts billed in full at signing. Future Heritage-specific transcendence features (AR reconciliation vs QBO, off-integration payment detection) are explicitly *out of scope* for this v1 generic CLI per the user's "Option A" pick — but build priorities should leave room to add them later.
- **No tenant data persists in tracked artifacts.** Token values, tenant IDs, account names, customer rows must not appear in research/proofs/manuscripts.
- **Output location.** CLI lands at `~/printing-press/library/jobber/`; user accepted bridging will be a post-generation step.

## Source Priority
Single source. No combo-CLI gate fired.

## Product Thesis
- **Name:** `jobber-pp-cli`
- **Tagline:** "Read-only Jobber GraphQL CLI for offline analysis and audit — every surface synced to SQLite, every relationship queryable, zero mutation risk."
- **Why it should exist:** No dedicated Jobber CLI exists today. Existing MCPs (Zapier, viaSocket, flutchai) are RPC-style proxies for individual API calls — they don't give you offline data, can't run SQL, can't be used during a flight or audit. M&A advisors, fractional CFOs, and accountants doing client-data analysis on Jobber need a tool that pulls the data once and lets them slice it with SQL and full-text search. The official GetJobber repos are app-template scaffolds, not analyst tools.
- **Beat the incumbents by:** Offline-first SQLite store, `--json`/`--csv`/`--select` agent-native output, FTS5 search across every text field, SQL composability, dry-run on every call, typed exit codes, no mutations (deliberately), cost-aware throttle handling.

## Reachability Mitigation
- One reachability ping (Phase 1.9) before generate: `query { account { id } }`
- Cost-budget guard: warn at `actual/max > 0.5`, hard-stop sync at `> 0.85`, sleep until restore replenishes
- Cursor-based incremental sync (don't refetch full history)
- Version header pulled from `JOBBER_GRAPHQL_VERSION` env var, with `doctor` warning if env value is older than 90 days

## Build Priorities
1. **OAuth client with refresh-token rotation** — non-negotiable. Refresh handler MUST persist new refresh token back to Windows user env. Without this, every other command breaks within 60 minutes (Jobber access token TTL).
2. **GraphQL client wrapper** (Go) — cost-aware throttle handling, automatic retry on transient errors, version header injection, query cost telemetry.
3. **Sync command** — full + incremental per resource_type. Uses `updatedAt` cursors where supported. Persists rows into `resources` table (and typed FTS tables where worth it).
4. **List/get commands** — one per verified root surface (15-18 commands). All read-only. Filter flags mirror Jobber filter attribute names.
5. **Search command** — FTS5 across synced data. The "what is this client's status" command power-users will live in.
6. **SQL command** — guarded read-only SQL passthrough to local SQLite. Block any DML/DDL.
7. **Doctor** — auth check (account query), version check, throttle budget check, sync state freshness check.
8. **Novel features** — to be defined at Phase 1.5 absorb gate.

## Open Questions Deferred to Tenant Work
Per `schema/brief.md`, these belong to Heritage-tenant follow-up, not the generic CLI:
- Concrete QBO ID storage in Jobber fields
- Required scopes by data surface
- Completeness of timesheets/invoices/payments/expenses for accounting-dependent analysis
