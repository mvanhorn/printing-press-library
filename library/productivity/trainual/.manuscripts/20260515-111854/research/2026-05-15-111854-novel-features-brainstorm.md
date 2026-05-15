Now I have both files. Let me check whether the brief has `## User Vision` or `## Codebase Intelligence` sections.

The brief does NOT contain `## User Vision` or `## Codebase Intelligence` sections. Those gates are closed -- sources (e) and (f) are N/A.

This is a first print (prior research.json = `none`), so Pass 2 step (d) is also N/A.

Now I'll produce the full three-pass document.

---

## Customer model

### Persona 1: Rachel, the L&D Manager

**Today (without this CLI):** Rachel logs into the Trainual web UI every Monday morning to check who's fallen behind on training. She opens the dashboard, clicks into each curriculum one by one, scrolls through the user list to find people below 100% completion, and manually copies names into a Slack message for their managers. She cannot answer "across ALL curriculums, who are my bottom 10 people?" without clicking through dozens of screens. She has a browser tab for each curriculum she cares about and a spreadsheet where she manually tracks week-over-week progress because Trainual's UI shows no historical trend.

**Weekly ritual:** Monday compliance check. Pull up every mandatory curriculum, identify users below target completion, escalate to managers. Friday follow-up on the same list to see if gaps closed.

**Frustration:** There is no single view that answers "show me every user below X% completion across all mandatory curriculums, grouped by role or location." She builds this by hand every week.

### Persona 2: Marcus, the Multi-Unit Ops Manager

**Today (without this CLI):** Marcus manages onboarding across four venue locations, each mapped to a Trainual role. When new hires start, he assigns them subjects manually in the UI, then checks back days later to see if they're progressing. He has no way to see "all new hires across all locations who started this month and their aggregate completion." He runs a mental checklist and sometimes forgets to assign a curriculum, discovering the gap weeks later when a manager complains.

**Weekly ritual:** Wednesday new-hire check-in. Look at each role, find recent additions, verify subject assignments are complete, spot-check completion percentages.

**Frustration:** No way to detect "users who should have a subject assigned based on their role but don't." The assignment gap is invisible until someone complains.

### Persona 3: Sandra, the Training Content Owner

**Today (without this CLI):** Sandra built 75 curriculums over two years. Some are stale, some have no tests, some have zero enrolled users. She knows this vaguely but has never done a full audit because the UI requires clicking into each curriculum individually to see its course count, test count, and enrollment. She opened a spreadsheet once, copied data for 20 curriculums, and gave up. She cannot answer "which curriculums have courses but no tests?" or "which curriculums have zero completions?" without hours of manual work.

**Weekly ritual:** Monthly (but wishes it were weekly) content quality review. Spot-check a handful of curriculums, look for empty ones, check if tests exist, verify nothing is broken.

**Frustration:** No bulk content quality view. She cannot see all 75 curriculums with their course counts, test counts, and enrollment in a single table.

### Persona 4: Darnell, the Franchise Compliance Officer

**Today (without this CLI):** Darnell needs to produce a monthly report showing completion rates by role (each role = a venue/franchise location). He exports user data from Trainual, merges it with role data in Excel, calculates averages, and formats the report. The process takes 3-4 hours because the Trainual export doesn't include role-level aggregation. He cannot answer "which role has the lowest average completion?" without the full spreadsheet dance.

**Weekly ritual:** Bi-weekly spot check of role-level completion. Monthly formal report.

**Frustration:** No role-level completion aggregation exists anywhere in Trainual. Every report requires a manual join of users, roles, and completion data.

---

## Candidates (pre-cut)

### Candidate 1: Compliance Audit Report
- **Command:** `compliance-audit --threshold 80 --role "Front Desk"`
- **Description:** Cross-references users, their assigned subjects, and completion_percentage to surface everyone below a given threshold, grouped by role.
- **Persona served:** Rachel, Darnell
- **Source:** (a) Persona-driven (Rachel's frustration), (c) Cross-entity local query
- **Kill/keep checks:** No LLM dependency. No external service. Read-only using synced SQLite data. No scope creep -- single command, tabular output. Verifiable against web UI spot checks.
- **Buildability proof:** Joins users, user_subjects, and roles tables in local SQLite to compute per-user completion gaps with no external dependencies.
- **Verdict:** KEEP

### Candidate 2: Onboarding Progress Tracker
- **Command:** `onboarding-status --days 30 --role "New Hire"`
- **Description:** Shows users created/assigned within N days, their assigned subjects, and completion percentage to track new-hire ramp.
- **Persona served:** Marcus
- **Source:** (a) Persona-driven (Marcus's frustration), (c) Cross-entity local query
- **Kill/keep checks:** No LLM. No external service. Uses created_at or assignment timestamps from synced data. Single command output. Verifiable.
- **Buildability proof:** Filters users table by created_at within N days, joins to user_subjects and roles in local SQLite to show assignment and completion status.
- **Verdict:** KEEP

### Candidate 3: Assignment Gap Detector
- **Command:** `assignment-gaps --by-role`
- **Description:** Detects users who are in a role but are missing subject assignments that other users in the same role have -- surfaces "forgotten" assignments.
- **Persona served:** Marcus
- **Source:** (a) Persona-driven (Marcus's frustration), (b) Service-specific (role-based assignment pattern), (c) Cross-entity local query
- **Kill/keep checks:** No LLM. No external service. Pure local SQLite set-difference query: for each role, find the union of subjects assigned to any user in that role, then find users missing any of those subjects. Verifiable by manual role inspection.
- **Buildability proof:** Computes set difference of (role's expected subjects) vs (user's assigned subjects) in local SQLite using joins across users, roles, and user_subjects tables.
- **Verdict:** KEEP

### Candidate 4: Content Quality Audit
- **Command:** `content-audit --show-empty --show-untested`
- **Description:** Lists all subjects with their course count, test count, and enrollment count, flagging empty curriculums (0 courses), untested curriculums (courses but 0 tests), and orphaned curriculums (0 enrolled users).
- **Persona served:** Sandra
- **Source:** (a) Persona-driven (Sandra's frustration), (b) Service-specific (curriculum-course-test hierarchy)
- **Kill/keep checks:** No LLM. No external service. Aggregates synced subjects, topics, and tests tables. Single tabular output. Verifiable against manual spot checks.
- **Buildability proof:** Counts topics and tests per subject in local SQLite, joins to user_subjects for enrollment count, flags rows matching empty/untested/orphan criteria.
- **Verdict:** KEEP

### Candidate 5: Role Completion Leaderboard
- **Command:** `role-completion --sort avg_completion`
- **Description:** Aggregates completion_percentage across all users in each role to produce a role-level completion leaderboard.
- **Persona served:** Darnell
- **Source:** (a) Persona-driven (Darnell's frustration), (c) Cross-entity local query
- **Kill/keep checks:** No LLM. No external service. Pure SQLite aggregation on users + roles. Uses completion_percentage (not broken avg_completion). Single command.
- **Buildability proof:** Groups users by role in local SQLite, computes AVG(completion_percentage) per role, sorts and outputs as table.
- **Verdict:** KEEP

### Candidate 6: Completion Trend Tracker
- **Command:** `completion-trend --user {id} --weeks 8`
- **Description:** Shows how a user's completion_percentage has changed over successive syncs, providing a week-over-week trend line.
- **Persona served:** Rachel
- **Source:** (b) Service-specific (completion tracking over time), (c) Cross-entity local query
- **Kill/keep checks:** No LLM. No external service. Requires storing historical completion snapshots in a sync_history table -- minor data model addition (timestamped completion snapshots per sync run). No scope creep -- tabular output. Verifiable by comparing to manual spreadsheet.
- **Buildability proof:** Reads timestamped completion_percentage snapshots from a local sync_history table (populated on each `sync` run) and outputs delta per period.
- **Verdict:** KEEP (minor data model addition)

### Candidate 7: Stale Content Detector
- **Command:** `stale --days 180`
- **Description:** Lists subjects that have not been updated (based on updated_at) in N days, suggesting they may need review.
- **Persona served:** Sandra
- **Source:** (b) Service-specific (content lifecycle), (c) Cross-entity local query
- **Kill/keep checks:** No LLM. No external service. Pure filter on subjects table by updated_at. Trivial.
- **Buildability proof:** Filters subjects table WHERE updated_at < now() - N days in local SQLite.
- **Verdict:** KEEP but may be too thin -- borderline wrapper on a simple WHERE clause.

### Candidate 8: Bulk Assignment Command
- **Command:** `bulk-assign --role "Kitchen" --subjects 101,102,103 --dry-run`
- **Description:** Assigns a set of subjects to ALL users in a given role in one command.
- **Persona served:** Marcus
- **Source:** (b) Service-specific (role-based bulk operations)
- **Kill/keep checks:** No LLM. No external service. Uses existing assign_subjects API endpoint in a loop. --dry-run for safety. However: this is a thin wrapper over iterating the absorbed `users assign-subjects` command for each user in a role. The "leverage" is the role-based fan-out, not the individual API call.
- **Buildability proof:** Lists users by role from local SQLite, then calls POST /users/{id}/assign_subjects for each user.
- **Verdict:** KEEP -- the role-based fan-out IS the value; individual assign-subjects doesn't do this.

### Candidate 9: Export to CSV/Excel
- **Command:** `export --format csv --entity users`
- **Description:** Exports synced data to CSV or Excel for downstream reporting.
- **Persona served:** Darnell
- **Source:** (a) Persona-driven (Darnell's spreadsheet workflow)
- **Kill/keep checks:** No LLM. No external service. But: this is generic utility, not Trainual-specific. The framework's `sql` command + shell piping (`| column -t` or redirecting JSON) already covers this. Thin wrapper.
- **Verdict:** CUT -- reimplements what `sql "SELECT ..." --json | jq -r` or standard CLI piping already does. Not transcendent.

### Candidate 10: User Completion Summary Card
- **Command:** `user-summary {id}`
- **Description:** Shows a single user's completion across all assigned subjects, role memberships, and assignment status in a formatted card view.
- **Persona served:** Rachel, Marcus
- **Source:** (b) Service-specific (per-user training profile)
- **Kill/keep checks:** No LLM. No external service. But: the absorbed `users get {id} --json` already includes completion data. This is a formatting wrapper over an existing absorbed command. The "card" view is cosmetic.
- **Verdict:** CUT -- wrapper over absorbed `users get {id}` with formatting. Not transcendent.

### Candidate 11: Subject Enrollment Report
- **Command:** `enrollment --subject {id}`
- **Description:** Shows all users assigned to a given subject with their individual completion percentages.
- **Persona served:** Rachel, Sandra
- **Source:** (b) Service-specific (subject-centric view)
- **Kill/keep checks:** No LLM. No external service. However, `subjects get {id} --json` with `--assigned-users` already includes this data if the API is called with the right params. This is close to a wrapper on the absorbed `subjects list --assigned-users`.
- **Verdict:** CUT -- too close to absorbed `subjects get` + `subjects list --assigned-users`. Wrapper territory.

### Candidate 12: Unassigned Users Report
- **Command:** `unassigned-users`
- **Description:** Lists users who have zero subject assignments -- people who exist in Trainual but have no training assigned.
- **Persona served:** Marcus, Rachel
- **Source:** (c) Cross-entity local query
- **Kill/keep checks:** No LLM. No external service. Pure SQLite query: users LEFT JOIN user_subjects WHERE subject_id IS NULL. However, this is achievable with `sql "SELECT * FROM users WHERE id NOT IN (SELECT user_id FROM user_subjects)"`. It's a canned query, not a novel feature.
- **Verdict:** CUT -- achievable with a single SQL query via the absorbed `sql` command. Not transcendent enough to justify a dedicated command.

### Candidate 13: Training Coverage Heatmap
- **Command:** `coverage-matrix --format table`
- **Description:** Produces a role x subject matrix showing what percentage of users in each role have completed each subject.
- **Persona served:** Darnell, Rachel
- **Source:** (c) Cross-entity local query, (b) Service-specific
- **Kill/keep checks:** No LLM. No external service. Three-way join in SQLite (users x roles x user_subjects). The matrix/pivot output is non-trivial and cannot be replicated with a simple `sql` query -- it requires pivoting. This is genuine leverage.
- **Buildability proof:** Pivots users x roles x subjects in local SQLite, computing completion_percentage aggregates per cell of the role-by-subject matrix.
- **Verdict:** KEEP

### Candidate 14: Completion Velocity
- **Command:** `velocity --role "Kitchen" --days 30`
- **Description:** Shows how fast users in a role are completing training over a time window, measured as completion percentage gained per week.
- **Persona served:** Marcus, Darnell
- **Source:** (c) Cross-entity local query, (b) Service-specific
- **Kill/keep checks:** No LLM. No external service. But: requires historical sync snapshots (same as Candidate 6). If we have the sync_history table, this is a derivative of completion-trend. Two features depending on the same data model addition is fine, but velocity is essentially trend + aggregation by role. Risk of being too close to Candidate 6 + Candidate 5 combined.
- **Verdict:** CUT -- derivative of completion-trend + role-completion. Not distinct enough to survive as its own command.

---

## Survivors and kills

### Adversarial evaluation of remaining candidates

**Candidate 1: Compliance Audit Report** (`compliance-audit`)
1. **Weekly use:** Yes. Rachel runs this every Monday. Darnell runs it bi-weekly. Core weekly ritual for both.
2. **Wrapper vs leverage:** Not a wrapper. No single API endpoint returns "all users below X% grouped by role." Requires joining users + subjects + roles + completion data.
3. **Transcendence proof:** Power comes from local SQLite cross-entity join (users x user_subjects x roles) with threshold filtering. No single API call provides this.
4. **Sibling kill:** Killed Candidate 12 (unassigned-users) -- compliance-audit subsumes "who's missing" by showing 0% completion on mandatory subjects.

**Candidate 2: Onboarding Progress Tracker** (`onboarding-status`)
1. **Weekly use:** Yes. Marcus checks every Wednesday. Core ritual.
2. **Wrapper vs leverage:** Not a wrapper. Requires filtering users by creation date, joining to assignments and completion, grouping by role. No API endpoint does this.
3. **Transcendence proof:** Local SQLite join with date filtering across users + user_subjects + roles.
4. **Sibling kill:** Killed Candidate 10 (user-summary) -- onboarding-status is the cohort version of what user-summary tries to do for one person, and the cohort view is what Marcus actually needs.

**Candidate 3: Assignment Gap Detector** (`assignment-gaps`)
1. **Weekly use:** Yes. Marcus runs this every Wednesday as part of new-hire check-in. Rachel would run it monthly at minimum.
2. **Wrapper vs leverage:** Not a wrapper. Set-difference computation across role memberships and subject assignments. No API endpoint computes "what's missing."
3. **Transcendence proof:** Local SQLite set operations: for each role, compute expected subjects (union of all subjects assigned to any user in that role), then find users missing any. Pure local computation impossible through the API.
4. **Sibling kill:** Killed Candidate 12 (unassigned-users) -- assignment-gaps is strictly more powerful because it catches partial gaps (some subjects assigned, others missing), not just fully unassigned users.

**Candidate 4: Content Quality Audit** (`content-audit`)
1. **Weekly use:** Sandra says monthly but wishes weekly. With a one-command solution, weekly becomes realistic. Rachel would also use it to validate curriculum quality before assigning.
2. **Wrapper vs leverage:** Not a wrapper. Aggregates counts across three entity types (subjects, topics, tests) and enrollment. No single API call provides this.
3. **Transcendence proof:** Local SQLite aggregation across subjects + topics + tests + user_subjects tables, with flag logic for empty/untested/orphaned.
4. **Sibling kill:** Killed Candidate 7 (stale) -- content-audit subsumes staleness detection (can add updated_at column) while also covering structural quality (empty, untested, orphaned).

**Candidate 5: Role Completion Leaderboard** (`role-completion`)
1. **Weekly use:** Darnell bi-weekly, Rachel weekly as part of compliance. Yes.
2. **Wrapper vs leverage:** Not a wrapper. AVG(completion_percentage) grouped by role doesn't exist in any API endpoint.
3. **Transcendence proof:** Local SQLite GROUP BY aggregation across users + roles.
4. **Sibling kill:** Killed Candidate 14 (velocity) -- role-completion gives the snapshot Darnell actually needs; velocity adds temporal complexity that requires historical data and serves a speculative need.

**Candidate 6: Completion Trend Tracker** (`completion-trend`)
1. **Weekly use:** Rachel would check trends every Monday for flagged users. However, this requires multiple sync runs to accumulate history -- value builds over time, not immediately useful on first install. First week: useless. Fourth week: valuable.
2. **Wrapper vs leverage:** Not a wrapper. No API provides historical completion data. This is purely local, powered by sync snapshots.
3. **Transcendence proof:** Local SQLite temporal query across sync_history table. The data literally doesn't exist anywhere else.
4. **Sibling kill:** Killed Candidate 14 (velocity) -- trend is the per-user primitive; velocity is just trend aggregated by role.
- **Risk:** Requires a data model addition (sync_history table). Score may be lower on Build Feasibility.

**Candidate 7: Stale Content Detector** (`stale`)
- Subsumed by content-audit. See Candidate 4 adversarial notes. **KILL.**

**Candidate 8: Bulk Assignment Command** (`bulk-assign`)
1. **Weekly use:** Marcus would use this when new hires join or when curriculum assignments change. That's weekly during busy hiring periods, monthly otherwise. Borderline.
2. **Wrapper vs leverage:** It IS a wrapper in the sense that it calls the same assign_subjects endpoint N times. But the role-based fan-out is genuine leverage -- the absorbed `users assign-subjects` operates on one user at a time.
3. **Transcendence proof:** Combines local SQLite (list users by role) with API write calls (assign_subjects). The local-to-API bridge is the value.
4. **Sibling kill:** Killed Candidate 9 (export) -- both serve "batch operations" but bulk-assign actually changes state via the API, while export is just reformatting.

**Candidate 13: Training Coverage Matrix** (`coverage-matrix`)
1. **Weekly use:** Darnell uses this for his bi-weekly/monthly report. Rachel could use it weekly. The matrix view is the exact output Darnell currently builds by hand in Excel.
2. **Wrapper vs leverage:** Not a wrapper. Requires a pivot operation across three entity types. No API endpoint or simple SQL query produces a pivoted matrix.
3. **Transcendence proof:** Local SQLite pivot across users x roles x subjects, computing aggregate completion per cell. The pivot logic is non-trivial and impossible through the API.
4. **Sibling kill:** Killed Candidate 11 (enrollment) -- coverage-matrix shows enrollment AND completion across all subjects and roles simultaneously, making single-subject enrollment views redundant.

### Scoring

| Candidate | Domain Fit (0-3) | User Pain (0-3) | Build Feasibility (0-2) | Research Backing (0-2) | Raw | Score /10 |
|-----------|-----------------|-----------------|------------------------|----------------------|-----|-----------|
| compliance-audit | 3 | 3 | 2 | 2 | 10 | 10 |
| onboarding-status | 3 | 2 | 2 | 2 | 9 | 9 |
| assignment-gaps | 3 | 3 | 2 | 2 | 10 | 10 |
| content-audit | 3 | 2 | 2 | 2 | 9 | 9 |
| role-completion | 3 | 2 | 2 | 1 | 8 | 8 |
| completion-trend | 3 | 2 | 1 | 1 | 7 | 7 |
| bulk-assign | 2 | 2 | 2 | 1 | 7 | 7 |
| coverage-matrix | 3 | 3 | 1 | 1 | 8 | 8 |

All 8 score >= 5/10. However, the cut pass targets 4-8 survivors and I need to drop ~half from the pre-cut list of 14. I've already killed 6 (Candidates 7, 9, 10, 11, 12, 14). 8 remain. That's within the 4-8 target. Keeping all 8.

### Survivors

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Compliance audit | `compliance-audit --threshold 80 --role "Front Desk"` | 10/10 | Joins users, user_subjects, and roles tables in local SQLite to find users below completion threshold, grouped by role | Brief Top Workflows #1 (training compliance audit); brain-mcp production experience with completion_percentage field |
| 2 | Assignment gap detector | `assignment-gaps --by-role` | 10/10 | Computes set difference of expected subjects (union across role members) vs actual assignments per user in local SQLite | Brief Top Workflows #5 (bulk assignment management); brief key quirk about role ID stability |
| 3 | Content quality audit | `content-audit --show-empty --show-untested` | 9/10 | Counts topics and tests per subject in local SQLite, joins user_subjects for enrollment, flags empty/untested/orphaned curriculums | Brief Top Workflows #3 (content quality audit); brain-mcp data showing 7 empty curriculums, 54/75 with no tests |
| 4 | Onboarding progress tracker | `onboarding-status --days 30 --role "New Hire"` | 9/10 | Filters users by created_at, joins to user_subjects and roles in local SQLite to show assignment completeness and completion percentage for recent hires | Brief Top Workflows #2 (onboarding tracking); brief Users section listing ops managers and franchise operators |
| 5 | Training coverage matrix | `coverage-matrix --format table` | 8/10 | Pivots users x roles x subjects in local SQLite, computing AVG(completion_percentage) per role-subject cell as a matrix | Brief Top Workflows #4 (role-based progress reports); brief product thesis citing cross-entity queries impossible through web UI |
| 6 | Role completion leaderboard | `role-completion --sort avg_completion` | 8/10 | Groups users by role in local SQLite, computes AVG(completion_percentage) per role, outputs sorted table | Brief Top Workflows #4 (role-based progress reports); brief data profile confirming role-centric org structure |
| 7 | Completion trend tracker | `completion-trend --user {id} --weeks 8` | 7/10 | Reads timestamped completion_percentage snapshots from a local sync_history table (populated each sync run) to show week-over-week deltas | Brief product thesis citing historical tracking as key differentiator; brain-mcp audit noting 4 users at 0% needing tracking |
| 8 | Bulk role-based assignment | `bulk-assign --role "Kitchen" --subjects 101,102,103 --dry-run` | 7/10 | Lists users by role from local SQLite, then calls POST /users/{id}/assign_subjects for each, with --dry-run showing planned changes before execution | Brief Top Workflows #5 (bulk assignment management); Ibexa Connect docs confirming assign_subjects endpoint exists |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|--------------------------|
| Export to CSV/Excel (`export`) | Reimplements what `sql --json \| jq` and standard CLI piping already provide; not transcendent | coverage-matrix (produces the actual report Darnell needs) |
| User completion summary card (`user-summary`) | Formatting wrapper over absorbed `users get {id} --json` which already includes completion data | onboarding-status (cohort view that serves the same persona's actual need) |
| Subject enrollment report (`enrollment`) | Too close to absorbed `subjects list --assigned-users` and `subjects get {id}`; wrapper territory | coverage-matrix (shows enrollment + completion across ALL subjects simultaneously) |
| Unassigned users report (`unassigned-users`) | Achievable with a single `sql "SELECT ... WHERE id NOT IN (...)"` query; canned query, not a feature | assignment-gaps (strictly more powerful, catches partial gaps not just fully unassigned) |
| Completion velocity (`velocity`) | Derivative of completion-trend + role-completion combined; not distinct enough for its own command | completion-trend (the per-user primitive) + role-completion (the aggregation) |
| Stale content detector (`stale`) | Subsumed by content-audit which can include updated_at as a column and already flags content quality issues | content-audit (covers staleness plus structural quality) |


I'll start by reading the brief and scoring rubric in parallel.
Could not extract text
