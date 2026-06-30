# Zoho Desk CLI Brief

## API Identity
- Domain: Help-desk / customer-support ticketing. Zoho Desk REST API v1.
- Base URL: `https://{deskHost}/api/v1` — DC hosts: us `desk.zoho.com`, eu `desk.zoho.eu`, in `desk.zoho.in`, au `desk.zoho.com.au`, jp `desk.zoho.jp`, cn `desk.zoho.com.cn`, ca `desk.zohocloud.ca`.
- Users: support agents, team leads, helpdesk admins, ops engineers doing bulk/automation work the web UI can't.
- Data profile: tickets (high gravity), threads/comments, contacts, accounts, agents, departments, teams, tasks, time-entries, products, tags, SLAs, KB articles.

## Auth
- OAuth2 refresh-token (self-client) flow. Access token TTL 1h; refresh token permanent until revoked.
- Request header: `Authorization: Zoho-oauthtoken {access_token}` (exact — hyphen, NOT "Bearer").
- Mandatory header on all calls except `/organizations` & `/accessibleOrganizations`: `orgId: {orgId}` (exact casing).
- Token endpoint: `POST https://{accountsHost}/oauth/v2/token?grant_type=refresh_token&client_id=..&client_secret=..&refresh_token=..` → `{access_token, expires_in, api_domain}`. accountsHost is per-DC and MUST match the DC the client was registered in.
- Env vars (canonical): `ZOHO_DESK_CLIENT_ID`, `ZOHO_DESK_CLIENT_SECRET`, `ZOHO_DESK_REFRESH_TOKEN`, `ZOHO_DESK_ORG_ID`, `ZOHO_DC` (default us). Optional cache: `ZOHO_DESK_ACCESS_TOKEN`.
- Scopes: `Desk.tickets.ALL`, `Desk.contacts.ALL`, `Desk.basic.ALL` (orgs/agents/depts), `Desk.tasks.ALL`, `Desk.settings.ALL`, `Desk.search.READ`, `Desk.articles.ALL`.

## Reachability Risk
- None/Low. Official authed REST API. Probe `GET /api/v1/organizations` (no token) → HTTP 401 `{"errorCode":"UNAUTHORIZED"}`, clean JSON, no Cloudflare/bot challenge. v1 current and stable.
- Common failure modes are config, not blocking: `INVALID_OAUTH`/401 (orgId vs token-org mismatch, or wrong DC accounts host), `invalid_client` (DC mismatch). The CLI must make DC+orgId correctness easy.

## Top Workflows
1. Triage / bulk-reassign — list tickets by status/assignee, PATCH or reassign to rebalance queues.
2. Reply automation — sendReply/draftReply; applyBlueprint to drive process state.
3. SLA & metrics reporting — pull /tickets/{id}/metrics, /slas, agent counts; breach/first-response stats.
4. Contact/CRM sync — paginate contacts by email, upsert; associate accounts.
5. Bulk export/backup — paginated tickets + threads + comments + attachments dump; close/trash housekeeping.

## Table Stakes (from Zendesk/Freshdesk ops CLIs + Zapier's 22 Desk actions)
- auth login (OAuth refresh persistence), multi-DC + multi-org profiles
- ticket list/get/create/update/close/merge/reply/comment
- bulk ticket update from CSV; full export/backup
- search with filters; attachments; agents/contacts/accounts CRUD
- JSON output for piping

## Data Layer
- Primary entities: tickets, contacts, accounts, agents, departments, teams, tasks, threads, comments, time-entries, tags, products.
- Sync cursor: `modifiedTime` / `sortBy=-modifiedTime` paged with `from`/`limit` (max 100, 1-based).
- FTS/search: ticket subject, contact name/email, account name, agent name — all into local SQLite for offline query + the transcendence commands.

## Pagination & Rate Limits
- `from` 1-based offset (default 1), `limit` (default 10, max 100; some legacy lists 50). `sortBy={f}` asc, `-{f}` desc. `include=`, `fields=` sparse.
- Credit + concurrency model (not req/min). 429 `TOO_MANY_REQUESTS`. Headers `X-Rate-Limit-Remaining-v3`, `Retry-After`. CLI must honor Retry-After, exponential backoff, bounded concurrency (5-10).

## Ecosystem (greenfield — this is the wedge)
- NO Zoho Desk CLI exists anywhere (GitHub/npm/PyPI). Closest hits are Zoho *Projects* CLIs.
- No official Node/Python/PHP Desk SDK. Only first-party = stale Java SDK (zohodesksdk 1.3.0, 2020).
- Best community source = thomas-kl1/php-sdk-zoho-desk (PHP, active v3.0.4 2024): Gateway facade, CRUD ops, ListCriteriaBuilder (filter/field-select), ConfigProvider (multi-DC, sandbox, orgId, token persistence). Read for ground-truth.
- Zapier MCP exposes 22 Desk actions = the absorb checklist (triggers, create/update ticket/contact/account, add comment/attachment, send email reply, search ticket/find-or-create contact).
- Zoho's own hosted MCP supports Desk (no public source).

## Spec Decision
- No official OpenAPI/Swagger for Desk (Zoho publishes OAS only for CRM). Hand-author an internal YAML spec from the documented contract. Browser-sniff/crowd-sniff skipped (skip-silent): the documented public REST API is authoritative; the Desk web UI's internal API is a different surface.

## Product Thesis
- Name: zoho-desk-pp-cli ("deskcli").
- Why it should exist: the only scriptable, agent-native interface to Zoho Desk. Handles the three things every thin wrapper gets wrong — OAuth token refresh, multi-DC+orgId correctness, and auto-pagination+auto-backoff — then adds offline SQLite, search, and SLA/triage analytics no UI exposes.

## Build Priorities
1. Foundation: OAuth refresh-token client (auto-refresh on 401/expiry), DC+orgId resolution (auto-detect orgId from /organizations), rate-limit backoff, SQLite store for primary entities, sync + search + sql.
2. Absorb: full CRUD + ops for tickets/threads/comments/contacts/accounts/agents/departments/tasks/time-entries/tags/products + reply/merge/close/bulk-update + search + export.
3. Transcend: SLA-breach radar, agent-load balancer, triage queue, ticket-thread digest, stale-ticket sweep, contact 360 — all offline joins over synced SQLite.
