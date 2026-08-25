# CFPB Complaints Absorb Manifest

## Product thesis
Reproducible complaint intelligence that never presents raw counts as market-adjusted quality scores.

## Absorbed surface
| Source | Features absorbed |
|---|---|
| CFPB complaint explorer and API | full-text/company/product/issue/geography/date filters, aggregations, detail, narratives, response/timeliness fields, exports |
| Notebook and generic search-client workflows | saved cohorts, trend tables, evidence IDs, CSV/JSON output |
| Printing Press framework | sync, SQLite, local search/SQL, structured output, MCP mirror |

## Transcendence
| # | Feature | Command | Score | Buildability | Why only this CLI | Long Description |
|---|---|---|---|---|---|---|
| 1 | Company pulse | `company pulse` | 10 | hand-code | Reproducible cohort mix and prior-window deltas. | Single-company briefing; use `compare companies` for peers. |
| 2 | Cohort-safe peer comparison | `compare companies` | 10 | hand-code | Enforces one shared cohort and mandatory denominator caveats. | Peer comparison; use `company pulse` for one company. |
| 3 | Emerging-theme delta | `emerging themes` | 9 | hand-code | Mechanical category/token deltas against a stored baseline. | Baseline trends; use `watch changes` for newly observed records. |
| 4 | Narrative evidence packet | `narratives packet` | 9 | hand-code | Deterministic stratification with complaint IDs and availability caveats. | Evidence packet; use `company pulse` for metrics. |
| 5 | Change watch | `watch changes` | 8 | hand-code | Finite comparison of successive local observations. | New records; use `emerging themes` for baseline growth. |

## Deliberately excluded
Company leaderboards, market-adjusted rates without denominators, sentiment/causal NLP, identity inference, maps based on raw counts, and alert daemons.

