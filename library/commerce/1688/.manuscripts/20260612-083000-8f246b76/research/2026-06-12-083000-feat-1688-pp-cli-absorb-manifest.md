# 1688-pp-cli — Absorb Manifest

API surface (verified this session): signed mtop offer search (`mtop.relationrecommend.WirelessRecommend.recommend`, appId=32517) on `h5api.m.1688.com` + reachable HTML offer-detail page (`detail.1688.com/offer/{id}.html`). Auth: none (anonymous token-bootstrap + md5-signed request). The signed call needs a hand-written client; the generator supplies the local store, FTS, and output scaffolding.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Keyword product search + pagination | Apify `songd/1688-search-scraper`, `ecomscrape/...-search`, Oxylabs | `1688-pp-cli search <keyword>` | Free (no paid scraper key), persists to local store, `--json`/`--select`/`--csv`/typed exit codes |
| 2 | Price-range filter | 1688 native search params | `(behavior in 1688-pp-cli search)` `--price-min` / `--price-max` | Typed, composable, validated |
| 3 | Supplier region filter | 1688 native | `(behavior in 1688-pp-cli search)` `--province` | Accepts Chinese province (广东) with pinyin→Chinese alias help |
| 4 | Sort order | 1688 native | `(behavior in 1688-pp-cli search)` `--sort` | Typed enum: `price-asc`/`price-desc`/`booked`/`newest` |
| 5 | Tiered/bulk price + MOQ | Apify/Oxylabs | `(behavior in 1688-pp-cli search)` price + MOQ in offer output | Structured fields, not screenshots |
| 6 | Transaction / sales-volume counts | all scrapers | `(behavior in 1688-pp-cli search)` `transaction_count` + `sales_volume` | Per-offer + tracked over time via snapshots |
| 7 | Supplier quality / trade-service scores | Oxylabs, Apify `devcake/...-supplier-scraper` | `(behavior in 1688-pp-cli search)` supplier trade-scores in output | Also rolled up by `supplier-report` |
| 8 | Product-detail / single-offer lookup | Apify `ecomscrape/...-details`, `jeff2go/1688-Crawler` | `1688-pp-cli offers <offerId>` | Full stored record by ID (price/supplier/reorder/factory), free, no key |
| 9 | Local store + sync | (none — all competitors stateless) | `1688-pp-cli sync <keyword>` | NEW: persistence no competitor has |
| 10 | Offline search over stored offers | (none) | `1688-pp-cli find <term>` | NEW: offline FTS over the local store |
| 11 | Arbitrary local analytics | (none) | `(behavior in 1688-pp-cli sql)` | NEW: SQL over the local SQLite store |
| 12 | Search-by-image | AliPrice extension | `(stub) requires image upload + a different mtop endpoint; out of v1 scope, documented in README ## Known Gaps` | Deferred |

## Transcendence (only possible with our approach)

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Factory-confidence rank + label | `factory-find <keyword> [--top N] [--min-trade N]` | hand-code | Weights factoryInspection/superFactory/businessInspection + 深度验厂 serviceTag + repurchase + supplier trade-scores into one ranking + emits `trader\|likely-factory\|verified-factory` label; no scraper ranks factory-vs-trader (Workflow 2) | none |
| 2 | Repurchase-rate leaderboard | `repurchase-top <keyword> [--min-tx N]` | hand-code | Ranks by 回头率 (reorder rate) with a min-transaction floor; competitors expose the raw field but never as a first-class ranking | none |
| 3 | Price / repurchase / transaction drift | `drift <offerId\|keyword>` | hand-code | Diffs latest snapshot vs prior `synced_at` rows in the local snapshot table; impossible for stateless scrapers (Workflow 4, headline differentiator) | none |
| 4 | Cross-supplier SKU compare | `compare <offerId> <offerId> ...` | hand-code | Joins offers + suppliers in local SQLite for side-by-side price/MOQ/tier/repurchase/transaction/factory-flags/trade-scores (Workflow 3) | Use for comparing specific OFFERS already synced. Do NOT use to rank a fresh keyword search; use `factory-find` (rank) or `repurchase-top` (sort). |
| 5 | Supplier reliability rollup | `supplier-report <memberId>` | hand-code | Aggregates one shop across all synced offers: trade-service scores + avg repurchase + total transactions + badges + offer count + price range (Workflow 5) | Operates on a SUPPLIER (memberId), rolling up synced history. For a single product use `offer`. |
| 6 | Saved-query watch (poll-once delta) | `watch <keyword> [--since]` | hand-code | Re-syncs a saved query, persists a new snapshot, prints the delta (price/repurchase/tx changes + new offers/suppliers); stateless competitors cannot diff across runs | Re-SYNCS a saved query and reports the delta, including new entrants. Use `drift` to read existing snapshot history WITHOUT a fresh sync. |
| 7 | Province price-spread | `region-spread <keyword>` | hand-code | Groups synced offers by province; reports min/median/max price + transaction count per region (weakest survivor, 5/10) | none |

**Hand-code count:** 7 transcendence rows (all `hand-code`) + the signed mtop client + the `search`/`sync`/`offer`/`find` commands (the generated client cannot do mtop md5-signing, so the core is hand-built). No `spec-emits` rows.

**Stubs:** row 12 (search-by-image) only — explicitly deferred, documented in README ## Known Gaps.
