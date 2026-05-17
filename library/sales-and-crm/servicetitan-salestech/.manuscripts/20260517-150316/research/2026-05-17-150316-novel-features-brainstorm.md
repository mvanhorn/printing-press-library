# Novel Features Brainstorm — servicetitan-salestech

> Full audit trail of the Phase 1.5c.5 novel-features subagent invocation
> (3-pass: customer model → candidates → adversarial cut).
> Generated 2026-05-17 against the Sales/Estimates brief.

## Customer model

**Persona A — Sales Manager Sam (JKA ops/owner-operator)**
- Today: Opens ST web UI every Monday, clicks through every Open estimate from the prior 14 days to ask "what's stuck?" One estimate per page load.
- Weekly ritual: Pipeline review meeting — needs stale-quote list, close-rate by rep, dismissed-reason patterns. Hand-compiles these from screenshots into a Google Sheet.
- Frustration: Cannot answer "which sales rep's quotes age the longest before selling?" without exporting and pivoting in Excel. The audit trail is buried two clicks deep per estimate.

**Persona B — Dispatcher Dana (CSR / coordinator)**
- Today: Phone rings — "where's the quote for job 4471?" She searches ST by job number, opens the estimate, reads line items aloud, then walks the status timeline to explain why it was dismissed last Tuesday.
- Weekly ritual: Triages overnight estimate changes — what got sold, what got unsold, what got dismissed and needs a callback.
- Frustration: No single screen shows "every estimate touched in the last 24 hours with its before/after status." She rebuilds this from the activity feed manually.

**Persona C — Field Tech Tom (presenting estimates on-site)**
- Today: Customer says "swap the 1-HP for a 1.5-HP." Tom calls back-office; CSR opens ST, edits line item, reads new total back over the phone.
- Weekly ritual: At end of day, reconciles his sold estimates against his commission tracker — did everything he closed today actually mark as Sold with him as `soldBy`?
- Frustration: No fast way to see "all estimates where I am soldBy this week, with totals and status." Has to ask Sam.

**Persona D — Pierce (automation owner / agent runtime)**
- Today: Builds agents that need to answer "what's the close rate by business unit this quarter?" in one MCP turn. Today the general ST MCP burns ~400 tools of context to answer.
- Weekly ritual: Runs nightly sync, then composes audits across estimates × items × status_changes for owner dashboards.
- Frustration: Cross-entity questions (close-rate, days-to-sell, dismissed-reason patterns, SKU frequency on sold vs dismissed) require either a custom script per question or a 400-tool MCP turn.

## Candidates (pre-cut)

1. stale-quotes (a, e) — list Open estimates older than N days, sorted by age × total. Sam.
2. rep-leaderboard (c, f) — per-employee: estimates created, sold, dismissed, close rate, avg days-to-sell, total sold $. Sam.
3. close-rate (c, e) — sold/(sold+dismissed) by business unit, rep, or month. Sam, Pierce.
4. days-to-sell (c, e) — p50/p90 days from estimate created → soldOn, broken out by rep/BU. Sam.
5. dismissed-reasons (NLP) — KILLED (NLP dependency). Reframed.
6. dismissed-reasons (mechanical) (b, c) — exact-match group-by on dismissal reason string. Sam.
7. estimate-audit (c, f) — single estimate forensic view: header + items + status timeline joined. Dana.
8. recent-changes (b, c) — every estimate whose status changed in window with from→to + actor. Dana.
9. find (f) — FTS5 across name/summary/jobNumber/sku.name with structured filters. Dana, Tom.
10. my-sold (b, c) — KILLED (thin wrapper).
11. commission-reconcile — KILLED (external file format not defined).
12. sku-frequency (b, c) — which SKUs appear most on sold vs dismissed estimates. Pierce, Sam.
13. unsold-watcher (b, c) — KILLED (subsumed by recent-changes filter).
14. health (f) — local-mirror health: row counts vs API, last sync cursor age, drift. Pierce.
15. bulk-dismiss (a) — KILLED (thin loop over absorbed dismiss).
16. estimate-diff — KILLED (we don't store historical line-item snapshots; would be misleading).
17. pipeline-snapshot (c, e) — total $ Open/Sold/Dismissed at past date using status_changes replay. Sam, Pierce.
18. estimate-export-replay — KILLED (duplicates sync --from with renamed flag).

## Survivors and kills

### Survivors

| # | Feature | Command | Why Only We Can Do This | Score | Persona | Buildability proof |
|---|---------|---------|-------------------------|-------|---------|--------------------|
| 1 | Stale quotes | `estimates stale --older-than 3d --status Open --json` | SQLite filter on Open + sort by computed (now - createdOn) × total; no API call sorts by computed age×$ | 10/10 | Sales Manager Sam | Reads local `estimates` table, filters status='Open', computes age days from createdOn, ORDER BY age*total DESC |
| 2 | Rep leaderboard | `reports rep-leaderboard --since 2026-01-01 --json` | Per-rep close-rate + avg days-to-sell + sold $ requires joining estimates × status_changes and aggregating | 10/10 | Sales Manager Sam | SQLite GROUP BY soldById JOIN status_changes WHERE toStatus='Sold' |
| 3 | Close rate | `reports close-rate --group-by businessUnit --since 90d` | sold/(sold+dismissed) pivoted on arbitrary dimension; not a single API call | 10/10 | Sam, Pierce | SQLite COUNT(CASE WHEN status='Sold') / COUNT(CASE WHEN status IN ('Sold','Dismissed')) GROUP BY chosen column |
| 4 | Days to sell | `reports days-to-sell --percentiles --since 90d` | Computes percentiles of (Sold timestamp - createdOn) per rep/BU; needs status_changes join | 9/10 | Sales Manager Sam | min(status_changes.changedAtUTC WHERE toStatus='Sold') - estimates.createdOn, NTILE for percentiles |
| 5 | Dismissed reasons | `reports dismissed-reasons --since 90d --top 20` | Exact-match group-by on reason text from status_changes; mechanical count | 7/10 | Sales Manager Sam | SQLite SELECT reason, COUNT(*) FROM status_changes WHERE toStatus='Dismissed' GROUP BY reason |
| 6 | Estimate audit | `audit estimate <id>` | Single-estimate forensic: header + every item + full status timeline in one shaped output | 10/10 | Dispatcher Dana | Three local SELECTs joined: estimates WHERE id, estimate_items WHERE estimateId, status_changes WHERE estimateId ORDER BY changedAtUTC |
| 7 | Recent changes | `audit recent-changes --since 24h --json` | All estimates whose status changed in window with from→to + actor; sibling pattern from CRM | 10/10 | Dispatcher Dana | SQLite JOIN status_changes (where changedAtUTC > now-24h) with estimates header |
| 8 | Find | `find "well pump" --status Open --min-total 5000` | FTS5 across name/summary/jobNumber/sku.name/sku.displayName with structured filters | 10/10 | Dana, Tech Tom | SQLite FTS5 virtual table + WHERE clauses on structured filters |
| 9 | Pipeline snapshot | `reports pipeline --as-of 2026-05-17 --json` | Reconstructs total $ Open/Sold/Dismissed for arbitrary past date by replaying status_changes; impossible via API | 8/10 | Sam, Pierce | For each estimate, compute status as-of date by finding last status_change <= as-of; sum totals by computed status |
| 10 | Health | `health` | Cross-source reconciliation: API count vs local count vs last cursor age per table | 8/10 | Pierce | Calls list endpoints with limit=1 for totalCount; compares to SQLite COUNT(*); reports cursor age per table |
| 11 | SKU frequency | `reports sku-frequency --on sold --since 90d --top 50` | Joins estimate_items with estimates filtered by status; API only returns items per single estimate | 7/10 | Pierce, Sam | SQLite JOIN estimate_items WITH estimates WHERE status='Sold' AND soldOn > since GROUP BY sku.id ORDER BY COUNT DESC |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| dismissed-reasons (NLP version) | Requires LLM clustering on free-text reasons; reframed as mechanical exact-match group-by | dismissed-reasons (reframed) |
| commission-reconcile | Requires external CSV format the brief doesn't define; ask Pierce later before building | my-sold (also killed) |
| bulk-dismiss | Thin loop over absorbed `dismiss`; xargs/agent loop handles it natively | (none — use absorbed `dismiss`) |
| estimate-diff | Misleading — we store status_changes but not historical line-item snapshots; diff would lie | estimate-audit |
| estimate-export-replay | Duplicates `sync --from <iso>` with a renamed flag; absorbed | sync (absorbed) |
| my-sold | Thin wrapper over `get-list --sold-by-id N --since 7d`; Tom can run absorbed list | rep-leaderboard |
| unsold-watcher | Subsumed by `recent-changes` with a `--to-status Unsold` filter | recent-changes |
