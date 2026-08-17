# 3CX XAPI CLI — Absorb Manifest

Sources catalogued: **getrav/3cx-cli** (Python, closest competitor), **SSIG-IT/3cx-mcp-server** (TypeScript MCP, 22 tools), **O-IT/3CX** & **xasz/3CX** & **luxzg/3CX-XAPI_examples** (PowerShell), **werddomain/3CX_XAPI** (C# client), **n8n-nodes-3cx**, **3cx/xapi-tutorial** (official). No tool covers more than ~20 of the 111 collections; none have offline state, config diff, or cross-entity audit.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List/get/create/update/delete users (extensions) | getrav 3cx-cli, SSIG MCP | (generated endpoint) Users list/get/post/patch/delete | Offline mirror, `--json/--select`, `--dry-run`, typed exits |
| 2 | Groups/Departments CRUD | both | (generated endpoint) Groups list/get/post/patch/delete | Same, plus FTS search |
| 3 | Assign role to user in group | getrav 3cx-cli | (generated endpoint) Users role assignment endpoint | Scriptable, idempotent |
| 4 | RingGroups manage | both (read-only) | (generated endpoint) RingGroups CRUD | Full CRUD where API allows, offline |
| 5 | Queues + queue agents/login status | both | (generated endpoint) Queues + AgentsInQueueStatistics | Cross-queue rollup (see transcend) |
| 6 | Receptionists/IVRs | getrav 3cx-cli | (generated endpoint) Receptionists list/get/manage | Offline, agent-native |
| 7 | Trunks list/get/detail | both | (generated endpoint) Trunks + TrunkTemplates | Routing trace (see transcend) |
| 8 | Inbound/Outbound rules | getrav 3cx-cli | (generated endpoint) InboundRules / OutboundRules | Audit + DID map (see transcend) |
| 9 | DID numbers | getrav 3cx-cli | (generated endpoint) DidNumbers | Routing trace |
| 10 | Office hours / Holidays | getrav 3cx-cli | (generated endpoint) OfficeHours / Holidays | Diff-tracked |
| 11 | Parkings | getrav 3cx-cli | (generated endpoint) Parkings | Offline |
| 12 | Active calls (list + drop) | getrav 3cx-cli | (generated endpoint) ActiveCalls list + drop action | Live-state merge (see transcend) |
| 13 | Call history with date/OData filters | both | (generated endpoint) CallHistoryView with `$filter` | Incremental sync cursor |
| 14 | Chat history | generated | (generated endpoint) ChatHistoryView / ChatMessagesHistoryView | Offline |
| 15 | Recordings list/download/delete | getrav 3cx-cli | (generated endpoint) Recordings | Offline index |
| 16 | Contacts search/export/delete | both | (generated endpoint) Contacts | FTS search |
| 17 | Forwarding profiles get/set; extension status | SSIG MCP | (generated endpoint) ForwardingProfiles / DN status | Routing trace |
| 18 | System status (health/version/license/disk) | both | (generated endpoint) SystemStatus (16 paths) | Live-state merge |
| 19 | Event logs / Activity log w/ severity filter | both | (generated endpoint) EventLogs / ActivityLog with `$filter` | Posture + changed feeds |
| 20 | Backups create/restore | getrav 3cx-cli | (generated endpoint) Backups | Snapshot complements |
| 21 | Restart PBX (guarded) | getrav 3cx-cli | (generated endpoint) restart action; mutation guarded `--dry-run` | Safe by default |
| 22 | Emergency numbers / E911 | getrav 3cx-cli | (generated endpoint) EmergencyGeoLocations / EmergencyNotificationsSettings | Offline |
| 23 | SIP devices / phones inventory | getrav 3cx-cli | (generated endpoint) SipDevices / PhonesSettings / DeviceInfos | FTS search |
| 24 | Blacklist numbers; IP blocklist | getrav 3cx-cli | (generated endpoint) BlackListNumbers / Blocklist | Security posture |
| 25 | OData pagination/filter on every list | both | (behavior in 3cx-xapi-pp-cli list commands) `--top/--skip/--filter/--select/--expand/--orderby/--search/--count` flags | Wired through all generated list endpoints |
| 26 | Token auto-refresh (60-min) | both | (behavior in 3cx-xapi-pp-cli auth) OAuth2 client-credentials lifecycle, cached token, auto-refresh | TCX_CLIENT_ID/SECRET env vars, doctor check |
| 27 | Full surface across all 111 collections | none (we exceed all) | (generated endpoint) SBCs, FXS, templates, firmware, CRM/M365/Google/Amazon integration settings, AI settings, fax, network, firewall, prompts, playlists, etc. | Nobody else covers this breadth |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Config integrity audit | `audit` | hand-code | Local join across RingGroups/Queues/InboundRules/OutboundRules/Receptionists destinations vs Users(DN)/DidNumbers — flags dangling references the console cannot detect | Use this command for graph-wide dangling-reference integrity. Do NOT use it for time-based config drift (use 'diff') or one extension's routing paths (use 'trace'). |
| 2 | Config snapshot + diff | `diff` | hand-code | Captures the whole config graph to local SQLite snapshots; row-compares two snapshots (any tenant) for added/removed/changed objects. No console concept of config history | Use this command for config drift between two snapshots or tenants. For broken references right now use 'audit'; for live event activity use 'changed'. |
| 3 | Bulk provision from CSV | `provision` | hand-code | Idempotent POST/PATCH to Users/Groups from CSV with `--dry-run` plan and typed exits — turns hundreds of console clicks into one auditable command | none |
| 4 | Queue/agent performance rollup | `qrollup` | hand-code | Local join across QueuePerformanceOverview + AgentsInQueueStatistics + AbandonedQueueCalls + BreachesSla into one cross-queue, week-over-week table | Use this command for cross-queue aggregate rollups and week-over-week comparison. For live calls use 'now'/active-calls; for raw per-call rows use the call-history list. |
| 5 | Offline directory search | `search` | spec-emits | FTS5 over the mirrored config graph (Users/Groups/RingGroups/Queues/Trunks/DidNumbers/Contacts) by number/name/email/extension without paging the live API | Use this command for fast fuzzy lookup of a known number/name across entity types. For exact OData-filtered lists use the per-resource list command; for what-references-this-extension use 'audit'/'trace'. |
| 6 | Security posture report | `posture` | hand-code | Aggregates Blocklist + BlackListNumbers + AntiHackingSettings + Firewall + failed-auth EventLogs + ReportAuditLog into one attack-surface report | Use this command for the consolidated security/attack-surface report. For raw severity-filtered event rows use the event-log list command. |
| 7 | Extension routing trace | `trace` | hand-code | For one DN, walks InboundRules/RingGroups/Queues/DidNumbers/forwarding in local SQLite to list every routing path reaching the extension | Use this command for all routing paths into one extension. For graph-wide broken references use 'audit'; for free-text lookup use 'search'. |
| 8 | Live state merge | `changed` | hand-code | One-shot time-windowed merge of ActivityLog + EventLogs + ActiveCalls + SystemStatus into a single "state of the PBX now / since T" feed | Use this command for the live activity/event/status merge over a recent window. For structural config drift use 'diff'; for security focus use 'posture'. |

**Hand-code transcendence rows: 7** (audit, snapshot/diff, provision, qrollup, posture, trace, changed). `search` is spec-emitted framework FTS5.

## Stubs
None. All transcendence rows are shipping scope.
