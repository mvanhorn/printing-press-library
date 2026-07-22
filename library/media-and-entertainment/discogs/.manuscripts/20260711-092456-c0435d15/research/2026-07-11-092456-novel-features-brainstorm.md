# Discogs CLI — Novel Features Brainstorm (Step 1.5c.5 audit trail)

## Customer model

**Persona A — "Marcus," the wantlist deal-hunter.** 150–300 entry wantlist of pressings he'd buy at the right price. Today: checks marketplace pages one at a time, uses `discogs-alert` for a few items but finds it heavyweight; can't answer "which wants are cheap vs their own history" (API has no price history). Weekly: sweep wantlist for anything dropped into buying range. Frustration: no price memory, no fill notification; 200 items by hand is impossible under 60/min.

**Persona B — "Dana," the part-time flipper/seller.** Few hundred listings, buys to resell. Today: pulls price_suggestions + stats per item, mentally subtracts fee, guesses what moves; repricing is a manual dashboard crawl. Weekly: decide what to list/sell, reprice stale. Frustration: ranking items by net-after-fee AND liquidity one at a time, no view of over/under market.

**Persona C — "Priya," the collection cataloger/valuer.** 2,000+ records in folders with a custom "paid" field. Today: reads the single min/median/max value number; no memory, no per-record breakdown. Weekly/monthly: check value + cost basis. Frustration: one number, zero history — can't chart trend or attribute change to records.

**Persona D — "Theo," the crate-digger/DJ.** Buys physical records in shops, phone in hand. Today: types catno/scans barcode → scrolls versions → separately checks ownership + value. Weekly: identify a record in-hand, buy/skip on the spot. Frustration: three lookups (identify → own it? → is it a deal?) under bad connectivity, needs one fast answer.

## Survivors (transcendence set — 7, all hand-code)

| # | Feature | Command | Score | How it works |
|---|---------|---------|-------|--------------|
| 1 | Wantlist limit-order fills (FLAGSHIP) | `fills` | 10 | Join `wantlist_items.max_price` (local) vs latest marketplace `stats` lowest_price; diff prior `price_snapshots` row to surface newly-filled limits; reads cached snapshots to stay under 60/min. |
| 2 | Portfolio value + history | `portfolio` | 10 | Sum current collection value from latest `price_snapshots`, diff vs earlier snapshot date, cost basis from a collection custom field. |
| 3 | Undervalued detection | `undervalued` | 9 | Compare live `price_suggestions`/`stats` vs trailing median from `price_snapshots`. |
| 4 | Condition-matched comps | `comps` | 9 | `price_suggestions` per condition + `price_snapshots` history for one release. |
| 5 | Fee-aware sell router | `sell-plan` | 9 | Join inventory/collection with `fee` + `price_suggestions` + `stats`, rank by net-after-fee × liquidity (num_for_sale + have/want). Absorbs the killed `liquidity` score as a ranking column. |
| 6 | Catno/barcode identity spine | `identify` | 9 | Resolve catno/barcode via `database search`, left-join local collection/wantlist + current `stats`. |
| 7 | Pressing value ranker | `pressings` | 8 | Expand `master_versions`, join each to `stats`/`price_suggestions`/snapshot median, rank by value+liquidity. |

## Killed candidates

| Feature | Kill reason | Closest sibling |
|---------|-------------|-----------------|
| liquidity | ranking input not a destination; folded into sell-plan + a column in fills/pressings | sell-plan |
| changed | generic diff already served by fills + portfolio on framework sync deltas | fills |
| gaps (discography completion) | months-long arc, not weekly; outside vision | identify |
| reprice | overlaps sell-plan + comps; not in vision, loses tie | sell-plan |
| dupes | occasional cleanup, narrow value | portfolio |
| gems | subjective/unverifiable; undervalued is the testable signal | undervalued |
| movers | strict subset of portfolio's per-record contribution view | portfolio |
