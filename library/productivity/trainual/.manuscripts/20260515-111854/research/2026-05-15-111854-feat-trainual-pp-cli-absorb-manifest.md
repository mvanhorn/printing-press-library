# Trainual CLI Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List users | Trainual API | users list --json | Offline, FTS searchable, --select, SQL composable |
| 2 | Get user details | Trainual API | users get {id} --json | Includes completion data, --compact |
| 3 | Create/invite user | Trainual API + Ibexa | users create --email --first-name --last-name --dry-run | Agent-native, --dry-run, --json |
| 4 | Update user | Trainual API | users update {id} --json --dry-run | --dry-run, batch |
| 5 | Archive user | Trainual API + Ibexa | users archive {id} --dry-run | Safe with --dry-run |
| 6 | Unarchive user | Ibexa Connect | users unarchive {id} --dry-run | Not in Zapier |
| 7 | Assign subjects to user | Trainual API + Ibexa | users assign-subjects {user-id} --subjects {ids} --dry-run | Batch, scriptable |
| 8 | Unassign subjects | Trainual API + Ibexa | users unassign-subjects {user-id} --subjects {ids} --dry-run | Batch |
| 9 | Assign roles to user | Trainual API + Ibexa | users assign-roles {user-id} --roles {ids} --dry-run | Batch |
| 10 | Unassign roles | Trainual API + Ibexa | users unassign-roles {user-id} --roles {ids} --dry-run | Batch |
| 11 | List subjects/curriculums | Trainual API + Ibexa | subjects list --json --assigned-users | Offline, searchable |
| 12 | Get subject | Trainual API + Ibexa | subjects get {id} --json | Includes courses/tests |
| 13 | List topics/courses | Trainual API + Ibexa | topics list --subject {id} --json | Offline |
| 14 | Get topic | Trainual API + Ibexa | topics get --subject {id} --topic {id} --json | |
| 15 | List tests/surveys | Trainual API + Ibexa | tests list --subject {id} --json | Offline |
| 16 | Get test | Trainual API + Ibexa | tests get --subject {id} --test {id} --json | |
| 17 | List roles | Trainual API | roles list --json --assigned-users | Offline |
| 18 | Sync all data | Framework | sync --full | Full SQLite store for all entities |
| 19 | Search across entities | Framework | search "term" | FTS5 across users, subjects, roles |
| 20 | SQL query | Framework | sql "SELECT ..." | Direct SQLite queries |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Compliance audit | compliance-audit --threshold 80 --role "Front Desk" | 10/10 | Joins users, user_subjects, and roles tables in local SQLite to find users below completion threshold, grouped by role | Brief Top Workflows #1; brain-mcp production experience with completion_percentage field |
| 2 | Assignment gap detector | assignment-gaps --by-role | 10/10 | Computes set difference of expected subjects (union across role members) vs actual assignments per user in local SQLite | Brief Top Workflows #5; brief key quirk about role ID stability |
| 3 | Content quality audit | content-audit --show-empty --show-untested | 9/10 | Counts topics and tests per subject in local SQLite, joins user_subjects for enrollment, flags empty/untested/orphaned curriculums | Brief Top Workflows #3; brain-mcp data showing 7 empty curriculums, 54/75 with no tests |
| 4 | Onboarding progress tracker | onboarding-status --days 30 --role "New Hire" | 9/10 | Filters users by created_at, joins to user_subjects and roles in local SQLite to show assignment completeness and completion percentage | Brief Top Workflows #2; brief Users section listing ops managers |
| 5 | Training coverage matrix | coverage-matrix --format table | 8/10 | Pivots users x roles x subjects in local SQLite, computing AVG(completion_percentage) per role-subject cell as a matrix | Brief Top Workflows #4; product thesis citing cross-entity queries |
| 6 | Role completion leaderboard | role-completion --sort avg_completion | 8/10 | Groups users by role in local SQLite, computes AVG(completion_percentage) per role, outputs sorted table | Brief Top Workflows #4; brief data profile confirming role-centric org structure |
| 7 | Completion trend tracker | completion-trend --user {id} --weeks 8 | 7/10 | Reads timestamped completion_percentage snapshots from sync_history table (populated each sync run) to show week-over-week deltas | Product thesis citing historical tracking; brain-mcp audit noting 4 users at 0% needing tracking |
| 8 | Bulk role-based assignment | bulk-assign --role "Kitchen" --subjects 101,102,103 --dry-run | 7/10 | Lists users by role from local SQLite, then calls POST /users/{id}/assign_subjects for each, with --dry-run preview | Brief Top Workflows #5; Ibexa Connect docs confirming assign_subjects endpoint |

## Killed Candidates
| Feature | Kill Reason | Closest Surviving Sibling |
|---------|-------------|--------------------------|
| Export to CSV/Excel | Reimplements what sql --json + jq piping provides | coverage-matrix |
| User completion summary | Formatting wrapper over absorbed users get {id} | onboarding-status |
| Subject enrollment report | Too close to absorbed subjects list --assigned-users | coverage-matrix |
| Unassigned users report | Achievable with single sql query | assignment-gaps |
| Completion velocity | Derivative of completion-trend + role-completion | completion-trend |
| Stale content detector | Subsumed by content-audit | content-audit |
