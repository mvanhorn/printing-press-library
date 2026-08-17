# Productive.io CLI Brief

## API Identity
- **Domain:** Productive.io — project management / PSA (professional services automation). This CLI targets the **financial subdomain** specifically.
- **Base URL:** `https://api.productive.io/api/v2/`
- **Spec:** JSON:API. No public OpenAPI; docs are ReadMe-rendered (`developer.productive.io/guides/*`). Contract ground-truth for this build came from the Productive MCP `describe_resource`/`describe_report` introspection + live unauthenticated path probes.
- **Users:** Finance / ops at agencies running Productive. For this build: a Productive agency org (currency SGD).
- **Data profile:** Financial line items, invoices, invoice attributions, deals/budgets. Monetary values are integer **smallest currency unit (cents)** — divide by 100 for SGD/most; JPY/KRW ÷1, KWD/BHD ÷1000.

## Auth (verified verbatim)
- Two required headers: `X-Auth-Token: <token>` (secret) + `X-Organization-Id: <id>` (not secret).
- `Content-Type: application/vnd.api+json` (bulk: `; ext=bulk`).
- Env vars: `PRODUCTIVE_API_TOKEN`, `PRODUCTIVE_ORGANIZATION_ID`.
- **Org id for testing supplied via `PRODUCTIVE_ORGANIZATION_ID`** — user supplies both token and org id.

## JSON:API conventions (verified verbatim)
- **Pagination:** `page[number]` (default 1), `page[size]` (default 30, **max 200**). Response `meta`: `current_page`, `total_pages`, `total_count`, `page_size`, `max_page_size`.
- **Filtering:** `filter[field]=v` or `filter[field][op]=v`. Ops: `eq, not_eq, contains, not_contain, gt, gt_eq, lt, lt_eq`. Logical groups: `filter[$op]=or|and` with indexed operands.
- **Sorting:** `sort=field` / `sort=-field`.
- **Sparse fieldsets + include:** standard JSON:API `fields[type]=...` and `include=...`.
- **Rate limits (must respect):** 100 req / 10 s; 4000 req / 30 min; **Reports: 10 req / 30 s**. The financial report path is the tight one — the CLI must throttle report calls and paginate carefully.

## Reachability Risk
- **None.** Live probes: `/api/v2/deals`, `/invoices`, `/line_items`, `/invoice_attributions`, `/reports/financial_item_reports` all return **401** unauthenticated (reachable, auth-gated). Phase 1.9 = PASS.
- Path shape verified: **reports are under `/api/v2/reports/<report_type>`** (bare `/api/v2/financial_item_reports` → 404). Flat resources are top-level `/api/v2/<resource>`.

## Load-bearing endpoint — `reports/financial_item_reports`
- Path: `GET /api/v2/reports/financial_item_reports`
- **Grouping:** `group=<dim>,<dim>` — for the target report: `group=budget,date:month`. Date-period dims take a bucket suffix: `date:{day|week|month|quarter|year}`.
- **Filters** (same names as source): `filter[date][gt_eq]=YYYY-MM-DD`, `filter[date][lt_eq]=YYYY-MM-DD`, `filter[financial_item_type][eq]=service` (enum: `booking_item, draft_booking_item, expense, invoice_attribution, revenue_item, service, time_entry`), `filter[budget_id]`, `filter[company_id]`, `filter[project_id]`, `filter[stage_type]` (1=Deal, 2=Budget), etc.
- **Key metrics** (money in cents): `total_recognized_revenue`, `total_projected_revenue`, `total_scheduled_revenue`, `total_invoiced` (finalized attributions, net), `total_draft_invoiced`, `total_credited` (signed negative), `total_cost`, `total_recognized_profit`, `average_recognized_margin`, `total_budget_total`, `total_budget_used`.
- **For invoiced-by-budget:** `filter[financial_item_type][eq]=invoice_attribution` + metric `total_invoiced` (net, finalized only; drafts excluded — use `total_draft_invoiced` separately).
- **For recognized-revenue-by-budget×month:** `group=budget,date:month`, metric `total_recognized_revenue`, date range filter. The brief's `financial_item_type=service` narrows to service (unused-budget) items; the command should expose `--financial-item-type` so the operator can pick (service vs invoice_attribution vs time_entry).
- Exact `group`/`date:month` raw-wire syntax to be confirmed on first live call in Phase 5; command is hand-built to make this easy to adjust.

## Flat resources (standard JSON:API list + get)
All confirmed live (401). Fields captured in full from MCP describe.
- **deals** (`stage_type`: 1=Deal, 2=Budget; brief wants `filter[stage_type][eq]=2` for budgets). Money fields: revenue, cost, profit, budget_total, budget_used, invoiced, pending_invoicing (all cents). Rich filters incl. company, responsible, status, stage_status, dates.
- **invoices** (`invoice_type`: 1=Invoice, 2=Credit note). amount/amount_tax/amount_with_tax/amount_paid/amount_unpaid/amount_written_off/amount_credited (cents), invoiced_on, pay_on, paid_on, finalized_on, company, subsidiary. Lifecycle: draft→finalized→sent→paid.
- **invoice_attributions** — junction: invoice ↔ budget, `amount` (cents), `date_from`/`date_to`, `budget`, `invoice`. **Not searchable, no sort.** Core reconciliation link.
- **line_items** — invoice lines: description, quantity, unit_id (1=Hour,2=Piece,3=Day), unit_price (cents), discount, tax_name/tax_value, amount/amount_tax/amount_with_tax (cents), invoice, service, expense. **Not searchable, no sort;** filter by `invoice` (this is the pagination fix — MCP forced 15–20 invoice-ids/batch; direct API paginates freely).

## Top Workflows
1. **Recognized revenue by budget × month** over a date range (the pipeline's core input).
2. **Invoiced amount by budget × month** (net, finalized) via invoice_attributions.
3. **Reconcile** recognized vs invoiced per budget×month → flag deltas (the reconciliation itself).
4. **Bulk raw pulls** of invoices / line_items / invoice_attributions / budgets → CSV for `raw_productive_data/*.csv` in the revenue-reconciliation repo.
5. **Offline re-query** of pulled data (local SQLite) without re-hitting the rate-limited API.

## Data Layer
- Primary entities to persist: `invoices`, `line_items`, `invoice_attributions`, `deals` (budgets), plus cached `financial_item_reports` rows.
- Sync cursor: `updated_at`/`created_at` where available (invoices have `updated_at`; deals `last_activity_at`/`created_at`).
- FTS/search: invoice number/subject, deal name, line-item description.

## Product Thesis
- **Name:** `productive-pp-cli` (binary), "Productive Revenue CLI".
- **Why it exists:** Replace the Productive **MCP connector** in the `revenue-reconciliation` pipeline. The MCP hit "response exceeds max token limit," forcing `line_items` pulls 15–20 invoice-ids at a time. Direct REST removes that ceiling: **free pagination over large result sets** + CSV/JSON output built for a reconciliation pipeline, not a chat window. Recognized-revenue-by-budget-month and recognized-vs-invoiced reconciliation are first-class commands, not multi-call chores.

## User Vision (from BUILD_BRIEF.md)
- Purpose: refresh raw-data pulls for a revenue-recognition reconciliation pipeline. Pagination over large sets matters more than tiny batches (MCP batching was an artifact). `financial_item_reports` (recognized revenue by budget×month, date range, `financial_item_type=service`) must NOT be an afterthought — it's the headline. Originally shipped to a private org repo; now contributed to the public library.

## Scope decision
Productive has ~90 resource types. This CLI is **deliberately scoped to the financial/revenue-reconciliation domain** (deals/budgets, invoices, line_items, invoice_attributions, payments, the financial + invoice + line_item reports), not general CRUD across tasks/surveys/etc. This matches the user's stated pipeline purpose and keeps the surface tight. Read-focused (list/get/report/sync/export); no mutation commands needed for a data-pull tool.

## Build Priorities
1. Data layer + JSON:API client (auth headers, pagination to exhaustion, report rate-limit throttle, cents handling).
2. Flat resource list/get: deals(budgets), invoices, line_items, invoice_attributions (+ payments).
3. `reports/financial_item_reports` recognized-revenue command (budget×month, date range, item-type) — hand-built.
4. Reconciliation transcendence: recognized-vs-invoiced by budget×month.
5. CSV export shaped for `raw_productive_data/*.csv`; offline sync/sql/search.
