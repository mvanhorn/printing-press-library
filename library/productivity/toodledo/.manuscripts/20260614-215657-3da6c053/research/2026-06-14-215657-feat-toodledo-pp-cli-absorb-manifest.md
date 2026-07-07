# Toodledo CLI — Absorb Manifest

**Scope:** 39 absorbed features (from toodledo-mcp's 17 tools + competitor table-stakes + full v3 API surface) + 6 transcendence features. Best existing tool is the user's own `toodledo-mcp` (17 GTD tools) and the abandoned `poodledo` (v2). No tool offers offline store + SQL + JSON-everywhere + agent-native MCP; that's our beat-everything edge.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List/get tasks (incomplete default) | toodledo-mcp `search_tasks`; poodledo `getTasks` | (generated endpoint) tasks list | `--json`/`--select`, store-backed offline, no rate burn |
| 2 | Get single task w/ full detail | toodledo-mcp `get_task` | (generated endpoint) tasks get | offline from store, `--json` |
| 3 | Add task (rich) | toodledo-mcp `add_task`; todoist quick-add | toodledo-pp-cli tasks add | name→id resolution, NL due dates, `--dry-run` |
| 4 | Edit task (partial) | toodledo-mcp `edit_task` | toodledo-pp-cli tasks edit | partial update, name→id, `--dry-run` |
| 5 | Complete task | toodledo-mcp `complete_task`; TaskWarrior `done` | toodledo-pp-cli tasks complete | sets completed ts, multi-id batch, `--dry-run` |
| 6 | Reopen completed task | todoist `reopen` | toodledo-pp-cli tasks reopen | clears completed, idempotent |
| 7 | Delete task | toodledo-mcp `delete_task` | toodledo-pp-cli tasks delete | multi-id batch (≤50), `--dry-run`, typed exit codes |
| 8 | Filter by folder (project) | toodledo-mcp; todoist project | (behavior in toodledo-pp-cli tasks list) `--folder <name>` | name resolution, offline |
| 9 | Filter by context | toodledo-mcp | (behavior in toodledo-pp-cli tasks list) `--context <name>` | offline |
| 10 | Filter by GTD status | toodledo-mcp | (behavior in toodledo-pp-cli tasks list) `--status <enum>` | enum decode |
| 11 | Filter by priority | toodledo-mcp | (behavior in toodledo-pp-cli tasks list) `--priority <enum>` | enum decode |
| 12 | Filter by star | toodledo-mcp `starred` | (behavior in toodledo-pp-cli tasks list) `--star` | |
| 13 | Include/exclude completed | toodledo-mcp `include_completed` | (behavior in toodledo-pp-cli tasks list) `--completed` | |
| 14 | Full-text search titles+notes | toodledo-mcp `search_text`; todoist search | (behavior in toodledo-pp-cli search) FTS5 | offline, cross-resource (tasks+notes+outlines) |
| 15 | Sort tasks | TaskWarrior; tod | (behavior in toodledo-pp-cli tasks list) `--sort priority\|due\|added` | |
| 16 | List folders | toodledo-mcp `list_folders` | (generated endpoint) folders list | archived filter, `--json` |
| 17 | Add folder | toodledo-mcp `add_folder` | (generated endpoint) folders add | form-encoded |
| 18 | Edit/rename/archive folder | poodledo `editFolder` | (generated endpoint) folders edit | |
| 19 | Delete folder | toodledo-mcp `delete_folder` | (generated endpoint) folders delete | |
| 20 | List contexts | toodledo-mcp `list_contexts` | (generated endpoint) contexts list | |
| 21 | Add context | toodledo-mcp `add_context` | (generated endpoint) contexts add | |
| 22 | Edit context | poodledo | (generated endpoint) contexts edit | |
| 23 | Delete context | toodledo-mcp `delete_context` | (generated endpoint) contexts delete | |
| 24 | List goals (by level) | toodledo-mcp `list_goals` | (generated endpoint) goals list | level decode |
| 25 | Add goal | toodledo-mcp `add_goal` | (generated endpoint) goals add | level + contributes |
| 26 | Edit goal | poodledo | (generated endpoint) goals edit | |
| 27 | Delete goal | poodledo | (generated endpoint) goals delete | |
| 28 | List locations | poodledo `Location` | (generated endpoint) locations list | lat/lon |
| 29 | Add location | poodledo | (generated endpoint) locations add | |
| 30 | Edit location | poodledo | (generated endpoint) locations edit | |
| 31 | Delete location | poodledo | (generated endpoint) locations delete | |
| 32 | Notes list/add/edit/delete | toodledo-python Notes; poodledo Notebook | (generated endpoint) notes list / notes add / notes edit / notes delete | searchable offline |
| 33 | Outlines list/add/edit/delete | Toodledo v3 API | (generated endpoint) outlines list / outlines add / outlines edit / outlines delete | |
| 34 | Custom lists list/add/edit/delete | Toodledo v3 API | (generated endpoint) lists list / lists add / lists edit / lists delete | |
| 35 | Account info / Pro status | toodledo-python `GetAccount` | (generated endpoint) account get | drives sync cursors + Pro-subtask warning |
| 36 | Incremental sync to local store | sachaos `sync`; toodledo `TaskCache`; tdcli `td_cache` | (behavior in toodledo-pp-cli sync) | `after`/`lastedit_*` cursors, rate-budget aware |
| 37 | SQL over synced data | (GOAT) | (behavior in toodledo-pp-cli sql) | composable, offline |
| 38 | Deleted-items reconciliation | TaskWarrior; sync | (behavior in toodledo-pp-cli reconcile) via `*/deleted.php` | clean offline mirror |
| 39 | Recurring tasks + subtasks | TaskWarrior recurrence; todoist subtasks | (behavior in toodledo-pp-cli tasks add) `--repeat <RRULE>` / `--parent <id>` | iCal RRULE passthrough; non-Pro subtask warning |

Every absorbed feature is also reachable by an agent through the MCP surface (runtime Cobra-tree mirror).

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | GTD next actions | next-actions [--context <name>] [--goal <name>] | 8/10 | hand-code | Local SQLite, joins tasks→contexts/goals, filters status=1, sorts priority desc / due asc | toodledo-mcp `gtd_next_actions`; brief Top Workflow #2 | none |
| 2 | Weekly review buckets | review | 9/10 | hand-code | Five offline aggregations in one pass: inbox (no folder+context), overdue, stalled projects (anti-join), waiting (status=5), someday (status=8) | toodledo-mcp `gtd_review`; brief Top Workflow #3 | Use this command for the full five-bucket weekly review. Do NOT use it when you only want stalled projects; use 'stalled-projects' instead. |
| 3 | Stalled projects | stalled-projects [--days N] | 7/10 | hand-code | LEFT JOIN folders→tasks; folders with incomplete tasks but zero status=1 Next Actions | GTD methodology; toodledo-mcp review stalled bucket | none |
| 4 | Goal progress rollup | goal-progress [--level lifetime\|long\|short] | 7/10 | hand-code | Joins tasks→goals, counts incomplete vs done per goal, walks `contributes` self-reference to roll children into parents | brief Data Layer (goal level 0/1/2 + contributes); API goal hierarchy | none |
| 5 | GTD dashboard | dashboard | 7/10 | hand-code | One offline pass over the local store: incomplete-task counts by status, priority, folder, and context, plus overdue/due-today/starred totals | toodledo-mcp `gtd_dashboard` tool (user-critical: one of the 17 MCP tools); brief Codebase Intel | Use this command for the full multi-axis status board. Do NOT use it for a single grouped count; use 'analytics --type tasks --group-by <field>' instead. |
| 6 | Sync cost preview | sync-cost [--resources <csv>] [--since 7d] | 8/10 | hand-code | Calls real account/get.php cursors, diffs vs local cursors, reports projected API-call count vs the 100-call/token budget without fetching rows | brief Reachability Risk (100 calls/token hard limit); Build Priority #3 | Use this command to preview sync cost only. Do NOT use it to fetch data; use 'sync' for the actual incremental fetch. |
| 7 | Batch capture | capture --file <path> | 7/10 | hand-code | Reads one title/line, resolves folder/context names→ids, writes via the `tasks=<JSON>` batch param in budget-aware chunks of 50 | brief Codebase Intel (50/call batch write + name→id); Top Workflow #1 | none |

## Stubs
None. All 45 features are shipping scope.

## Additional hand-code scope (beyond transcendence rows)
- Ergonomic task writes (absorb #3–7): `tasks add/edit/complete/reopen/delete` build the `tasks=<JSON-array>` batch param + name→id resolution + NL/`YYYY-MM-DD` date parsing. Hand-code (the generator can't ergonomically express the JSON-string batch param).
- Store-backed filtered `tasks list` flags (absorb #8–13, 15): `--folder/--context/--status/--priority/--star/--completed/--sort`.
- OAuth token exchange + refresh must use HTTP Basic `client_id:client_secret` (Toodledo requirement). The generator emits form-encoded creds; a `internal/cli/toodledo_auth.go` override is the planned fix (also flag for retro).
- Cloudflare-aware error handling (403≠401) and a sane `User-Agent` required header.
- Non-Pro subtask detection via `account/get`.
