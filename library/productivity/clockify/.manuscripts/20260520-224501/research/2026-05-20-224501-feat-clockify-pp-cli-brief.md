# Clockify CLI Brief

## API Identity
- **Domain:** Time tracking, timesheets, and team time management. Clockify (by CAKE.com) is the most-used free time tracker.
- **Users:** Freelancers and consultants logging billable hours; agency/team members filling weekly timesheets; managers approving timesheets, running reports, and invoicing.
- **Data profile:** Workspaces -> projects -> tasks; time entries (the core record); tags, clients; expenses; invoices; time-off (PTO) policies/requests/balances; holidays; scheduling assignments; approval requests; users/user-groups; custom fields; cost/hourly rates; webhooks.
- **Spec:** Official live OpenAPI 3.0.1 served at `https://api.clockify.me/api/v3/api-docs` — **99 paths, 155 operations** (48 GET, 41 POST, 27 PUT, 23 DELETE, 16 PATCH). Base host `https://api.clockify.me/api`.
- **Auth:** `X-Api-Key` header (per-user API key from Profile Settings -> API). Also `x-addon-token` and `x-marketplace-token` schemes for add-ons (not relevant to a user CLI). Env var: `CLOCKIFY_API_KEY`.

## Reachability Risk
- **None.** Official documented REST API. `GET /api/v3/api-docs` -> 200 (live spec). `GET /api/v1/workspaces` -> 401 (auth required, as expected). No bot protection, no community reports of blocked programmatic access. Spec-based generation, no discovery needed.

## Top Workflows
1. **Track time** — start/stop a timer, see what's running, edit the entry afterward. The everyday loop.
2. **Fill the weekly timesheet** — the page the user pointed at (`app.clockify.me/timesheet`). Enter hours per project/task per day, review the week grid, submit for approval.
3. **Log time after the fact** — add a completed entry with explicit start/end or duration; clone yesterday's entry.
4. **Report billable hours** — summary by project/client/day for a date range; export for invoicing or a client update.
5. **Manage projects/tasks/clients/tags** — the scaffolding power users curate so time entries land in the right buckets.

## Table Stakes
Drawn from the competitor landscape (every feature below exists in at least one shipping tool — our CLI must match all of them):
- Timer: `start` (interactive or clone-previous), `stop`, `status`/in-progress, `discard` active timer.
- Time entries: list by day/week/range, `add` manual (duration or start/end), `edit`, `delete`, `duplicate`/clone.
- **Bulk** update / delete of time entries (JeremyVyska MCP).
- Projects: list (filter archived/client/name), create (color, billable), update (budget/estimate), delete.
- Tasks, clients, tags: full CRUD with archive/unarchive.
- Workspaces: list, switch active, get, update.
- Users / user-groups: list.
- Reports: today / this-week / last-month / current-month / custom range; CSV export.
- Week-view / timesheet grid; breaks report (BlythMeister).
- Config: init/set/view; `auth login`/`status`/`logout`.
- Intelligent time input (24h/12h/abbreviated, smart AM/PM) — BlythMeister.
- Time-entry templates (artefactual-labs).
- Jira import + Tempo upload (BlythMeister) — niche; treat as out-of-scope unless user asks.

## Competitor Landscape
| Tool | Lang | Stars | Surface |
|------|------|-------|---------|
| lucassabreu/clockify-cli | Go | 194 | Most-used. Timer + entries + reports + projects/workspaces/users/tags/clients + config. Actively maintained (v0.63.0, Mar 2026). |
| JeremyVyska/clockify-mcp | TS | — | 33-tool MCP. Full CRUD across workspaces/clients/projects/tasks/tags + 7 time-entry tools incl. bulk update/delete + timer ops. |
| BlythMeister/ClockifyCli | C# | 1 | Jira/Tempo bridge. add/start/stop/status/edit(split)/week-view/breaks-report/timer-monitor. |
| mentarch/clockify-cli | TS | 0 | Timer + pause/resume + reports today/week/custom + CSV export + offline caching. |
| artefactual-labs/clockify-tool | Ruby | — | Entry CRUD + time-entry templates. |
| inakianduaga / aslamanver MCP servers | — | — | Time entries, projects, summary reports. |
| Python wrappers | Python | — | clockify-sdk, clockify-api-client, clockifyclient — library methods (workspaces, entries, projects, reports). |

**No competing CLI touches:** invoices, expenses, time-off/PTO, holidays, approval requests, scheduling assignments, custom fields, cost rates, webhooks. The full official API surface is wide open.

## Data Layer
- **Primary entities:** workspaces, projects, tasks, clients, tags, time-entries, users, expenses, invoices, time-off policies/requests, holidays.
- **Highest-gravity entity:** `time-entries` — every transcendence feature joins against it.
- **Sync cursor:** `time-entries` GET supports `start`/`end` date range and pagination; sync the trailing window. Workspace `entities/created|updated|deleted` endpoints give a change feed for incremental sync.
- **FTS/search:** time-entry descriptions, project names, task names, client names, tag names — all worth full-text indexing for `search`.

## Why Install This Instead Of The Incumbent
lucassabreu/clockify-cli is the strong incumbent — but it is a **timer + thin-report wrapper**. It has no local store, so it cannot answer any question that needs history or a cross-entity join, and it covers only ~1/3 of the API. This CLI:
- Keeps a **local SQLite mirror** of every entry, project, client, tag — so reports, gaps, trends, and timesheet reconstruction run offline and instantly, and survive across weeks.
- Covers the **entire official API**: invoices, expenses, PTO, approvals, scheduling — features no competing CLI exposes.
- Ships **agent-native** output everywhere: `--json`, `--select` dotted-path filtering, typed exit codes, `--dry-run` on every mutation.
- Treats the **timesheet** (the page the user cares about) as a first-class object: reconstruct the week grid offline, find untracked gaps, submit for approval — none of which the incumbent does.

## User Vision
- User pointed at `https://app.clockify.me/timesheet` — the weekly timesheet entry page. Treat the **timesheet workflow as the headline**: the week grid, gap detection, and approval submission are flagship surfaces, not afterthoughts. No additional briefing context was given.

## Product Thesis
- **Name:** Clockify (display) / `clockify-pp-cli` (binary).
- **Why it should exist:** Every Clockify CLI today is a timer with a thin reporting bolt-on. None remembers anything. A power user filling weekly timesheets, chasing billable hours, and answering "where did my week go?" needs the data to *persist and be queryable* — and needs the half of Clockify (invoices, expenses, PTO, approvals) that no terminal tool touches. This CLI is the first Clockify tool that is a local time database, not a remote-API puppet.

## Build Priorities
1. **Data layer** for time-entries + projects + tasks + clients + tags + workspaces; sync + FTS search + SQL path.
2. **Timer + entry core** — start/stop/status/discard, add/edit/delete/duplicate/bulk, the everyday loop.
3. **Timesheet family** — week-grid reconstruction (offline), gap detection, approval submit/list. The headline.
4. **Full API coverage** — projects/tasks/clients/tags/users CRUD, then expenses/invoices/time-off/holidays/scheduling/approvals/custom-fields/rates/webhooks.
5. **Transcendence** — offline reports, billable/untagged audits, time-where-did-my-week-go, trends from historical snapshots (see absorb manifest).
