# Forkable CLI Absorb Manifest

## Landscape
Forkable has **no public API, no docs, and zero community/competing tools** (confirmed by research). There is nothing to absorb from competitors. The "absorbed" surface is only the direct read commands the generator emits from the reverse-engineered GraphQL spec, plus framework commands (sync, search, sql, analytics, doctor).

## Absorbed (spec-emitted read commands)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Current user / preferences | Forkable `me` query | (generated endpoint) account me | Offline, --json, agent-native |
| 2 | List deliveries (with orders/pieces/receipts) | Forkable `myDeliveries` query | (generated endpoint) deliveries list | Offline mirror, --json, --select |
| 3 | In-progress delivery IDs | Forkable `myInProgressDeliveryIds` | (generated endpoint) deliveries in-progress-ids | --json |
| 4 | Menu detail (items/modifiers/ratings) | Forkable `menus` query | (generated endpoint) menus get | --json, --select |
| 5 | Meal clubs (teams, billing, membership) | Forkable `mealClubsAs` query | (generated endpoint) clubs list | Offline, --json |
| 6 | Buffet addresses | Forkable `myBuffetAddresses` | (generated endpoint) buffet-addresses list | --json |
| 7 | Meal auto-selection scores | Forkable `mealGenerationScores` | (generated endpoint) meal-scores list | --json |
| 8 | Venue usage over date range | Forkable `venueUsage` query | (generated endpoint) venue-usage get | --json |
| 9 | Account notifications | Forkable `myNotifications` query | (generated endpoint) notifications list | --json |
| 10 | Offline full-text search | (framework) | (behavior in forkable-pp-cli search) | FTS over synced deliveries/menus/clubs |
| 11 | SQL over local mirror | (framework) | (behavior in forkable-pp-cli sql) | Arbitrary SELECT over SQLite |
| 12 | Sync to local store | (framework) | (behavior in forkable-pp-cli sync) | Local SQLite mirror of all entities |

## Transcendence (only possible with our approach) — ALL shipping scope
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Served-meal history | served-history | hand-code | Joins local deliveries→orders→pieces→menus over the synced window; web SPA shows one delivery at a time and never aggregates | none |
| 2 | Preference-vs-served drift | preference-drift | hand-code | Mechanical set-membership join of `me` likes/dislikes/restrictions against served pieces; no such view exists anywhere | none |
| 3 | Auto-selection explainer | why-picked | hand-code | Joins mealGenerationScores + menus + me preferences to explain a delivery's auto-pick; SPA hides scoring | Use to explain a single delivery's auto-selection. Do NOT use for aggregate preference conformance across many deliveries; use 'preference-drift'. |
| 4 | Spend trend over time | spend-trend | hand-code | Buckets myDeliveries receipts into per-week/month totals; SPA has no cross-period spend view or CSV export | Use for time-bucketed spend totals. For allowance-vs-consumed utilization per club, use 'allowance-burn'. |
| 5 | Allowance utilization | allowance-burn | hand-code | Joins mealClubsAs allowances against myDeliveries receipts per club (incl. multi-club comparison) | Use for allowance-vs-spend utilization, including multi-club comparison. For raw time-series spend, use 'spend-trend'. |
| 6 | Week-ahead digest | upcoming-digest | hand-code | Joins in-progress + future deliveries + menus into one agent-shaped line per day | none |
| 7 | Venue rotation | venue-rotation | hand-code | Ranks venues by served-frequency + recency from local pieces→venues; distinct from single-venue `venue-usage get` | Use for cross-venue frequency/recency ranking. For one venue's raw usage stats, use the 'venue-usage get' command. |

## Stubs
None. All 7 transcendence rows are shipping scope (hand-code).

## Hand-code commitment
- 7 hand-code transcendence features (each ~50-150 LoC + root.go wiring).
- 0 spec-emits transcendence features.
- ~12 spec-emitted/framework read commands (auto-generated).
