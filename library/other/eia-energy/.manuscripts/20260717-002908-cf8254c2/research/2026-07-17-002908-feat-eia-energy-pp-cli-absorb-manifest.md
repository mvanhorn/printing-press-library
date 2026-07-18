# EIA Energy Absorb Manifest

## Product thesis
Metadata-aware energy analysis with explicit unit/frequency alignment, pagination, provenance, and revisions.

## Absorbed surface
| Source | Features absorbed |
|---|---|
| EIA API v2 | route discovery, metadata, facets, frequencies, data columns, generic query, offset/length pagination |
| EIA explorer and client/notebook workflows | electricity/petroleum/gas shortcuts, saved series, comparisons, chart-source queries |
| Printing Press framework | sync, SQLite history, local search/SQL, agent output, MCP mirror |

## Transcendence
| # | Feature | Command | Score | Buildability | Why only this CLI | Long Description |
|---|---|---|---|---|---|---|
| 1 | Latest grid pulse | `grid pulse` | 9 | hand-code | Joins grid measures to a trailing baseline with freshness and units. | none |
| 2 | Alignment-safe series comparison | `state compare` | 10 | hand-code | Refuses incompatible frequency/unit combinations using cached route schemas. | Like-for-like comparisons; use `spread` for defined cross-series arithmetic. |
| 3 | Guarded cross-series spread | `spread` | 10 | hand-code | Aligns periods and validates units/frequencies before arithmetic. | Difference/ratio; use `state compare` for comparison tables. |
| 4 | Explainable anomaly scan | `anomaly` | 9 | hand-code | Emits deterministic baseline statistics and exact source rows. | One series; use `watch run` for saved rule collections. |
| 5 | Saved watch evaluation | `watch run` | 9 | hand-code | Evaluates threshold, freshness, change, and missing-data rules once. | Saved rules; use `anomaly` for exploratory deviation analysis. |
| 6 | Source revision diff | `revisions` | 7 | hand-code | Compares versioned snapshots of the same route/facet/period key. | Source revisions; use `anomaly` for adjacent-period movement. |

## Deliberately excluded
Hidden universal unit conversion, AI causal explanations, forecasts, dashboards, unlimited mirroring, and redundant completeness reports.

