# Canvas LMS CLI — Absorb Manifest

## Sources Analyzed
1. **vishalsachdev/canvas-mcp** (TS, 129★) — 88 MCP tools, student+instructor+admin
2. **DMontgomery40/mcp-canvas-lms** (TS, 96★) — 54 tools, Docker/K8s support
3. **fuller** (Rust, 2★) — list courses/todos/assignments, view detail, ignore
4. **canvas-cli-and-mcp** (TS, 0★) — courses, upcoming, todos, assignments, submissions, submit, discussions, files, modules, pages, announcements
5. **canvasctl** (Python, 0★) — sync, today, upcoming, diff, announcements
6. **canvas-ai-cli** (Python, 0★) — assignments due --days N, plan, do (AI agent wrapper)
7. **canvasapi** (Python SDK, 660★) — full ORM-style wrappers (method catalog)
8. **node-canvas-api** (JS, 72★) — 54+ functions

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|------------|-------------------|-------------|
| 1 | List enrolled courses | canvas-mcp, fuller, all | `canvas courses list` | Offline, --json, --select, --compact |
| 2 | Get course detail | canvas-mcp | `canvas courses get <id>` | --json, --compact |
| 3 | List assignments for course | canvas-mcp, fuller, canvasapi | `canvas assignments list <course_id>` | Transparent pagination, FTS offline, due-date sort |
| 4 | Get assignment detail | canvas-mcp, fuller | `canvas assignments get <course_id> <id>` | --json with full rubric |
| 5 | Create assignment | canvas-mcp (instructor) | `canvas assignments create <course_id>` | --dry-run, --stdin JSON |
| 6 | Update assignment | canvas-mcp | `canvas assignments update <course_id> <id>` | --dry-run |
| 7 | Delete assignment | canvas-mcp | `canvas assignments delete <course_id> <id>` | --dry-run |
| 8 | List submissions | canvas-mcp, canvas-cli-and-mcp | `canvas submissions list <course_id> <assignment_id>` | --json, pagination |
| 9 | Get my submission | canvas-mcp, canvas-cli-and-mcp | `canvas submissions get <course_id> <assignment_id>` | --json |
| 10 | Submit assignment (text) | canvas-cli-and-mcp | `canvas submissions submit <course_id> <assignment_id> --body` | --dry-run |
| 11 | Submit assignment (URL) | canvas-cli-and-mcp | `canvas submissions submit ... --url` | --dry-run |
| 12 | Submit assignment (file) | canvas-cli-and-mcp | `canvas submissions submit ... --file` | --dry-run, progress |
| 13 | Grade submission | canvas-mcp (instructor) | `canvas submissions grade <course_id> <assignment_id> <user_id> --score` | --dry-run, --comment |
| 14 | List announcements | canvas-mcp, all | `canvas announcements list` | Cross-course by default, --course filter |
| 15 | Get announcement | canvas-mcp | `canvas announcements get <id>` | --json |
| 16 | Create announcement | canvas-mcp (instructor) | `canvas announcements create <course_id>` | --dry-run |
| 17 | List modules | canvas-cli-and-mcp | `canvas modules list <course_id>` | --json |
| 18 | List module items | canvas-cli-and-mcp | `canvas modules items <course_id> <module_id>` | --json, completion status |
| 19 | List files | canvas-cli-and-mcp, canvasapi | `canvas files list <course_id>` | --json, size/type filter |
| 20 | Download file | canvas-mcp, canvasapi | `canvas files download <file_id>` | Progress bar, --out path |
| 21 | Upload file | canvasapi | `canvas files upload <course_id> <path>` | --dry-run, progress |
| 22 | List discussions | canvas-mcp, canvas-cli-and-mcp | `canvas discussions list <course_id>` | unread count, --json |
| 23 | Get discussion | canvas-mcp | `canvas discussions get <course_id> <id>` | --json with entries |
| 24 | Create discussion post | canvas-mcp (instructor) | `canvas discussions create <course_id>` | --dry-run |
| 25 | Reply to discussion | canvas-cli-and-mcp | `canvas discussions reply <course_id> <topic_id>` | --dry-run |
| 26 | List pages | canvas-cli-and-mcp | `canvas pages list <course_id>` | --json |
| 27 | Get page | canvas-cli-and-mcp | `canvas pages get <course_id> <url_or_id>` | --json, --render (strip HTML) |
| 28 | List todos | fuller, canvas-cli-and-mcp | `canvas todos list` | --json, cross-course |
| 29 | Ignore todo | fuller | `canvas todos ignore <id>` | --dry-run |
| 30 | List upcoming items | canvasctl, canvas-cli-and-mcp | `canvas planner upcoming` | Uses /planner/items; cross-course |
| 31 | Get user profile | canvas-cli-and-mcp, canvasapi | `canvas me` | --json, shows enrollment info |
| 32 | List enrollments | canvas-mcp, canvasapi | `canvas enrollments list <course_id>` | --json |
| 33 | List calendar events | canvas-mcp | `canvas calendar list` | --start/--end date range, --json |
| 34 | Create calendar event | canvas-mcp | `canvas calendar create` | --dry-run |
| 35 | List conversations (inbox) | canvas-mcp | `canvas inbox list` | unread indicator, --json |
| 36 | Get conversation | canvas-mcp | `canvas inbox get <id>` | --json with participants |
| 37 | Send message | canvas-mcp | `canvas inbox send` | --dry-run, --to, --subject, --body |
| 38 | List rubrics | canvas-mcp | `canvas rubrics list <course_id>` | --json |
| 39 | Get rubric | canvas-mcp | `canvas rubrics get <course_id> <id>` | --json with criteria |
| 40 | List quiz submissions | canvas-mcp | `canvas quizzes submissions <course_id> <quiz_id>` | legacy quizzes only |
| 41 | Sync all data locally | canvasctl, canvas-ai-cli | `canvas sync` | All resources, --full, --since date |
| 42 | Search local store | canvasctl (diff) | `canvas search <query>` | FTS5 across assignments/announcements/files |
| 43 | SQL query | none | `canvas sql "<SELECT...>"` | Agent-friendly raw SQL against local store |
| 44 | Doctor / health check | canvas-mcp | `canvas doctor` | Verify token, base URL, API reachable |

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Grade impact calculator | `canvas impact <course>` | 10 | Joins `assignments.points_possible` (unsubmitted) × `enrollments.current_score` × `final_points_possible` — math no single endpoint does |
| 2 | Deadline pressure index | `canvas pressure` | 10 | Cross-course join: `assignments.due_date` × `points_possible` × `submission.workflow_state` × `enrollments.current_score` across ALL courses |
| 3 | Grade drift tracker | `canvas drift` | 9 | Compares current `enrollments.current_score` against SQLite-stored prior snapshots — Canvas has no historical grade API |
| 4 | Silent drop detector | `canvas going-dark` | 9 | Joins `announcements.unread_count` × `module_items.completion_status` × `assignments.due_date` — three separate resources, no combined endpoint |
| 5 | Late submission window detector | `canvas late-window <course>` | 9 | Aggregates `submitted_at - due_date` deltas where `late=true` AND `score > 0` across all past submissions — single API call returns one at a time |
| 6 | Cross-course workload forecast | `canvas forecast [--weeks 2]` | 8 | Sums `points_possible × effort_weight(submission_type)` across all courses per day — Canvas has no calendar-aggregate endpoint |
| 7 | Unread-to-deadline correlation | `canvas heads-up` | 8 | Joins `announcements.posted_at` × `assignments.due_date` × `submissions.missing` — three resources, same course_id, no combined endpoint |
| 8 | Completion gap report | `canvas gaps <course>` | 8 | Joins `module_items.completion_status` × `type='Assignment'` × `assignments.due_date` — Canvas returns modules and assignments in separate calls |
| 9 | LLM assignment digest | `canvas digest <assignment_id>` | 7 | Synthesizes `assignments.description` + `rubric_settings` + `announcements.body` across three entities locally; joins are only useful with local store |
| 10 | Submission type mismatch alert | `canvas check-type` | 7 | Builds per-student submission type frequency table across all course history, then diffs with upcoming `assignments.submission_types` |

## Stubs
- `canvas quizzes submissions` — New Quizzes only available via LTI, not REST; this covers legacy quizzes only. Will ship with `(stub — New Quizzes requires LTI, not REST)` note.
