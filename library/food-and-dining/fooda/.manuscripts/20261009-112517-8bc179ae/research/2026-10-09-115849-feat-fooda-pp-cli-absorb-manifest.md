# Fooda Absorb Manifest

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Menu per event/restaurant | npm fooda (2016) + app | fooda-pp-cli menu | Works with current site, --json, filters |
| 2 | Events by date/building | app /my + searchPublicEvents | fooda-pp-cli events | Popup/Cafe/Delivery/Catering, agent output |
| 3 | Restaurants list | searchPublicEvents | fooda-pp-cli restaurants | Cuisine filter, offline |
| 4 | Order history list | app /settings/orders | fooda-pp-cli orders | Synced locally, filterable |
| 5 | Order detail | app /settings/select_order/<uuid> | fooda-pp-cli orders get | Structured output |
| 6 | Subsidy view | getUserSubsidies | fooda-pp-cli subsidy | Parses remaining cents from qrInfo |
| 7 | Fooda Card balances | getFoodaCardOutstandingBalances | fooda-pp-cli card | Agent output |
| 8 | Profile/building | app /settings/profile | fooda-pp-cli whoami | Shows account/building ids |
| 9 | Recommendations | GetRecommendationsWithItem | fooda-pp-cli recommend | Passthrough |
| 10 | Place/cancel order | app mutations (not yet captured) | (stub) fooda-pp-cli order - mutation shape not captured; ships dry-run only | --confirm gated once captured |
| 11 | Sync/search/doctor/auth | framework | fooda-pp-cli sync | Chrome cookie import via auth login --chrome |

## Transcendence
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Served history (9) | served-history --since 90d | hand-code | Longitudinal view from synced order list | none |
| 2 | Venue rotation (8) | venue-rotation --since 120d | hand-code | Vendor frequency/recency from local orders | none |
| 3 | Spend trends (8) | spend-trends --since 6mo --group-by month | hand-code | Time-series over local orders | none |
| 4 | Week-ahead digest (7) | week-ahead | spec-emits | One-line digest across days of events | none |
| 5 | Subsidy status (7) | subsidy-status | spec-emits | Remaining $ today from qrInfo | none |
| 6 | Menu search (6) | menu-search "vegan" --type POPUP | spec-emits | Filter menus across today's events | none |
