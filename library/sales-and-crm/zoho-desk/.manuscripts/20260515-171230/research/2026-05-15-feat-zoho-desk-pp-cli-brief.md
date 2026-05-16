# Zoho Desk CLI Brief

## API Identity
- Domain: Customer support ticketing, helpdesk, knowledge base, community
- Users: Support managers (SLA monitoring), agents (daily ticket work), team leads (workload balancing), execs (CSAT/throughput dashboards)
- Data profile: Tickets are conversational threads (parent + comments + replies + history events); contacts/accounts are stable refs; agents/departments/teams are org structure; SLA + status transitions are high-volume mutations
- Auth: OAuth 2.0 only. Requires `Authorization: Zoho-oauthtoken <token>` + `orgId` header. Refresh-token grant for headless usage.

## Reachability Risk
- **None** — official OAS spec from zoho/zohodesk-oas repo. API is well-documented at https://desk.zoho.com/DeskAPIDocument.

## Top Workflows
1. **Triage incoming tickets** — list new/open tickets by department, assign to agent, set priority/category
2. **Reply to a ticket** — fetch latest thread, draft reply via comment/thread create, mark as awaiting customer
3. **SLA / aging check** — list tickets approaching due date, breached SLA, in escalation
4. **Workload review** — tickets per agent, open vs resolved, response time stats
5. **Find by content** — search ticket subject/description/comments for a customer mention, error string, or product reference

## Table Stakes (from competing tools / MCP servers)
- **CRUD on tickets, comments, threads** — every helpdesk client has this
- **State transitions** — close, reopen, mark as spam, assign, escalate
- **Contact + account CRUD** — customer records, attached organizations
- **Search across modules** — tickets, contacts, accounts, KB articles
- **Statistics endpoints** — ticket counts, agent stats, queue counts
- **Bulk operations** — bulk update tickets, contacts; mass action error handling
- **Attachments, history, tags** — full ticket conversation context
- **Departments, agents, profiles, roles** — org structure CRUD

## Data Layer
- Primary entities: tickets, threads, comments, contacts, accounts, agents, departments, teams, roles, profiles, tasks
- Sync cursor: `modifiedTime` / `_modifiedTimeRange` on collection endpoints
- FTS/search: ticket subject + description + first-thread content + comments; contact name/email/phone; account name; agent name/email

## Codebase Intelligence
- Source: Zoho's own MCP servers (`ZohoDesk_*` tools, ~200 methods) — exposes full v1 API surface
- Auth: OAuth2 refresh-token flow, regional data center URLs (`desk.zoho.com`, `desk.zoho.eu`, `desk.zoho.in`, `desk.zoho.com.au`, `desk.zoho.com.cn`)
- Data model: Modular REST. Mass actions follow `bulkUpdate*` pattern. Search via `/api/v1/search?module=Tickets&searchStr=...`
- Rate limiting: ~10 req/s per org, per IP. 429 on burst.
- Architecture: Resource-per-module, sub-resources for relationships (e.g., `/tickets/{id}/comments`, `/tickets/{id}/threads`)

## Product Thesis
- **Name:** zoho-desk-pp-cli
- **Why it should exist:** Zoho's MCP server has 200+ tools but requires a live OAuth-authenticated server connection. A standalone CLI with a local SQLite store unlocks: (1) offline FTS across ticket bodies + comments (impossible against the API; search endpoint is keyword-only and rate-limited), (2) cross-entity analytics (SLA-at-risk view joining tickets + agents + business hours), (3) cron-driven workflows (ticket-aging alerts, daily reassignment) without keeping a server running.

## Build Priorities
1. Foundation: SQLite mirroring tickets, threads, comments, contacts, accounts, agents, departments
2. Absorb: CRUD on tickets+comments+threads, contact/account CRUD, search, statistics, bulk updates
3. Transcendence: SLA breach detection, agent workload imbalance, ticket-aging triage, conversational FTS (search across thread content), reassignment helpers
