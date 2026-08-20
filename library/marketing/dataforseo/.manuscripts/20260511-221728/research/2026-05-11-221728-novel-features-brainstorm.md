## Customer model

**Persona A — Joey, the multi-business operator running ContentBot's SEO scoring loop**

*Today (without this CLI):* Joey runs the SEO blog engine across FTM + FSM. `seo_engine/scoring/serp.py` POSTs cleaned keywords to `/keywords_data/google_ads/search_volume/live` and pipes volume into 40/40/20 scoring. When a batch fails with task-level 40501 (one bad keyword poisoning all 1000), the top-level returns 20000 and the pipeline silently scores everything as volume=0. He has the DataForSEO dashboard, a Postgres `content_queue`, and a half-broken `_clean_for_dfs` regex open in three tabs. He cannot answer "did this week's keyword batch actually return volume, or did one comma kill 200 rows?"

*Weekly ritual:* 2-3× per week — hydrate volume for ~50-200 new keyword candidates per business, score them, push the top N into `content_queue` for Haiku to draft.

*Frustration:* The 40501 silent-fail trap. He's been burned at least once (the `feedback_dataforseo_keyword_cleaning.md` memory exists for a reason). Pre-cleaning is reactive; he wants a tool that refuses to send a poisoned batch.

**Persona B — The FSM/FTM rank-tracker Joe (himself, in a different hat)**

*Today:* 200+ FTM landing pages and ~228 FSM pages are live across 15 counties. To know which pages are ranking, he opens GSC Performance, filters by page, and squints at position trends one URL at a time. To know whether AI Overviews are eating his organic clicks, he opens Google in incognito and types "tree service [city]" by hand. He cannot answer "which of my 400 pages lost a SERP feature this week" without spending an afternoon.

*Weekly ritual:* Monday morning rank check across the FTM/FSM page inventory — currently a manual GSC + incognito-Google dance.

*Frustration:* No single command turns "here's my sitemap of 400 URLs" into "here's the 12 that lost ground vs last week, and here's why (AI Overview ate the SERP, competitor moved up, etc.)". Every rank-tracker SaaS ($150-400/mo) does this but charges per keyword; he already pays DataForSEO per call.

**Persona C — The AI-search visibility tracker (Joey's `ai-seo` skill, productized)**

*Today:* FSM's "Send a Photo + Schedule = 10% Off" copy lives on 228 pages. Whether ChatGPT/Claude/Gemini/Perplexity surface that offer when answering "best stump grinding company Daytona" is a black box. He occasionally asks each LLM by hand. He cannot answer "did my brand visibility in ChatGPT answers change this week."

*Weekly ritual:* Weekly LLM-answer check across a curated keyword list ("stump grinding {city}", "tree service {county}").

*Frustration:* The AI Optimization family (36 paths) is brand new, the official MCP exposes it but doesn't diff week-over-week. He needs delta-since-last-run, not absolute scores.

**Persona D — The cost-anxious FSM backlink auditor (Joey during citation-submission season)**

*Today:* During FSM directory submissions (Manta, Hotfrog, BBB, etc.), he wants to confirm the new citations are actually showing up as backlinks. Today that means logging into Ahrefs ($199/mo) or paying for a one-off Semrush export. He has DataForSEO credit on file but is terrified of accidentally calling `backlinks/bulk_backlinks/live` against 200 domains and burning $40. He cannot run audits casually because he can't predict cost.

*Weekly ritual:* Weekly check of "did the new FSM citations land as backlinks, and is anchor text on-brand."

*Frustration:* Cost opacity. Live vs Standard pricing is 3.3× different; no tool tells him "this command will cost $X" before he runs it.

## Candidates (pre-cut)

| # | Name | Command | Source | Description | Kill/keep verdict |
|---|------|---------|--------|-------------|-------------------|
| 1 | Keyword pre-cleaner with reject report | `keywords clean <file>` then auto-applied to `volume`/`ideas` | (a) Persona A, (e) ContentBot wire | Strips keywords >10 words, punctuation, special chars; prints rejects to stderr; refuses to send poisoned batches | KEEP |
| 2 | 40501 batch-fail surfacer | `--fail-on-task-error` (default on) | (b) DataForSEO-specific | Reads task-level status codes from the envelope; exits non-zero on any task 40501 even when top-level is 20000 | KEEP — fold into #1 |
| 3 | Auto-mode router (Live↔Standard) | `--auto-mode` on `volume`, `serp`, `keyword-ideas` | (b) Live vs Standard cost gap | Batch ≤5 → Live; >5 → Standard with managed poll loop | KEEP |
| 4 | Cost estimator pre-flight | `cost estimate <subcommand args>` | (b), Persona D | Computes predicted spend from static price table + planned batch size | KEEP |
| 5 | Task bundler for Standard mode | `task bundle <endpoint> --in keywords.json` | (b) tasks_ready polling tax | One command does task_post → poll → task_get → merge | KEEP |
| 6 | Local SQLite store + offline FTS | `search "<query>"` | (c) cross-entity local | Mirror result rows into SQLite; FTS over text fields | KEEP |
| 7 | Volume delta tracker | `keywords delta --since 7d` | (c) local-join, Persona A | Joins current `volume` against last stored value per keyword | KEEP |
| 8 | Rank delta tracker over URL list | `rank track --sitemap <url>` | (a) Persona B, (e) FTM/FSM 400 pages | SERP per URL's target keyword, diff vs last run | KEEP |
| 9 | SERP feature delta watcher | `serp features --diff` | (b) AI Overview etc. | Flag SERP features that appeared/disappeared | KEEP — fold into #8 |
| 10 | Brand-visibility delta in LLM answers | `ai-visibility track --brand ...` | (e) ai-seo, Persona C | AI Optimization endpoints + local diff week-over-week | KEEP |
| 11 | Backlink-since-last-run | `backlinks new --domain <domain>` | (a) Persona D, (c) local-join | Diffs current backlinks vs last snapshot | KEEP |
| 12 | On-page audit JSON export | `on-page audit <url> --json` | (a) Persona A wire | Wraps one endpoint | KILL (thin wrapper) |
| 13 | Sandbox-mode default for dry runs | `--sandbox` | absorbed | already covered | KILL (absorbed) |
| 14 | PDF audit report for client deliverables | `audit report --pdf` | (b) agency-deliverable | PDF generation for white-label | KILL (scope creep) |
| 15 | Slack/Telegram digest | `rank track --notify telegram` | external | Posts digest to Telegram | KILL (external service) |
| 16 | LLM "explain why page lost rank" | `rank explain <url>` | LLM-dependent | Summarizes SERP changes | KILL (LLM dependency) |
| 17 | Competitor SERP overlap | `serp overlap --domain ... --vs ...` | (c) cross-entity | Joins two domains' SERP corpora | KILL (soft, weekly-use) |
| 18 | Resumable task ledger | `task ls` / `task get` | (b) Standard mode | List in-flight task_ids | KEEP — fold into #5 |

## Survivors and kills

(Pass-3 force-answers as documented in subagent output.)

### Survivors

| # | Feature | Command | Score | Persona | How It Works | Evidence |
|---|---------|---------|------:|---------|--------------|----------|
| 1 | Keyword pre-cleaner with 40501 surfacer | `keywords clean <file>` + auto-middleware on `volume`/`ideas` (default-on `--fail-on-task-error`) | 10/10 | A | Local filter (>10 words, punctuation, special chars) before `/keywords_data/google_ads/search_volume/live`; parses task-level status codes from envelope and exits non-zero on any 40501 even if top-level is 20000 | Brief User Vision + `feedback_dataforseo_keyword_cleaning.md` memory; existing `_clean_for_dfs` regex in `seo_engine/scoring/serp.py` |
| 2 | Auto-mode router (Live↔Standard) | `--auto-mode` flag on batch endpoints | 10/10 | A, D | Batch size ≤5 → Live endpoint; >5 → Standard `task_post` + managed poll loop. Warns when user forces Live on a large batch. | Brief Build Priority #2; 3.3× cost differential documented in API spec; no competitor MCP routes automatically |
| 3 | Cost estimator pre-flight | `cost estimate <subcommand> <args>` + `--confirm-over $N` gate | 10/10 | D, A | Static price table (Live vs Standard per endpoint family) × planned input size from the actual args; no API call needed | Brief Build Priority #4; "top community complaint" cited in brief; no competitor offers pre-call cost gating |
| 4 | Standard-mode task bundler with local ledger | `task bundle <endpoint> --in batch.json`; `task ls`; `task get <id>` | 9/10 | A | One command runs `task_post` → polls `tasks_ready` respecting 20/min limit → fetches `task_get` → merges results. Task IDs persist to local SQLite so user can resume. | Brief Build Priority #6; tasks_ready polling tax documented in spec; absorbs #18 |
| 5 | Local SQLite store + offline FTS | `search "<query>"` scans synced keywords/SERPs/backlinks/AI-mentions | 8/10 | A, B, C, D | Every result-returning call upserts into local SQLite (`keywords`, `serp_results`, `backlinks`, `ai_mentions`) with FTS5; offline `search` runs FTS5 MATCH without re-billing | Brief Build Priority #5; rubric's canonical transcendence pattern; powers features 6-9 |
| 6 | Volume delta tracker | `keywords delta --since 7d` | 9/10 | A | Joins current `volume` API response against last stored value per keyword in local SQLite; outputs movers sorted by absolute delta | Cross-entity join only possible with local store; Joey's 2-3×/week volume-hydration ritual |
| 7 | Rank + SERP-feature delta over URL list | `rank track --sitemap <url>` (`--features` for AI Overview / featured-snippet diff) | 9/10 | B | Parses sitemap, infers target keyword per URL (or accepts `--map url,keyword`), calls `serp/google/organic/live/advanced` per keyword, diffs position + SERP features against last run, flags movers and feature gains/losses | Brief Top Workflow #2; 200+ FTM + 228 FSM pages per memory; 76 Google SERP endpoints incl. AI Overview |
| 8 | AI-visibility delta in LLM answers | `ai-visibility track --brand "<brand>" --keywords keywords.txt` | 9/10 | C | Calls AI Optimization family endpoints for each keyword, stores per-LLM mentions/excerpts in SQLite, diffs week-over-week presence + mention count per LLM | Brief Top Workflow #5; AI Optimization family (36 paths); Joey's `ai-seo` skill memory entry; no competitor diffs week-over-week |
| 9 | Backlinks-since-last-run | `backlinks new --domain <domain>` | 8/10 | D | Snapshots `backlinks/summary` + `referring_domains` + `anchors` into SQLite; on each run diffs against prior snapshot, surfaces newly-acquired referring domains with anchor text | Brief Top Workflow #3; FSM citation-submission workstream per memory; replaces $400+/mo Ahrefs |

### Killed candidates

| Feature | Kill reason | Closest-surviving sibling |
|---------|-------------|---------------------------|
| #9 SERP-feature delta watcher (standalone) | Same data path as #7 — promoted from sibling to sub-flag (`--features`) on rank delta | #7 Rank + SERP-feature delta |
| #12 On-page audit JSON export | Thin wrapper; `--json` is standard scaffolding | Covered by endpoint-mirror generator |
| #13 Sandbox-mode default | Already absorbed | Covered by `--sandbox` flag |
| #14 PDF audit report | Scope creep into agency-deliverable templating | None (out of CLI lane) |
| #15 Slack/Telegram digest | Rubric "External service" — Telegram token not in spec | #7 (pipe target via FTMAlertBot manually) |
| #16 LLM "explain rank loss" | Rubric "LLM dependency"; mechanical version is #7 piped through `claude` | #7 |
| #17 Competitor SERP overlap | Soft kill — no documented weekly ritual in memory | #7 (per-URL approach is the documented ritual) |
| #18 Resumable task ledger | Merged into #4 as `task ls` / `task get` | #4 |
