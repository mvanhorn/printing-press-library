# CPSC Recalls Absorb Manifest

## Product thesis
Actionable recall screening with evidence-preserving product matches and explicit uncertainty.

## Absorbed surface
| Source | Features absorbed |
|---|---|
| CPSC/SaferProducts Recall API | recall filters/detail, products, hazards, remedies, incidents/injuries, manufacturers/retailers, images, contacts, dates, JSON/XML |
| CPSC web/app/email workflows | search, alerts, actionable remedy packet, historical recall lookup |
| Printing Press framework | sync, SQLite, local search/SQL, structured output, MCP mirror |

## Transcendence
| # | Feature | Command | Score | Buildability | Why only this CLI | Long Description |
|---|---|---|---|---|---|---|
| 1 | Evidence-preserving inventory check | `inventory-check` | 10 | hand-code | Joins inventory rows to nested products and explains exact/fuzzy match fields. | Inventory screening; use `packet` for one confirmed recall. |
| 2 | Brand/product change watch | `watch changes` | 9 | hand-code | Compares successive nested recall observations for material changes. | Changed records; use `hazard-pulse` for cohort composition. |
| 3 | Hazard pulse | `hazard-pulse` | 8 | hand-code | Flattens hazard, remedy, incident, injury, and category relationships with no-rate caveats. | Cohort composition; use `watch changes` for new records. |
| 4 | Actionable recall packet | `packet` | 9 | hand-code | Joins one recall's remedy/contact/images to inventory-match evidence. | One action packet; use `inventory-check` for bulk screening. |

## Deliberately excluded
Definitive fuzzy identity, incident-rate claims without exposure, semantic hazard classification, persistent notification infrastructure, and redundant remedy queues.

