# ClickUp CLI Build Log

## Generated (Priority 0 + 1)
- 137-operation v2 surface from official OpenAPI 3.1 spec (tasks, lists, folders, spaces, comments, time tracking, views, goals, custom fields, checklists, tags, guests, webhooks, relationships, members, groups, templates, attachments).
- Local SQLite store (resources + typed task table), FTS5 search, sync (parent-keyed dependent walker: team -> team_task).
- Cloudflare MCP pattern auto-applied (137 endpoints > 50): code orchestration, hidden endpoint tools, stdio+http transport.
- Auth: personal token in Authorization header (no Bearer), env CLICKUP_API_TOKEN/CLICKUP_TOKEN.

## Built (Priority 2 - transcendence, all 7 shipping scope, hand-built)
1. changed-since - snapshot-diff activity delta (added pm_snapshot table in store extras).
2. my-day - offline triage across all lists, due-window + assignee filter.
3. workload - assignee balance (tasks + estimates + running timers join).
4. time-in-status - per-status dwell from synced time_in_status history; --rank.
5. stale - ClickUp-aware (date_updated ms-epoch) stale detection + status filter.
6. unblocked / blocked - dependency-aware ready/blocked set.
7. resolve - offline status/assignee/NL-date resolver, zero API calls.

## User-approved addition
- docs (v3 API): list, get, pages, page-content, page-get, page-create, page-edit, create. Same client/base-url/auth; writes are print-by-default + --confirm.

## Removed
- Framework pm_stale.go (newStaleCmd): queried generic resources table with RFC3339/epoch fields; could not match ClickUp's date_updated ms-epoch strings. Replaced by the ClickUp-aware novel `stale`.

## Tests
- novel_pm_test.go: table-driven tests for parseMSField, parsePMTask, open/closed, matchAssignee, parseDurationWindow, resolveDue, parseClock, fingerprint diff, accumulateStatusMinutes. All pass.

## Behavioral verification (synthetic store, 6 tasks)
- my-day ordering, stale (40d/20d + status), workload (priya 3h / marco 0.5h+timer / unassigned), unblocked vs blocked (t3 blocked by open t1), time-in-status aggregation (review 600 / open 360 / in progress 120), changed-since diff (status transition), resolve (me->id, date, status) all correct.

## Notes / deferred
- resolve status-id: when a task status name exactly matches but the list-status (with id) only substring-matches, the id can be empty; in real ClickUp data task and list status names are identical so the list id is returned. Minor.
- Docs v3 write commands require --confirm; not live-tested without a token.
