# Novel-Features Brainstorm: BotSee CLI

## Customer model

**Persona 1: Maya, Head of Growth at a Series B SaaS startup (~80 people)**

*Today (without this CLI):* Maya logs into BotSee's dashboard every Monday at 8am, clicks through her three product sites, manually copies competitor mention counts into a Google Sheet, then pastes the top three competitors into a Slack post for the exec channel. To get week-over-week deltas she keeps last week's screenshot in a Notion page and eyeballs the difference. She cannot answer "did our visibility for 'best AI sales tool' drop on Perplexity specifically this week?" without scrolling through five analysis pages.

*Weekly ritual:* The 60-90 min Monday visibility review. Pull latest analyses across 3 sites → scan competitor share-of-voice → write the exec Slack post → file three tickets for content gaps.

*Frustration:* The dashboard shows current state beautifully but has no programmatic delta. She re-derives "what changed since last week" by hand every single Monday. The exec scorecard is 40 minutes of copy-paste that should be one command.

**Persona 2: Ravi, Founder/solo-dev at an AI tooling startup**

*Today (without this CLI):* Ravi runs Claude Code in his terminal all day. He has a BotSee account because his investor told him AI visibility matters. He kicks off an analysis with `curl`, waits, then runs three more `curl` calls to pull keywords, competitors, and sources. He never checks BotSee twice in the same week because each run costs ~$6.60 and he has to re-query results to remember what last week said. He cannot answer "what should I write a blog post about THIS week?" without burning credits on a fresh analysis just to re-read old results.

*Weekly ritual:* Friday afternoon: run one new analysis on the product site → pipe results into Claude Code → ask Claude to suggest one blog topic. Read-heavy aggregation between analyses is impossible without re-paying.

*Frustration:* Every read of historical analysis data re-hits the API (and the dashboard, which he refuses to open). He wants an offline cache his agents can query for free. He also wants pre-flight cost estimates before he runs a multi-persona analysis, because the 2x post-completion multiplier has bitten him twice.

**Persona 3: Dana, Senior Strategist at a 12-person GEO/SEO agency**

*Today (without this CLI):* Dana manages 14 client sites in BotSee. Every other Friday she builds a per-client deliverable: a PDF showing competitor share-of-voice, top citation gaps, recommended blog topics. She has a Python notebook that hits the BotSee API per client, paginates competitors, computes a citation-completeness score she invented, and writes to a per-client folder. The notebook breaks every time BotSee adds a field. She cannot answer "across all 14 clients, which domains are getting cited most often by ChatGPT this month?" without writing a one-off script.

*Weekly ritual:* Bi-weekly client deliverable run — 14 sites × 6 result types = 84 API calls minimum, then aggregate in pandas, then format. ~4 hours every other Friday.

*Frustration:* The cross-client rollup (which sources are winning across her portfolio) is the most valuable insight she could offer agency clients, but it requires writing custom Python every cycle because BotSee's API is per-site. She also has no way to alert clients when a competitor's mentions spike — she finds out 2 weeks late.

**Persona 4: Sam, Platform engineer at a 200-person B2B SaaS, owns the "AI search ops" pipeline**

*Today (without this CLI):* Sam wired BotSee webhooks into an internal Slack bot six months ago. The bot fires on `analysis.completed` but the payload is minimal — to know whether the new analysis showed a competitive regression, the bot has to call back into the BotSee API. Sam's webhook handler is a tangle of "fetch analysis → fetch competitors → diff against last analysis → format Slack message." When BotSee added the 2x post-completion cost multiplier, Sam's monthly bill jumped 40% before he noticed.

*Weekly ritual:* Monitor the cron job that runs analyses across 6 sites every Wednesday night. Glance at the Slack channel Thursday morning. Investigate any visibility regression alerts.

*Frustration:* The webhook payload is too thin to make decisions from; the local follow-up calls are expensive and brittle. No local source-of-truth means every alert handler re-derives "what changed." He wants a `webhook listen --local` proxy that catches the event, syncs the changed analysis into SQLite, and exits — so downstream handlers query free local data, not the paid API.

## Candidates (pre-cut)

[See Survivors and kills below — full pre-cut list embedded in killed table]

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Weekly visibility delta | `delta --site <uuid> --since 7d` | 9/10 | hand-code | Joins the two most-recent rows of local `analyses` per site and diffs `analysis_competitors` mention/share counts in SQLite, no API call | Brief Top Workflow #3 (drift detection) + Maya's stated frustration ("re-derive what changed by hand every Monday"); category gap — no dashboard exposes this programmatically |
| 2 | Executive scorecard | `scorecard --sites all` | 8/10 | hand-code | Aggregates `analyses` + `analysis_competitors` + `analysis_sources` across all sites in SQLite, formats markdown rollup with WoW deltas | Brief Top Workflow #5 (5-minute executive scorecard ritual) + Maya persona; competitor research confirms uniformly dashboard-first category, no CLI offers this |
| 3 | Pre-flight cost estimator | `analysis estimate --site <uuid> --personas <ids> --models <csv>` | 9/10 | hand-code | Multiplies cached `/pricing` per-model credits × locally-synced question count × 2x post-completion factor, returns credits + USD before any spend | Codebase Intelligence section explicitly flags this as high-leverage ("the 2x cost multiplier means cost predictions matter — an offline cost estimator before running analysis is high-leverage") |
| 4 | Webhook local-tunnel sync | `webhook listen --local --port 7777` | 7/10 | hand-code | Runs a foreground HTTP listener; on each `analysis.*` event POST, calls the absorbed `analysis get` + result-fetch commands to upsert into local SQLite, then exits 0 | Codebase Intelligence section names this verbatim ("a webhook listen --local proxy would close the loop for ops teams") + Sam persona's thin-payload frustration |
| 5 | Opportunity rollup | `opportunities top --site <uuid> --limit 10` | 8/10 | hand-code | UNION ALL of local `analysis_keyword_opportunities` + `analysis_source_opportunities` across the site's last N analyses, ranked by `frequency * competitor_coverage` | Brief Top Workflow #4 (opportunity mining) + brief Build Priorities ("opportunity heatmap") + Ravi/Dana persona overlap |
| 6 | Portfolio source authority | `sources portfolio --group-by domain` | 8/10 | hand-code | Cross-site SQLite aggregation of `analysis_sources` grouped by domain, counting distinct_sites_citing + citation_count + first_seen | Dana persona's #1 frustration (agency cross-client rollup); BotSee API is strictly per-site so this is impossible without local cache |
| 7 | Model-divergence detector | `analysis divergence <analysis_uuid>` | 7/10 | hand-code | GROUP BY on `analysis_competitors.model_coverage` / `analysis_keywords.model_coverage` to surface single-model-only items per analysis | Brief Table Stakes ("Parallel multi-LLM") makes divergence the defining service trait; brief Top Workflow #1 mentions scanning across engines |
| 8 | Cost-bottleneck report | `usage bottleneck --since 30d` | 6/10 | hand-code | GROUP BY on locally-cached `/usage` rows: model + site + persona → credits spent, ranked descending | Codebase Intelligence (per-model pricing varies 2→8 credits) + Sam persona ("monthly bill jumped 40% before he noticed") |
| 9 | Query library audit | `questions audit --site <uuid>` | 7/10 | hand-code | LEFT JOIN `questions` to most-recent `analysis_responses` per question, flags: never-analyzed, stale > N days, zero-brand-mention | Brief Top Workflow #6 (query library management is an explicit BotSee blog ritual) + Dana/Maya persona overlap |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Intent-cluster heatmap (`keywords heatmap`) | Weekly use is shaky — intent clusters change slowly; signal overlaps `opportunities top` which is the actionable version | Opportunity rollup (#5) |
| Citation completeness score | Useful but a single derived column; collapses cleanly into `scorecard` as one line rather than a standalone command — fails wrapper-vs-leverage when isolated | Executive scorecard (#2) |
| Competitor reaction tracker (`competitors track`) | Adversarial cut: the time-series-for-one-name view is `delta` filtered to a single competitor; doesn't earn its own row | Weekly visibility delta (#1) |
| Show HN-style "what's new" feed | Reverse-chrono of recent analyses duplicates scorecard's deltas section and adds no decision the user wouldn't already get from `scorecard` | Executive scorecard (#2) |
| Content gap blog draft | Already absorbed (manifest row 33, `analysis content --output`); re-proposing is reimplementation | Absorbed `analysis content` |
| Persona coverage matrix | Subset of query library audit grouped by persona instead of question — collapse into `questions audit` with optional `--group-by persona` rather than ship twice | Query library audit (#9) |
| Sentiment leaderboard | Thin wrapper around `analysis competitors` with an `ORDER BY sentiment` — fails Pass 3 Q2 (wrapper vs leverage) | Weekly visibility delta (#1) |
| Auto-recharge sanity check | Monthly not weekly — fails Pass 3 Q1 (weekly use); the credit angle is covered by `usage bottleneck` which IS weekly for Sam | Cost-bottleneck report (#8) |
