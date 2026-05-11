# Jira CLI Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|-------------------|-------------|
| 1 | Issue list with filtering | ankitpokhrel/jira-cli (5.6k★) | FTS5 offline + JSON/CSV/table | Offline, regex, SQL-composable |
| 2 | Issue create | All tools | --stdin batch, --dry-run, ADF conversion | Agent-native, scriptable |
| 3 | Issue edit/update | jira-cli, go-jira | Any field, ADF support | --dry-run |
| 4 | Issue view | All tools | Full detail: comments, worklogs, links | --json --select |
| 5 | Issue transition | All tools | Named transition lookup | --dry-run |
| 6 | Issue assign | jira-cli, MCP servers | By name or accountId | --dry-run |
| 7 | Issue comment add | All tools | Plain text → ADF conversion | --dry-run |
| 8 | Issue worklog add/list | jira-cli, jira-pilot | Natural language time (2h, 30m, 1d) | --dry-run |
| 9 | Issue link/unlink | jira-cli | All link types (blocks, relates-to, duplicates) | --dry-run |
| 10 | Issue clone | jira-cli | Copies fields, optional new summary | --dry-run |
| 11 | Issue delete | jira-cli | Confirmation prompt | --dry-run |
| 12 | Issue watch/unwatch | jira-cli, jira-pilot | Self or named user | |
| 13 | Issue attach file | jira-pilot, cosmix/jira-mcp | X-Atlassian-Token header handled | |
| 14 | Epic list/create/add/remove | jira-cli | Full epic lifecycle | --dry-run |
| 15 | Sprint list | jira-cli, jira-pilot | Active/future/closed filter | |
| 16 | Sprint add issues | jira-cli | | --dry-run |
| 17 | Project list | All tools | Local cache | |
| 18 | Board list | jira-cli, jira-pilot | Filter by type | |
| 19 | Me / current user | All tools | Validates auth, shows account info | |
| 20 | User search | jira-pilot, MCP servers | By name/email | |
| 21 | JQL search | All tools | Local saved named filters + live JQL | Offline after sync |
| 22 | Open in browser | jira-cli | --launch opt-in, print URL by default | Side-effect safe |
| 23 | JSON/CSV/table output | jira-cli, jira-pilot | --json, --csv, --select, --compact | Full agent support |
| 24 | Shell completion | jira-cli, go-jira | bash/zsh/fish | |
| 25 | Bulk transition/assign/label | jira-pilot, ACLI | JQL-scoped: --jql "project=FOO AND status='To Do'" | --dry-run |
| 26 | Sprint start/complete | jira-pilot | Agile API | --dry-run |
| 27 | Sprint status with metrics | Warzuponus/mcp-jira | Done/remaining/in-progress counts + story points | Offline after sync |
| 28 | Team workload analysis | Warzuponus/mcp-jira | Per-assignee issue counts + story points | Offline |
| 29 | Standup report | jira-pilot, Warzuponus/mcp-jira | Yesterday/Today/Blockers, assignee-scoped | Offline after sync |
| 30 | Subtask create | jira-pilot, cosmix/jira-mcp | parent flag | --dry-run |
| 31 | Epic children list | cosmix/jira-mcp | Sorted by status | |
| 32 | Server info / doctor | OrenGrinker mcp | Auth validation + connectivity | |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| T1 | Cross-sprint velocity | `sprint velocity --project KEY --last N` | 9/10 | SQLite join: sprints × issues; story points committed vs completed per closed sprint | Canonical scrum metric; zero CLI tools have it; Scrum Master persona explicit; jira-pilot has only current sprint status |
| T2 | Assignee workload (cross-project) | `workload [--project KEY,KEY2] [--sprint active]` | 8/10 | SQLite join: open issues × assignee across multiple projects; sum story points; API can't do cross-project | Absorbed #28 single-project MCP only; PM persona "assign blind" frustration explicit; cross-project requires local store |
| T3 | Blocked chain traversal | `blocked [--project KEY] [--depth 2]` | 8/10 | SQLite graph walk on issue_links: follows "is blocked by" links recursively; groups by assignee | Standup blockers slot absorbed but not link-graph traversal; daily standup ritual; requires local store |
| T4 | Cycle time analytics | `cycle-time --project KEY [--type Bug] [--last 30d]` | 9/10 | SQLite: resolved_at − created_at per issue; output p50/p75/p90 + histogram | No CLI tool has this; SLA analysis need; first-class agile metric; no API equivalent |
| T5 | Worklog gap detection | `worklog gaps [--week] [--month] [--user me]` | 8/10 | Synced worklogs + calendar; flag zero-log days with active issue list for that day | Tempo getMissingWorklogDays confirms real pain; consultant/billing persona explicit; unabsorbed |
| T6 | Sprint retro export | `sprint retro [--format md\|csv]` | 7/10 | SQLite: sprint issues grouped Done/Carried over; story points, cycle time, comment count per issue | Scrum Master retro prep ritual; absorbed sprint status (#27) has counts only; no existing tool does this |
| T7 | Issue changelog diff | `issue changelog <KEY> [--field status,assignee]` | 7/10 | /rest/api/3/issue/{key}/changelog; field-by-field transition log with author+timestamp | Real API endpoint; debugging "who changed this" workflow; no surveyed tool surfaces it readably |
| T8 | Stale issue detector | `stale [--days 14] [--sprint active]` | 7/10 | SQLite: updated_at < now−N days AND status not closed; shows age, assignee, last comment date | Standard scrum anti-pattern; PM triage workflow; no configurable staleness CLI exists |
