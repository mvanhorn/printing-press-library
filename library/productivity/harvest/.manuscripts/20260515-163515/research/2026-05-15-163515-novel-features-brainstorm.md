# Harvest CLI — Novel Features Brainstorm (subagent audit trail)

## Customer model

**Persona A — Maya, Delivery PM at a 30-person dev shop**
- Today: Pings engineers in Slack every Friday for missing hours, exports CSVs from Harvest web UI, rebuilds the same pivot in Google Sheets to see project burn and who's under-logged.
- Weekly ritual: Mon — set up project allocations; Wed — mid-week burn check on her 6 active projects; Fri — chase missing time, send client status with hours-used vs budget.
- Frustration: No way to see "which of my devs have gaps this week" or "which projects burned >80% of budget" without exporting + manual joins. Harvest web filters reset between sessions.

**Persona B — Ravi, Senior Engineer juggling 4 client projects**
- Today: Forgets to log time, reconstructs the week Friday afternoon from calendar + git log + Slack. Uses harvest-cli or the web UI's task switcher.
- Weekly ritual: Daily start-timer in the morning, often forgets to switch when context-switching; Friday backfill of 5-15 entries with notes.
- Frustration: Can't search his own historical notes ("what did I call that auth refactor last month?"), can't see his own billable% trend, retypes the same `--project X --task Y` flags every day.

**Persona C — Dana, Finance/Ops doing month-end invoicing**
- Today: Pulls the Harvest "Uninvoiced" report, cross-checks against project budgets, manually flags clients whose retainers are over/under, drafts invoices in Harvest UI one client at a time.
- Weekly ritual: 1st-3rd of month — uninvoiced sweep; 15th — retainer burn check; ad-hoc — answer "is client X profitable?" from leadership.
- Frustration: Harvest reports don't blend time + expenses + cost-rates locally, so margin/realization rate questions take hours. No diff between this month's uninvoiced and last month's.

**Persona D — Sam, Agent/Script author (Dan-style power user)**
- Today: Runs Harvest MCP on Cloud Run for AI agents, but wants offline/local queries for cron-driven scripts and ad-hoc terminal work without network round-trips.
- Weekly ritual: Nightly sync cron; scripts that pipe `harvest-pp` output into jq/claude/sqlite for custom dashboards.
- Frustration: Existing CLIs don't expose a queryable local store; every script re-paginates the API and re-implements the same joins. Rate limit (100/15s) bites when reconciling a year of data.

## Candidates (pre-cut)

(See subagent output above for full list of 16 candidates with kill/keep verdicts.)

## Survivors and kills

### Survivors (8 features scoring >= 5/10)

| # | Feature | Command | Score | How It Works | Evidence |
|---|---------|---------|-------|-------------|----------|
| 1 | Timesheet gap detection | `timesheet gaps --user <u> --from <d> --to <d> --min-hours 6` | 8/10 | Joins local users × time_entries × workday calendar; emits rows where daily total < threshold | Maya's Friday chase ritual; no competitor ships this |
| 2 | Project burn + projection | `project burn [--threshold 80] [--projection]` | 9/10 | projects (budget) JOIN time_entries; 4-week velocity → projected exhaust date; exit 2 if over threshold | Brief priority; Maya's mid-week check; harvest-toolkit only does flat reports |
| 3 | Notes FTS | `notes search "<query>" [--mine] [--project] [--from/--to]` | 9/10 | SQLite FTS5 index on time_entries.notes populated during sync | Brief Product Thesis explicitly cites offline FTS; zero competitor offers it |
| 4 | Client margin / realization | `client margin --client <c> --from <d> --to <d>` | 8/10 | Joins time_entries × billable_rates × cost_rates × clients; revenue − cost, realization % | Dana's leadership Q; absorb #24 only lists rates, no joined margin |
| 5 | Utilization trend | `utilization [--user] [--weeks 12]` | 7/10 | Local group-by on time_entries.billable per user per ISO week | Top Workflow 3 ("billable %"); agency-defining metric |
| 6 | Day reconstruction stubs | `day reconstruct --user <u> --date <d> --target-hours 8` | 7/10 | Reads existing entries, computes deficit, emits JSON stubs proportional to recent project mix; pipes into create --stdin | Ravi's Friday backfill; no tool generates these |
| 7 | Entry repeat | `time-entries repeat <id> [--to <date>] [--days N]` | 6/10 | POSTs copies with new spent_date; idempotent via natural key | Ravi's daily pain; hrvst-cli lacks; mechanical |
| 8 | Reconcile local vs API | `reconcile --from <d> --to <d>` | 6/10 | Re-fetches range, diffs against local snapshots, prints drifted IDs and field changes | Sam/agent persona; foundation-level integrity |

### Killed candidates

| Feature | Why killed | Sibling kept |
|---------|-----------|--------------|
| `timesheet chase` | Subset of `timesheet gaps` | `timesheet gaps` |
| `my week` | Thin wrapper; flag-recall is shell concern | `notes search` |
| `uninvoiced diff` | Reframes as flag on absorbed `reports uninvoiced` | Absorbed `reports uninvoiced` |
| `stale projects` | Better as flag on `projects list --active --no-entries-since` | Absorbed `projects list` |
| `task mix` | Thin renaming of `reports time --group-by task` | Absorbed `reports time` |
| `rate audit` | Requires historical rate snapshots; build cost high for niche value | `client margin` (current rates only) |
| `who-on-what` | Overlaps `project burn` + `utilization`; no persona uses weekly | `project burn`, `utilization` |
| `budget-alert` | Folded into `project burn --threshold 80` | `project burn` |
