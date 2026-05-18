# Freshservice CLI Brief

## API Identity
- Domain: IT Service Management (ITSM) / IT Asset Management (ITAM)
- Users: IT administrators, helpdesk agents, SRE/DevOps teams, and AI agents automating IT workflows
- Data profile: Tickets (incidents/service requests), Assets (hardware/software), Changes (ITIL change management), Users (requesters + agents), Problems, Releases, Knowledge Base

## Auth
- Type: HTTP Basic Auth — API key as username, empty string as password, Base64-encoded
- Header: `Authorization: Basic base64(apikey:)`
- Env vars: `FRESHSERVICE_APIKEY` + `FRESHSERVICE_DOMAIN` (e.g., `yourcompany.freshservice.com`)
- Base URL: `https://{FRESHSERVICE_DOMAIN}/api/v2`
- OAuth 2.0 available since April 2024 but rarely used in community tools

## Reachability Risk
- Low. Standard HTTPS REST API; well-established by multiple community tools including a 36-star PowerShell module and an active MCP server

## Rate Limits
- Starter: 100 req/min | Growth: 200 | Pro: 400 | Enterprise: 500
- 429 response with `Retry-After` header; invalid requests count toward limit
- Headers: `X-RateLimit-Total`, `X-RateLimit-Remaining`

## Pagination
- Default 30/page, max 100 with `?page=N&per_page=N`
- Link header provides next/prev page URLs

## Query Filter Syntax (CRITICAL)
- Filter queries MUST be wrapped in double quotes: `"status:2 AND priority:1"`
- Unquoted queries return HTTP 500 from the Freshservice API
- Operators: `<`, `>`, `<=`, `>=`, `AND`, `OR`

## Top Workflows
1. **Ticket triage** — list open/pending tickets by priority, filter by assignee or group, assign/update status
2. **Change approval workflow** — create change → add approval group → monitor approval status → close
3. **Asset discovery** — search assets by type/name, link to tickets, view components and contracts
4. **Agent workload management** — list tickets by agent, reassign, check SLA breach risk
5. **Requester management** — look up users, create requesters, convert to agents, merge duplicates

## Table Stakes (match the ecosystem)
- Ticket CRUD with full field support (subject, description, priority, status, source, assignee, group)
- Ticket filtering with query syntax (`status:2 AND priority:3`)
- Ticket conversations (notes, replies, conversation history)
- Change management full lifecycle (create, plan, approval groups, close)
- Change tasks and time entries
- Agent and requester CRUD with field introspection
- Group management (agent groups, requester groups)
- Service catalog browsing and service request creation
- Asset CRUD with components/contracts/requests
- Knowledge base (solution categories and articles)
- Canned response browsing
- Workspace listing

## Data Layer
- Primary entities: tickets, changes, assets, requesters, agents, groups
- Sync cursor: ticket `updated_since`, agent/requester list with pagination
- FTS/search: ticket subject/description, asset name, requester name/email
- High-value local indexes: open tickets by priority+assignee, SLA breach candidates, change approval queue

## Codebase Intelligence
- Source: effytech/freshservice_mcp (31 stars, Python, MIT, ~114KB server.py, 100+ tools)
- Auth: `base64(f"{FRESHSERVICE_APIKEY}:")` → `Authorization: Basic {encoded}`
- Data model: tickets ↔ conversations ↔ notes; changes ↔ tasks ↔ time_entries ↔ notes ↔ approvals
- Filter quirk: queries must be double-quoted strings passed as `query` param
- Pagination: Link header regex parsing for next/prev
- Status enums: ticket (OPEN=2, PENDING=3, RESOLVED=4, CLOSED=5), change (OPEN=1, PLANNING=2, AWAITING_APPROVAL=3, PENDING_RELEASE=4, PENDING_REVIEW=5, CLOSED=6)
- Priority enums: LOW=1, MEDIUM=2, HIGH=3, URGENT=4
- Change type: MINOR=1, STANDARD=2, MAJOR=3, EMERGENCY=4
- Risk: LOW=1, MEDIUM=2, HIGH=3, VERY_HIGH=4

## User Vision
- Primary consumer is AI agents (MCP-first, agent-native JSON output)
- Coverage scope: tickets, assets, changes, users
- PAT (Personal Access Token) auth — will be provided later via safe channel
- ITSM compliance workflow automation is the primary use case

## Product Thesis
- Name: `freshservice-pp-cli`
- Why it should exist: Freshservice has a 36-star PowerShell module, an early-stage Python MCP server, and zero general-purpose terminal CLI. There is no `fs ticket list`, no offline ticket search, no agent-native JSON pipeline. This fills the gap jira-cli fills for Jira — a fast, composable, offline-capable Go binary for Freshservice that beats every existing tool and exposes everything through MCP.

## Build Priorities
1. Ticket management — the core ITSM workflow; filter, assign, comment, reply
2. Change lifecycle — approval groups, tasks, time entries; the ITIL compliance story
3. Asset management — ITAM coverage no existing CLI has
4. User management — requesters + agents + groups; context for all other operations
5. Offline search + SQLite store — what no other tool offers
6. Service catalog + knowledge base — round out the ITSM picture
