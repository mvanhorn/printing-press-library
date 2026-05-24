# Novel-Features Brainstorm: American Reindustrialization

## Customer model

**Persona A — Asha, VC associate covering industrial / hardtech deal flow at a mid-stage fund.**

- **Today (without this CLI):** Asha keeps `americanreindustrialization.com` open as a pinned tab next to Pitchbook and a Notion dashboard. Every Monday she scrolls the company list, eyeballs new entries, and copies a half-dozen fields per company (name, founded_year, employee_range, funding_stage, primary_sector, HQ state) into a sector-tracking spreadsheet. She can't easily answer "how many Series A robotics companies in the Midwest gained jobs since last Monday?" because the site shows lists, not slices, and gives her no week-over-week diff.
- **Weekly ritual:** Monday morning sweep — what's new since last week, what funding stages are filling in, where the geographic clusters are forming in her covered sectors (robotics, advanced materials, defense tech).
- **Frustration:** No diff. The site has no "new this week" view, so she has to remember what was there last time. Copying 30-field company rows into a spreadsheet takes 20 minutes and she still can't slice the data afterward.

**Persona B — Marcus, mechanical engineer pivoting from consumer hardware into industrial / manufacturing roles.**

- **Today (without this CLI):** Marcus opens the jobs tab, clicks through 25 pages of 20 jobs each, opens promising roles in new tabs. He can filter by `work_mode` and `experience_level` on the site but can't combine "remote OR hybrid + senior + salary > 150k + at companies under 200 employees + Texas or Colorado." He ends up with 40 browser tabs and forgets which ones he already triaged.
- **Weekly ritual:** Sunday evening job hunt — fresh listings from the past week, filtered down to the ~10 roles worth a real application.
- **Frustration:** The site's filter UI doesn't compose. Salary, geography, and company-size filters either don't exist or don't combine. He wants one query that returns the shortlist, not 25 pages he has to manually intersect.

**Persona C — Priya, freelance journalist writing a long feature on the American manufacturing revival.**

- **Today (without this CLI):** Priya needs concrete lists for sidebars — "robotics startups in the Midwest founded since 2020," "Texas advanced-materials companies hiring senior engineers," "the ten densest geographic clusters in the directory." She copy-pastes from the site into Google Docs, loses formatting, and has no way to export a clean CSV for her editor's fact-checker.
- **Weekly ritual:** During an active piece (every couple of weeks), she pulls 3-5 themed lists with stable criteria — founded year ranges, geographic filters, sector intersections — for inclusion as story sidebars.
- **Frustration:** No export. No way to compose "founded since 2020 + state in (TX, OH, MI, PA) + primary_sector = robotics." She wants a clean CSV she can paste into a story and a fact-checker can verify.

**Persona D — Agent driving the directory on behalf of any of A/B/C.**

- **Today (without this CLI):** Hits `/api/companies?page=N` repeatedly to paginate the entire 96-company catalog into context, pulls 30 fields per company every time. With no `--select`, every analytics question burns the same tokens fetching the same fields. No local store means every cross-entity question (companies × jobs, sectors × geography) becomes N+1 HTTP calls.
- **Weekly ritual:** On-demand — answers ad-hoc questions like "which companies in robotics are hiring senior remote engineers in TX?" by re-fetching the catalog and joining in-prompt.
- **Frustration:** Token-expensive enumeration. No SQL surface, no field pruning, no composed multi-resource filters. Every question pays the full catalog cost.

## Candidates (pre-cut)

| # | Name | Command | Description | Persona | Source | Inline verdict |
|---|------|---------|-------------|---------|--------|----------------|
| 1 | Diff since last sync | `whats-new --since <date>` | Companies and jobs added or changed since a given timestamp, computed from local store's `created_at` / `updated_at` vs prior sync snapshot | A | (a) persona frustration, (c) cross-entity local | KEEP — local SQLite, weekly Asha ritual, no API endpoint exists for this |
| 2 | Job intersect filter | `jobs find` | Compose `work_mode`, `experience_level`, `salary_min`, `state`, `company_size`, `posted_since`, `sector` on the local jobs+companies join in one shot | B | (a), (c) | KEEP — site filters don't compose, requires join on company.employee_range and company.state |
| 3 | Sector heatmap | `analytics sector-heatmap` | Sector × state crosstab of company counts, optionally weighted by jobs_count or filtered by funding_stage | A, C | (b) service-content, (c) cross-entity | KEEP — no aggregation endpoint exists; pure local SQL |
| 4 | Geographic clusters | `analytics geo-clusters --radius <km>` | Group companies by lat/lon proximity using simple grid bucketing or DBSCAN-style spatial clustering, output cluster centroid + member companies + dominant sector | A, C | (b) lat/lon is a service-specific signal, (c) | KEEP — lat/lon is in payload but unused by site; this is a verifiable mechanical computation |
| 5 | Funding-stage × sector table | `analytics funding-by-sector` | Crosstab of funding_stage × primary_sector with company counts and median employee_range | A | (b), (c) | KEEP — funding_stage signal is a service-specific field; no API surface exposes this |
| 6 | Top hiring companies | `companies top-hiring --limit N` | Rank companies by jobs_count descending; optionally filter by sector/state/funding_stage | A, B | (a), (c) | KEEP — exploits jobs_count on company payload + filters |
| 7 | Company profile sheet | `companies profile <slug>` | Single-shot rich profile: company fields + jobs at that company + similar companies (same primary_sector + employee_range) | A, B, C | (a) | KEEP — joins companies + jobs locally, "similar companies" is a local query no API offers |
| 8 | New companies feed | `companies new-since <date>` | Companies added since timestamp, ordered by founded_year or added_at | A | (a) | SOFT KILL — collapses into #1 (whats-new); kill as redundant in Pass 3 |
| 9 | New jobs feed | `jobs new-since <date>` | Jobs posted since timestamp | B | (a) | SOFT KILL — collapses into #1 |
| 10 | Founded-year cohort | `companies cohorts --by founded_year` | Bucket companies by founded_year, show counts and dominant sectors per cohort | C | (b), (c) | KEEP — founded_year is a service-specific signal Priya wants for "founded since 2020" sidebars |
| 11 | Tag co-occurrence | `tags co-occurrence --tag <slug>` | For a given tag, which other tags appear most often on the same companies | A | (b), (c) | WEAK KEEP — useful for sector exploration but Asha would not run weekly; mark for Pass 3 scrutiny |
| 12 | Category tree view | `categories tree` | Render hierarchical categories using parent_id with company counts at each node | C | (b) hierarchical categories | KEEP — parent_id is structural service content; supports Priya's exploration |
| 13 | CSV/agent-native export | `companies export --csv --select <fields>` | Export filtered company set with selected fields as CSV (or JSON) for spreadsheet/agent consumption | A, C, D | (a), (d) generator-default | KILL — `--select` and `--json` are generator-default surfaces (`cliutil` adds them to absorbed commands); proposing this as a novel feature is feature-rationalizing a default |
| 14 | Watch for changes | `watch --interval <duration>` | Long-running poll loop checking for new companies/jobs and printing diffs | A, B | (a) | KILL — scope creep (persistent background process), one-command equivalent is #1 (`whats-new`) |
| 15 | Salary distribution | `jobs salary-stats --sector <slug>` | Salary band distribution (p25/p50/p75) across jobs filtered by sector or experience_level | B | (b), (c) | KEEP — salary_min/max are service-specific; this is statistics on local data Marcus can't get from site |
| 16 | Companies-without-jobs | `companies dormant` | Companies with `jobs_count = 0` — signals strategic gaps or early-stage entries | A | (c) | KILL — niche, monthly-at-best, no persona has this on a weekly ritual; sibling #6 (top-hiring) covers the hiring-signal axis better |

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Diff since last sync | `whats-new --since <date>` | 10/10 | hand-code | Snapshots local SQLite store on each sync, diffs current vs prior snapshot for companies and jobs by `slug` + `updated_at`/`posted_at` | Brief Top Workflow #5 ("Track changes over time"); Persona A frustration ("no diff"); no API endpoint exists |
| 2 | Composed jobs filter | `jobs find --work-mode --experience --salary-min --state --company-size --sector --posted-since` | 10/10 | hand-code | Joins local `jobs` table to `companies` on company_id and applies all filters in one SQL query | Brief Top Workflow #2; brief confirms site's `state` filter on `/api/jobs` is silently ignored; Persona B frustration |
| 3 | Sector-by-state heatmap | `analytics sector-heatmap [--funding-stage <s>] [--weight jobs]` | 9/10 | hand-code | GROUP BY on local `companies.primary_sector × companies.state` with optional weight by `jobs_count`, output table or CSV | Brief Build Priorities P2 (analytics); Persona A + C ritual; `/api/categories/counts` is flat, no crosstab endpoint |
| 4 | Funding × sector breakdown | `analytics funding-by-sector` | 9/10 | hand-code | Crosstab of `companies.funding_stage × primary_sector` with company counts and median employee_range bucket | Brief Data Layer enumerates funding_stage as a primary signal; Persona A weekly ritual; no aggregation endpoint |
| 5 | Top hiring companies | `companies top-hiring [--sector] [--state] [--funding-stage] [--limit N]` | 8/10 | hand-code | ORDER BY `jobs_count DESC` on local `companies`, with optional sector/state/funding filters applied first | Persona A (hiring signal) + Persona B (where the openings are); site has no ranking view |
| 6 | Company profile sheet | `companies profile <slug>` | 8/10 | hand-code | Joins `companies` + `jobs` (jobs at this company) + similar companies (same `primary_sector` and `employee_range` bucket) in one local query | Brief Top Workflow #3; "similar companies" has no API equivalent |
| 7 | Geographic clusters | `analytics geo-clusters [--state <code>] [--radius-km N]` | 8/10 | hand-code | Grid-bucket companies by lat/lon (default 50km cells), output cluster centroid, member count, member companies, dominant sector | Brief Data Layer enumerates lat/lon; Persona C "Midwest robotics startups" / "Texas advanced materials" ritual; site has no map view exposing clusters |
| 8 | Salary distribution | `jobs salary-stats [--sector] [--experience-level] [--state]` | 8/10 | hand-code | Compute p25/p50/p75 of `(salary_min+salary_max)/2` over filtered local `jobs` rows; null salaries reported separately | Persona B weekly ritual; brief enumerates salary_min/salary_max; no aggregation endpoint |
| 9 | Founded-year cohorts | `companies cohorts [--by founded_year] [--bucket 5y]` | 7/10 | hand-code | GROUP BY bucketed `founded_year` over local `companies`, with company counts and top-3 sectors per cohort | Persona C "founded since 2020" sidebar; Persona A new-company cohort tracking; founded_year is service-specific |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| #8 New companies feed (`companies new-since`) | Collapses into `whats-new`, which already returns companies + jobs deltas | #1 whats-new |
| #9 New jobs feed (`jobs new-since`) | Collapses into `whats-new`; redundant separate command | #1 whats-new |
| #11 Tag co-occurrence (`tags co-occurrence`) | Fails weekly-use check — exploration tool, not ritual; no persona runs this on a recurring basis | #3 sector-heatmap (covers sector-relation analytics) |
| #12 Categories tree (`categories tree`) | Wrapper-leaning — `categories list` + parent_id formatting, not transcendence; non-weekly orientation tool | #3 sector-heatmap (richer analytic use of category data) |
| #13 CSV export (`companies export --csv`) | `--json` / `--csv` / `--select` are generator-default surfaces emitted by `cliutil`; not a novel feature | All survivors inherit `--json` and `--select` from generator defaults |
| #14 Watch (`watch --interval`) | Scope creep — persistent background process. One-shot equivalent is `whats-new` | #1 whats-new |
| #16 Companies dormant (`companies dormant`) | Niche signal — monthly-at-best, no persona on weekly ritual; the hiring-signal axis is better served by ranking the populated side | #5 top-hiring |
