# Linear CLI Absorb Manifest (reprint, v3.10.0)

## Sources Cataloged (carried from prior, no churn detected)
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

## Transcendence (12 features — only possible with our local data layer or v3 surface)

| # | Feature | Command | Why Only We Can Do This | Score | Persona | Status |
|---|---------|---------|------------------------|-------|---------|--------|
| 1 | Today view | `today` | Compound query across assignments, cycles, priority, state for current user across all teams | 8/10 | Devon, Sam | prior-built |
| 2 | Stale issue radar | `stale --days 30` | Local `updatedAt` tracking + team/project grouping | 7/10 | Maya, Priya | prior-built |
| 3 | Bottleneck detection | `bottleneck` | Joins issues + assignees + cycles + issue_relations to compute per-assignee load and blocked counts | 8/10 | Maya | prior-built |
| 4 | Duplicate detection | `similar "login bug"` | FTS5 across all issue titles + descriptions, ranked | 7/10 | Maya, Priya | prior-built (v3 fixes FTS5 fragility) |
| 5 | Velocity trends | `velocity --weeks 8` | Historical cycle completion rates from SQLite | 7/10 | Maya, Priya | prior-built |
| 6 | Project burndown | `projects burndown PROJ` | Linear-regress remaining estimate against velocity, project landing date with band | 8/10 | Maya, Priya | **reprint-required (was deferred)** |
| 7 | Cycle comparison | `cycles compare 42 43` | Two-cycle SQL diff: completion %, scope-added, scope-cut, carryover | 8/10 | Maya | **reprint-required (was deferred)** |
| 8 | Slipped issues | `slipped` | Diffs `issue.cycle` across cycle-history rows where N-1 → N; groups by reason heuristic | 7/10 | Maya | **new** |
| 9 | Blocking queue | `blocking` | Graph walk over issue_relation table where `relatedIssue.assignee=me AND type=blocks` | 7/10 | Devon | **new** |
| 10 | Initiatives health | `initiatives health` | Rolled-up portfolio view: child project progress + milestone target-vs-projected | 7/10 | Priya | **new** |
| 11 | Test fixture lifecycle | `pp-test list` / `pp-cleanup` | Local `pp_created` table records issue IDs from `issues create`; cleanup invokes the real archive endpoint per row | 6/10 | Sam, Devon, dogfood | **new** |
| 12 | Trust mode | `--trust-mode strict` (root flag + config) | Pre-mutation guard: looks up target issue ID in `pp_created`; refuses if absent under strict | 5/10 | Sam, dogfood | **new** |

## v3 MCP Surface Enrichment (spec-level, not commands)

The spec will be enriched before generation with:
- `mcp.transport: [stdio, http]` — remote-capable
- `mcp.orchestration: code` — emits `linear_search` + `linear_execute` pair (~1K tokens)
- `mcp.endpoint_tools: hidden` — suppress raw per-endpoint mirrors
- `mcp.intents` — 3-5 named intents:
  - `triage_inbox` — list triage queue, agent classifies, bulk-applies
  - `daily_standup` — pull `today` + `blocking` + `slipped` joined for the user
  - `sprint_plan` — given a team, fetch open issues + last cycle's velocity + current cycle's capacity, return rebalance plan
  - `weekly_update` — generate Maya's Friday update payload (cycle progress, shipped, slipped, projected landings)

## Dropped from prior

- **Workload Balance (`workload`)** — sibling-killed by `bottleneck`, which subsumes per-assignee load and adds blocked-count signal. Two commands for the same question split adoption.
