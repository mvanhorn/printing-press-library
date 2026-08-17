# Expensify CLI Brief

## API Identity
- Domain: Expense management, corporate card reconciliation, AP workflow
- Users: Finance controllers, AP clerks, founders reconciling AmEx, accounting firms, expense-admins, policy-admins at mid-market companies
- Data profile: Reports (states: OPEN/SUBMITTED/APPROVED/REIMBURSED/ARCHIVED), Expenses, Policies (workspaces), Categories, Tags, Report Fields, Employees, Domain Cards, Reconciliation rows

## Reachability Risk
None. Expensify publishes Integration Server docs, mints credentials self-serve, and explicitly markets scripted access. Rate limits are documented (5 req / 10s and 20 req / 60s) — trivial to handle with a token bucket. No GitHub issues reporting maintainer blocking.

## API Surface
- Base URL: `https://integrations.expensify.com/Integration-Server/ExpensifyIntegrations` (single dispatcher)
- Auth: `credentials: { partnerUserID, partnerUserSecret }` inside form-encoded `requestJobDescription` JSON body. Credentials minted at https://www.expensify.com/tools/integrations/
- Env convention (from wrappers): `EXPENSIFY_PARTNER_USER_ID`, `EXPENSIFY_PARTNER_USER_SECRET`
- Job types:
  - **Export**: Report Exporter (Freemarker template; csv/xls/xlsx/txt/pdf/json/xml; onFinish: email/markAsExported/sftpUpload), Downloader, Reconciliation
  - **Create**: Report, Expense, Policy, Expense Rules
  - **Get**: Policy List, Policy, Domain Cards, Card Owner Data
  - **Update**: Report Status, Policy, Advanced Employee Updater, Expense Rules, Tag Approvers
- Quirks: Freemarker .ftl templates for exports; date range capped at 1 year; `markedAsExported` label is the de-facto idempotency cursor
- No REST resource paths, no pagination, no webhooks, no OAuth, no GraphQL

## Top Workflows
1. **Month-end close export** — pull APPROVED+REIMBURSED reports for period, render Freemarker template for NetSuite/QBO/Xero/Sage, mark exported atomically
2. **Corporate card reconciliation** — match Domain Cards + Reconciliation rows to submitted expenses; flag missing receipts
3. **Policy config as code** — version-control categories/tags/GL codes/report fields across workspaces
4. **Employee provisioning** — Advanced Employee Updater: add/remove/reassign approver chains when org changes
5. **Bulk expense/report creation** — programmatic entry from card feeds or migrations

## Data Layer
- Primary entities: reports, expenses, policies, categories, tags, report_fields, employees, domain_cards, reconciliation, expense_rules
- Sync cursor: `markedAsExported` label sentinel on Report Exporter; timestamp on policy getters
- FTS/search: FTS5 over merchant, comment, category, tag, report title
- What local store unlocks: cross-year search (API has no search); fast report-lifecycle dashboards without 429s; policy diff (YAML → API); reconciliation staging table; GL rollups

## Codebase Intelligence
- Source: two MCP servers examined
- **primrose-mcp-expensify** (TS): 22 tools covering policy/report/expense/employee/reconciliation. Zero adoption. Commercial pattern.
- **agenticledger/expensify-mcp-http** (JS): 13 tools, closer to raw API. Zero adoption.
- Auth pattern confirmed: partnerUserID + partnerUserSecret in requestJobDescription body
- Neither MCP has: local store, offline search, policy diff, reconciliation UX

## User Vision
User has login to expensify.com (Chrome) and wants to get Integration Server credentials during the build. No specific workflow priorities stated — default to finance-ops power user.

## Product Thesis
- **Name**: `expensify-pp-cli` (binary). Naming positions this as the definitive Expensify CLI since no serious competitor exists.
- **Why install this over Web UI + curl scripts**:
  1. `expensify sync` → local SQLite store of all reports/expenses/policies/cards — enables cross-year search the web UI can't do
  2. `expensify close --month 2026-03 --template netsuite` → renders the right Freemarker template, handles 429 backoff, marks-as-exported atomically
  3. `expensify policy diff` / `policy apply` → categories/tags/report fields as version-controlled YAML (no competitor does this)
  4. `expensify recon` → joins card feed vs expenses locally, flags missing receipts
  5. Offline-first means CFOs/controllers iterate on queries without burning API budget

## Build Priorities
1. **Priority 0: Data layer** — SQLite tables for reports, expenses, policies (and nested: categories, tags, report_fields, employees, expense_rules), domain_cards, reconciliation. FTS5 over merchant/comment/category/tag/title.
2. **Priority 1 absorbed commands** — cover all 22 primrose tools + 13 agenticledger tools. Every verb in the API gets a command: report export/create/mark/status, expense create, policy get/list/create/update, categories/tags/report-fields update, employee add/update/remove, domain-cards list, reconciliation export, expense-rules crud.
3. **Priority 2 transcendence** —
   - `close` (month-end orchestrator with built-in GL templates)
   - `policy diff` / `policy apply` (YAML <-> API)
   - `recon match` (local card-feed to expense join)
   - `search` (FTS over years of data)
   - `missing-receipts` / `stale-reports` (local queries)
   - `gl` (GL-bucket rollups by category/tag/policy)
