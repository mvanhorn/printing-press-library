# HaloPSA Absorb Manifest

## Source Tools (catalog)

| Tool | Type | Language | URL | Notes |
|---|---|---|---|---|
| homotechsual/HaloAPI | PS module | PowerShell | github.com/homotechsual/HaloAPI | Dominant wrapper. Full CRUD across tickets/clients/sites/assets/agents/users/contracts/invoices/KB/reports. Batch helpers. Token caching. Azure Key Vault integration. Automatic retry. Recommends OAuth2 client_credentials. |
| mspautomator/Halo_ServiceDesk | PS module | PowerShell | github.com/mspautomator/Halo_ServiceDesk | Earlier PS wrapper. Username/password auth. Basic CRUD. |
| HaloPSA (FryanO) | SDK | Python | pypi.org/project/HaloPSA | Pre-alpha. `pip install HaloPSA`. All base endpoints implemented except DELETE. `halo.Users.search()` pattern. Env vars HALO_CLIENT_ID/HALO_SECRET/HALO_TENANT. |
| greenlighttec/pyhaloapi | SDK | Python | github.com/greenlighttec/pyhaloapi | Modular GET/POST wrapper. Token refresh on expiry. `connect(url, clientid, clientsecret)`. |
| amplify-msp/py-halo | SDK | Python | github.com/amplify-msp/py-halo | Python wrapper for HaloPSA. |
| panoramicdata/HaloPsa.Api | NuGet | C# | github.com/panoramicdata/HaloPsa.Api | C# SDK on NuGet. |
| ssmanji89/halopsa-workflows-mcp | MCP server | Python | github.com/ssmanji89/halopsa-workflows-mcp | MCP exposing getWorkflows / getWorkflowSteps / getWorkflow / deleteWorkflow / createWorkflows / healthcheck. OAuth2 client_credentials. |
| ssmanji89/haloapi-mcp-tools | MCP server | Python | glama.ai/mcp/servers/@ssmanji89/haloapi-mcp-tools | Schema explorer + arbitrary `halopsa_api_call`. 800+ table schema discovery; SQL query tool. |
| @adamhancock/halopsa-mcp | MCP server | TypeScript | npmjs.com/package/@adamhancock/halopsa-mcp | npm MCP for HaloPSA. |
| Switchboard666/halopsa-mcp | MCP server | TypeScript | glama.ai/mcp/servers/@Switchboard666/halopsa-mcp | DB schema + endpoint discovery + arbitrary API call. |
| tim-impendingtech-halopsa-mcp-server | MCP server | TypeScript | lobehub.com/mcp/tim-impendingtech-halopsa-mcp-server | MCP HaloPSA server. |
| lwhitelock/HaloPSA-Automation | scripts | PowerShell | github.com/lwhitelock/HaloPSA-Automation | M365 contact sync, Halo automation patterns. |
| n8n HaloPSA node | integration | n8n | n8n.io/integrations/halopsa | Workflow integration node. |

## Auth ground-truth (from MCP source + SDK READMEs)

- **Mechanism:** OAuth2 client_credentials at `https://<tenant>.halopsa.com/auth/token`. POST body `grant_type=client_credentials&client_id=<id>&client_secret=<secret>&scope=all`. Returns JSON `{access_token, expires_in, token_type:"Bearer"}`.
- **Use:** `Authorization: Bearer <access_token>` on every `<tenant>.halopsa.com/api/...` request.
- **Env var convention** (cross-wrapper consensus): `HALOPSA_CLIENT_ID`, `HALOPSA_CLIENT_SECRET`, `HALOPSA_TENANT` (or `HALO_CLIENT_ID`/`HALO_SECRET`/`HALO_TENANT` in some wrappers).
- **Token caching:** Refresh on expiry using same credentials; HaloAPI and pyhaloapi both do this automatically.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-----------|
| 1 | OAuth2 client_credentials connect | HaloAPI `Connect-HaloAPI` | `auth login --client-id --client-secret --tenant` + token cache to `~/.halopsa-pp-cli/auth.json` | Cross-platform (no PS), agent-friendly env-var auth, `--json` exit codes |
| 2 | Token refresh on expiry | HaloAPI/pyhaloapi | Auto-refresh in client.go (expires_in - 60s buffer) | Same behavior, headless |
| 3 | List tickets | HaloAPI `Get-HaloTicket` | `tickets list` with --client --team --agent --status --priority --search --since | Offline FTS, --json/--select/--csv, --limit, auto-pagination |
| 4 | Get ticket by id | HaloAPI | `tickets get <id>` with --include-details --include-actions | --json default, agent-shaped |
| 5 | Create ticket | HaloAPI `New-HaloTicket` | `tickets create --summary --client-id --details --priority --status --user-id` | --stdin batch, --dry-run, idempotent on summary+client+date |
| 6 | Update ticket | HaloAPI `Set-HaloTicket` | `tickets update <id> --status --priority --agent-id --team-id ...` | --dry-run, atomic |
| 7 | Close ticket | HaloAPI | `tickets close <id> --action-note "..."` | Pipe-friendly |
| 8 | Delete ticket | HaloAPI `Remove-HaloTicket` | `tickets delete <id>` | Confirmation gate, --yes |
| 9 | List actions on ticket | HaloAPI `Get-HaloAction` | `actions list --ticket-id <id>` | Local cache, --json |
| 10 | Add action / reply / note | HaloAPI `New-HaloAction` | `actions add --ticket-id <id> --outcome --note --hours --time-taken` | --stdin, --dry-run |
| 11 | List clients | HaloAPI `Get-HaloClient` | `clients list --search --include-inactive` | FTS5 over local store |
| 12 | Get client | HaloAPI | `clients get <id>` | --include-sites --include-contracts |
| 13 | Create/Update client | HaloAPI | `clients create/update` | --stdin |
| 14 | List sites | HaloAPI `Get-HaloSite` | `sites list --client-id` | --json |
| 15 | Site CRUD | HaloAPI | `sites get/create/update/delete` | Standard |
| 16 | List users (end-users / contacts) | HaloAPI `Get-HaloUser` | `users list --client-id --site-id --search` | Local FTS |
| 17 | User CRUD | HaloAPI | `users get/create/update/delete` | --stdin |
| 18 | List agents | HaloAPI `Get-HaloAgent` | `agents list --team-id --department-id` | Cached |
| 19 | Agent CRUD | HaloAPI | `agents get/update` | (create/delete admin-gated) |
| 20 | Teams | HaloAPI `Get-HaloTeam` | `teams list/get`, `teams tree` (hierarchy) | --json |
| 21 | List assets | HaloAPI `Get-HaloAsset` | `assets list --client-id --asset-type-id --search` | FTS over tag + name + custom fields |
| 22 | Asset CRUD | HaloAPI | `assets get/create/update/delete` | --stdin |
| 23 | Asset types & fields | HaloAPI | `asset-types list/get` | Cached schema |
| 24 | List contracts | HaloAPI `Get-HaloContract` | `contracts list --client-id --status` | --json |
| 25 | Contract CRUD | HaloAPI | `contracts get/create/update/delete` | Standard |
| 26 | Invoices list/get | HaloAPI `Get-HaloInvoice` | `invoices list/get --client-id --since` | --csv export |
| 27 | Recurring invoices | HaloAPI | `recurring-invoices list/get/create/update/delete` | Standard |
| 28 | Quotations | HaloAPI | `quotations list/get/create/update/delete` | Standard |
| 29 | Purchase orders | spec | `purchase-orders list/get/create/update/delete` | Spec-driven |
| 30 | List KB articles | HaloAPI `Get-HaloKBArticle` | `kb list --search --category` | FTS5 over title+body |
| 31 | KB article CRUD | HaloAPI | `kb get/create/update/delete` | --stdin |
| 32 | KB vote | spec POST /KBArticle/vote | `kb vote <id> --up/--down` | Spec-driven |
| 33 | Statuses | HaloAPI `Get-HaloStatus` | `statuses list/get` | Cached |
| 34 | Workflows | ssmanji89 MCP getWorkflows | `workflows list/get/create/delete` | Full coverage |
| 35 | Workflow steps | ssmanji89 MCP getWorkflowSteps | `workflows steps <id>` | --json |
| 36 | Ticket rules | spec /TicketRules | `ticket-rules list/get/create/update/delete` | Spec-driven |
| 37 | Ticket types / request types | spec /TicketType | `ticket-types list/get` | Cached |
| 38 | Priorities | spec /Priority | `priorities list/get` | Cached |
| 39 | Departments | spec | `departments list/get` | Standard |
| 40 | Custom fields | spec /CustomField | `custom-fields list/get` | --json |
| 41 | Custom field schemas | HaloAPI | bundled into entity --include-extras | Auto-merged |
| 42 | Reports list/get | HaloAPI `Get-HaloReport` | `reports list/get` | --json |
| 43 | Run report | spec /Report/print | `reports run <id> --params @file.json` | Pipe-friendly |
| 44 | Time entries / timesheets | HaloAPI | `time list --agent-id --since --billable` | Local aggregation |
| 45 | Add time entry | HaloAPI | `time add --ticket-id --hours --note --billable` | --dry-run |
| 46 | Appointments | spec /Appointment | `appointments list/get/create/update/delete` | Standard |
| 47 | Canned responses | spec /CannedText | `canned list/get/create/update/delete` | --stdin |
| 48 | Notifications | spec /Notifications | `notifications list/get/mark-read` | Standard |
| 49 | Dashboards | spec /DashboardLinks | `dashboards list/get` | --json |
| 50 | Attachments — download | HaloAPI Attachment | `attachments get <id> --output <path>` | Stream to stdout if --output - |
| 51 | Attachments — upload | HaloAPI | `attachments upload --ticket-id <id> --file <path>` | --dry-run |
| 52 | Integration data (cloud SaaS catalog) | spec /IntegrationData (157 paths) | `integrations list/get` (subset; admin-only ones gated) | Coverage exposure |
| 53 | Address book | spec /Addressbook | `addressbook list/get/create/update/delete` | Standard |
| 54 | Service status / outages | spec /ServiceStatus | `service-status list/get` | --json |
| 55 | Auto-pagination on lists | All SDKs | All list commands stream via auto-page; `--limit N` caps | Server-driven without ceremony |
| 56 | Search clients/tickets/assets via API | n8n / SDKs | server-side via `?search=` on list endpoints | Backed by FTS5 locally |
| 57 | Health check | ssmanji89 MCP healthcheck | `doctor` | Same + verbose with timing + auth proof |
| 58 | Schema explorer | ssmanji89 MCP halopsa_list_tables/columns/api_endpoints | `schema endpoints`, `schema fields <entity>` | Pre-baked from spec |
| 59 | Search API endpoints | ssmanji89 MCP halopsa_search_api_endpoints | `schema search "<word>"` | Offline, no API call needed |
| 60 | Arbitrary API call | ssmanji89 MCP halopsa_api_call | `api <METHOD> <path> [--body @file.json] [--query k=v]` | Escape hatch for the long tail |
| 61 | SQL exec (local store) | ssmanji89 MCP halopsa_query | `sql "SELECT * FROM tickets WHERE ..."` | SELECT-only over local SQLite (not against the live DB) |
| 62 | Build query helper | ssmanji89 MCP halopsa_build_query | `sql --explain` (show query plan) | Native sqlite EXPLAIN |
| 63 | Pre-flight validate | HaloAPI Invoke-HaloPreFlightCheck | `doctor --strict` (auth + permissions + sample request) | Same |
| 64 | Batch helpers | HaloAPI *Batch | `tickets update --stdin` (each line=one update json) | --stdin / --concurrency N |
| 65 | Retry on 429/5xx | HaloAPI | client.go middleware with backoff | Built-in |

Total absorbed: **65 features**. This is the table-stakes scope. Everything every other tool offers, we offer.

## Transcendence (only possible with our approach)

13 novel features survived the adversarial cut, all scoring 6/10 or higher. These exploit the local SQLite store and cross-entity joins that no API call, no MCP server, and no PowerShell cmdlet can do.

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Triage queue | `triage [--team T] [--agent A]` | 10/10 | Joins local tickets × agents × statuses in SQLite to render per-agent load, stale count, and 24h breach count in one table | Brief Top Workflow #1, Build Priority #3, Maria persona |
| 2 | Bulk age-out closer | `tickets age-out --status "X" --stale-days N [--apply]` | 9/10 | Local SQLite filter on status_id + lastactiondate; --apply issues batch close + templated action via API | Brief Top Workflow #2; absorb #6/#10; Maria's Monday ritual |
| 3 | SLA breach radar | `sla breaching --within 24h` | 9/10 | Selects local tickets.targetdate BETWEEN now AND now+N, joins agent + client, sorts by time-to-breach | Build Priority #3 ("about to breach SLA"); ticket field `targetdate` in spec |
| 4 | Agent workload | `agents load [--team T]` | 8/10 | Joins local tickets × actions × time_entries × agents to compute open/touched-this-week/billable-hours/oldest-open per agent | Build Priority #3 ("Who's overloaded"); Maria + Priya frustration |
| 5 | Client desk card | `clients card <id-or-name>` | 10/10 | Cross-joins clients + sites + tickets (open) + contracts (active + hours used) + assets (count) + kb_links to render one panel | Brief Top Workflow #3; Build Priority #3 ("client health card"); Devon's defining frustration |
| 6 | Asset ticket history | `assets history <tag-or-id>` | 7/10 | Filters local tickets by asset_id chronologically with agent + time per ticket | Brief Top Workflow #3; absorb #20-23; CMDB pattern from MSP domain |
| 7 | KB suggest for ticket | `kb suggest --ticket <id>` | 9/10 | Local FTS5 query against kb_articles using the ticket's summary + details + last action body as search text; ranks top 5 | Brief Top Workflow #5; absorb #30-32 (KB FTS5); Devon persona |
| 8 | Time gap finder | `time gaps --agent me --week current` | 8/10 | Set-diff: tickets where agent appears in actions.who_id for the week MINUS tickets where agent has a time_entries row that week | Brief Top Workflow #4; absorb #44-45; Devon's Friday archaeology |
| 9 | Contract burn-down | `contracts burn [--client X]` | 9/10 | Sums local time_entries.hours WHERE billable AND client_id AND date IN period, compares to contracts.hours_bank, projects overage | Brief Top Workflow #4; absorb #24; Priya billing prep |
| 10 | Ticket rules dump | `rules dump [--workflow W]` | 6/10 | Reads local ticket_rules + workflows + statuses and prints each rule's conditions → actions as flat text | Brief Top Workflow #6; absorb #33-40 |
| 11 | Changed-since | `tickets changed-since <when> [--mine]` | 9/10 | Queries local store for tickets/actions with lastupdated >= when, groups by ticket; backed by incremental sync cursor | Brief Data Layer (sync cursor); Sam persona; absorb #56 |
| 12 | Standup digest | `standup --team T --since yesterday` | 7/10 | Aggregates closed-count, reopened-count, time-logged, top-client per agent for the window from local tickets + actions + time_entries | Maria daily standup; Build Priority #3 |
| 13 | Multi-client overlay | `clients overlay --metric open_tickets --top 10` | 8/10 | Group-by + rank on local clients joined to a chosen metric table (tickets / time_entries / contracts); pluggable metric param | Brief Data Layer (MSP multi-client); Build Priority #3; Priya persona |

## Stubs and gated features

None. Every absorbed and transcendence feature ships as a working implementation. No stubs.

## Dropped candidates

- **Billable hours export** — thin wrapper over `time list --billable --csv` already absorbed (#44-45)
- **Recurring-pattern finder** — drifted into NLP-shaped territory; weekly utility unclear
- **Action templates apply** — Halo already has canned text (#47); thin rewrap
- **Tickets watch mode** — scope creep toward TUI/daemon; one-shot `changed-since` covers the real need

(Full audit trail with kill reasons in `2026-05-21-131235-novel-features-brainstorm.md`.)

