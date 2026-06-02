# ClickUp CLI Brief

## API Identity
- Domain: Project / work management. Hierarchy: Workspace (team) > Space > Folder > List > Task, plus Comments, Time Tracking, Views, Goals/OKRs, Custom Fields, Checklists, Tags, Guests, Webhooks, Task Relationships.
- Users: Developers and ops people who live in the terminal; AI agents driving task management; team leads wanting reporting.
- Data profile: Deeply nested, relational, slow-changing. Tasks are the gravity center (status, assignees, due dates, custom fields, time-in-status, relationships). Verbose JSON payloads (tens of KB per list call).
- Spec: Official OpenAPI 3.1, 82 paths / 137 operations. Server https://api.clickup.com/api. Auth: personal token in `Authorization` header (security scheme `Authorization_Token`), or OAuth. Docs are a SEPARATE v3 API (not in the v2 spec).

## Reachability Risk
- None. Public documented API. Unauthenticated GET returns HTTP 400/401 (real API response demanding auth), no bot protection.

## Top Workflows
1. Triage / list tasks: "what's assigned to me, due soon, in this list/space" with filters (assignee, status, due date).
2. Create / update tasks fast from the terminal (status, assignee, due date, custom fields, markdown description).
3. Time tracking: start/stop timer, log entries, query timesheets by range.
4. Reporting: sprint board grouped by status, what changed recently, workload across the team.
5. Search across the workspace for a task/comment by text.

## Table Stakes (must match incumbents)
- Full CRUD across Workspaces, Spaces, Folders, Lists, Tasks, Comments, Time Tracking, Views, Goals, Custom Fields, Checklists, Tags, Guests, Webhooks, Task Relationships. (blockful does 134/135 endpoints; we match the 137-op surface.)
- JSON-default output, `--json`/`--select`/`--csv`, typed exit codes, `--dry-run` on mutations.
- Custom Task IDs (`--custom-task-ids` + `--team-id`), markdown task descriptions.
- Workspace-level task search with filters.
- Fuzzy status matching and assignee resolution ("me", name, id) -- triptechtravel's standout UX.
- Sprint board view grouped by status; timesheets by date range.
- Shell completions, non-interactive auth via env var.

## Data Layer (what deserves a local store)
- Primary entities: workspaces(teams), spaces, folders, lists, tasks, comments, time_entries, custom_fields, tags, members/users, goals, views, webhooks.
- Sync cursor: task `date_updated` (ms epoch); incremental sync per list/space.
- FTS/search: FTS5 over task name + description + comment text + custom field values. NO incumbent persists locally -- this is the wedge.

## Codebase Intelligence
- blockful/clickup-cli (Go): the bar. 27 command groups, 134/135 endpoints, JSON default, custom task IDs, markdown, --dry-run, exit codes. Weakness: no offline store, no FTS, no cross-entity analytics. Also supports Docs v3.
- triptechtravel/clickup-cli (Go): dev UX. Git branch->task linking, sprint dashboard grouped by status, timesheets by range, fuzzy status matching, inbox, @mentions, chat messages.
- taazkareem/clickup-mcp-server (~150 tools): natural language dates, deep search, fuzzy ID resolution, bulk ops, chat, audit logs, multi-account.
- Auth pattern observed: `Authorization: <personal_token>` header (no "Bearer" prefix for personal tokens), env var `CLICKUP_TOKEN` / `CLICKUP_API_TOKEN`.

## Product Thesis
- Name: clickup-pp-cli
- Why it should exist: Every ClickUp CLI today is a thin API mirror -- one verbose call per command, online-only, no memory. None persist your workspace locally, so none can answer "what changed since yesterday," "who's overloaded," "how long do tasks sit in review," or full-text search offline. We match the full 137-endpoint surface AND add a local SQLite store with FTS and cross-entity analytics nobody else has.

## Build Priorities
1. Priority 0 -- data layer for all primary entities + sync + FTS5 search + SQL passthrough.
2. Priority 1 -- match the full spec surface: tasks, lists, folders, spaces, comments, time tracking, views, goals, custom fields, checklists, tags, guests, webhooks, relationships. Plus absorbed UX: fuzzy status, assignee "me" resolution, custom task IDs, markdown, natural language dates.
3. Priority 2 -- transcendence: offline workspace search, "what changed since", workload balance, time-in-status / cycle-time analytics, stale-task detection, sprint velocity. (See absorb manifest.)
