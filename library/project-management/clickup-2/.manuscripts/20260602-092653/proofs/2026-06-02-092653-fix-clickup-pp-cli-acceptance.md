# ClickUp CLI Acceptance Report

Level: Full live verification (manual full matrix) + binary-owned quick gate
Gate: PASS (phase5-acceptance.json status=pass, 12/12)

## Live environment
- Authenticated against a real ClickUp test workspace (the test workspace) via personal token (read-only).
- Hierarchy sync pulled 818 tasks, 87 lists, 45 folders, 8 spaces across 3 workspaces, plus the authenticated user record.

## Bugs found in live dogfood and FIXED before ship
1. team_task endpoint (GET /v2/team/{id}/task) returns HTTP 500 ECODE ITEMV2_003 on every workspace -> the generator's only task path was broken, so the task store stayed empty.
   Fix: replaced the single team_task dependent with the full reliable hierarchy (team -> space -> folder -> list -> list tasks) in dependentResourceDefs(). Sync now populates 818 tasks with 0 errors.
2. Assignee "me" resolution failed (my-day --assignee me returned 0; resolve --assignee me unresolved) because the authenticated user was not synced.
   Fix: added the user resource (GET /v2/user) to default sync. "me" now resolves to the viewer's id; my-day --assignee me returns the viewer's open tasks.
3. changed-since accepted an invalid scope arg with exit 0 on an empty store (short-circuited before validation).
   Fix: validate the scope argument before any store/dry-run/empty short-circuit.
4. docs write commands printed a plain-text dry-run preview under --json.
   Fix: docsDryRun emits JSON when --json/--agent/piped.

## Flagship feature live results (verified correct against real data)
- my-day: 818 open tasks sorted by due, overdue flagged; --assignee me returns the viewer's open tasks.
- stale --days 30: 590 stale tasks, oldest 336 days, status filter works.
- workload: per-assignee open-task counts + summed estimates (e.g. one member 28h across 9 tasks), unassigned bucket, running-timer detection. Real team members (names redacted).
- unblocked: 793 ready tasks; --blocked: 25 blocked tasks each naming real open blocker ids.
- time-in-status: honest empty (exit 0) when history not synced; aggregation verified against synthetic history.
- changed-since: baseline on first run, stable (0 changes) on second run, status-transition diff verified on a synthetic mutation.
- resolve: status name -> id, assignee me -> viewer id, natural-language dates (friday 5pm, tomorrow, 3d) -> ms epoch.
- search: 50 hits for a term across synced tasks.
- docs (v3): list returned 13 real docs; get/pages/page-get reach the v3 API; writes gated behind --confirm.
- task get --select name,status.status returns only requested fields.
- doctor: auth configured, API reachable, credentials valid.

## Quick gate (binary-owned, deterministic)
- 12/12 passed, status pass. The earlier full run (262/271) surfaced only non-flagship items: two now-fixed (docs no-workspace help, docs json dry-run), one generated endpoint needing a query param (group get-teams1, out of scope), one transient sync error, and a probe substring heuristic.

## PII
- The token is never written to any artifact. Member names and emails from live responses are described generically here.
