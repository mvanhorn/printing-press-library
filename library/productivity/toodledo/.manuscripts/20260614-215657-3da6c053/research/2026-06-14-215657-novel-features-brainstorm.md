# Toodledo CLI — Novel Features Brainstorm (audit trail)

## Customer model

**Dana — The GTD Practitioner.** Runs a textbook GTD system in Toodledo (folders=projects, contexts, goals, full status enum). Today lives in the web UI + a personal MCP, but every "what's next" / weekly review refetches the whole task universe and burns the 100-call token budget. Weekly ritual: Sunday-night review walking inbox → overdue → stalled projects → waiting-for → someday. Frustration: reviews are slow, online-only, and one chatty session exhausts the rate limit.

**Sam — The Rapid Capturer.** Throws tasks at Toodledo from a terminal alias; processes inbox in batches. Today adds one task at a time (one call each), eyeballs a web list for "what now". Weekly ritual: inbox processing (assign folder/context/priority, bulk-complete trivia). Frustration: no fast offline "next action for @work right now"; name→id resolution only exists in their MCP; batch capture isn't rate-budget aware.

**Riley — The Agent Integrator.** Wires Toodledo into agent workflows; wants composable offline JSON primitives. Today post-processes MCP `gtd_dashboard`/`gtd_review` JSON and hits the rate ceiling. Weekly ritual: scripts pulling counts-by-status/priority/folder, week-over-week diffs. Frustration: can't SELECT across tasks+folders+goals without paying API calls; can't preview a sync's cost.

## Candidates (pre-cut)
C1 next-actions (KEEP), C2 review (KEEP), C3 dashboard (CUT — dup analytics), C4 stalled-projects (KEEP), C5 inbox (CUT — thin filter/dup review bucket), C6 waiting (CUT — rename of list --status 5), C7 goal-progress (KEEP), C8 agenda (CUT — list --overdue + presentation), C9 sync-cost (KEEP), C10 today (CUT — saved-view union), C11 context-load (CUT — subset of next-actions), C12 orphans (CUT — collides framework `orphans`), C13 stale (CUT — collides framework `stale`), C14 capture (KEEP), C15 throughput (CUT — dup analytics), C16 repeat-audit (CUT — poor verifiability).

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | GTD next actions | `next-actions [--context <name>] [--goal <name>]` | 8/10 | hand-code | Reads local SQLite, joins tasks→contexts/goals, filters status=1, sorts priority desc / due asc | toodledo-mcp `gtd_next_actions`; brief Top Workflow #2 | none |
| 2 | Weekly review buckets | `review` | 9/10 | hand-code | Five offline aggregations in one pass: inbox (no folder+context), overdue, stalled projects (anti-join), waiting (status=5), someday (status=8) | toodledo-mcp `gtd_review`; brief Top Workflow #3 | Use for the full five-bucket weekly review; for only stalled projects use `stalled-projects`. |
| 3 | Stalled projects | `stalled-projects [--days N]` | 7/10 | hand-code | LEFT JOIN folders→tasks; folders with incomplete tasks but zero status=1 Next Actions | GTD methodology; brief stalled-projects bucket; toodledo-mcp review | none |
| 4 | Goal progress rollup | `goal-progress [--level lifetime\|long\|short]` | 7/10 | hand-code | Joins tasks→goals, counts incomplete vs done per goal, walks `contributes` self-reference to roll children into parents | brief Data Layer (goal level + contributes); API goal hierarchy | none |
| 5 | Sync cost preview | `sync-cost [--resources <csv>] [--since 7d]` | 8/10 | hand-code | Calls real `account/get.php` cursors, diffs vs local cursors, reports projected API-call count vs the 100-call/token budget without fetching rows | brief Reachability Risk (100 calls/token); Build Priority #3 | Use to preview cost only; `sync` performs the actual incremental fetch. |
| 6 | Batch capture | `capture --file <path>` (or stdin) | 7/10 | hand-code | Reads one title/line, resolves folder/context names→ids, writes via the `tasks=<JSON>` batch param in budget-aware chunks of 50 | brief Codebase Intel (50/call batch + name→id); Top Workflow #1 | none |

### Killed candidates
| feature | kill reason | closest-surviving-sibling |
|---------|-------------|--------------------------|
| C3 dashboard | Multi-axis count duplicates `analytics --group-by`. | C2 review |
| C5 inbox | Thin compound filter; subsumed by review's inbox bucket. | C2 review |
| C6 waiting | Rename of `tasks list --status 5`; a review bucket. | C2 review |
| C8 agenda | `tasks list --overdue --sort due` + presentation grouping. | C1 next-actions |
| C10 today | Saved-view union; convenience, not leverage. | C1 next-actions |
| C11 context-load | Subset of `next-actions --context`. | C1 next-actions |
| C12 orphans | Collides with framework-reserved `orphans`. | C2 review |
| C13 stale | Collides with framework-reserved `stale`. | C4 stalled-projects |
| C15 throughput | Grouped completed counts = `analytics --group-by`. | C7 goal-progress |
| C16 repeat-audit | Poor verifiability (server regenerates repeats). | C4 stalled-projects |
