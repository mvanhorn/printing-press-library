## Absorb Manifest

### Absorbed

| # | Feature | Best source | Our implementation | Added value |
|---|---|---|---|---|
| 1 | Authenticate and inspect current identity | Official API `/me` | `(generated endpoint) identity me` | Bearer env-var setup, agent JSON, and doctor |
| 2 | List and get projects | Official API | `(generated endpoint) workspaces projects-list and project-get` | Pagination and structured output |
| 3 | List project workspaces | Official API | `(generated endpoint) workspaces project-workspaces-list` | Channel-aware deep links |
| 4 | Create and inspect workspaces | Official API | `(generated endpoint) workspaces workspace-create and workspace-get` | Safe body-JSON fallback for project or repository launch |
| 5 | Rename, sleep, and archive workspaces | Official API | `(generated endpoint) workspaces workspace-rename, workspace-sleep, and workspace-archive` | Dry-run and explicit mutation commands |
| 6 | List, create, and inspect sessions | Official API | `(generated endpoint) workspaces workspace-sessions-list, session-create, and session-get` | Typed agent/model/effort inputs |
| 7 | Rename and archive sessions | Official API | `(generated endpoint) workspaces session-rename and session-archive` | Dry-run and structured errors |
| 8 | Send, list, and fetch messages | Official API | `(generated endpoint) workspaces message-create, session-messages-list, and message-get` | Cursor-aware transcript reads |
| 9 | Inspect workspace and session status | Official API | `(generated endpoint) workspaces workspace-status-get and session-status-get` | Stable status envelopes for orchestration |
| 10 | Cancel a running session | Official API | `(generated endpoint) workspaces session-cancel` | Idempotent request plus higher-level confirmation polling |
| 11 | Query transcript SQL | Official API | `(generated endpoint) roundhouse-public-sql sql-query` | Constrained read-only input and agent JSON |
| 12 | Launch a workspace and first task | ENG-525 | `conductor-pp-cli launch` | One bounded command returns workspace, session, and deep link |
| 13 | Monitor work to real completion | ENG-525 | `conductor-pp-cli monitor` | Handles queued false-idle, fast turns, transcript cursors, and timeout |
| 14 | Steer a running session | ENG-525 | `conductor-pp-cli steer` | Sends follow-up guidance with clear delivery state |
| 15 | Launch, monitor, and collect output | ENG-525 | `conductor-pp-cli run` | One command with bounded polling and no merge or deploy side effects |
| 16 | Separate planning and implementation sessions | ENG-525 | `conductor-pp-cli plan-implement` | Keeps reviewer and implementer context separate in one workspace |
| 17 | Summarize recent transcript activity | ENG-525 | `conductor-pp-cli daily-report` | Mechanical transcript rows and stats ready for downstream analysis |
