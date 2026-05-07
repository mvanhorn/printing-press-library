# Novel Features Brainstorm — Trigger.dev (reprint)

> Subagent output preserved for retro/dogfood debugging per
> `references/novel-features-subagent.md` Step 3 (audit trail).

## Customer model

**Persona 1: Maya — AI-product engineer at a 30-person startup**

*Today (without this CLI):* Keeps two browser tabs permanently pinned to the Trigger.dev dashboard — one filtered to her `daily-digest` task, one on the runs feed. When a customer reports "the agent didn't reply," she opens the dashboard, eyeballs the runs list, drills into a span tree, copies the model name and token totals into a Notion doc, and re-does this for every incident. She cannot answer "which model+task pair is eating my LLM budget this week?" without a manual export-to-CSV ritual.

*Weekly ritual:* Triages last week's failed AI runs, files the dominant failure signature with engineering, reports rough LLM spend to her CTO before the Stripe invoice arrives.

*Frustration:* The dashboard's run-search loses her filter when she paginates, and there's no aggregation view across many runs — the LLM cost per (model, task) lives one click deep on each individual span.

**Persona 2: Diego — backend engineer rotating through oncall**

*Today (without this CLI):* When a deploy goes out at 4pm Friday, he keeps the dashboard's run feed open in a tab and refreshes it every few minutes for the rest of the afternoon. Trigger.dev only pages him via email and Slack alerts that fire after the platform's own latency budget — he learns about failures 2-5 minutes late. When a customer pings #incidents, he doesn't know whether his deploy caused it without manually filtering the dashboard by deployment ID.

*Weekly ritual:* Watches the post-deploy window for the runs that matter (his team's tasks), responds to PagerDuty pages, looks up "did this fail because of my code or because of the platform?"

*Frustration:* No terminal-native way to be notified the second a failure lands; the dashboard is the only signal source and it requires eyes-on a tab.

**Persona 3: Priya — platform/SRE engineer who owns Trigger.dev for the whole org**

*Today (without this CLI):* Manages 12 Trigger.dev projects across dev/staging/prod. After a Postgres URL rotation, she manually clicks into each project's schedules page and visually scans for "last run" dates older than the rotation. She maintains a hand-typed spreadsheet of which env vars exist in which environment. When a teammate asks "why does this work in staging but not prod," she opens two browser tabs side-by-side and squints.

*Weekly ritual:* Schedule hygiene sweep, env-var drift audit, deployment-promotion review, queue concurrency tuning.

*Frustration:* Eye-scanning two dashboard tabs to diff env vars; finding zombie schedules requires guessing which schedules to click into; no scriptable way to assert "all 12 projects have a healthy cron firing this week."

**Persona 4: Coding agent (Claude Code / similar) operating Trigger.dev on behalf of any of the above**

*Today (without this CLI):* Has to use the official MCP (5 read-shaped tools) plus shell-out to `curl` for everything else. Cannot branch on run terminal state without parsing JSON; cannot grep error messages without an online HTTP round-trip per query; loses the user's filter context across multi-step debugging sessions.

*Weekly ritual:* Whatever Maya, Diego, or Priya pastes into chat — debug a run, draft an incident report, rotate env vars, replay a batch.

*Frustration:* No typed exit codes on terminal-state commands forces an extra parse step on every loop iteration; the dashboard's TRQL endpoint requires building a SQL string from scratch each time instead of typing one canonical CLI flag.

## Candidates (pre-cut)

(See subagent output above; 16 candidates sourced from personas, service-specific patterns, cross-entity joins, and reprint reconciliation. 7 prior-keep, 1 prior-drop, plus 8 new candidates.)

## Survivors and kills

### Survivors (8)

| # | Feature | Command | Score | How It Works | Evidence | Source |
|---|---------|---------|-------|-------------|----------|--------|
| 1 | LLM span cost rollup | `runs span-cost --since 7d --by model,task --top 20` | 9/10 | Local SQL group-by over synced run-span enrichment, fills gaps via real `/api/v1/runs/{id}/spans/{spanId}` | Brief workflow #6; Maya frustration; spec exposes per-span model+tokens+cost | prior (kept) |
| 2 | Real-time failure watch | `runs watch --task <id> --notify --sound` | 9/10 | Polls `/api/v1/runs?status=FAILED` + local cursor; non-zero exit + OS notify + sound when new failures arrive; short-circuits in `cliutil.IsVerifyEnv()` | Brief workflow #5; Diego frustration | prior (kept) |
| 3 | Stale schedule detection | `schedules stale --days 14 --min-success-rate 0.5` | 8/10 | Joins synced `schedules` table with recent runs in local SQLite | Brief workflow #3; Priya frustration | prior (kept) |
| 4 | Env var diff | `envvars diff --from prod --to staging` | 8/10 | Two real `/api/v1/projects/{ref}/envvars/{env}` GETs + Go set-diff with masked values | Brief workflow #7; Priya frustration | prior (kept) |
| 5 | Recurring failure patterns | `failures top --since 7d --top 20` | 8/10 | Local group-by on (task, regex-normalized error signature) over cached runs | Brief workflow #2; Maya frustration | prior (kept) |
| 6 | FTS over run errors | `runs find "<query>" --status FAILED --since 30d` | 8/10 | SQLite FTS5 over cached error/tag/metadata; composes with status/since/json filters offline | Brief data layer ("FTS/search: critical") | prior (kept) |
| 7 | Typed exit codes per terminal status | `runs get <run-id>` | 7/10 | Cobra `pp:typed-exit-codes` annotation maps run terminal states to exit codes 0/20/21/22/23/3/4 | Brief workflow #1; agent-frustration | prior (kept) |
| 8 | TRQL recipe library | `query recipes` / `query run <recipe>` | 6/10 | Static-curated recipe catalog (`// pp:novel-static-reference`) + real `/api/v1/query` POST with parameter substitution | TRQL is brief's distinctive analytics surface; agents can't author TRQL from scratch reliably | new |

### Killed candidates (8)

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| Cross-task health rollup | Duplicates `failures top` + `runs span-cost` + `runs find` without a sharper persona moment | Failures top (#5) |
| Cross-project schedule sweep | Scope creep — printed CLIs are single-key/single-project by design | Stale schedule detection (#3) |
| Queue saturation watch | Monthly-only use (capacity tuning) | Real-time failure watch (#2) |
| Run replay-with-overrides | Already absorbed; `--override-payload` is a flag on the absorbed wrapper | (absorb manifest #2) |
| Cost-budget alarm | Thin scripting wrapper over `runs span-cost --json \| jq` | LLM span cost rollup (#1) |
| Deployment promotion audit | Borderline weekly-use; diff itself is thin | Env var diff (#4) |
| Waitpoint stuck audit | Weekly only for waitpoint-heavy teams | Stale schedule detection (#3) |
| Schedules preview | Belongs as a `--dry-run` flag on `schedules create` | Stale schedule detection (#3) |

## Reprint verdicts

| Prior feature | Verdict | Justification |
|---------------|---------|---------------|
| Real-time failure watch (`runs watch`) | Keep | Diego persona's frustration unchanged; brief workflow #5; score 9/10 |
| Cross-task health rollup (`tasks health`) | **Drop** | Duplicates survivor trio (`failures top` + `runs span-cost` + `runs find`); persona-fit didn't materialize on re-scoring |
| Stale schedule detection (`schedules stale`) | Keep | Priya persona's weekly hygiene ritual; brief workflow #3; score 8/10 |
| Env var diff (`envvars diff`) | Keep | Priya persona's frustration; brief workflow #7; score 8/10 |
| LLM span cost rollup (`runs span-cost`) | Keep | Maya persona's CTO/finance ritual; brief workflow #6; score 9/10 |
| Recurring failure patterns (`failures top`) | Keep | Maya's incident-triage ritual; brief workflow #2; score 8/10 |
| FTS over run errors (`runs find`) | Keep | Brief data layer explicitly calls out FTS5; score 8/10 |
| Typed exit codes (`runs get`) | Keep | Brief workflow #1; coding-agent persona's primary loop affordance; score 7/10 |
