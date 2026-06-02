# ClickUp CLI Absorb Manifest

## Source tools
- blockful/clickup-cli (Go) - 134/135 endpoints, JSON default, custom task IDs, markdown, --dry-run
- triptechtravel/clickup-cli (Go) - git linking, sprint board, timesheets, fuzzy status, inbox, mentions, chat
- taazkareem/clickup-mcp-server (~150 tools) - NL dates, deep search, fuzzy ID resolution, bulk ops
- dang3r/clickupy (Python) - library + CLI
- hauptsacheNet/clickup-mcp - read-minimal/read/write modes, task context with comments

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Task CRUD | blockful, taazkareem | endpoint-mirror + store cache | offline get, --json/--select |
| 2 | Workspace/Space/Folder/List CRUD | blockful | endpoint-mirror | offline hierarchy walk |
| 3 | Workspace task search w/ filters | blockful, triptech | endpoint-mirror | composable --select |
| 4 | Comments CRUD + threaded replies | blockful, taazkareem | endpoint-mirror | offline read |
| 5 | Time tracking: timer + manual entries + history | all | endpoint-mirror | --dry-run safe |
| 6 | Timesheets by date range | triptech | endpoint-mirror | csv export |
| 7 | Views CRUD + view tasks | blockful | endpoint-mirror | — |
| 8 | Goals/Key-Results CRUD | blockful | endpoint-mirror | — |
| 9 | Custom Fields list/set/remove | blockful, taazkareem | endpoint-mirror | — |
| 10 | Task Checklists + items | blockful, taazkareem | endpoint-mirror | — |
| 11 | Tags CRUD + add/remove | all | endpoint-mirror | — |
| 12 | Guests invite/edit/remove + access | blockful | endpoint-mirror | — |
| 13 | Webhooks CRUD | blockful, taazkareem | endpoint-mirror | — |
| 14 | Task dependencies + links | blockful, taazkareem | endpoint-mirror | — |
| 15 | Members/Users, User Groups, Roles | blockful | endpoint-mirror | — |
| 16 | Templates (task/list/folder) | blockful | endpoint-mirror | — |
| 17 | Attachments upload | blockful, triptech | endpoint-mirror | — |
| 18 | Custom Task IDs (--custom-task-ids + --team-id) | blockful | flag plumbing | — |
| 19 | Markdown task descriptions | blockful, triptech | flag plumbing | — |
| 20 | Fuzzy status matching | triptech | local resolver | works offline |
| 21 | Assignee resolution (me/name/id) | triptech | local resolver | works offline |
| 22 | Sprint board grouped by status | triptech | endpoint-mirror + grouping | — |
| 23 | Natural language dates | taazkareem | local parser | — |
| 24 | Full-text search across synced workspace | NONE persist locally | FTS5 store | the wedge |
| 25 | SQL passthrough over local store | NONE | sqlite | composable analytics |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|-------------------------|
| 1 | Activity delta since last sync | `changed-since <ts\|last>` | 8 | Diffs two synced task snapshots in local SQLite; no API call answers "what changed since yesterday" |
| 2 | My-day offline triage | `my-day [--due 7d]` | 8 | Local read of my tasks across all lists, sorted by due, overdue/stuck flags, fully offline |
| 3 | Assignee workload balance | `workload --space <id>` | 7 | Local join tasks+members+time_entries; ClickUp gates workload behind paid Dashboards |
| 4 | Time-in-status / cycle-time | `time-in-status <list\|task> [--rank]` | 8 | Captures status-change timestamps at sync into a history table; computes per-status dwell |
| 5 | Stale task detection | `stale --days <n> [--status review]` | 7 | Local query for tasks with no movement in N days |
| 6 | Unblocked / blocked dependencies | `unblocked` / `blocked` | 7 | Local join tasks↔dependencies↔status returns the transitive ready-to-work set |
| 7 | Offline batch resolver | `resolve --status/--assignee/--due` | 7 | Local lookup over synced statuses/members + NL date parser; zero API calls for agents |

All transcendence features are shipping scope (none are stubs).

## Docs v3 (user-approved addition, hand-built Phase 3 — separate v3 API)
Base off the same client (server api.clickup.com/api; /v3/... paths resolve). Same personal-token auth.
| # | Feature | Method + Path | Command |
|---|---------|---------------|---------|
| D1 | Search/list docs | GET /v3/workspaces/{wid}/docs | `docs list --workspace <wid>` |
| D2 | Create doc | POST /v3/workspaces/{wid}/docs | `docs create --workspace <wid> --name <n>` |
| D3 | Get doc | GET /v3/workspaces/{wid}/docs/{doc_id} | `docs get <doc_id> --workspace <wid>` |
| D4 | Page listing | GET /v3/workspaces/{wid}/docs/{doc_id}/pageListing | `docs pages <doc_id> --workspace <wid>` |
| D5 | Get pages (content) | GET /v3/workspaces/{wid}/docs/{doc_id}/pages | `docs page-content <doc_id> --workspace <wid>` |
| D6 | Create page | POST /v3/workspaces/{wid}/docs/{doc_id}/pages | `docs page-create <doc_id> --name <n> --content <md>` |
| D7 | Get page | GET /v3/workspaces/{wid}/docs/{doc_id}/pages/{page_id} | `docs page-get <doc_id> <page_id>` |
| D8 | Edit page | PUT /v3/workspaces/{wid}/docs/{doc_id}/pages/{page_id} | `docs page-edit <doc_id> <page_id> --content <md>` |

All Docs commands are shipping scope (read commands testable live; write commands --dry-run + honest messaging).
