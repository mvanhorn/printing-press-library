# Novel Features Brainstorm: Jira CLI

## Customer model

**Persona 1: The Senior Engineer (daily CLI user)**
Today: Opens Jira in browser to check what's assigned, closes it, goes back to terminal. Switches context 8–12 times per day. Weekly ritual: Monday sprint planning tab sprawl, Friday closes stale issues. Frustration: No way to see "what did I actually touch this week" without clicking through 20 tickets.

**Persona 2: The Scrum Master / Delivery Lead**
Today: Pulls sprint board in browser before standup, manually counts done/in-progress/blocked. Weekly ritual: Sprint review prep — compares velocity across last 3 sprints. Frustration: Velocity data exists in Jira but extracting it requires a paid plugin or tedious JQL + counting.

**Persona 3: The PM / Team Lead**
Today: Triages new bugs overnight, assigns, transitions. Weekly ritual: Checks assignee load before assigning new work. Frustration: Assigning work blind without knowing who is actually busy vs. idle.

**Persona 4: The Consultant / Agency Dev**
Today: Works across 3–5 Jira projects simultaneously. Weekly ritual: Generates client status summaries. Frustration: No single view across projects; time tracking is manual busywork.

## Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|--------------|----------|
| S1 | Cross-sprint velocity | `jira sprint velocity --project KEY --last N` | 9/10 | SQLite join of sprints × issues: closed sprint story points committed vs completed | Sprint velocity is canonical scrum metric; jira-pilot has sprint status but not history; Scrum Master persona explicit |
| S2 | Assignee workload with cross-project | `jira workload [--project KEY,KEY2] [--sprint active] [--by label/component]` | 8/10 | SQLite join open issues × assignee; cross-project aggregation impossible via single API call | Absorbed #28 covers one project only; PM persona "assign blind" frustration; cross-project join is new |
| S3 | Blocked issue chain traversal | `jira blocked [--project KEY] [--assignee me] [--depth 2]` | 8/10 | SQLite graph walk on issue_links table following "is blocked by" links recursively | Standup report (#29) names blockers but doesn't traverse link graph; daily ritual for Senior Engineer/Scrum Master |
| S4 | Cycle time distribution | `jira cycle-time --project KEY [--type Bug] [--last 30d]` | 9/10 | SQLite date arithmetic: resolved_at - created_at; p50/p75/p90 + histogram | No existing CLI surfaces cycle time; delivery leads need for SLA analysis; no API equivalent |
| S5 | Worklog gap detection | `jira worklog gaps [--week] [--month] [--user me]` | 8/10 | Synced worklogs + calendar generation; flags zero-log days with active issue count | Tempo getMissingWorklogDays confirms pain; jira-pilot worklog has no gap detection; consultant persona explicit |
| S6 | Sprint retro export | `jira sprint retro --sprint <id/active> [--format markdown/csv]` | 7/10 | SQLite: sprint issues grouped by Done/Carried over, story points, cycle time, comment count | Scrum Master retro prep explicit; absorbed sprint status (#27) has counts only; no existing tool |
| S7 | Issue changelog diff | `jira issue changelog <KEY> [--field status,assignee]` | 7/10 | /rest/api/3/issue/{key}/changelog API; formats field-by-field transition log | Real API endpoint; "who changed this and when" debugging workflow; no surveyed tool surfaces this |
| S8 | Stale issue detector | `jira stale [--days 14] [--project KEY] [--sprint active] [--status "In Progress"]` | 7/10 | SQLite: WHERE updated_at < (now - N days) AND status not closed; shows age, assignee, last comment | Standard scrum anti-pattern alert; PM triage persona; no existing CLI does configurable staleness |
