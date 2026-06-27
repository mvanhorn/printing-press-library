# Flipp Absorb Manifest

## Absorbed

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search item deals by keyword and ZIP | thomas-chong/flipp-cli search | flipp-pp-cli items milk --zip 85001 | Generated typed command, JSON/select/csv, MCP, local syncable surface |
| 2 | Browse active local flyers | thomas-chong/flipp-cli flyers | flipp-pp-cli flyers list --zip 85001 | Local cache and SQL over flyer validity windows |
| 3 | List Flipp merchants | thomas-chong/flipp-cli merchants | flipp-pp-cli merchants --zip 85001 | Agent-readable merchant inventory by ZIP |
| 4 | Detect ZIP/postal code by IP | thomas-chong/flipp-cli locate | flipp-pp-cli location | No-auth bootstrap for local defaults |
| 5 | Fetch combined flyer/coupon data | thomas-chong/flipp-cli getCouponData | flipp-pp-cli flyers data --zip 85001 | Exposes coupons and flyers in one cacheable payload |
| 6 | Extract item clippings from a flyer | Kiizon/flippscrape get_flyer_items | flipp-pp-cli flyers items 8005907 | Structured flyer item rows with clipping image URLs |
| 7 | Rank top discounts across grocery terms | thomas-chong/flipp-cli deals | flipp-pp-cli deals scan --category groceries --zip 85001 | Adds partial-failure accounting and bounded fan-out |
| 8 | Compare unit prices from item names | thomas-chong/flipp-cli unit-price | flipp-pp-cli unit-price milk --zip 85001 | Adds local-store fallback and warnings for compound listings |
| 9 | Bulk search multiple shopping-list terms | thomas-chong/flipp-cli search multi-query | flipp-pp-cli basket price --items milk,eggs,bread --zip 85001 | Turns separate searches into basket totals by merchant |
| 10 | Include flyer clipping images for vision fallback | thomas-chong/flipp-cli --with-images | (behavior in flipp-pp-cli flyers items 8005907) include image URLs in output | Agents can inspect original flyer image when text is ambiguous |

## Transcendence

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Basket price comparison | basket price --items milk,eggs,bread --zip 85001 | hand-code | Requires fan-out search, merchant grouping, partial failure accounting, and local deal normalization | Use this command to compare a grocery list across nearby merchants. Do NOT use it for one-off item search; use items instead. |
| 2 | Deal scan by category pack | deals scan --category groceries --zip 85001 | hand-code | Requires curated query packs, bounded scan effort, deduplication, and discount ranking | Use this command to discover deals across a staple category. Do NOT use it when the user names exact items; use basket price instead. |
| 3 | Expiring-soon savings | expiring --days 3 --zip 85001 | hand-code | Requires local snapshots and flyer validity windows across items, flyers, and coupons | Use this command to prioritize deals about to leave local flyers. |
| 4 | Merchant coverage map | coverage --zip 85001 | hand-code | Requires joining merchants, flyers, item hits, and coupon presence in the local mirror | Use this command to see which local merchants have useful food savings coverage. |
| 5 | Unit-price normalized search | unit-price milk --zip 85001 | hand-code | Requires parsing item sizes from names and retaining warnings when listings are compound or ambiguous | Use this command when price alone is misleading because package sizes differ. |
| 6 | Shopping-list watch snapshot | watchlist add milk --target-price 3.50 --zip 85001 | hand-code | Requires persistent local watch criteria and comparison against future synced snapshots | Use this command to track recurring staple targets across flyer refreshes. |

## Stub List
- None approved. All transcendence rows above are shipping scope.
