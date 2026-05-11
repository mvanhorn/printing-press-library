# Chainels Absorb Manifest

## Absorbed (every endpoint as typed Cobra command + MCP tool)

| # | Family | Source | Coverage |
|---|--------|--------|----------|
| 1 | Companies (46 ops) | Chainels OpenAPI 3.1 | list/get/create/update/delete + accounts, entities, signup requests, pinned accounts |
| 2 | Communities (9) | Chainels OpenAPI | get/list/members/spaces/channels |
| 3 | Accounts + Roles + Profile (5+3+6) | Chainels OpenAPI | `/accounts/me`, entity roles, profile |
| 4 | Messages + Replies (13+4) | Chainels OpenAPI | post/list/get/update/delete + threaded replies |
| 5 | Timeline (2) | Chainels OpenAPI | community timeline read |
| 6 | Events (7) + Survey (7) | Chainels OpenAPI | create/list/respond |
| 7 | Issues + Issue actions (5+15) | Chainels OpenAPI | report, list categories, assign, transitions |
| 8 | Alarm config (3) + Alarm actions (8) | Chainels OpenAPI | configure recipients, fire, ack, reply |
| 9 | Bans + Ban access + Store ban (3+4+1) | Chainels OpenAPI | grant/revoke bans across community + store |
| 10 | Periodic Reporting (Schemes 6 + Reports 10 + Statistics 1) | Chainels OpenAPI | scheme CRUD, submit/list reports, stats |
| 11 | Turnover Reporting (Schemes 5 + Reports 6) | Chainels OpenAPI | scheme CRUD, submit/list reports |
| 12 | Invoices (6) | Chainels OpenAPI | list/get/create/update + linkage |
| 13 | Discounts (5 + deprecated 4 + workflow 2) | Chainels OpenAPI | offer create/list/state |
| 14 | Agreements + Accounts + Items (5+5+5) | Chainels OpenAPI | lease-like agreement CRUD, parties, line items |
| 15 | Payments (5) | Chainels OpenAPI | list/get + status |
| 16 | Request Forms (Forms 2 + Submissions 5 + Workflow 5) | Chainels OpenAPI | form CRUD, submissions + transitions |
| 17 | Bookings (4) + Bookables (4) + Workflow (3) | Chainels OpenAPI | rooms/resources + reservation lifecycle |
| 18 | Footfall + Energy + Metrics (3+3+5) | Chainels OpenAPI | time-series metrics |
| 19 | Service Accounts (13) | Chainels OpenAPI | machine-account CRUD + token mgmt |
| 20 | Files (6) | Chainels OpenAPI | upload/list/delete |
| 21 | Channels (2) + Spaces (4) | Chainels OpenAPI | community channels/spaces |
| 22 | AI Rules (3) + Groups (3) | Chainels OpenAPI | automation rules + member groups |
| 23 | Invite Templates (5) | Chainels OpenAPI | onboarding invite templates |
| 24 | User Profile (6) + Warnings (3) | Chainels OpenAPI | profile + warning records |
| 25 | OAuth (3 grants) | Chainels OAuth docs + `Chainels/oauth2-provider-php` | authorization_code, client_credentials, group_token |

**Total absorbed:** 236 operations auto-promoted to typed CLI subcommands + MCP tools.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Persona | How It Works | Evidence |
|---|---------|---------|-------|---------|--------------|----------|
| 1 | Cross-community FTS | `search` (framework) | 9/10 | Maya | FTS5 index over synced messages/issues/agreements in local SQLite; one query spans every community | Brief Data Layer ("Property managers cannot grep across communities in the web UI today") |
| 2 | Issue assignee load | `issues load` | 8/10 | Maya | Local groupby on synced `/issues` rows; assignee × age-bucket counts | Brief P2; workflow #2 |
| 3 | Stale issue digest | `issues stale` | 7/10 | Maya | Local query: issues `updated_at < now - N`, cross-community | Brief P2 |
| 4 | Turnover variance | `turnover variance` | 8/10 | Maya, Rashid | Per-tenant variance vs trailing-N median on synced `/turnover/reports` | Brief P2; workflow #3 |
| 5 | Turnover laggards | `turnover pending` | 8/10 | Maya | Set-difference between expected submitters and actual reports for a period | Workflow #3 |
| 6 | Agreement renewals | `agreements renewals` | 7/10 | Maya, Devi | Local filter on `agreements.end_at` within window | Brief P2; workflow #5 |
| 7 | Member-load audit | `members audit` | 7/10 | Maya, Devi | Join accounts + entity-roles + community members; per-account role counts, dup/orphan flag | Brief P2; workflow #5 |
| 8 | Alarm recipient diff | `alarms diff` | 6/10 | Ines, Maya | Diff two alarm-config snapshots (two communities, or one community over time) | Brief P2 |
| 9 | Since-sync changed | `changed` | 7/10 | Devi | Union of `updated_at >= ts` across messages/issues/bookings/agreements/turnover | Brief Data Layer + P2 |

All 9 features score >= 5/10. None are stubs. All are local-SQLite-or-join-shaped, none require external services or LLM calls.

## Build Plan

- **Priority 0 (framework auto):** SQLite store + FTS5 + `sync` for primary entities (companies, communities, accounts, messages, issues, bookings, agreements, alarms, turnover reports, periodic reports, invoices, files).
- **Priority 1 (framework auto):** 236 endpoint-mirror subcommands. Auth: OAuth client_credentials default flow (CHAINELS_CLIENT_ID + CHAINELS_CLIENT_SECRET).
- **Priority 2 (hand-built):** 8 hand-built novel commands (skip #1; framework's built-in `search` covers it).
- **MCP:** Cloudflare pattern — `transport: [stdio, http]`, `orchestration: code`, `endpoint_tools: hidden`. 236+8 tools → ~1K-token agent context via `<api>_search + <api>_execute`.
