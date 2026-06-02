# ClickUp novel-features brainstorm (audit trail)

## Customer model

**Priya - Backend engineer on a 9-person product squad**
- Today: Lives in terminal+tmux. Resents the web app (verbose JSON, loading spinners). Has a clickup-cli alias that mirrors the API but goes blind when WiFi drops.
- Weekly ritual: Monday standup prep, EOD status updates, time-logging across context switches. Closes 5-15 tasks/week.
- Frustration: Can't answer "what changed on my tasks since yesterday" without per-task activity scrolling. Offline her CLI is a brick. Searching old comments means clicking through spaces.

**Marco - Engineering team lead / delivery owner**
- Today: Two squads, accountable for velocity and burnout. Web reports need a paid Dashboard add-on.
- Weekly ritual: Friday sprint review - what shipped/slipped/who's overloaded. Pulls timesheets, eyeballs board, guesses which tasks are stuck in review.
- Frustration: "Who's overloaded?" and "how long in review?" need manual counting or paid tier. Exports CSVs and pivots in a spreadsheet every Friday.

**Ada - AI agent operator**
- Today: Agents triage tasks, set fields, post comments. Needs --json/--select, --dry-run, env token.
- Weekly ritual: Batch ops across lists; resolving fuzzy input (assign me, status review, due tomorrow) to hard IDs.
- Frustration: Every resolver call is another HTTP request and rate-limit token. A 200-task triage becomes 800 API calls without a local mirror.

(Full candidate list and kill reasons preserved below.)

## Survivors (>=5/10)
1. changed-since <ts|last>  8/10 - activity delta from diffing synced snapshots
2. my-day [--due 7d]        8/10 - offline triage of my tasks across all lists
3. workload --space <id>    7/10 - assignee balance via tasks+members+time_entries join
4. time-in-status <list|task> 8/10 - cycle-time from status-change history captured at sync
5. stale --days <n>         7/10 - tasks with no movement in N days
6. unblocked / blocked      7/10 - dependency-aware "ready to work" set
7. resolve                  7/10 - offline batch resolver (status/assignee/date -> IDs), zero API calls

## Killed candidates
- Scoped search wrapper (too thin over absorbed FTS) -> my-day
- Timesheet pivot (overlaps workload) -> workload
- Sprint velocity (sprint modeling unverifiable) -> time-in-status
- Bottleneck ranking (fold as --rank on time-in-status) -> time-in-status
- Custom-field report (niche, no evidence) -> workload
- Goal-progress rollup (sparse linkage, often empty) -> time-in-status
- Reconcile/orphan detector (niche, no weekly need) -> stale
- Comment-thread export (collapses to --format md flag) -> my-day
- Sync --watch once (sync infra, not a feature) -> data layer
