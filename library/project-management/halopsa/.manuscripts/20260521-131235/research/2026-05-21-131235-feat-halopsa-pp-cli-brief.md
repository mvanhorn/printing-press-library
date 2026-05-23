# HaloPSA CLI Brief

## API Identity
- **Domain:** MSP / IT Service Management — Professional Services Automation. Tickets, clients, sites, assets, agents, time, contracts, invoicing, knowledge base, workflows. Same REST API powers HaloITSM, HaloPSA, HaloServiceDesk, and HaloCRM.
- **Users:** MSP technicians, dispatchers, and ops/admins. Also internal helpdesks at companies using HaloITSM.
- **Data profile:** Tenant-scoped (`https://<tenant>.halopsa.com/api`). OpenAPI 3.0.1, 952 paths, 808 GETs, 408 POSTs, 246 DELETEs. Bearer auth via OAuth2 client_credentials at `https://<tenant>.halopsa.com/auth/token`. Offset pagination (`page_no`/`page_size`, server returns `record_count`).
- **Spec source:** `https://haloacademy.halopsa.com/api/swagger/v2/swagger.json` (4.5 MB, public, no auth needed for the spec itself).

## Reachability Risk
- **None.** The spec is freely fetchable; the API is tenant-scoped REST behind OAuth2. No bot-protection on the spec or auth endpoint. Auth requires the user's tenant + a client app they create in their portal. No GitHub issues across the dominant wrappers (`homotechsual/HaloAPI`, Python `HaloPSA`) report 403/blocked/deprecated. Tenants are the only gate; nothing else.

## Top Workflows
1. **Triage tickets** — list open tickets (filtered by team, agent, status, priority, SLA), get one ticket end-to-end with actions/attachments, reassign, change status.
2. **Bulk close / age-out tickets** — find tickets stale for N days in a status (e.g., "Awaiting Customer Reply > 14d") and bulk-close with a templated action.
3. **Client/asset lookup at the desk** — search clients, drill to sites, list a client's assets/contracts/active tickets, find which agents own them.
4. **Time entry & timesheet pull** — log time against a ticket, export the week's billable time per agent for invoicing prep.
5. **Knowledge-base search + insertion** — search KB articles, paste into a ticket as an action.
6. **Workflow / ticket-rule audits** — list workflows, rules, automations; understand what's firing.

## Table Stakes (must match the best competitor)
Match every feature that exists across the ecosystem:
- All entity CRUD: tickets, clients, sites, agents, users, assets, actions, contracts, invoices, quotations, recurring invoices, KB articles, teams, statuses, workflows, ticket rules, time-sheets, appointments, reports.
- OAuth2 client_credentials login, tenant URL, token cache, token refresh on expiry.
- Pagination handling (auto-page-through, `--limit`/`--page-size`/`--page`).
- Attachments (download/upload).
- Field selection (`includedetails`/`includeextraobjects` query params).
- Filters: by client, team, agent, status, priority, date range, search term.
- Batch action operations (multiple actions on one ticket per POST).

## Data Layer
- **Primary entities:** Tickets (with Actions), Clients, Sites, Users, Agents, Assets, Contracts, Invoices, Quotations, KBArticles, Teams, Statuses, Workflows, TicketRules, Timesheets, Appointments, Reports.
- **Sync cursor:** Most resources support `lastupdatedfrom`/`lastupdatedto` query params — perfect for incremental sync.
- **FTS/search:** SQLite FTS5 over tickets (summary, details, actions), clients (name, custom fields), assets (tag, name, fields), KB articles (title, body). Joins on the local store unlock cross-entity queries that the API can't do server-side (e.g., "tickets stale > N days assigned to overloaded agent X").

## Codebase Intelligence
- **Source:** GitHub README + ecosystem research; no DeepWiki run yet (will run in 1.5a.6).
- **Auth:** OAuth2 client_credentials at `<tenant>/auth/token`, returns Bearer access_token. Header: `Authorization: Bearer <token>`. Scopes from the API application config (typically `all` for full access).
- **Data model:** REST with denormalized objects. Tickets contain nested Actions, Asset_Fields, Custom Fields. Pagination via `page_no` + `page_size`; total in `record_count`. Most lists accept include flags (`includedetails`, `includeextraobjects`).
- **Rate limiting:** Not documented as a hard ceiling — Halo recommends being polite; tenants share resources but throttling is generally generous for legitimate automation. Retry on 429/5xx with exponential backoff is the standard approach in the SDKs.
- **Architecture:** REST + Bearer + offset pagination. PUT bodies for updates, POST for creates, DELETE for removal. The single Halo API surface powers all four product lines — same endpoints, different licensing.

## Build Priorities
1. **Tickets are the headline.** `tickets list/get/create/update/close`, `actions list/add`, `--filter`, `--status`, `--team`, `--agent`, `--stale-days`, `--json`/`--select`/`--csv`.
2. **Sync to local SQLite** for tickets, clients, sites, agents, users, assets, contracts, KB articles. Use `lastupdatedfrom` for incremental.
3. **Cross-entity transcendence:** "Who's overloaded", "what's about to breach SLA", "client health card", "stale tickets to close", "what changed since I last looked", "KB suggestion for ticket" — these are the commands that only work because everything is in one local store.
