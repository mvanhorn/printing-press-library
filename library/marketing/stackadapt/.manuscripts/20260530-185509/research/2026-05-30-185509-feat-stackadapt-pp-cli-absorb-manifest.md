# StackAdapt — Absorb Manifest

Sources: official `@stackadapt/pa-typescript-sdk` (v0.73.0), Supermetrics/Improvado/Fivetran reporting connectors (feature parity for metrics/dims), live GraphQL introspection (64 read queries). No competing CLI/MCP exists. All READ-ONLY (98 mutations excluded).

## Absorbed (match or beat the SDK + connectors — all read)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | List advertisers | SDK advertisers | `stackadapt advertisers list` | offline, JSON, `--select` |
| 2 | Get advertiser | SDK advertiser | `stackadapt advertisers get <id>` | compact high-gravity fields |
| 3 | List campaigns | SDK campaigns | `stackadapt campaigns list` | filter by advertiser/status, offline |
| 4 | Get campaign | SDK campaign | `stackadapt campaigns get <id>` | budget + status inline |
| 5 | List campaign groups | SDK campaignGroups | `stackadapt campaign-groups list` | offline |
| 6 | Get campaign group | SDK campaignGroup | `stackadapt campaign-groups get <id>` | rollup |
| 7 | List ads | SDK ads | `stackadapt ads list --campaign <id>` | offline |
| 8 | List audiences | SDK audiences | `stackadapt audiences list` | offline |
| 9 | List custom segments | customSegments | `stackadapt segments list` | type surfaced |
| 10 | List third-party segments | thirdPartySegments | `stackadapt segments third-party` | offline |
| 11 | Account / user | account, user, tokenInfo | `stackadapt account` | token scope + account info |
| 12 | Campaign delivery report | connectors + campaignDelivery | `stackadapt report campaign-delivery --since 30d` | local cache, agent-native |
| 13 | Campaign insight report | campaignInsight | `stackadapt report campaign-insight --campaign <id>` | spend/CTR/ROAS/CPA |
| 14 | Campaign-group delivery | campaignGroupDelivery | `(generated endpoint) report campaign-group-delivery` | group rollup |
| 15 | Advertiser delivery | advertiserDelivery | `stackadapt report advertiser-delivery` | advertiser rollup |
| 16 | Audience insights | audienceInsights | `stackadapt report audience-insights` | segment performance |
| 17 | Conversion path | conversionPath | `stackadapt report conversion-path` | attribution timeline |
| 18 | Reach & frequency | reachFrequency | `(generated endpoint) report reach-frequency` | reach/freq |
| 19 | Geos lookup | geos | `(generated endpoint) geos list` | targeting reference |
| 20 | Cross-entity search | (none) | `stackadapt search <term>` | offline FTS across names |
| 21 | Raw SQL | (none) | `stackadapt sql "<query>"` | composable analysis |
| 22 | Local sync | (none — connectors are warehouse ETL) | `stackadapt sync` | builds the local store |
| 23 | Health/auth check | (none) | `stackadapt doctor` | validates token + reachability; verify-safe |

## Transcendence (only possible with local time-series + cross-joins)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Pacing observatory | `pacing --advertiser <id>` | hand-code | Joins campaign budget + campaignDelivery spend over time to flag under/over-pacing — connectors dump data, they don't compute pace. | Use to find campaigns off their budget pace. NOI command. |
| 2 | Delivery drift | `delivery-drift --campaign <id> --window 7d` | hand-code | CTR/CPM/spend WoW drift needs persisted history no single call returns. | Use to catch a degrading campaign early. |
| 3 | Spend reconcile | `reconcile --since 30d` | hand-code | campaignDelivery vs campaignInsight per-day spend diff; flags attribution drift. | Use to audit reported vs insight spend. |
| 4 | Audience overlap | `audience-overlap A B [C ...]` | hand-code | Pairwise segment overlap/cannibalization from audiences + insights. | Use to find segments competing for the same users. |
| 5 | Bottleneck | `bottleneck --advertiser <id>` | hand-code | Highest-spend campaigns ranked by worst ROAS/CPA with a 'why' column — local join across campaigns + delivery. | Use to find where budget is wasted. |
| 6 | Stale campaigns | `stale-campaigns --days 14` | hand-code | Active campaigns with zero recent delivery — needs delivery history to know 'zero'. | Use to find live-but-not-delivering campaigns. |

All transcendence rows ≥5/10, grounded in: connector research (pacing/spend/ROAS are the real metrics), the NOI, and the local-history thesis. Minimum 5 met (6 here).

## Out of scope (read-only)
All 98 mutations: create/update/upsert/delete/archive/restore/pause/resume/copy/schedule campaigns, ads, advertisers, segments, pixels, webhooks, profiles. The CLI does not write to StackAdapt.

## Stubs
None. CTV/DOOH/footfall/brand-lift queries are available but trimmed from v1 commands (niche); they remain reachable via the generated endpoint surface if included. Final set decided at the gate.
