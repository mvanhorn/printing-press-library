# Sumble CLI — Novel Features Brainstorm (subagent audit trail)

## Customer model

**Dana — Outbound SDR at a 40-person devtools startup (Pro tier, ~9,900 cr/mo).** Today: builds prospect lists in the Sumble web app, exports CSV, hand-pastes into the CRM; no idea how many credits a "find orgs using Kafka" run burns until it's already spent. Weekly ritual: Monday ICP technographic pull (`organizations/find` by technologies), triage, save to org-list, then `people/find` the buying committee. Frustration: blows the monthly budget mid-month re-running the same searches and re-enriching orgs from last week — web app has no dedup, no cost preview.

**Raj — RevOps / growth engineer scripting prospecting (Pro tier, shared key).** Today: throwaway Python against bare v6, hand-rolls pagination, re-queries entities already fetched; can't see a running balance (no REST balance endpoint) so discovers he's out of credits only when a call 422s. Weekly ritual: CSV/CRM reconciliation via `organizations/match`, then enrich matched. Frustration: no budget guardrail; a bad loop drains the org's monthly credits in an afternoon.

**Mia — Founder doing account-based sourcing herself (Free tier, 500 cr/mo, brutally cost-sensitive).** Today: manually clicks orgs, copies emails; terrified of `people/enrich` (phone = 80cr each, she has 500/mo). Weekly ritual: resolve one account, find org chart, reveal a few emails. Frustration: one accidental phone-enrich on 6 people = 480cr = whole month gone, no confirmation gate.

**An AI agent operating the CLI (the explicit vision persona).** Needs machine-readable cost estimates, a queryable running balance, and a hard budget ceiling to act autonomously without bankrupting the human's account.

## Survivors (transcendence features)

| # | Feature | Command | Score | Buildability | How It Works |
|---|---------|---------|-------|--------------|--------------|
| 1 | Pre-call cost estimate (dry-run) | `cost-estimate <cmd> [args]` | 9/10 | hand-code | Multiplies static per-endpoint credit table by requested row/limit; prints credits-that-would-be-spent without dialing. Zeros out rows already in local cache. |
| 2 | Running balance from ledger | `balance` | 8/10 | hand-code | Reads most-recent `credits_remaining` persisted in local `credit_ledger` — the only way to see balance since REST has no endpoint. |
| 3 | Budget ceiling guard | `budget set <n>` + `--budget` | 8/10 | hand-code | Compares each billed call's estimate against ledger remaining/ceiling; aborts nonzero before dialing when it would exceed. |
| 4 | Spend report | `spend [--since] [--by endpoint]` | 7/10 | hand-code | Aggregates `credit_ledger` rows in SQLite into total / per-endpoint / per-day spend. |
| 5 | Stale-cache report | `stale [--older-than 24h]` | 7/10 | hand-code | Queries cached-entity timestamps against Sumble's ~24h freshness lag to flag what's worth re-billing. |
| 6 | Tech-stack diff | `stack-diff <orgA> <orgB>` | 7/10 | hand-code | Cross-joins two cached `organizations/enrich` rows to list shared vs unique technologies — zero credits. |
| 7 | Free-first match→enrich reconcile | `reconcile <csv>` | 6/10 | hand-code | Runs cheap `organizations/match` (1cr matched, unmatched free) over a CSV, caches resolved IDs, reports which still need a billed enrich. |

### Killed candidates
| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| `cache-check` dedup preflight | Folds into `cost-estimate` (checks cache, zeroes cached rows) + `stale` | cost-estimate |
| `intent <tech>` ranker | Thin wrapper over `jobs find --order-by jobs_count_growth_6mo` | stale |
| `list-read --estimate` | Subset of cost-estimate applied to list reads | cost-estimate |
| `org-chart` assembler | Needs pre-synced relation graph; low verifiability; overlaps people find-related-people | stack-diff |
| `costs` cheatsheet | Static table too thin to stand alone; numbers surface in cost-estimate | cost-estimate |
| `--confirm-over` gate | Duplicates budget guard's estimate-vs-ceiling check | budget |
| `intent --watch` | Scope creep — background polling | stale |
| `enrich-emails` saver | Effectively `people enrich` with phone suppressed; gating belongs in budget | budget |
| `plan <goal>` planner | LLM dependency, unverifiable | cost-estimate |
