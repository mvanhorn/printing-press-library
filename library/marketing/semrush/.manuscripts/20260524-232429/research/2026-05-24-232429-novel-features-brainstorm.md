# Novel features brainstorm — semrush

Full audit trail from the Phase 1.5c.5 subagent. Customer model and killed
candidates are NOT in the absorb manifest but ARE retained here for
retro / dogfood debugging per the subagent contract.

---

## Customer model

**Persona 1: Maya, in-house SEO lead at a Series B SaaS company**

- **Today (without this CLI):** Maya keeps three Semrush browser tabs open continuously: Domain Overview for her product domain, Keyword Gap vs. two named competitors, and Position Tracking for her ~200 priority keywords. Every Monday she exports CSVs from each tab into a Google Sheet so she can compute week-over-week deltas by hand because the in-product graphs reset when she changes filters. She cannot answer "which keywords did we lose to which competitor this week" without VLOOKUPing two exports together, and she cannot answer "show me every keyword where KD dropped below 30 in the last 60 days" at all — Semrush keeps history but doesn't expose that as a query.
- **Weekly ritual:** Monday morning: pull Domain Overview + top-200 organic keywords + position-tracking snapshot, paste into the running Google Sheet, color-code movement, write a 3-bullet Slack update for the marketing channel.
- **Frustration:** Reconciling this week's CSV against last week's CSV by hand. Semrush has the data but won't diff it for her, and the in-product position-tracker email is a static PDF with no row-level deltas.

**Persona 2: Diego, freelance SEO consultant managing 8 client domains**

- **Today (without this CLI):** Diego juggles 8 client domains across 4 country databases. He burns API credits re-pulling the same keyword and backlink reports every time a client emails him a question ("did anything new link to us this week?"). His workflow is to open Semrush, run `domain organic` for the client domain, eyeball the top page, then run `backlink list` and scroll. He has no memory of what he pulled last week, so he often re-spends credits to answer questions he already answered. He can't tell a client "you gained 12 new referring domains this week, here they are" without manually diffing two exports.
- **Weekly ritual:** Friday afternoon "client check-in" pass — for each of 8 clients, pull latest backlinks, latest organic keyword movement, write a paragraph in a status email. Spends ~3 hours, burns ~800 credits.
- **Frustration:** Every client question is a fresh API spend because nothing he pulled yesterday is reachable today. Credit budget anxiety dominates the workflow; he avoids exploratory queries.

**Persona 3: Priya, growth analyst piping Semrush data into a BI warehouse**

- **Today (without this CLI):** Priya runs a nightly Python script that hits ~15 Semrush endpoints across 20 domains (her company + 19 competitors), shoves the CSV into BigQuery, and a Looker dashboard updates in the morning. The script is brittle — Semrush CSV column codes change occasionally, the script eats whole responses on a single bad row, and she has no idea how many credits each nightly run burns until the monthly bill arrives. She cannot answer "what did last night's run actually cost" without a custom wrapper.
- **Weekly ritual:** Monday she audits last week's nightly runs for failures, re-runs any failed domains, and updates Looker. Tuesday-Friday the script just runs.
- **Frustration:** Credit accounting is invisible until end-of-month. A loose query against a busy domain can drain hundreds of credits before she notices, and she can't preview cost without writing it herself.

**Persona 4: Sam, agent operator who orchestrates Semrush through Claude/an MCP client**

- **Today (without this CLI):** Sam uses the official Semrush MCP (3-tool code-orchestration shape) to ask agents "what's the keyword gap between brand A and brand B in DE?" The agent makes the right API calls but every conversation starts cold — there is no memory of last week's pull, so the agent re-spends credits to answer follow-ups in a new session. The mrkooblu MCP exposes 77 tools but no local store, so identical issue. Sam cannot ask the agent "compare what you found this Monday to what you find today" because Monday's data is gone.
- **Weekly ritual:** Ad-hoc agent conversations whenever a stakeholder asks an SEO question. Two or three sessions a week, each starting from zero.
- **Frustration:** Statelessness. The agent has no memory across sessions, so every query is full-cost, and trend questions ("did our backlink profile improve since last month?") are impossible without manual export-and-paste.

---

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Inline verdict |
|---|------|---------|-------------|---------|--------|----------------|
| C1 | Weekly drift report | `semrush-pp-cli drift <domain> --since 7d` | Diff current `keyword_positions` + `domains` snapshot against last sync; output gainers, losers, new entries, lost keywords with Δposition and ΔKD. Pure local SQL across two snapshots. | Maya | (a) persona-driven | KEEP — pure local, mechanical, no LLM |
| C2 | Snapshot tagging + diff | `semrush-pp-cli snapshot tag <label>` / `snapshot diff <label-a> <label-b>` | Tag the current local-store state of one or more resources with a label (e.g. `monday-baseline`), then diff any two labels later. Maya's Monday-vs-Monday workflow as one command. | Maya | (a) persona-driven | KEEP — local SQLite only |
| C3 | Backlink delta since last sync | `semrush-pp-cli backlink new --target <domain> --since 7d` | Pull `referring_domains` and `backlinks` rows where `first_seen > last_sync`. The "you gained 12 new referring domains" query. | Diego | (a) persona-driven | KEEP — `first_seen` already persisted |
| C4 | Free-first answer with explicit cost gate | `semrush-pp-cli ask <question>` or implicit on every read | Resolve the question against the local store first; only hit the API if the local store is missing data or stale beyond `--max-age`. Print "free (local)" or "would cost N units, continue?" before any API call. | Diego | (a) persona-driven | REFRAME as a flag `--local-only` on top of existing reads. Cut as a standalone command. |
| C5 | Cost ledger | `semrush-pp-cli budget` / `budget --since 30d` | Logs every API call's unit cost to a `credit_log` table; `budget` rolls up spend by day/command/resource, forecasts month-end burn, names the top-3 commands by spend. | Priya, Diego | (a)+(b) persona+content-pattern | KEEP — exploits unique-to-Semrush per-call unit cost |
| C6 | Keyword gap (real) | `semrush-pp-cli keyword gap <me> <them> [<them2>...]` | Set-difference of `keyword_positions` between two or more domains in the same database. | Maya | (a) persona-driven | KEEP — cross-entity join the API can't natively do |
| C7 | Backlink gap | `semrush-pp-cli backlink gap <me> <them> --min-ascore 70` | Referring domains that link to a competitor but not to me, filtered by authority score. | Diego, Maya | (a)+(c) persona+cross-entity | KEEP — local join, brief calls it out explicitly |
| C8 | Audit triage | `semrush-pp-cli audit triage <project-id>` | Join `audit_issues` + `audit_pages` + issue catalog to rank pages by weighted issue severity, output the top-N pages to fix first. | Maya (tech-SEO mode) | (b) content-pattern | KEEP — pure local SQL over already-synced tables |
| C9 | Position drift watch | `semrush-pp-cli tracking drift <project-id> --since 30d` | Diff `tracking_positions` snapshots over time: which tracked keywords moved >3 positions, which dropped off page 1, which entered page 1. | Maya, Sam | (a)+(c) persona+cross-entity | KEEP — local time-series query |
| C10 | Content brief from People-Also-Ask | `semrush-pp-cli content-brief <seed-keyword> --db us` | Pull `keyword questions` + `keyword related` + top-10 organic SERP, structure as a brief with question clusters. | Maya | (b) content-pattern | KEEP, but verifiability is medium — flag |
| C11 | One-shot domain card | `semrush-pp-cli domain card <domain>` | Print a Domain Overview + top-10 organic keywords + top-5 competitors + backlink summary + recent ad copies in one structured envelope. | Diego | (a) persona-driven | REFRAME — opinionated multi-call wrapper. KILL in Pass 3 unless it earns local-store leverage |
| C12 | Where-clause sugar | `--where 'kd<30 and nq>100'` on every list endpoint | Translate Go-style predicates to Semrush's `+|Field|Op|Value` `display_filter` URL syntax. | Priya, Sam | (b) content-pattern | KILL — already covered by absorb manifest |
| C13 | Country fan-out | `semrush-pp-cli domain overview <d> --databases us,uk,de,fr,it` | Fan out one query across N country databases, persist all rows. | Maya, Priya | (b)+(a) | KEEP — multi-region join, persist for later drift |
| C14 | SERP feature monitor | `semrush-pp-cli serp-features <keyword> --since 30d` | Track which SERP features (featured snippet, PAA, video, image pack) appear/disappear for a tracked keyword over time. | Maya | (b) content-pattern | KEEP — exploits already-persisted column nobody else queries |
| C15 | Keyword surge | `semrush-pp-cli keyword surge --window 7d --min-delta 50%` | Local query for keywords whose `nq` (search volume) jumped >X% in latest snapshot vs prior. | Sam, Priya | (c) cross-entity | REFRAME — folded into broader `--since` query pattern |
| C16 | Cannibalization detector | `semrush-pp-cli cannibalization <domain>` | Find phrases in `keyword_positions` where the same domain ranks multiple URLs for the same query. | Maya | (b)+(c) | KEEP — service-specific SEO content pattern, local-only |
| C17 | MCP-friendly intent router | `semrush-pp-cli intent "<natural language>"` | Map "what does <domain> rank for in Germany under KD 30" to the right command + flags. | Sam | (a) persona-driven | KILL — LLM dependency; user can pipe to claude |
| C18 | Audit regression watch | `semrush-pp-cli audit regression <project-id>` | Diff latest `audit_snapshots` vs prior: new issues introduced, issues resolved, severity shift. | Maya | (a)+(c) | KEEP — pure local diff over already-persisted snapshots |

Pre-cut count: 18 candidates. Killed inline: C12 (already absorbed), C17 (LLM dependency).

---

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Weekly drift report | `semrush-pp-cli drift <domain> --since 7d` | 9/10 | hand-code | Local SQL over two `keyword_positions` + `domains` snapshots; emits gainers/losers/new/lost with Δposition and ΔKD | Brief Top Workflow #3 ("set-difference logic the API can't return natively"); Maya's Monday CSV-diff ritual; both competitors lack persistence |
| 2 | Snapshot tag + diff | `semrush-pp-cli snapshot tag <label>` / `snapshot diff <a> <b>` | 8/10 | hand-code | Writes a `snapshot_labels(label, resource_type, query_signature, taken_at)` row pointing at current store state; `diff` joins two labels' rows for the named resource | Maya's weekly-baseline workflow; brief Build Priority 3 explicitly names `snapshot` as a transcendence command |
| 3 | Backlink delta since last sync | `semrush-pp-cli backlink new <domain> --since 7d` | 8/10 | hand-code | Selects from `referring_domains` and `backlinks` where `first_seen > now() - 7d` | Diego's "did anything new link to us" weekly question; brief Top Workflow #4 |
| 4 | Cost ledger / budget | `semrush-pp-cli budget [--since 30d]` | 9/10 | hand-code | Logs every API call's documented unit cost to `credit_log(ts, command, resource, units, balance_after)`; `budget` rolls up by day/command and projects month-end burn | Brief Reachability Risk; osodevops `--dry-run`; Diego's credit anxiety and Priya's invisible-spend frustration |
| 5 | Keyword gap | `semrush-pp-cli keyword gap <me> <them> [<them2>...]` | 9/10 | hand-code | Set-difference query over `keyword_positions` rows from two-or-more domains in the same `database` column | Brief Top Workflow #3 explicitly; Maya's #1 weekly question |
| 6 | Backlink gap | `semrush-pp-cli backlink gap <me> <them> --min-ascore 70` | 8/10 | hand-code | Left-anti-join over `referring_domains` between two `target_domain` values, filtered by `domain_ascore` | Brief Top Workflow #4 explicitly; Diego's client workflow |
| 7 | Audit triage | `semrush-pp-cli audit triage <project-id>` | 7/10 | hand-code | Joins `audit_issues` × `audit_pages` × generated issue catalog, weights `errors*3 + warnings*1 + notices*0.1`, orders pages by weighted score | Brief Top Workflow #5; absorb rows 67/70/71/73 persist data but nobody aggregates it |
| 8 | Position drift | `semrush-pp-cli tracking drift <project-id> --since 30d` | 7/10 | hand-code | Window function over `tracking_positions` grouped by `(phrase, region, device)`, latest snapshot vs prior, emits movers crossing page-1 / top-3 thresholds | Brief Top Workflow #5; Maya's tracker-email frustration |
| 9 | Country fan-out | `semrush-pp-cli domain overview <d> --databases us,uk,de,fr,it` | 6/10 | hand-code | Sequential fan-out across N `database` values into the existing `domain overview` flow, all rows persisted with `(domain, database, snapshot_date)` key for later cross-region drift | Brief Data Layer; user's existing `dataforseo`/`serpapi-google-local` use `--source` fan-out pattern |
| 10 | SERP feature monitor | `semrush-pp-cli serp-features <keyword> --since 30d` | 6/10 | hand-code | Local time-series query over the SERP-feature flag columns persisted by `keyword organic-serp`; reports first-seen/last-seen for each feature per keyword | Absorb row 41 persists per-position SERP-feature flags but no command surfaces them as a time series |
| 11 | Cannibalization detector | `semrush-pp-cli cannibalization <domain>` | 7/10 | hand-code | `SELECT phrase, COUNT(DISTINCT ur) FROM keyword_positions WHERE domain=? GROUP BY phrase HAVING COUNT(DISTINCT ur) > 1` | Service-specific SEO content pattern; brief Top Workflow #1 |
| 12 | Audit regression watch | `semrush-pp-cli audit regression <project-id>` | 7/10 | hand-code | Diff latest two `audit_snapshots` rows for a project: new issue_ids, resolved issue_ids, count delta per severity | Brief Top Workflow #5; Maya's tech-SEO mode |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C4 Free-first ask | Better expressed as a `--local-only` flag on existing reads + the doctor's stale-hint helpers; standalone command doesn't earn its keep | Survivor #4 budget |
| C10 Content brief | Verifiability is medium and the structure overlaps with simply running `keyword questions` + `keyword related` directly; agents can compose | Survivors #5/#6 keyword-gap/backlink-gap |
| C11 Domain card | Thin multi-endpoint wrapper with no local-store leverage; agents can compose the same payload with one MCP call per endpoint | Survivor #1 drift |
| C12 Where-clause sugar | Already covered by absorb manifest rows 9 and 37 as a `--where` flag on list endpoints | n/a — already shipped |
| C15 Keyword surge | Folded into the broader local-time-predicate pattern (`--since` flag on list reads); not enough standalone value once drift + serp-features + backlink-new exist | Survivors #1/#3/#10 |
| C17 Intent router | LLM dependency; MCP server already emitted lets agents do this themselves | Auto-emitted MCP server (absorb row 94) |

Note: I (the orchestrator) added C10 "Content brief" to the killed list when synthesizing for the manifest. The subagent had it as "KEEP but verifiability is medium — flag." On reflection, content-brief reframing as a derived report would be a Phase 6 polish concern, not a transcendence row. Twelve survivors is already at the high end of the 4-8 rubric target.
