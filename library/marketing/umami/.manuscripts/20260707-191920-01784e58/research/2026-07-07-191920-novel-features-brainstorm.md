# Novel-features brainstorm — umami-pp-cli (audit trail)

Full subagent response (customer model → 13 candidates → adversarial cut → 6 survivors).

## Customer model

### David — agency dev running 10 client sites (self-hosted Umami v3)

**Today (without this CLI):** David self-hosts Umami v3 at umami.davidbarbier.com with 10 client sites (restodom.fr, cabinet-echinard.fr, aurarios.fr, dindonstudio.fr, …) across 5+ teams. To answer "how are all my sites doing?" he opens the Umami UI once per site — there is no rollup view (community issues #3362, #3174, #3193). For client SEO reports (his current branch is literally `feat/seo-reporting`) he manually reads top referrers and channel splits per site and eyeballs Google's share (restodom.fr: 2161/2811 visits from google.com). He cannot answer "which client site changed abnormally this week?" without visiting all ten.

**Weekly ritual:** Monday portfolio sweep across all 10 sites, then per-client acquisition reporting: organic vs direct vs social split, top entry pages, Google share, week-over-week movement — pasted into client emails.

**Frustration:** Assembling the cross-site view and the per-client SEO narrative by hand; and discovering *weeks late* that a client's tracker broke or traffic quietly collapsed, because nothing watches the portfolio for him.

### Inès — indie hacker instrumenting a product site

**Today (without this CLI):** Runs custom events and funnel/goal/journey reports through the Umami UI or curl scripts where she computes epoch-ms timestamps by hand. Her scripts target v2 wrappers that break against v3 (`url`→`path` renames).

**Weekly ritual:** Weekly conversion check — event series, funnel drop-off, goal counts — compared against last week to see if a product change moved the needle.

**Frustration:** Comparing this week's numbers to prior periods requires re-running everything twice and diffing by hand; mid-month she can never tell whether the month is *on pace* to beat last month.

### Marc — privacy-conscious team ops (backup + scheduled reporting)

**Today (without this CLI):** Maintains the self-hosted instance for a team. CSV export is missing from the self-hosted UI (issues #946, #2456, #3208), so backups mean hand-rolled pagination scripts. For alerts and digests he glues together Thunderbottom/umami-alerts or n8n template #2520.

**Weekly ritual:** Scheduled weekly digest to Slack/email; periodic data export for retention/compliance.

**Frustration:** Alerting on traffic anomalies requires building and hosting a separate tool; there is no baseline-aware "tell me only when something is off."

## Candidates (pre-cut)

1. **Portfolio anomaly watch** — `watch` — trailing 28-day per-weekday local baseline scan across all sites; print only deviating sites (poll-once, cron-friendly). Persona: David, Marc. Source: (a)+(c). **Keep.**
2. **SEO acquisition report** — `seo` — per site: organic-search share (incl. Google share), channel mix with WoW delta, top organic entry pages, top referrers. Persona: David. Source: (e)+(b). **Keep.**
3. **Tracker health / silent-tracker detection** — `coverage` — flag sites with zero pageviews / snapshot gaps, confirmed live. Persona: David, Marc. Source: (a). **Keep.**
4. **Movers report** — `movers` — biggest page/referrer risers & fallers this period vs previous from local metrics rows. Persona: David, Inès. Source: (c). **Keep.**
5. **New-referrer discovery** — `new-referrers` — referrer domains first seen this window vs full local history (backlink discovery). Persona: David. Source: (c). **Keep.**
6. **Month pacing** — `pace` — MTD live projection vs prior full month from local snapshots. Persona: David, Inès. Source: (c). **Keep (borderline).**
7. **Page decay detector** — `decay` — pages declining N consecutive weeks. **Soft-kill flag:** overlap with movers; monthly cadence.
8. **UTM hygiene audit** — `utm-audit` — case/spelling variants of utm values. **Soft-kill flag:** monthly chore; campaigns owns weekly UTM.
9. **Referral-spam detector** — `spam` — heuristic spam-referrer flagging. **Verifiability flag:** no ground truth in dogfood.
10. **Two-site comparison** — `compare-sites`. **Soft-kill flag:** occasional; portfolio covers it.
11. **Funnel run history + diff** — `funnel-diff`. **Feasibility flag:** needs new report-history table; per-experiment cadence.
12. **ASCII sparkline chart** — `chart`. **Soft-kill flag:** visualization without decision.
13. **Traffic records** — `records` — all-time best day/week. **Soft-kill flag:** trivia.

## Survivors and kills

Pass 3: all six survivors pass weekly-use, wrapper-vs-leverage, and transcendence-proof checks; all `hand-code`; Long Descriptions validated against surviving/shipped command names.

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|-------|--------------|--------------|----------|------------------|
| 48 | Portfolio anomaly watch | `watch` | 10/10 | hand-code | Compares each site's latest daily stat snapshots against a trailing 28-day per-weekday baseline in local SQLite; prints only deviating sites (poll-once, cron-friendly). Sibling killed: `chart`. | Brief Data Layer names "traffic-drop detection against local baselines"; Thunderbottom/umami-alerts exists as a standalone tool = explicit demand. | Use this command for daily/weekly anomaly detection across synced local history for all sites. Do NOT use this command for last-30-minutes realtime spike checks; use 'pulse' instead. |
| 49 | SEO acquisition report | `seo` | 9/10 | hand-code | Joins local metrics rows (type=channel, referrer, entry-path) across current vs prior window to compute organic/Google share, channel deltas, and top organic entry pages per site. Sibling killed: `utm-audit`. | User Vision: branch `feat/seo-reporting`; restodom.fr 2161/2811 Google visits. | Use this command for a per-site organic/referral acquisition report (channel mix, Google share, entry pages). Do NOT use it for UTM campaign performance; use 'campaigns' instead. Do NOT use it for a full-site traffic summary; use 'overview' or 'digest' instead. |
| 50 | Tracker health check | `coverage` | 8/10 | hand-code | Scans local daily snapshots for zero-traffic gaps per site, then confirms via live `websites active`/`daterange` calls; reports silent trackers. Sibling killed: `spam`. | Agency profile with 10 sites and no rollup UI — a dead tracker goes unnoticed. | Use this command to detect client sites whose tracking script stopped sending data. Do NOT use it to check CLI auth or API connectivity; use 'doctor' instead. |
| 51 | Movers report | `movers` | 8/10 | hand-code | Self-joins local metrics rows for two adjacent windows per site to rank page- and referrer-level risers/fallers with absolute and % deltas. Sibling killed: `decay`. | Weekly client reports need "what changed"; `trends` is site-level only. | Use this command for page/referrer-level week-over-week risers and fallers on one site. Do NOT use it for site-level growth acceleration across periods; use 'trends' instead. |
| 52 | New-referrer discovery | `new-referrers` | 7/10 | hand-code | Anti-joins current-window referrer metrics against the site's full local referrer history to list first-seen referrer domains. Sibling killed: `compare-sites`. | SEO workflow; impossible via API alone — requires local history no wrapper retains. | Use this command to list referrer domains first seen in the current window (new backlinks/mentions). Do NOT use it for overall referrer rankings; use 'seo' or the generated 'websites metrics' with type=referrer instead. |
| 53 | Month pacing | `pace` | 6/10 | hand-code | Fetches month-to-date stats live, projects to month-end using elapsed-day ratio, compares against the prior full month summed from local snapshots. Sibling killed: `funnel-diff`. | Scheduled reporting cadence implies "are we on track" mid-cycle. | Use this command for a month-to-date projection against the prior full month. Do NOT use it for comparing two completed periods with growth %; use the generated 'websites stats' with --compare instead. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| Page decay detector (`decay`) | Monthly content-audit cadence; a decay row is a movers faller filtered over more weeks. | `movers` |
| UTM hygiene audit (`utm-audit`) | Hygiene is monthly at best; weekly UTM owned by shipped `campaigns`. | `seo` |
| Referral-spam detector (`spam`) | Fails verifiability — no ground truth for a "spam" label in dogfood. | `coverage` |
| Two-site comparison (`compare-sites`) | Occasional curiosity; shipped `portfolio` renders all sites side by side. | `new-referrers` |
| Funnel run history + diff (`funnel-diff`) | Needs a new report-history table; per-experiment cadence. | `pace` |
| ASCII sparkline (`chart`) | Visualization without a decision. | `watch` |
| Traffic records (`records`) | All-time-high trivia, occasional use. | `watch` |
