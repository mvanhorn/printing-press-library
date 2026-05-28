# HubSpot CRM Contacts Absorb Manifest

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-----------------|
| 1 | List contacts | hubspot-api-python `basic_api.get_page` | `(generated endpoint) contacts list` | Cursor pagination, `--select`, `--json`, offline SQLite caching |
| 2 | Get contact by ID | hubspot-api-python `basic_api.get_by_id` | `(generated endpoint) contacts get` | `--include-properties`, falls back to local store |
| 3 | Search contacts (filter groups) | hubspot-api-python `search_api.do_search` | `(generated endpoint) contacts search` | Server filter language exposed via flags, paginates to completion when asked |
| 4 | Batch read contacts | hubspot-api-python `batch_api.read` | `(generated endpoint) contacts batch_read` | Stdin-friendly id list, hard-bounded batch size |
| 5 | Properties schema | hubspot-api-python `properties_api.get_all` | `(generated endpoint) properties list` | Caches schema locally so search filters can validate field names |
| 6 | Get contact by email (lookup) | leonelquinteros/hubspot `GetByEmail` | `(behavior in hubspot-pp-cli contacts search) --email <addr>` | Auto-routes to `search` with `EQ email` filter; one-shot lookup |
| 7 | Sync all contacts to local store | (no public CLI does this) | `hubspot-pp-cli sync` | Incremental via `lastmodifieddate` cursor, snapshots `hubspotscore` per sync for trend math |
| 8 | Full-text contact search | HubSpot UI global search | `hubspot-pp-cli search "<query>"` | Local FTS5 over name, email, company; works offline |
| 9 | Composable SQL | (no public CLI does this) | `hubspot-pp-cli sql "<select-only-query>"` | SELECT-only with allow-list of tables, JSON/CSV output |
| 10 | Doctor / health check | (no public CLI does this) | `hubspot-pp-cli doctor` | Verifies token, portal id, scope list, prints last-sync staleness |

Every row above is a feature we MUST ship. Generated endpoints come from the spec automatically; rows 6-10 are wired in code.

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|--------------------------|-------------------|
| 1 | Engagement decay | engagement-decay --window 30d --min-prior-opens 3 | hand-code | Requires joining historical engagement snapshots against current silence window — HubSpot UI is set-based, not sequence-based. | Use for "previously-hot leads going dark" Monday-morning re-engagement lists. Do NOT use for new-lead discovery; use `contacts search` instead. |
| 2 | Lifecycle stuck | lifecycle-stuck --stage MQL --multiplier 2x | hand-code | Requires team-median dwell-per-stage baseline computed from local snapshots, then flags outliers relative to that. HubSpot only exposes current stage + entry timestamp. | Use to surface SLA breaches in MQL/SQL/Opportunity dwell time. Do NOT use for funnel-conversion reporting; HubSpot Sales Hub does that natively. |
| 3 | Stale but valuable | stale-but-valuable --score-min 70 --silent-days 21 | hand-code | Composite rank by `value_score = f(hubspotscore, total_revenue, num_deals) / engagement_recency` — UI sorts on one column only. | Use to save the whales before they churn — high-value contacts whose engagement has cooled. |
| 4 | Source ROI | source-roi --since 2026-01-01 --min-contacts 20 | hand-code | Joins `hs_analytics_source*` to deal/revenue properties with statistical-minimum filtering — UI shows raw counts only. | Use to defund channels that produce tire-kickers vs. revenue. |
| 5 | Score drift | score-drift --window 14d --drop-pct 25 | hand-code | Requires snapshotting `hubspotscore` per sync to compute deltas. HubSpot has no native score-history view without Operations Hub. | Use as a leading indicator before lifecycle stage catches up. Pair with `engagement-decay` for triangulation. |
| 6 | Owner overload | owner-overload --weight-by-stage --top 10 | hand-code | Weighted aggregation across owners × stages × contacts, compared to team median. UI shows flat counts, no weighting, no median. | Use to flag pipeline imbalance before AE attrition. |
| 7 | Silent after first touch | silent-after-first-touch --since 90d | hand-code | Sequence-based filter ("exactly one event, then nothing") — HubSpot's filter builder is set-based. | Use to distinguish tire-kicker one-clicks from genuinely interested follow-up candidates. |
| 8 | Duplicate suspects | duplicate-suspects --threshold 0.85 | hand-code | Fuzzy match on email-local-part, normalized name, company — HubSpot's native dedupe is exact-email only. | Use for periodic CRM hygiene. Outputs candidate pairs with similarity scores; merge stays manual (write scope intentionally out). |
| 9 | Daily digest | daily-digest --since yesterday | hand-code | Diffs today vs. yesterday across score, stage, engagement — HubSpot's email digest doesn't cover deltas across these dimensions. | Use as a sales-manager morning briefing. Mirrors the Stripe daily-digest pattern shipped in PR #915. |

9 transcendence commands. All hand-code. No stubs — every row ships fully implemented.
