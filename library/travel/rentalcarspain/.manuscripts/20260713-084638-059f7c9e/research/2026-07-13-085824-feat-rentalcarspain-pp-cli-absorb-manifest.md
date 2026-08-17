# Rental Car Spain — Absorb Manifest

## Landscape
No consumer car-rental price-search CLI exists (GitHub/npm/PyPI all fleet-management toys). Feature vocabulary absorbed from: DoYouSpain results page, DiscoverCars filter set, Kayak Cars, Booking.com Demand cars, AutoSlash price tracking, and the printing-press hotel-goat/flight-goat command pattern.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Search cheapest suppliers in an area | DoYouSpain results | `rentalcarspain-pp-cli search <loc> <pickup> <dropoff>` | Offline SQLite snapshot, --json, agent-native, full-insurance default |
| 2 | Resolve a place name to a location code | DoYouSpain autocomplete | `rentalcarspain-pp-cli locations <query>` | Cached codes, --json; maps "Malaga airport" → MAL02 |
| 3 | Filter by supplier | Kayak/DiscoverCars supplier filter | `(behavior in rentalcarspain-pp-cli search --supplier delpaso,recordgo,sixt)` | Presets for the user's 3 companies; isolates Record Go/Wiber from aggregator |
| 4 | Filter by car class/category | DiscoverCars class filter | `(behavior in rentalcarspain-pp-cli search --class economy,compact,suv)` | Local filter over parsed groups |
| 5 | Max total / max per-day price cap | Kayak price filter | `(behavior in rentalcarspain-pp-cli search --max-total N --max-per-day N)` | Local numeric filter, cron-friendly |
| 6 | Per-day AND total price | Kayak cars | `(behavior in rentalcarspain-pp-cli search)` output shows both | Always both, dual currency where present |
| 7 | Full-insurance / zero-excess pricing | DiscoverCars Full Coverage, DoYouSpain Full Insurance | `(behavior in rentalcarspain-pp-cli search)` full-insurance default, `--base` for bare rate | Default = how the user actually books |
| 8 | Deposit / excess amount shown | DiscoverCars deposit+excess | `(behavior in rentalcarspain-pp-cli search)` surfaces deposit/excess when present | Parsed from offer/detail HTML |
| 9 | Sort cheapest | Kayak/DoYouSpain sort | `(behavior in rentalcarspain-pp-cli search --sort cheapest|per-day|total)` | Default cheapest-total |
| 10 | Direct supplier quote (Delpaso) | Delpaso /offers | `rentalcarspain-pp-cli delpaso <pickup> <dropoff>` | Live cross-check of the aggregator price, full coverage |
| 11 | Driver-age pricing | Every aggregator API (driverAge first-class) | `(behavior in rentalcarspain-pp-cli search --driver-age N)` | Default 35; surfaces young/senior sensitivity |
| 12 | Currency normalization | hotel-goat FX | `(behavior in rentalcarspain-pp-cli search --currency EUR|GBP)` | DoYouSpain ships EUR+GBP inline; label source |
| 13 | Health check | flight-goat/hotel-goat doctor | `rentalcarspain-pp-cli doctor` | Verifies both sources reachable + WAF-UA correctness |
| 14 | Show one offer's details/T&C | DoYouSpain detail page | `rentalcarspain-pp-cli show <token>` | Fetch `/do/detail/en` + conditions, parse coverage terms |

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Aggregator-vs-direct cross-check | `compare <pickup> <dropoff>` | hand-code | The user's exact ritual in one call: DoYouSpain cheapest supplier vs Delpaso's own site, side by side with the delta. No site or API does this. | Use this to confirm the aggregator's Delpaso price against Delpaso direct. Do NOT use for multi-supplier browsing; use 'search'. |
| 2 | Price drift over time | `drift <saved-search>` | hand-code | Requires local snapshots of the same search across days — DoYouSpain shows only "now". | Use this to see whether a Málaga rental is trending up or down before booking. Requires prior snapshots from 'search'/'watch'. |
| 3 | Price-drop watch with target | `watch <saved-search> --target-price N` | hand-code | Cron-friendly re-quote loop with typed exit codes (0 hit / 10 still above), replicating AutoSlash for Spain. | Use in a cron job to alert when a tracked Málaga rental drops to your target. Do NOT use for one-off quotes; use 'search'. |
| 4 | Cheapest-pickup-date sweep | `dates <loc> --from <d> --to <d> --nights N` | hand-code | Fans out N searches and ranks by total — no single DoYouSpain call spans dates. | Use to find the cheapest pickup date in a window for a fixed rental length. |
| 5 | Save/label a recurring search | `saved add\|list\|remove` | hand-code | Local SQLite named searches power watch/drift; the site is stateless per query. | Use to name a Málaga search you re-run; feeds 'watch' and 'drift'. |
| 6 | Supplier price summary | `suppliers <pickup> <dropoff>` | hand-code | Local aggregation across a result set: cheapest offer per supplier, ranked — including the user's 3 companies. | Use to see one line per supplier (Delpaso, Record Go, Wiber, Sixt…) with their cheapest full-insurance offer. |

## Stubs
None. Record Go and Wiber are covered as suppliers within `search`/`suppliers`, not as stub commands (user-approved).

## Novel-feature hand-code count
6 transcendence rows, all `hand-code`. Absorbed rows 1, 2, 10, 13, 14 are hand-built source clients (no generator endpoint mirror — both sources are HTML-scrape); rows 3–9, 11, 12 are flags/behaviors inside `search`.
