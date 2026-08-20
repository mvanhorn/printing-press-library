# Freshservice CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | List tickets (paginated) | effytech MCP get_tickets | `tickets list` with `--page`, `--per-page`, `--status`, `--priority` | Offline FTS, `--json`, `--select`, `--csv` |
| 2 | Get ticket by ID | effytech MCP get_ticket_by_id | `tickets get <id>` | `--json` with full field dump + `--select` |
| 3 | Create ticket | effytech MCP create_ticket | `tickets create --subject --description --priority --status --source --email` | `--dry-run`, `--json` response |
| 4 | Update ticket | effytech MCP update_ticket | `tickets update <id> --status --priority --assignee-id --group-id` | `--dry-run`, typed exit codes |
| 5 | Delete ticket | effytech MCP delete_ticket | `tickets delete <id>` | `--dry-run`, confirmation gate |
| 6 | Filter tickets by query | effytech MCP filter_tickets | `tickets filter --query "status:2 AND priority:3"` | Auto-quoting of query string, `--json` |
| 7 | Create ticket note | effytech MCP create_ticket_note | `tickets note <id> --body "..."` | `--dry-run`, stdin support |
| 8 | Reply to ticket | effytech MCP send_ticket_reply | `tickets reply <id> --body "..." --cc --bcc` | `--dry-run`, stdin |
| 9 | List ticket conversations | effytech MCP list_all_ticket_conversation | `tickets conversations <id>` | Offline-readable with `--json` |
| 10 | Get ticket activities | Freshservice API /activities | `tickets activities <id>` | Full audit trail in JSON |
| 11 | Get ticket form fields | effytech MCP get_ticket_fields | `tickets fields` | Enumerate valid values for creation |
| 12 | List changes | effytech MCP get_changes | `changes list --status --priority` | Offline with `--json` |
| 13 | Get change by ID | effytech MCP get_change_by_id | `changes get <id>` | Full field dump |
| 14 | Create change | effytech MCP create_change | `changes create --subject --type --priority --impact --risk` | `--dry-run`, enum validation |
| 15 | Update change | effytech MCP update_change | `changes update <id> --status --priority` | `--dry-run` |
| 16 | Delete/close change | effytech MCP delete_change / close_change | `changes delete <id>` / `changes close <id>` | `--dry-run` |
| 17 | Filter changes | effytech MCP filter_changes | `changes filter --query "status:1 AND priority:3"` | Auto-quoting |
| 18 | List change tasks | effytech MCP get_change_tasks | `changes tasks list <change-id>` | `--json` |
| 19 | Create change task | effytech MCP create_change_task | `changes tasks create <change-id> --title --description` | `--dry-run` |
| 20 | Update change task | effytech MCP update_change_task | `changes tasks update <change-id> <task-id> --status` | `--dry-run` |
| 21 | Delete change task | effytech MCP delete_change_task | `changes tasks delete <change-id> <task-id>` | `--dry-run` |
| 22 | Change time entries (CRUD) | effytech MCP create/list/update/delete_change_time_entry | `changes time-entries <change-id> [create/list/update/delete]` | `--dry-run` |
| 23 | Change notes (CRUD) | effytech MCP create/list/update/delete_change_note | `changes notes <change-id> [create/list/update/delete]` | Offline readable |
| 24 | Change approval groups | effytech MCP create/list/update/cancel_change_approval_group | `changes approvals group <change-id> [create/list/cancel]` | `--dry-run`, structured JSON |
| 25 | Change approval chain | effytech MCP update_approval_chain_rule_change | `changes approvals chain <change-id> --type parallel\|sequential` | `--dry-run` |
| 26 | List/view change approvals | effytech MCP list_change_approvals / view_change_approval | `changes approvals list <change-id>` / `changes approvals get <change-id> <approval-id>` | `--json`, monitor pending |
| 27 | Resend/cancel approval reminder | effytech MCP send/cancel_change_approval | `changes approvals resend <change-id> <approval-id>` / `cancel` | `--dry-run` |
| 28 | Change form fields | effytech MCP list_change_fields | `changes fields` | Enum validation for creation |
| 29 | List assets | Freshservice API /assets | `assets list --search --filter` | Offline FTS search |
| 30 | Get asset by display_id | Freshservice API /assets/{display_id} | `assets get <display-id>` | `--json` |
| 31 | Create asset | Freshservice API POST /assets | `assets create --name --asset-type-id --...` | `--dry-run` |
| 32 | Update asset | Freshservice API PUT /assets/{display_id} | `assets update <display-id> --...` | `--dry-run` |
| 33 | Delete/restore asset | Freshservice API DELETE/PUT restore | `assets delete <display-id>` / `assets restore <display-id>` | `--dry-run` |
| 34 | List asset components | Freshservice API /assets/{id}/components | `assets components <display-id>` | `--json` |
| 35 | List asset requests | Freshservice API /assets/{id}/requests | `assets requests <display-id>` | Linked tickets view |
| 36 | List asset contracts | Freshservice API /assets/{id}/contracts | `assets contracts <display-id>` | `--json` |
| 37 | List requesters | effytech MCP get_all_requesters | `requesters list` | Offline FTS by name/email |
| 38 | Get requester by ID | effytech MCP get_requester_id | `requesters get <id>` | `--json` |
| 39 | Create requester | effytech MCP create_requester | `requesters create --first-name --last-name --email` | `--dry-run` |
| 40 | Update requester | effytech MCP update_requester | `requesters update <id> --...` | `--dry-run` |
| 41 | Filter requesters | effytech MCP filter_requesters | `requesters filter --query "..."` | Auto-quoting |
| 42 | Requester fields | effytech MCP list_all_requester_fields | `requesters fields` | |
| 43 | Deactivate/reactivate requester | Freshservice API DELETE/PUT reactivate | `requesters deactivate <id>` / `requesters reactivate <id>` | `--dry-run` |
| 44 | Convert requester to agent | Freshservice API PUT convert_to_agent | `requesters convert <id>` | `--dry-run` |
| 45 | Merge requesters | Freshservice API PUT merge | `requesters merge --primary-id --secondary-ids` | `--dry-run` |
| 46 | List agents | effytech MCP get_all_agents | `agents list` | Offline FTS |
| 47 | Get agent | effytech MCP get_agent | `agents get <id>` | `--json` |
| 48 | Create agent | effytech MCP create_agent | `agents create --first-name --last-name --email --roles` | `--dry-run` |
| 49 | Update agent | effytech MCP update_agent | `agents update <id> --...` | `--dry-run` |
| 50 | Agent fields | effytech MCP get_agent_fields | `agents fields` | |
| 51 | List agent groups | effytech MCP get_all_agent_groups | `groups list` | `--json` |
| 52 | Get/create/update agent group | effytech MCP getAgentGroupById/create_group/update_group | `groups get/create/update` | `--dry-run` |
| 53 | List requester groups | effytech MCP get_all_requester_groups | `requester-groups list` | `--json` |
| 54 | Manage requester group members | effytech MCP list/add_requester_to_group | `requester-groups members <id>` / `requester-groups add-member` | `--dry-run` |
| 55 | List service catalog items | effytech MCP list_service_items | `catalog list` | `--json` |
| 56 | Place service request | effytech MCP create_service_request | `catalog request <item-id> --...` | `--dry-run` |
| 57 | List products | effytech MCP get_all_products | `products list` | `--json` |
| 58 | Get/create/update product | effytech MCP get_products_by_id/create_product/update_product | `products get/create/update` | `--dry-run` |
| 59 | List canned responses | effytech MCP get_all_canned_response | `canned-responses list` | `--json` |
| 60 | Get canned response | effytech MCP get_canned_response | `canned-responses get <id>` | `--json` |
| 61 | List workspaces | effytech MCP list_all_workspaces | `workspaces list` | `--json` |
| 62 | List solution categories | effytech MCP get_all_solution_category | `solutions list` | `--json` |
| 63 | Get/create solution category | effytech MCP get/create_solution_category | `solutions get/create` | `--dry-run` |
| 64 | SQL query against local store | Steampipe plugin (SQL interface) | `sql "<SELECT ...>"` | Any ad-hoc query across synced data |
| 65 | Full-text search | (no existing tool) | `search "<term>"` | Cross-entity FTS across tickets+assets+changes+requesters |

## Transcendence (only possible with our approach)
*(Novel features being brainstormed by subagent — will populate after subagent completes)*

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| — | SLA breach risk | `tickets breach-risk --hours N` | Requires local store with ticket SLA data + time math |
| — | Agent workload | `agents workload` | Requires local join across open tickets + assignees |
| — | Change risk score | `changes risk-score <id>` | Combines impact+priority+approval status locally |
| — | Ticket velocity | `tickets velocity --days 30` | Requires historical snapshots in SQLite |
| — | Stale tickets | `tickets stale --days N` | Local time-windowed query, no single API call |
| — | Requester profile | `requesters profile <id>` | Cross-entity join: tickets + assets + changes for one requester |
| — | My queue | `me queue` | Local filter by authenticated user's assignments + SLA proximity |

---

## Transcendence (only possible with our approach)

| # | Feature | Command | Why Only We Can Do This |
|---|---------|---------|------------------------|
| 1 | SLA Breach Countdown | `breach-risk [--hours 4] [--group X] [--assignee me]` | Requires computing `due_by - now` against the full open-ticket corpus + group/agent joins simultaneously |
| 2 | My Queue | `my-queue [--agent email] [--json]` | Cross-entity join: open tickets + pending change approvals for one agent in one offline query |
| 3 | Cross-Entity Full-Text Search | `search "<query>" [--in tickets,assets,changes,kb]` | SQLite FTS5 across all entity corpora with unified ranking; API has per-entity search only |
| 4 | Agent Workload Heatmap | `workload [--group X] [--json]` | Full open-ticket corpus grouped by assignee + group join — Freshservice UI shows one agent at a time |
| 5 | Change Collision Detector | `change-collisions [--window 48h] [--ci "prod-db-01"]` | Self-join on changes table over time ranges + CI cross-reference; no Freshservice UI surfaces this |
| 6 | Incident Recurrence Fingerprinter | `recurrence [--asset FS-1042] [--days 90]` | FTS similarity across historical ticket corpus + asset/requester joins; API approach hits rate limits |
| 7 | Knowledge Gap Finder | `kb-gaps [--group X] [--days 30] [--min-tickets 3]` | Simultaneous FTS across ticket and KB article corpora to find coverage gaps |
| 8 | Asset Orphan Detector | `orphan-assets [--type laptop] [--days 60]` | Left-join across assets + tickets + contracts + users; no combined Freshservice UI view exists |
| 9 | Department SLA Leaderboard | `dept-sla [--period 30d] [--sort breach-rate]` | Grouping full resolved-ticket history by department — Freshservice Analytics is a separate paid add-on |
| 10 | On-Call Coverage Gap Finder | `oncall-gap [--group X] [--period 4w] [--severity P1,P2]` | Correlates ticket timestamps vs. group membership over time — no Freshservice endpoint provides this |

---

## Additional Resources Discovered (api.freshservice.com/v2/)

These resources are documented in the official API but not covered by the effytech MCP server. Mark as absorb scope:

| # | Feature | Source | Status |
|---|---------|--------|--------|
| 66 | List/CRUD Problems | Freshservice API /problems | Stub — ITIL problem management |
| 67 | List/CRUD Releases | Freshservice API /releases | Stub — software release tracking |
| 68 | List Departments | Freshservice API /departments | Full implementation |
| 69 | List Locations | Freshservice API /locations | Full implementation |
| 70 | List Software (ITAM) | Freshservice API /software | Stub — software asset management |
| 71 | List Contracts | Freshservice API /contracts | Full implementation |
| 72 | List Vendors | Freshservice API /vendors | Full implementation |
| 73 | SLA Policies | Freshservice API /sla_policies | Read-only list |
| 74 | Announcements | Freshservice API /announcements | Read-only list |

---

## Ticket API Gaps (MCP covers only 23.5% — adding missing 39 endpoints)

| # | Feature | Source | Our Implementation |
|---|---------|--------|-------------------|
| 75 | Restore deleted ticket | Official API | `tickets restore <id>` |
| 76 | Move ticket to workspace | Official API | `tickets move <id> --workspace-id` |
| 77 | Delete conversation | Official API | `tickets conversations delete <ticket-id> <conv-id>` |
| 78 | Delete conversation attachment | Official API | `tickets conversations delete-attachment <ticket-id> <conv-id> <attach-id>` |
| 79 | Create ticket task | Official API | `tickets tasks create <ticket-id> --title --description` |
| 80 | List ticket tasks | Official API | `tickets tasks list <ticket-id>` |
| 81 | Get ticket task | Official API | `tickets tasks get <ticket-id> <task-id>` |
| 82 | Update ticket task | Official API | `tickets tasks update <ticket-id> <task-id>` |
| 83 | Delete ticket task | Official API | `tickets tasks delete <ticket-id> <task-id>` |
| 84 | Create ticket time entry | Official API | `tickets time-entries create <ticket-id> --time-spent --note` |
| 85 | List ticket time entries | Official API | `tickets time-entries list <ticket-id>` |
| 86 | Get ticket time entry | Official API | `tickets time-entries get <ticket-id> <entry-id>` |
| 87 | Update ticket time entry | Official API | `tickets time-entries update <ticket-id> <entry-id>` |
| 88 | Delete ticket time entry | Official API | `tickets time-entries delete <ticket-id> <entry-id>` |
| 89 | Create child ticket | Official API | `tickets child-create <parent-id> --subject --description` |
| 90 | Add requested item to ticket | Official API | `tickets requested-items add <ticket-id> --item-id` |
| 91 | Update requested item | Official API | `tickets requested-items update <ticket-id> <item-id>` |
| 92 | Get ticket activities | Official API | `tickets activities <ticket-id>` |
| 93 | Get CSAT response | Official API | `tickets csat <ticket-id>` |
| 94 | Delete ticket attachment | Official API | `tickets delete-attachment <ticket-id> <attach-id>` |
| 95 | Create ticket approval | Official API | `tickets approvals create <ticket-id>` |
| 96 | List ticket approvals | Official API | `tickets approvals list <ticket-id>` |
| 97 | Get ticket approval | Official API | `tickets approvals get <ticket-id> <approval-id>` |
| 98 | Update approval | Official API | `tickets approvals update <approval-id>` |
| 99 | Delete approval | Official API | `tickets approvals delete <approval-id>` |
| 100 | Send approval reminder | Official API | `tickets approvals remind <approval-id>` |
| 101 | Create ticket approval group | Official API | `tickets approval-groups create <ticket-id>` |
| 102 | List ticket approval groups | Official API | `tickets approval-groups list <ticket-id>` |
| 103 | Update ticket approval group | Official API | `tickets approval-groups update <ticket-id> <group-id>` |
| 104 | Delete ticket approval group | Official API | `tickets approval-groups delete <ticket-id> <group-id>` |
