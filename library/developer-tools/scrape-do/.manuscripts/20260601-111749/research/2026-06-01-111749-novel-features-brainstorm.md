# Scrape.do CLI — Novel Features Brainstorm (audit trail)

Subagent output from Phase 1.5 Step 1.5c.5. First print (no prior research).

## Customer model

**Maya — the SERP rank-drift operator running an agent swarm.**
Today: runs a content-SEO shop, 6–8 Claude/GPT agents each scraping Google SERPs via raw curl against `api.scrape.do/plugin/google/search`. Brittle Sheets+cron diffing. Cannot answer "did rank-1 for `best crm software` change since Tuesday?" without re-spending credits and hand-diffing JSON.
Weekly ritual: re-pull ~200 tracked queries, look for movers worth flagging to clients.
Frustration: every drift check costs a fresh 10-credit call even when nothing changed; agents step on each other (double-spend, 429s).

**Devraj — the multi-agent orchestrator terrified of the concurrency cap.**
Today: a dozen autonomous agents fan out scrape jobs against one account; no shared view of in-flight requests; hard-codes sleeps and prays he stays under `ConcurrentRequest`. Polling `/info` per-agent trips the 10/min `/info` cap.
Weekly ritual: launch batches across agents, babysit 429 storms, restart stalled ones.
Frustration: no local shared concurrency token — each agent is blind to the others; a single misbehaving agent can blow the monthly budget for everyone.

**Priya — the cost-accountable lead.**
Today: reconciles spend from the dashboard's coarse monthly number; cannot attribute spend to an agent/client/query-family, cannot tell render/super/google mix apart; finds out about the cap by getting 401s.
Weekly ritual: pull remaining credits, sanity-check burn vs days-left, decide whether to throttle chatty agents.
Frustration: the authoritative per-call cost (`Scrape.do-Request-Cost` header) is discarded — no local ledger to attribute or forecast spend.

## Candidates (pre-cut)
(8-16 generated; see Survivors/Killed below for the cut)
1. Concurrency-gated request lease (e) — KEEP
2. Pre-flight cost estimator (e) — KEEP
3. Local credit ledger + spend attribution (c,e) — KEEP
4. SERP rank-drift diff (a,b,c) — KEEP
5. Multi-agent safe fan-out (d→e,a) — KEEP
6. Spend-ceiling guard (e) — KEEP
7. Concurrency-headroom watch (b) — KILL (thin; live numbers already in usage/tail)
8. Result cache / dedupe (c) — KEEP
9. Offline SERP search (c) — KILL (absorbed framework search)
10. Cross-query movers digest (a,b,c) — KEEP
11. SERP share-of-voice by domain (b,c) — KILL (absorbed analytics --group-by)
12. AI-Overview/PAA citation tracker (b,c) — KILL (verifiability: intermittent blocks)
13. Per-agent rate pacer (e) — KILL/MERGE into lease
14. Cost analytics over time (c) — KILL (absorbed analytics; attribution lives in budget)
15. Screenshot/markdown archive browser (b) — KILL (thin file reader)
16. Domain-override cost warner (e) — MERGE into cost estimator

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Concurrency-gated request lease | shared lease on `scrape`/`google search`/`batch` (`--max-concurrency` auto from `/info`) | 9/10 | hand-code | SQLite-backed shared counter capped at cached `ConcurrentRequest`; acquired before any billed call, released on completion, so N agents on one account never exceed the plan cap | Brief HEADLINE REQUIREMENT + Top Workflow 3; concurrency tiers + 429-on-exceed |
| 2 | Pre-flight cost estimator | `cost` | 8/10 | hand-code | Maps requested mode against the credit table (1/5/10/25/google=10 + overrides like LinkedIn 30) to print expected credits with zero API spend | Credit Cost Model table; Top Workflow 3 |
| 3 | Local credit ledger + spend attribution | `budget` | 8/10 | hand-code | Joins `credit_ledger` (debited from `Scrape.do-Request-Cost`) with cached `usage_snapshots` to attribute spend by agent/query-family/mode and forecast burn vs days-left | Data Layer (`credit_ledger`) + Build Priority 3 |
| 4 | SERP rank-drift diff (single query) | `drift` | 9/10 | hand-code | Diffs the two most recent `serp_snapshots` for one query+params-hash; position deltas (new/dropped/moved) entirely offline, no re-spend | Top Workflow 4; `serp_snapshots` keyed by (query, params-hash) |
| 5 | Multi-agent safe fan-out | `batch` | 8/10 | hand-code | Reads a URL/query list, dispatches through the shared concurrency lease + per-call ledger debit, auto-retries only the non-billed 429/502/510 classes | Top Workflow 5; Billing rule (those codes free) |
| 6 | Spend-ceiling guard | `--max-credits`/`--max-monthly-pct` on dispatching commands | 7/10 | hand-code | Estimates request cost (feat #2) and refuses dispatch with non-zero exit if ledger+estimate would breach the ceiling | "burn monthly credits" risk + Priya; Build Priority 3 |
| 7 | Cross-query movers digest | `movers` | 7/10 | hand-code | Scans every tracked query's latest-vs-prior `serp_snapshots`, lists only queries whose top positions moved past a threshold | Top Workflow 1+4; Maya's weekly client report |
| 8 | Result cache / dedupe | cache-first default + `--fresh` bypass on `scrape`/`google search` | 6/10 | hand-code | Hashes request params, returns a recent matching `scrape_jobs`/`serp_snapshots` body instead of re-spending unless `--fresh` | `scrape_jobs` result-cache; Maya's double-spend |

### Killed candidates

| Feature | Kill reason | Closest-surviving-sibling |
|---------|-------------|---------------------------|
| Concurrency-headroom watch | Standalone value thin; live numbers already in usage/tail; safety comes from the lease | #1 lease |
| Offline SERP search | Covered by framework `search` (FTS5); not novel | #4 drift |
| SERP share-of-voice by domain | Delivered by framework `analytics --group-by domain` | #7 movers |
| AI-Overview/PAA citation tracker | Verifiability fails: intermittent blocks, can't reliably dogfood | #4 drift |
| Per-agent rate pacer | Sibling of the lease; a separate command is scope-thin | #1 lease |
| Cost analytics over time | Covered by framework `analytics --type scrape_jobs`; attribution lives in budget | #3 budget |
| Screenshot/markdown archive browser | Thin file reader, low weekly value | #8 cache |
| Domain-override cost warner | One branch of the cost estimator's table, not standalone | #2 cost |
