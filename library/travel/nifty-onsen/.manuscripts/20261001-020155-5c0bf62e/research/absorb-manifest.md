# Approved Nifty Onsen build manifest

Approved by the user's explicit autonomous build instruction; no stubs. Only public read-only discovery; source remains Nifty Onsen. User limits delegation to exactly one fresh-context reviewer, so ideation is performed by the sole builder.

## Absorbed source website/app workflows
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Keyword/region/filter search | Nifty public search | nifty-onsen-pp-cli bath search | Bounded organic listings, IDs, source URLs, explicit pagination |
| 2 | Coordinate nearby discovery | Nifty map query | nifty-onsen-pp-cli bath nearby | Sorted distances with explicit bounded source coverage |
| 3 | Facility admission/hours/access | Nifty SSR detail | nifty-onsen-pp-cli bath show | Source text preserved, unknown policies explicit |
| 4 | Ratings | Nifty search/details | (behavior in nifty-onsen-pp-cli bath show) source rating and review count | Source attribution, no review corpus |
| 5 | Public coupon information | Nifty coupon page | nifty-onsen-pp-cli bath coupons | Validity, membership/app restrictions, price/conditions preserved; no issuance |
| 6 | Geographic browsing | Nifty source links | nifty-onsen-pp-cli regions | Japanese names and reusable source slugs |
| 7 | Discoverable filter catalog | Nifty source controls | nifty-onsen-pp-cli filters | Japanese label and verified source parameter |

## Agent capabilities
| # | Feature | Command | Buildability | Why useful | Long Description |
|---|---|---|---|---|---|
| 1 | Unknown-safe bath identity | bath show | hand-code | Keep natural onsen/ordinary bath/hotel and room vs rentable bath evidence separate | none |
| 2 | Conditional admission/coupon text | bath coupons | hand-code | Avoid presenting a subscription, pair, or holiday condition as a universal discount | none |
| 3 | Bounded nearby distance ranking | bath nearby | hand-code | A finite source window with computed distances, no inventory guarantees | none |
| 4 | Small structured field projection | bath search | hand-code | Agents can select only shortlist facts while metadata remains intact | none |
| 5 | Parsed freshness cache | bath show | hand-code | Reuse exact source facts with timestamp, offline staleness explicit, no credential storage | none |

## Acceptance
All commands functional; no account writes/booking/coupon issuance. Live searches across regions and source filters, show source ID/canonical consistency for spa/sento/hotel, private/family bath evidence, public coupon conditions. Measure cached and fresh command bytes, token estimate, requests, latency and memory. One independent fresh-context review, builder fixes, fresh tests and live matrix, native Press shipcheck and live acceptance, local promotion only.
