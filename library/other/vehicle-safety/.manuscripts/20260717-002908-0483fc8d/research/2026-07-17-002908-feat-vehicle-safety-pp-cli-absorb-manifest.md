# Vehicle Safety Absorb Manifest

## Product thesis
An evidence-first NHTSA safety dossier with honest comparisons and local change history.

## Absorbed surface
| Source | Features absorbed |
|---|---|
| NHTSA datasets/APIs and SaferCar | VIN decode, recalls, complaints, crash ratings, investigations, manufacturer communications, product lookup |
| WhatWeDrive / Vehicle Safety Hub / Autobot / NHTSA MCP projects | one-vehicle lookup, complaint/recall summaries, comparisons, alert-shaped workflows |
| Printing Press framework | sync, SQLite history, search, SQL, structured/agent output, MCP mirror |

## Transcendence
| # | Feature | Command | Score | Buildability | Why only this CLI | Long Description |
|---|---|---|---|---|---|---|
| 1 | Safety dossier | `dossier` | 10 | hand-code | Joins identity and six safety record families with scope caveats. | Comprehensive one-vehicle report; use `compare` for two vehicles. |
| 2 | Garage recall watch | `watch` | 10 | hand-code | Compares first/last-seen campaign and remedy snapshots. | none |
| 3 | Defect signal timeline | `signals` | 10 | hand-code | Aligns structured complaint components with investigation, recall, and communication dates. | Component chronology; use `dossier` for a complete vehicle report. |
| 4 | Honest model comparison | `compare` | 10 | hand-code | Normalizes model identities and exposes missing complaint denominators. | Two-model comparison; use `dossier` for one vehicle. |
| 5 | VIN/model recall reconciliation | `recall-coverage` | 9 | hand-code | Contrasts VIN-specific unrepaired recalls with model-wide campaigns. | Coverage reconciliation; use `dossier` for the full report. |
| 6 | Complaint-to-bulletin bridge | `bulletin-bridge` | 8 | hand-code | Shows structured co-occurrence without semantic or causal claims. | Bulletin overlap; use `signals` for broader chronology. |

## Deliberately excluded
Risk scores, bulk VIN sweeps, narrative NLP, causal claims, persistent daemons, and duplicated formatting-only commands.

