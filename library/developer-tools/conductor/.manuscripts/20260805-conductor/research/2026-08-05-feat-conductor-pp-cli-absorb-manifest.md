# Conductor Cloud absorb manifest

The public search found no Conductor Cloud CLI, SDK, or MCP server to absorb. The shipping manifest therefore matches all 21 official API operations and implements the six orchestration workflows approved in ENG-525.

## Shipping scope

| # | Feature | Best source | Our implementation | Added value |
|---|---|---|---|---|
| 1 | Authenticate and inspect current identity | Official API `/me` | `(generated endpoint) identity me` | Bearer env-var setup, agent JSON, and doctor |
| 2 | List and get projects | Official API | `(generated endpoint) projects list and projects get` | Pagination and structured output |
| 3 | List project workspaces | Official API | `(generated endpoint) projects workspaces` | Channel-aware deep links |
| 4 | Create and inspect workspaces | Official API | `(generated endpoint) workspaces create and workspaces get` | Safe body-JSON fallback for project or repository launch |
| 5 | Rename, sleep, and archive workspaces | Official API | `(generated endpoint) workspaces rename, workspaces sleep, and workspaces archive` | Dry-run and explicit mutation commands |
| 6 | List, create, and inspect sessions | Official API | `(generated endpoint) sessions list, sessions create, and sessions get` | Typed agent/model/effort inputs |
| 7 | Rename and archive sessions | Official API | `(generated endpoint) sessions rename and sessions archive` | Dry-run and structured errors |
| 8 | Send, list, and fetch messages | Official API | `(generated endpoint) messages send, messages list, and messages get` | Cursor-aware transcript reads |
| 9 | Inspect workspace and session status | Official API | `(generated endpoint) workspaces status and sessions status` | Stable status envelopes for orchestration |
| 10 | Cancel a running session | Official API | `(generated endpoint) sessions cancel` | Idempotent request plus higher-level confirmation polling |
| 11 | Query transcript SQL | Official API | `(generated endpoint) transcripts query` | Constrained read-only input and agent JSON |
| 12 | Launch a workspace and first task | ENG-525 | `conductor-pp-cli launch` | One bounded command returns workspace, session, and deep link |
| 13 | Monitor work to real completion | ENG-525 | `conductor-pp-cli monitor` | Handles queued false-idle, fast turns, transcript cursors, and timeout |
| 14 | Steer a running session | ENG-525 | `conductor-pp-cli steer` | Sends follow-up guidance with clear delivery state |
| 15 | Launch, monitor, and collect output | ENG-525 | `conductor-pp-cli run` | One command with bounded polling and no merge or deploy side effects |
| 16 | Separate planning and implementation sessions | ENG-525 | `conductor-pp-cli plan-implement` | Keeps reviewer and implementer context separate in one workspace |
| 17 | Report recent transcript activity | ENG-525 | `conductor-pp-cli daily-report` | Mechanical transcript rows and stats ready for downstream analysis |

## Scope decision

The adversarial feature pass also proposed `fleet`, `stuck`, `queue-audit`, `cleanup-plan`, and `audit-export`. Those are valid later additions but are not part of ENG-525. They remain in the brainstorm artifact and are excluded from this build.
