# Clockify CLI — Absorb Manifest

Sources catalogued: lucassabreu/clockify-cli (Go, 194★), JeremyVyska/clockify-mcp (33-tool MCP), BlythMeister/ClockifyCli (C#), mentarch/clockify-cli (TS), artefactual-labs/clockify-tool (Ruby), 3 MCP servers, Python wrappers (clockify-sdk et al.). Official OpenAPI spec: 99 paths / 155 operations.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Start timer (interactive / clone-previous) | lucassabreu, BlythMeister | `start` → POST /time-entries | `--dry-run`, `--json`, idempotent |
| 2 | Stop running timer | all | `stop` → PATCH in-progress entry | typed exit codes |
| 3 | Show in-progress timer | all | `status` → GET /time-entries/status/in-progress | `--json`, offline last-known fallback |
| 4 | Discard active timer | BlythMeister | `discard` → DELETE current entry | `--dry-run` confirm |
| 5 | Pause / resume timer | mentarch | `pause` / `resume` (stop + restart) | preserves description/tags |
| 6 | List entries by day/week/range | all | `time-entries list` → GET /user/{id}/time-entries | local store + `--select`, `--csv` |
| 7 | Add manual entry (duration or start/end) | all | `time-entries create` → POST | intelligent time parser, `--dry-run` |
| 8 | Edit time entry | all | `time-entries update` → PUT /time-entries/{id} | `--dry-run` shows diff |
| 9 | Delete time entry | all | `time-entries delete` → DELETE /time-entries/{id} | `--dry-run` |
| 10 | Duplicate / clone entry | lucassabreu | POST /user/{id}/time-entries/{id}/duplicate | — |
| 11 | Bulk update time entries | JeremyVyska MCP | PATCH /user/{id}/time-entries | `--stdin` batch |
| 12 | Bulk delete time entries | JeremyVyska MCP | DELETE /user/{id}/time-entries | `--dry-run` lists targets |
| 13 | Projects CRUD (archived/client/name filters) | all | spec project endpoints | local store, `--json` |
| 14 | Tasks CRUD (estimate, assignments) | JeremyVyska | spec task endpoints | local store |
| 15 | Clients CRUD (archive/unarchive) | JeremyVyska, lucassabreu | spec client endpoints | local store |
| 16 | Tags CRUD (archive/unarchive) | JeremyVyska, lucassabreu | spec tag endpoints | local store |
| 17 | Workspaces list/get/update + switch active | all | spec endpoints + local config | persisted default workspace |
| 18 | Users / user-groups list | lucassabreu, JeremyVyska | spec user endpoints | local store |
| 19 | Reports today/week/month/custom | lucassabreu, mentarch | `recap` (transcendence #5) computed offline | works offline, no Reports-API host needed |
| 20 | CSV export | mentarch | `--csv` output mode | generator-native, on every list command |
| 21 | Breaks report | BlythMeister | offline aggregation of break-type entries | folds into `recap` |
| 22 | Week-view / timesheet grid | BlythMeister | `timesheet week` (transcendence #1) | offline, gap-aware, submit-capable |
| 23 | Config init/set/view | all | generator-native `config` command | — |
| 24 | Auth login/status/logout | mentarch | generator-native `auth` + `doctor` | env-var + stored token |
| 25 | Intelligent time input (24h/12h/abbreviated) | BlythMeister | duration/time flag parser | shared across create/edit |
| 26 | Time-entry templates | artefactual-labs | local template store + `apply` | persists in SQLite |
| 27 | Timer monitor / notifications | BlythMeister | `watch` command (curtails under dogfood) | — |
| 28 | Expenses + categories CRUD | (no competitor) | generator-emitted typed commands | first terminal tool to cover it |
| 29 | Invoices full lifecycle (items/payments/status/export) | (no competitor) | generator-emitted typed commands | first terminal tool to cover it |
| 30 | Time-off / PTO policies, requests, balances | (no competitor) | generator-emitted typed commands | first terminal tool to cover it |
| 31 | Holidays | (no competitor) | generator-emitted typed commands | feeds gap detection |
| 32 | Approval requests (submit/approve/reject/resubmit) | (no competitor) | generator-emitted + `timesheet`/`team` commands | first terminal tool to cover it |
| 33 | Scheduling assignments (resource planning) | (no competitor) | generator-emitted typed commands | first terminal tool to cover it |
| 34 | Custom fields | (no competitor) | generator-emitted typed commands | — |
| 35 | Cost-rate / hourly-rate management | (no competitor) | generator-emitted typed commands | feeds billable audit |
| 36 | Webhooks | (no competitor) | generator-emitted typed commands | — |

Every competitor feature is matched; the official-API half no competing CLI touches (rows 28-36) is covered by generator-emitted typed commands.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Persona |
|---|---------|---------|-------|--------------|-------------------------|---------|
| 1 | Timesheet week reconstruct | `timesheet week [--date] [--submit]` | 9/10 | hand-code | Pivots synced time-entries into a project/task × weekday grid in local SQLite with per-day/project/week totals; `--submit` runs a gap check then POSTs an approval-request. Incumbent has no local store. | Dana |
| 2 | Timesheet gap finder | `timesheet gaps [--date] [--workday 8h]` | 8/10 | hand-code | Compares each day's synced entry total against a workday target, reports missing hours with surrounding entry context — pure local computation. No competitor detects holes. | Dana |
| 3 | Billable leakage audit | `audit billable [--range]` | 8/10 | hand-code | Joins time-entries × projects × clients × tags in SQLite to flag billable-but-projectless, billable-task-marked-nonbillable, and untagged-billable entries. | Marcus |
| 4 | Team submission tracker | `team timesheets [--date]` | 8/10 | hand-code | Diffs the full workspace users list against the week's approval-requests to surface submitted / pending / not-submitted-at-all members, plus per-member hours/billable/sanity flags. | Priya |
| 5 | Where-did-my-week-go recap | `recap [--range]` | 7/10 | hand-code | Aggregates synced entries into a ranked project/client/tag breakdown with billable vs non-billable split and % of tracked time, offline. | Marcus |
| 6 | Unbilled billable balance | `billable pending [--client]` | 7/10 | hand-code | Sums billable entry time not yet covered by any synced invoice's date range, grouped by client — the invoice-ready number. | Marcus |
| 7 | Project budget burn | `project burn [--client]` | 6/10 | hand-code | Joins synced entries against project budget/estimate fields to show logged-vs-estimated hours and percent consumed per project. | Priya |
| 8 | Full-text entry search | `search <query> [--billable] [--range]` | 6/10 | hand-code | SQLite FTS over synced time-entry descriptions and project/task/client/tag names with date and billable filters. | Marcus |
| 9 | Draft entries from logs and files | `import [--from csv\|shell-history\|session-log] [--file PATH] [--commit]` | 9/10 | hand-code | Reconstruct time you forgot to track: source-specific parsers turn a CSV export, shell history, or a CLI session log into draft entries with inferred time windows; preview by default, `--commit` POSTs them. Impossible for a timer-only tool with no parsers and no draft/review loop. | Dana / Marcus (user-added) |

9 transcendence features, all hand-code (Cobra file + `root.go` wiring + SQLite-join or parser logic, ~50-200 LoC each). No stubs. Feature #9 added at the Phase 1.5 gate from the user's stated killer feature ("draft entries from spreadsheets, CSV files, shell history, and CLI session logs") — it also answers the user's stated pain point ("forgetting to start/stop timers") by reconstructing untracked time from artifacts. See `2026-05-20-224501-novel-features-brainstorm.md` for the customer model and killed-candidate audit trail.
