# EPA ECHO Absorb Manifest

## Product thesis
Identifier-preserving environmental due diligence with transparent evidence instead of an opaque risk score.

## Absorbed surface
| Source | Features absorbed |
|---|---|
| EPA ECHO search/report services | all-media and media facility search, DFR, enforcement cases, corporate screening, effluent/loading, geospatial filters |
| ECHO web and bulk workflows | facility dossiers, parent/facility screening, nearby results, bulk-data boundary |
| Printing Press framework | sync, SQLite history, local search/SQL, agent output, MCP mirror |

## Transcendence
| # | Feature | Command | Score | Buildability | Why only this CLI | Long Description |
|---|---|---|---|---|---|---|
| 1 | Explainable facility resolution | `resolve facility` | 10 | hand-code | Ranks matches with FRS/program IDs, conflicts, and provenance. | none |
| 2 | Facility dossier change set | `dossier diff` | 10 | hand-code | Diffs inspections, violations, cases, penalties, and pollutant snapshots. | One facility; use `watch --portfolio` for many. |
| 3 | Portfolio compliance watch | `watch --portfolio` | 10 | hand-code | Crosswalks portfolio rows into an evidence-bearing change queue. | Many facilities; use `dossier diff` for one. |
| 4 | Explained nearby scan | `nearby explain` | 9 | hand-code | Links nearby facilities to transparent concern drivers and source IDs. | none |
| 5 | Effluent exceedance trend | `effluent trend` | 8 | hand-code | Historizes pollutant exceedances under stable facility/program identity. | none |
| 6 | Enforcement chain timeline | `enforcement timeline` | 9 | hand-code | Orders evaluations, violations, cases, and penalties across record families. | Enforcement sequence; use `dossier diff` for all changes. |

## Deliberately excluded
Composite risk scores, external-news enrichment, persistent dashboards, ambiguous missing-data grades, and thin media-search aliases.

