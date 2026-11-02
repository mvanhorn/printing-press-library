# Government Data absorb manifest

Read-only user-approved workflows; administrative upload/update/delete and generic CKAN admin methods are outside scope.

### Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Dataset search | ckanapi + Data.gov MCP | government-data-pp-cli catalog find | Common source selector, explicit per-source counts/errors |
| 2 | Dataset metadata | Data.gov MCP source | government-data-pp-cli dataset show | Normalized view plus raw metadata/provenance |
| 3 | Resource listing | California docs + Data.gov MCP | government-data-pp-cli dataset resources | Download URLs and formats |
| 4 | Resource download | Data.gov MCP resource template | government-data-pp-cli resource download | Explicit output path and bounded download |
| 5 | Organizations/publishers | California docs + federal v4 | government-data-pp-cli catalog organizations | Peer discovery |
| 6 | Topics/keywords | Data.gov MCP group_list/tag_list | government-data-pp-cli catalog topics | Source-native categories clearly labeled |
| 7 | Pagination/export | ckanapi search/dump | government-data-pp-cli catalog export | Bounded pages with resume/cursor output |
| 8 | Local metadata sync | ckanapi metadata export | government-data-pp-cli sync | Persist query scope with source identity |
| 9 | Offline lookup | Printing Press framework | government-data-pp-cli search | Local FTS and stale/unsynced hints |

### Transcendence
| # | Feature | Command | Score | Buildability | How It Works | Evidence | Persona served | Long Description |
|---|---|---|---|---|---|---|---|---|
| 1 | Shared resource evidence | `overlap` | 8/10 (fit 3, pain 2, feasibility 2, backing 1) | hand-code | This uses saved dataset-to-resource rows from both catalogs to join exact resource URL strings and emit all contributing catalog IDs, with no external dependencies. | Brief Top Workflows #4 explicitly calls for identical agency download URL detection; Data.gov publishing guide explains harvested agency resource metadata. | California water-policy analyst | Use this command to find shared resource URLs across saved catalog records. Do NOT use it to compare records over time; use 'changes' instead. |
| 2 | Saved metadata changes | `changes` | 8/10 (fit 3, pain 2, feasibility 1, backing 2) | hand-code | This uses two explicit local metadata snapshots to compute field-level and resource-URL changes and snapshot-membership differences, with no external dependencies. | Brief weekly source refresh and reporter verification rituals; Data.gov user guide/publishing model describes ongoing harvested metadata updates; ckanapi exports make old metadata preservation feasible. | Civic-data journalist | Use this command to compare two saved metadata snapshots over time. Do NOT use it to identify cross-catalog resource overlap; use 'overlap' instead. |
| 3 | Selection provenance lock | `intake lock` | 8/10 (fit 3, pain 2, feasibility 2, backing 1) | hand-code | This uses explicitly selected saved dataset/resource rows to write a deterministic manifest containing catalog IDs, exact resource URLs, metadata SHA-256 values, selection scope and capture times, with no external dependencies. | Brief research-engineer repeatable intake ritual and metadata-versus-file distinction; California and federal metadata provide resource URLs and identifiers. | Research data engineer | Use this command to preserve the selected resources and their catalog provenance. Do NOT use it to diagnose missing metadata; use 'intake audit' instead. |
| 4 | Metadata intake audit | `intake audit` | 8/10 (fit 3, pain 2, feasibility 2, backing 1) | hand-code | This uses saved raw metadata and normalized resource rows to compute explicit missing-field findings for title, publisher, license, modified date, URL and format, with no external dependencies. | Brief intake workflow inspects license, dates and formats; CKAN and federal catalog responses expose these metadata fields or their absence. | Research data engineer / California water-policy analyst | Use this command to find missing metadata before selecting resources. Do NOT use it to preserve a selected intake manifest; use 'intake lock' instead. |
| 5 | Saved format coverage | `coverage` | 7/10 (fit 2, pain 2, feasibility 2, backing 1) | hand-code | This uses explicit saved topic-result membership joined to resource formats and catalog identity to compute a source-by-format matrix plus records without declared formats, with no external dependencies. | Brief weekly analyst topic sweep and equal-catalog vision; California browser capture demonstrates format filters and resource-format metadata. | California water-policy analyst | Use this command to compare format availability within a saved topic result set. Do NOT use it to inspect individual metadata omissions; use 'intake audit' instead. |


## Boundaries
No stubs. All five novel features require custom Go and snapshot/scope persistence. DEMO_KEY is exploration-only; no automatic broad refresh. Exact URL overlap is evidence of shared resource links, not semantic dataset identity. Snapshots cover bounded queries, not entire catalogs.

### SEC EDGAR additions
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 10 | Company resolution by ticker/CIK | SEC submissions metadata and ticker directory | government-data-pp-cli edgar company | Explicit CIK and ticker provenance |
| 11 | Full-text filing search | Captured SEC EFTS search | government-data-pp-cli edgar search | Query, dates, forms, bounded pagination and source links |
| 12 | Company filing history | SEC submissions API | government-data-pp-cli edgar filings | Form/date filters; bounded older-file traversal with coverage shown |
| 13 | Filing documents | SEC archive | government-data-pp-cli edgar documents | List document metadata and download an explicitly selected filing document |
| 14 | Company financial facts | SEC companyfacts and companyconcept APIs | government-data-pp-cli edgar facts | Preserve taxonomy, units, periods, accession and filed date; concept selection |

SEC uses configurable SEC_USER_AGENT, conservative shared pacing, and retry/backoff. No contact address embedded in source or examples. No trading, filing submission, inferred financial statement restatement, or paid proxy service. Full-text search website contract can change; failures must be explicit.
