# Absorb Manifest: American Reindustrialization CLI

## Existing Ecosystem
Phase 1.5a search returned **zero** competing tools — no MCP server, no Claude plugin, no npm/PyPI SDK, no community CLI. The site is too new and niche for any ecosystem to have emerged. There is **nothing to absorb from competitors**; the table-stakes tier is purely "mirror the public API surface."

## Absorbed (mirror the public read API — auto-emitted by the generator)
| # | Surface | Command (auto-emitted) | Added Value |
|---|---------|------------------------|-------------|
| 1 | List companies | `companies list` | Local-cache + `--select` + `--csv` + `--json` |
| 2 | Get company by slug | `companies get <slug>` | Same + cached |
| 3 | Search companies | `companies search <q>` | Same + paged client-side |
| 4 | List categories | `categories list` | Local-cache |
| 5 | Get category by slug | `categories get <slug>` | Local-cache |
| 6 | Category counts | `categories counts` | Resolves UUID → slug + name from cache for human-readable output |
| 7 | Search categories | `categories search <q>` | Local-cache |
| 8 | List tags | `tags list` | Local-cache + `tag_type` filter |
| 9 | Get tag by slug | `tags get <slug>` | Local-cache |
| 10 | Tag counts | `tags counts` | Resolves UUID → slug + name |
| 11 | Search tags | `tags search <q>` | Local-cache |
| 12 | List jobs | `jobs list` | Local-cache + working server-side filters (`work_mode`, `experience_level`) |
| 13 | Get job by slug | `jobs get <slug>` | Local-cache |
| 14 | Jobs companies autocomplete | `jobs companies` | Local-cache |
| 15 | Jobs titles autocomplete | `jobs titles --q <q>` | Local-cache |
| 16 | Jobs categories autocomplete | `jobs categories` | Local-cache |
| 17 | Jobs tags autocomplete | `jobs tags` | Local-cache |
| 18 | List news | `news list` | Local-cache (empty at capture; defensive shape) |

Plus generator-emitted foundation commands: `sync`, `search` (FTS across all entities), `sql` (read-only SQLite), `stale`, `doctor`, `version`, `agent-context`, `context`, `which`.

## Transcendence (9 features, all hand-code, all scoring ≥7/10)
| # | Feature | Command | Score | Buildability | How It Works | Evidence |
|---|---------|---------|-------|--------------|--------------|----------|
| 1 | Diff since last sync | `whats-new --since <date>` | 10/10 | hand-code | Snapshots local SQLite store on each sync, diffs current vs prior snapshot for companies and jobs by `slug` + `updated_at`/`posted_at` | Brief Top Workflow #5; Persona A frustration; no API endpoint |
| 2 | Composed jobs filter | `jobs find --work-mode --experience --salary-min --state --company-size --sector --posted-since` | 10/10 | hand-code | Joins local `jobs` to `companies` and applies all filters in one SQL query | Site's `state` filter on jobs is silently ignored; Persona B frustration |
| 3 | Sector-by-state heatmap | `analytics sector-heatmap [--funding-stage] [--weight jobs]` | 9/10 | hand-code | GROUP BY on local `companies.primary_sector × companies.state` with optional `jobs_count` weight | `/api/categories/counts` is flat; Personas A + C |
| 4 | Funding × sector breakdown | `analytics funding-by-sector` | 9/10 | hand-code | Crosstab of `funding_stage × primary_sector` with company counts and median employee_range | Persona A weekly ritual; no aggregation endpoint |
| 5 | Top hiring companies | `companies top-hiring [--sector] [--state] [--funding-stage] [--limit N]` | 8/10 | hand-code | ORDER BY `jobs_count DESC` with optional filters | Site has no ranking view |
| 6 | Company profile sheet | `companies profile <slug>` | 8/10 | hand-code | Joins `companies` + `jobs` (at this company) + similar companies (same sector + employee_range) | "Similar companies" has no API equivalent |
| 7 | Geographic clusters | `analytics geo-clusters [--state] [--radius-km N]` | 8/10 | hand-code | Grid-bucket companies by lat/lon (default 50km cells); output centroid + members + dominant sector | Brief enumerates lat/lon; site has no map/cluster view |
| 8 | Salary distribution | `jobs salary-stats [--sector] [--experience-level] [--state]` | 8/10 | hand-code | Compute p25/p50/p75 of `(salary_min+salary_max)/2` over filtered local jobs; null salaries reported separately | Persona B; no aggregation endpoint |
| 9 | Founded-year cohorts | `companies cohorts [--bucket 5y]` | 7/10 | hand-code | GROUP BY bucketed `founded_year` with top-3 sectors per cohort | Persona C "founded since 2020" sidebar |

## Hand-code commitment
**All 9 transcendence rows are tagged `hand-code`.** Each requires ~50-150 LoC of Cobra command + SQL + output formatting plus root.go wiring. Approving this manifest commits to ~700-1300 LoC of novel-feature implementation in Phase 3.

## Killed candidates (audit trail)
- `companies new-since` / `jobs new-since` — collapsed into `whats-new`
- `tags co-occurrence` — non-weekly exploration tool
- `categories tree` — wrapper-leaning, orientation-only
- `companies export --csv` — generator-default surface, not novel
- `watch --interval` — scope creep (persistent process); covered by one-shot `whats-new`
- `companies dormant` — niche signal, no weekly persona ritual
