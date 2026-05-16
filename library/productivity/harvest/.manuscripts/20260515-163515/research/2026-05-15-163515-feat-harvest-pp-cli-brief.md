# Harvest CLI Brief

## API Identity
- Domain: Time tracking, project management, invoicing for service-based businesses
- Users: Agencies, consultancies, dev shops, freelancers. Daily-driver users are PMs (tracking allocation), engineers (logging time), finance (invoicing/reports)
- Data profile: Time entries are append-mostly with edits; projects/clients/tasks/users are reference data that changes slowly; invoices and expenses are write-heavy at month-end
- Auth: Personal Access Token (`HARVEST_ACCESS_TOKEN`) + `HARVEST_ACCOUNT_ID` header. OAuth2 also supported but PAT is the default daily-driver path.

## Reachability Risk
- **None** — public spec, well-documented v2 API, returns 200 on `GET /users/me` with valid creds. No 403/CF/WAF protection observed. Spec hosted on GitHub raw (200).

## Top Workflows
1. **Log time** — start/stop a timer or post a time entry for today against a project+task with hours and notes
2. **Edit recent entries** — fix yesterday's hours, recategorize a task, add notes after a meeting
3. **Weekly time review** — pull this week's entries by user/project, see total hours, billable %, gaps
4. **Project allocation reports** — hours by project, by client, over a date range, with billable totals
5. **Invoice prep** — list uninvoiced billable hours by client, generate invoice draft

## Table Stakes (from competing tools)
- **CRUD on time entries** — every CLI does this (kgajera/hrvst-cli, zenhob/hcl, lucasconstantino/harvest-cli, droath/harvest-toolkit)
- **Start/stop/restart timer** — Harvest API has dedicated endpoints; every wrapper exposes them
- **List + filter** projects, clients, tasks, users with pagination
- **Reports** — time, expense, project budget; daily/weekly/monthly aggregation
- **Invoice CRUD** — list, get, mark paid; some create/update
- **Expense tracking** — categories, expense entries with receipts
- **Estimates** — quote drafts, accept/decline
- **`me` shortcut** — current user info, current company

## Data Layer
- Primary entities: time_entries, projects, clients, tasks, users, invoices, expenses, estimates, roles, user_assignments, task_assignments, expense_categories, invoice_payments, invoice_messages
- Sync cursor: `updated_since` query param (ISO 8601) on every collection endpoint — perfect for incremental sync
- FTS/search: project name, client name, task name, time entry notes — agents need fuzzy text search

## Codebase Intelligence
- Source: ianaleck/harvest-mcp-server (60+ tools), taiste/harvest-mcp-server, southleft/harvest-mcp (referenced in user's Cloud Run deployment)
- Auth: `Authorization: Bearer <PAT>` + `Harvest-Account-Id: <account>` + `User-Agent: <app> (<email>)`
- Data model: REST collection-style. Pagination via `page` + `per_page` (default 100, max 2000). `total_pages`/`total_entries` in response.
- Rate limiting: 100 requests / 15-second window per account. 429 on exceed with `Retry-After`
- Architecture: Each collection endpoint follows `GET /<resource>`, `GET /<resource>/<id>`, `POST /<resource>`, `PATCH /<resource>/<id>`, `DELETE /<resource>/<id>`. Timer endpoints under `/time_entries/<id>/restart` and `/time_entries/<id>/stop`.

## User Vision
- "Let it research and decide" — standard run. User already has harvest-mcp running on Cloud Run for MCP access; CLI gives them: offline SQLite cache, FTS over time-entry notes, --json/--select for agents, structured exit codes, scriptable batch operations.

## Product Thesis
- **Name:** harvest-pp-cli
- **Why it should exist:** Existing CLIs (hrvst-cli, hcl, harvest-cli) are good at time-entry CRUD but none offer offline FTS, local SQLite for cross-entity joins, or agent-native output. The MCP servers are good for AI agents but require a server running. A standalone CLI with a local store unlocks: (1) querying "what did I do last Tuesday" without an API call, (2) joining time entries with project/client data locally, (3) historical trend analysis (utilization, billable %, project burn) that needs aggregation across the full year, not just paginated lists.

## Build Priorities
1. **Foundation:** Local SQLite store mirroring time_entries, projects, clients, tasks, users. Incremental sync via `updated_since`. FTS5 on notes/names.
2. **Absorb (table stakes):** Full CRUD on time entries, projects, clients, tasks, users, invoices, expenses, estimates. Timer start/stop/restart. Reports endpoints.
3. **Transcendence (novel):** TBD per absorb manifest — likely candidates: weekly summary, gap detection (days with no entries), billable vs non-billable trends, project budget burn rate, "what's missing" (timesheet completion check), cross-project hours per client, idle time alerts.
