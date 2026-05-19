# Linear CLI Absorb Manifest (v4.9.0 reprint)

## Sources Cataloged (carried from prior — ecosystem stable in 11-day delta)

1. Finesssee/linear-cli (Rust, 60+ commands) — most comprehensive existing CLI
2. schpet/linear-cli (Ruby, git-aware, agent skills)
3. czottmann/linearis (Deno, agent-optimized, token-efficient)
4. dorkitude/linctl (Go, Cobra, agent-first)
5. evangodon/linear-cli (Go, ~10 commands)
6. Official Linear MCP (mcp.linear.app)
7. tacticlaunch/mcp-linear (community MCP)
8. @linear/sdk (official TypeScript SDK)
9. linear-api (Python, Pydantic models)

## Absorbed (40 features — match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | List issues with filtering | Finesssee list | `issues list --state --assignee --label --team --priority` | FTS5 offline search, SQL composable, --json/--csv/--select |
| 2 | Get issue by ID | All CLIs | `issues get ABC-123` | Smart ID resolution, offline cached, related data joined |
| 3 | Create issue | Finesssee create | `issues create --title --team --assignee --priority --label --stdin` | Agent-native, --dry-run, batch via --stdin, records ID into `pp_created` for safe cleanup |
| 4 | Update issue | All CLIs | `issues update ABC-123 --state --priority --assignee` | Idempotent, --dry-run, typed exit codes, optional `--trust-mode strict` guard |
| 5 | Delete/archive issue | Finesssee archive | `issues archive ABC-123` | Confirm prompt, --force for agent use, optional `--trust-mode strict` guard |
| 6 | Search issues by text | schpet query, linearis | `issues search "login bug"` | FTS5 offline, regex, works without API call |
| 7 | Start issue (assign + in-progress) | schpet start | `issues start ABC-123` | Creates git branch, updates state atomically |
| 8 | Close/complete issue | Finesssee close | `issues close ABC-123` | Auto-detects "Done" state |
| 9 | Assign issue | Finesssee assign | `issues assign ABC-123 --to @user` | Bulk assign via piped IDs |
| 10 | Move issue between teams | Finesssee move/transfer | `issues move ABC-123 --team ENG` | Preserves labels, reassigns state |
| 11 | Issue comments CRUD | All tools | `comments list ABC-123`, `comments add ABC-123 --body "..."` | Offline cache, search across comments |
| 12 | Issue relations | Finesssee relations | `issues relate ABC-123 --blocks DEF-456` | Visualize dependency chains |
| 13 | Issue parent/sub-issues | Finesssee parent | `issues parent ABC-123 --child DEF-456` | Tree view of sub-issues |
| 14 | List projects | All tools | `projects list --status --lead` | Offline, --json, sorting |
| 15 | Get/create/update project | Finesssee, linearis | `projects get/create/update` | Full CRUD with --dry-run |
| 16 | Project members | Finesssee | `projects members PROJ` | Show roles and assignment counts |
| 17 | List teams | All tools | `teams list` | With member counts, offline |
| 18 | Team members | schpet, Finesssee | `teams members TEAM` | Issue counts per member |
| 19 | Cycles CRUD | Finesssee | `cycles list/get/create/update/complete` | Historical data in SQLite |
| 20 | Current cycle | Finesssee current | `cycles current` | Shows progress, completion % |
| 21 | Sprint planning | Finesssee plan | `cycles plan` | Suggests carry-over from previous |
| 22 | Labels CRUD | Finesssee | `labels list/create/update` | Offline, used in filtering |
| 23 | Workflow states | All tools | `states list --team` | Shows transition rules |
| 24 | Documents CRUD | schpet, linearis | `documents list/get/create/update/delete` | FTS5 search across doc content |
| 25 | Milestones CRUD | schpet, linearis | `milestones list/get/create/update` | Target date tracking |
| 26 | Initiatives | Finesssee | `initiatives list/get` | Roadmap visibility |
| 27 | Notifications | Finesssee | `notifications list/read/archive` | Unread count, bulk mark-read |
| 28 | Attachments | Finesssee, linearis | `attachments list/get/create` | File upload + URL linking |
| 29 | Custom views | Finesssee | `views list/get` | Save and recall filtered views |
| 30 | Users/me | All tools | `me`, `users list/get` | Current user info, team memberships |
| 31 | Triage | Finesssee | `triage list/claim/snooze` | Inbox-zero workflow |
| 32 | Bulk operations | Finesssee | `bulk update-state/assign/label` | Pipe issue IDs, --dry-run |
| 33 | Git integration | schpet, Finesssee | `git checkout/branch ABC-123` | Creates branch from issue ID + title |
| 34 | Watch mode | Finesssee | `watch ABC-123` | Real-time issue updates via polling |
| 35 | Favorites | Finesssee | `favorites list/add/remove` | Quick access to pinned items |
| 36 | Webhooks | Finesssee | `webhooks list/create/delete` | HMAC-SHA256 verification |
| 37 | Sync all data | (innovation) | `sync --full` / `sync --incremental` | SQLite persistence, incremental cursor |
| 38 | SQL queries | (innovation) | `sql "SELECT * FROM issues WHERE priority < 2"` | Direct SQL against local store |
| 39 | Doctor command | (innovation) | `doctor` | Validates auth, API connectivity, store health |
| 40 | Auth setup | linearis, Finesssee | `auth login`, `auth status` | API key config, doctor integration |

## Transcendence (13 features — only possible with our local data layer or v4 surface)

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Source |
|---|---------|---------|-------|--------------|-------------|----------|--------|
| 1 | Today View | `today` | 9/10 | hand-code | SQL join over issues × cycles × workflow_states × users in local SQLite, filtered to assignee=me + WIP states, ranked by (priority, cycle.endsAt) | Sam's daily ritual; competitor CLIs require N separate calls | prior (kept) |
| 2 | Bottleneck Detection | `bottleneck` | 8/10 | hand-code | SQL group-by issues × users in WIP states + issue-relation graph walk for blocked status | Maya's pre-sprint-planning frustration | prior (kept) |
| 3 | Project Burndown | `projects burndown` | 8/10 | hand-code | Linear regression on local cycle_snapshot velocity series against project's remaining estimate sum | Maya/Priya's "when will X land"; v4 GraphQL sync template removes prior emit blocker | prior (kept) |
| 4 | Cycle Comparison | `cycles compare` | 8/10 | hand-code | Two-row SQL diff over cycle_snapshot + issues.cycle_id: completion %, scope added/cut, carryover, mean cycle time | Maya's Friday-update ritual | prior (kept) |
| 5 | Stale Issue Radar | `stale` | 7/10 | hand-code | Local updatedAt < now()-N days scan over issues grouped by team/project — avoids burning Linear complexity budget | Maya/Priya backlog grooming | prior (kept) |
| 6 | Slipped Issues | `slipped` | 7/10 | hand-code | Diff issue.cycle_id against historical issue_history rows; row appears in current cycle but state.type != completed in prior cycle | Maya's Friday update | prior (kept) |
| 7 | Blocking Queue | `blocking` | 8/10 | hand-code | Graph walk over issue_relation where type=blocks AND from_assignee=me; ranks by downstream count × max(downstream priority) | Devon's daily frustration; Linear shows the relation per-issue, never as a personal queue | prior (kept) |
| 8 | Duplicate Detection | `similar` | 7/10 | hand-code | FTS5 MATCH on issue_fts index over title + description + comments; rank by bm25 | Maya/Devon triage; competitor CLIs are online-only and can't FTS | prior (kept) |
| 9 | Velocity Trends | `velocity` | 7/10 | hand-code | Aggregation over cycle_snapshot history: completed estimate per cycle for last N cycles, with team filter | Feeds project burndown; Maya's Monday planning | prior (kept) |
| 10 | Initiatives Health | `initiatives health` | 8/10 | hand-code | Rollup over initiatives → projects → milestones joining each project's burndown projection against its milestone target_date; flags projected > target | Priya's Tuesday portfolio review | prior (kept) |
| 11 | Test Fixture Lifecycle | `pp-test list` / `pp-cleanup` | 9/10 | hand-code | `pp_created` local ledger writes on every successful `issues create`; cleanup iterates rows and calls the real `issueArchive` mutation, scoped to session | Sam's Friday cleanup intent; brief live-test constraint mandates this | prior (kept) |
| 12 | Trust Mode Mutation Guard | `--trust-mode strict` (root flag + config) | 8/10 | hand-code | Pre-mutation lookup in `pp_created`; on miss, return typed exit code 2 without calling the API | Sam's safety net per brief live-test constraint | prior (kept) |
| 13 | At-Risk Milestones | `milestones at-risk` | 7/10 | hand-code | For each portfolio milestone, compute projected landing from burndown regression on parent project; rank by (projected − target) descending | Priya's "which milestone is most at risk" verbatim in brief Frustration | new |

**All 13 survivors tagged `hand-code`.** Phase Gate 1.5's hand-code commitment is 13 features (~50–150 LoC each + root.go wiring).

## Killed candidates (Pass 3)

| Feature | Kill reason | Closest surviving sibling |
|---|---|---|
| Weekly Update Pack (`weekly-update`) | Pure composite of cycles compare + slipped + blocking. Maya can pipe; no new join | cycles compare |
| Standup Brief (`standup`) | Thin sugar over today + blocking; MCP intent layer binds them through `linear_execute` already | today |
| Issue History Replay (`issues history`) | Linear's GraphQL has `issueHistory` as first-class; hand-coding would be wrapper or reimplementation | (none — killed cleanly) |
| Workspace Activity Pulse (`activity`) | Overlaps velocity with weaker question; fails weekly-use test | velocity |
| Branch from Issue (`git checkout`) | Already in absorb manifest row #33; not novel | absorb #33 |

## Reprint verdicts

All 12 prior novel features kept under the v4 reprint. Detail in `2026-05-18-232543-novel-features-brainstorm.md`. One new feature added: `milestones at-risk` (motivated by Priya's verbatim portfolio frustration that v3's `initiatives health` rollup did not directly answer).

## v4 MCP Surface Enrichment (spec-level, not commands)

The spec will be enriched before generation with:

- `mcp.transport: [stdio, http]` — remote-capable
- `mcp.orchestration: code` — emits `linear_search` + `linear_execute` pair (~1K tokens)
- `mcp.endpoint_tools: hidden` — suppress raw per-endpoint mirrors
- `mcp.intents` — 5 named multi-step intents:
  - `triage_inbox` — list triage queue, agent classifies, bulk-applies
  - `daily_standup` — pull `today` + `blocking` + `slipped` joined for the user
  - `sprint_plan` — given a team, fetch open issues + last cycle's velocity + current cycle's capacity, return rebalance plan
  - `weekly_update` — generate Maya's Friday payload (cycle progress, shipped, slipped, projected landings)
  - `backlog_grooming` — pull stale + duplicates for the user's teams

## Dropped from prior

None. All 12 prior features kept; 1 new added (`milestones at-risk`).
