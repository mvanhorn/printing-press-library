# CourtListener Absorb Manifest

## Product thesis
Reproducible docket and litigation research that keeps RECAP coverage and document availability explicit.

## Absorbed surface
| Source | Features absorbed |
|---|---|
| CourtListener REST v4.4 | search, courts, dockets, entries, RECAP documents, parties/attorneys, opinions/clusters, judges, oral arguments, alerts/tags |
| CourtListener/RECAP and PACER workflows | docket chronology, advanced search, watches, document availability links |
| Printing Press framework | sync, SQLite, local search/SQL, structured output, MCP mirror |

## Transcendence
| # | Feature | Command | Score | Buildability | Why only this CLI | Long Description |
|---|---|---|---|---|---|---|
| 1 | Chronological docket brief | `docket` | 10 | hand-code | Joins parties, counsel, entries, and document availability into one timeline. | One docket; use `new-filings` for saved-watch changes. |
| 2 | New-filing watch report | `new-filings` | 10 | hand-code | Uses cursors and first-seen timestamps across dockets, filings, documents, and opinions. | Watch changes; use `docket` for complete history. |
| 3 | Cross-docket party map | `party` | 9 | hand-code | Maps selected party identities across matters and counsel. | Litigant cases; use `counsel` for attorney/firm patterns. |
| 4 | Recurring counsel map | `counsel` | 9 | hand-code | Shows observed representations across parties, courts, and time. | Counsel patterns; use `party` for litigant cases. |
| 5 | Non-predictive judge context | `judge` | 9 | hand-code | Descriptive, sourced opinion/court/case-type context without outcome prediction. | none |
| 6 | RECAP availability audit | `recap-gaps` | 9 | hand-code | Separates available, metadata-only, unavailable, and ambiguous document coverage. | Availability audit; use `docket` for chronology. |

## Deliberately excluded
Outcome prediction, causal judge scoring, inferred party identity, unlicensed PACER automation, semantic document summaries, and persistent alert infrastructure.

