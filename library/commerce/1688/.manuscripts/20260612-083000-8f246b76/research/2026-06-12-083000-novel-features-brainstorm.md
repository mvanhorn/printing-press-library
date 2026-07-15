# 1688-pp-cli — Novel Features Brainstorm (subagent audit trail)

## Customer model

**1. Dropship Dana — Shopify/TikTok-Shop dropshipper, US, solo.**
- Today: searches 1688 via Google-Translate in a tab forest, screenshots offers into Notion; can't read 回头率 or factory badges, picks by lowest price, gets burned by traders.
- Weekly ritual: hunts 5-10 trending products, pulls one cheap supplier per product, re-checks prices before reordering.
- Frustration: no record of last week's price; can't tell if a 限时价 actually dropped or if the listing is a reseller marking up a factory two clicks away.

**2. Sourcing-Agent Sam — China-based sourcing agent serving Western clients.**
- Today: juggles 30-50 candidate offers per client SKU across tabs + a manual spreadsheet; reads shop trade-service scores and badges one shop at a time.
- Weekly ritual: per client SKU, assembles a "real factory vs trader" shortlist ranked by reliability with MOQ + tiered price.
- Frustration: "find the actual factory among resellers" is pure manual badge-reading; nothing rolls factoryInspection + superFactory + 深度验厂 + repurchase into one confidence signal.

**3. Importer Ingrid — Amazon FBA private-label seller, mid-volume, US.**
- Today: commits PO volume to one supplier per SKU; tracks price + reliability in a hand-updated spreadsheet.
- Weekly ritual: before each reorder, re-checks whether her supplier's price moved, transaction count is still healthy, and whether a better alternative appeared.
- Frustration: no drift history; can't answer "did price/reorder-rate move since I last looked?" without manual records; paid scrapers are stateless.

**4. Procurement Pat — small import company's procurement lead, many SKUs.**
- Today: approved-supplier list in Excel; reliability is institutional memory.
- Weekly ritual: vets new suppliers, audits existing ones; needs a per-shop reliability rollup across all their offers.
- Frustration: supplier reliability lives across dozens of offer pages; nothing aggregates a shop's footprint into one auditable record.

## Candidates (pre-cut)

(See Survivors/Kills below for verdicts.) Generated 14: C1 factory-find (b/a, KEEP), C2 repurchase-top (b, KEEP), C3 drift (c/a, KEEP), C4 compare (c/a, KEEP), C5 supplier-report (c/b, KEEP), C6 classify (b, FOLD→C1), C7 watch (a/c, KEEP descoped), C8 landed-cost (a, KILL — freight/duty not in spec), C9 translate-term (a, KILL — external/LLM), C10 new-offers (c, FOLD→C7), C11 tiers (b, KILL — absorbed), C12 region-spread (c/b, KEEP provisional), C13 shortlist (a, FOLD→C1), C14 low-risk (b/a, FOLD→C1).

## Survivors and kills

### Survivors

| # | Feature | Command | Score | Buildability | How It Works | Long Description |
|---|---------|---------|-------|--------------|--------------|------------------|
| 1 | Factory-confidence rank + label | `factory-find <keyword> [--top N] [--min-trade N]` | 9/10 | hand-code | Re-ranks local synced offers by weighting factoryInspection/superFactory/businessInspection + 深度验厂 serviceTag + offerRepurchaseRate + supplier trade-scores; emits a `trader\|likely-factory\|verified-factory` column | none |
| 2 | Repurchase-rate leaderboard | `repurchase-top <keyword> [--min-tx N]` | 7/10 | hand-code | Sorts synced offers/suppliers by 回头率 with a min-transaction floor to suppress low-volume noise | none |
| 3 | Price/repurchase/transaction drift | `drift <offerId\|keyword>` | 9/10 | hand-code | Diffs latest snapshot vs prior synced_at rows (price_cny / repurchase_rate / booked_count) | none |
| 4 | Cross-supplier SKU compare | `compare <offerId> <offerId> ...` | 8/10 | hand-code | Joins offers + suppliers in local SQLite for side-by-side price/MOQ/tier/repurchase/transaction/factory-flags/trade-scores | Use for comparing specific OFFERS already synced. Do NOT use to rank a fresh keyword search; use `factory-find` (rank) or `repurchase-top` (sort). |
| 5 | Supplier reliability rollup | `supplier-report <memberId>` | 7/10 | hand-code | Aggregates one shop across all synced offers: trade-service scores + avg repurchase + total transactions + badges + offer count + price range | Operates on a SUPPLIER (memberId), rolling up synced history. The absorbed `supplier <memberId>` fetches one live profile; this aggregates stored offers. For a single product use `offer`. |
| 6 | Saved-query watch (poll-once delta) | `watch <keyword> [--since]` | 7/10 | hand-code | Re-syncs a saved query, persists a new snapshot, prints the delta vs last run (price/repurchase/tx changes + new offers/suppliers) | Re-SYNCS a saved query and reports the delta, including new entrants. Use `drift` to read existing snapshot history WITHOUT a fresh sync. |
| 7 | Province price-spread | `region-spread <keyword>` | 5/10 | hand-code | Groups synced offers by province; reports min/median/max price + transaction count per region | none |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| C6 classify | Same logic as factory-confidence; splits one signal across two commands | factory-find (emits label as column) |
| C8 landed-cost | Freight/duty not in spec; remainder is a user multiplier, no leverage | compare |
| C9 translate-term | Requires translation = external/LLM service not in spec | none |
| C10 new-offers | Additive half of a watch delta; duplicates diff logic | watch |
| C11 tiers | Tiered price + MOQ already in absorbed search/offer output | compare |
| C13 shortlist | factory-find + --csv/--top framework flags; no new logic | factory-find |
| C14 low-risk | Trade-service scores are part of the same reliability weighting | factory-find |
