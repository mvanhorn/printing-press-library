# Redmine CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Issues CRUD + filtering (project/tracker/status/priority/assignee/custom fields, sort, include journals/relations/attachments/watchers/children) | onozaty/redmine-mcp-server (`getIssues`/`getIssue`/`createIssue`/`updateIssue`/`deleteIssue`) + spec `/issues.{format}` | (generated endpoint) issues list/get/create/update/delete | Offline search+SQL once synced, `--dry-run`, typed exit codes |
| 2 | Issue watchers | onozaty (`addWatcher`/`removeWatcher`) + spec `/issues/{id}/watchers` | (generated endpoint) issues watchers add/remove/list | agent-native JSON |
| 3 | Issue relations (blocks/precedes/duplicates/relates) | onozaty (`addRelatedIssue`/`removeRelatedIssue`) + spec `/issues/{id}/relations` | (generated endpoint) issues relations add/list/delete | `--dry-run` |
| 4 | Projects CRUD + archive/unarchive/close/reopen | onozaty (`createProject`/`archiveProject`/`closeProject`) + spec `/projects*` | (generated endpoint) projects create/update/delete/archive/unarchive/close/reopen/list/get | |
| 5 | Project memberships (roles per user/group per project) | onozaty (`getMemberships`/`createMembership`) + spec `/projects/{id}/memberships` | (generated endpoint) memberships list/create/update/delete | |
| 6 | Users CRUD + current user | onozaty (`getUsers`/`getCurrentUser`) + spec `/users*` | (generated endpoint) users list/get/create/update/delete/current | |
| 7 | Groups + group membership | onozaty (`createGroup`/`addUserToGroup`) + spec `/groups*` | (generated endpoint) groups list/get/create/update/delete/add-user/remove-user | |
| 8 | Time entries CRUD, filtered by project/issue/user/date/activity | onozaty (`getTimeEntries`) + spec `/time_entries*`, `/projects/{id}/time_entries`, `/issues/{id}/time_entries` | (generated endpoint) time-entries list/get/create/update/delete | |
| 9 | Versions (roadmap) CRUD | onozaty (`getVersions`/`createVersion`) + spec `/projects/{id}/versions`, `/versions/{id}` | (generated endpoint) versions list/get/create/update/delete | |
| 10 | Wiki pages CRUD + version history | onozaty (`getWikiPages`/`getWikiPageByVersion`) + spec `/projects/{id}/wiki/*` | (generated endpoint) wiki list/get/get-version/update/delete | |
| 11 | News CRUD, project-scoped and global | onozaty (`getNewsList`/`createNews`) + spec `/news*`, `/projects/{id}/news` | (generated endpoint) news list/get/create/update/delete | |
| 12 | Issue categories CRUD | onozaty (`getIssueCategories`) + spec `/projects/{id}/issue_categories`, `/issue_categories/{id}` | (generated endpoint) issue-categories list/get/create/update/delete | |
| 13 | Attachments + files: upload (2-step token), download, thumbnail, project files list/add | onozaty (`uploadAttachment*`/`downloadThumbnail*`) + spec `/uploads`, `/attachments/*`, `/projects/{id}/files` | (generated endpoint) attachments upload/get/update/delete/download; files list/add | |
| 14 | Reference data: trackers, issue statuses, enumerations (priorities/time-entry-activities/document-categories), roles, custom fields, queries | onozaty (`getTrackers`/`getIssueStatuses`/`getIssuePriorities`/`getRoles`/`getCustomFields`/`getQueries`) + spec | (generated endpoint) trackers list; statuses list; enumerations priorities/activities/doc-categories; roles list/get; custom-fields list; queries list | |
| 15 | Search (cross-entity live search) | onozaty (`search`) + spec `/search`, `/projects/{id}/search` | (generated endpoint) search | |
| 16 | My account (get/update) | onozaty (`getMyAccount`/`updateMyAccount`) + spec `/my/account` | (generated endpoint) my-account get/update | |
| 17 | Journals (update a journal / journal notes) | onozaty (`updateJournal`) + spec `/journals/{id}` | (generated endpoint) journals update | |
| 18 | Repository revisions ↔ issues linking | spec `/projects/{id}/repository/{repo_id}/revisions/{rev}/issues*` (unique to this spec — not in onozaty's tool list) | (generated endpoint) repository-issues list/get | |
| 19 | Sync/search/SQL local data layer | Printing Press framework | redmine-pp-cli sync / search / sql | offline, composable, works from cron/CI not just an agent chat window |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|--------------------------|-------------------|
| 1 | Roadmap burndown | `roadmap burndown <version> [--project X]` | hand-code | Redmine's Roadmap UI has no REST equivalent — requires a local join of synced `issues` + `versions` + `time_entries` to compute open/closed counts, avg done_ratio, and estimated-vs-logged hours | Use this for version/roadmap progress and close-readiness. Do NOT use it for a single issue's status; use `issues get` instead. |
| 2 | Assignee workload/overload report | `workload [--project X] [--threshold N]` | hand-code | Requires grouping the local `issues` table by `assigned_to_id` and summing open-issue counts + estimated hours per user — no aggregate endpoint exists | Use this for an aggregate view across all assignees. Do NOT use it to see one person's issue list; use `issues list --assigned-to <user>` instead. |
| 3 | Stale issue detection | `issues stale [--project X] --days 14` | hand-code | Redmine has no "stale" concept; requires filtering the local `issues` table on `updated_on` age while status remains open | Use this to find issues that have gone quiet (no recent activity). Do NOT use it for recently changed issues; use `digest` instead — its semantics are the opposite (recent activity, not inactivity). |
| 4 | Since-last-sync digest | `digest --since 7d [--mine] [--watched]` | hand-code | Requires join-and-filter logic across `issues` on `created_on`/`updated_on`/`closed_on`, optionally scoped to the current user's assignments or watches — no single Redmine endpoint provides this | Use this for a personal activity report over a time window. Do NOT confuse with the framework's `sync --since`, which controls what data is pulled from the API, not what is reported from local data. Do NOT use for inactivity detection; use `issues stale` instead. |
| 5 | Cycle-time / time-to-resolution by tracker | `issues cycle-time --group-by tracker [--project X]` | hand-code | Requires computing avg(`closed_on` - `created_on`) grouped by tracker/project over the local `issues` table — a duration aggregation the generic `analytics` command (row counts only) cannot do | Use this for duration-based aggregation (average days to close). Do NOT use the generic `analytics` command for this — it only counts rows per group, it does not compute durations. |
| 6 | Transitive blocker-chain traversal | `issues blockers <id> [--depth N]` | hand-code | Requires a recursive traversal over the local `issue_relations` table to walk multi-hop "blocks"/"blocked by" chains — the API only returns one issue's direct relations per call | Use this for the full transitive dependency chain across multiple issues. Do NOT use it for a single issue's direct relations; use `issues relations list <id>` instead (one hop only). |

Minimum 5 transcendence features required — 6 delivered, all scoring >= 6/10 (four score 9-10/10). See `20260826-150701-novel-features-brainstorm.md` for the full customer model, pre-cut candidate pool, and kill reasons for the 8 candidates cut in the adversarial pass.

## Stubs

None. All 19 absorbed rows generate directly from the `d-yoshi/redmine-openapi` spec; all 6 transcendence rows are approved shipping-scope hand-code commands with no external dependency or paid-tier blocker.
