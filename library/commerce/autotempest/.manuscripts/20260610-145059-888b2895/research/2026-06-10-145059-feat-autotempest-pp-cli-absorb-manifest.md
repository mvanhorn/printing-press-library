# AutoTempest CLI Absorb Manifest

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Filtered search (make/model/year/price/mileage/zip/radius/body/drive/fuel/trans/color/title/seller) | AutoTempest website | (behavior in autotempest-pp-cli find) | offline store, --json, --select, --csv |
| 2 | Multi-source aggregation across 10 sources | AutoTempest website | (behavior in autotempest-pp-cli find) parallel fan-out per sitecode | dedupe, agent-native |
| 3 | Sort (best_match/price/year/mileage/distance/date) | AutoTempest website | (behavior in autotempest-pp-cli search --sort) | typed |
| 4 | Per-listing fields (vin/price/mileage/year/dealer/source/title status/distance/url/img) | AutoTempest website | (behavior in autotempest-pp-cli find) | typed JSON, SQLite-persisted |
| 5 | Pagination (more results) | AutoTempest website (searchAfter) | (behavior in autotempest-pp-cli search --limit / sync --max-pages) | cursor handled |
| 6 | Makes reference list | website /api/get-makes | (generated endpoint) makes | offline cache |
| 7 | Models-for-make reference | website /api/get-models/{make} | (generated endpoint) models | offline cache |
| 8 | Scrape listings to spreadsheet | Coder-Boiiiiiii/AutoTempest-Scraper (Selenium→Excel) | autotempest-pp-cli search --csv | no browser, all filters/sources, pipeable |
| 9 | Hosted search scraper | Apify ecomscrape actor | (behavior in autotempest-pp-cli find) | local, free, no account/credits |
| 10 | Per-listing price history | AutoTempest website (priceHistory field) | (behavior in autotempest-pp-cli find) + local snapshots | persisted + diffable |
| 11 | Source registry (which marketplaces) | website window.sitestosearch | (generated/behavior) sources | offline list of 10 sources |

Disposition prefixes are Phase-3-verifiable. Search/listings are hand-built (custom SHA-256 token signing + multi-source fan-out); makes/models/sources are generator-emitted or thin.

## Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------------------------|------------------|
| 1 | Price-drop watch | drops | hand-code | Cross-snapshot price delta in local SQLite; site offers no anon saved-search diff | Use to find listings that dropped in price since a prior sync. Do NOT use for new/removed listings; use 'diff' for that. Reads local snapshots; run 'watch run' or 'sync' first. |
| 2 | Cross-source VIN dedupe | dedupe | hand-code | GROUP BY vin across sitecode; same physical car on multiple marketplaces collapsed cheapest-first | Use to collapse the same VIN across marketplaces into one row with per-source prices. For per-marketplace price ranges use 'spread'. |
| 3 | Deal-vs-market score | deal | hand-code | Mechanical median-delta over comparable local rows (model+year+mileage band); transcends opaque per-listing dealGauge | Use for a mechanical median-delta ranking within comparable cars. deal_score is computed, not an opinion. For per-source ranges use 'spread'; for same VIN across sources use 'dedupe'. |
| 4 | Cross-source price spread | spread | hand-code | Per-source min/median/max aggregation over local SQLite; no site equivalent | Use to compare price distributions across marketplaces for one model. For a single listing's standing use 'deal'; to collapse VINs use 'dedupe'. |
| 5 | Saved-search registry + replay | watch | hand-code | Persisted search entity + sync replay; enables drops/diff for anonymous users | Use to register and re-run named searches that feed 'drops'. 'watch run' re-syncs all saved searches; for a one-off query use 'search'. |
| 6 | Live auction slice | auctions | spec-emits | Filter on auction saletype exposing native currentBid/bids over local store | none |

Hand-code transcendence rows: 5 (drops, dedupe, deal, spread, watch). spec-emits: 1 (auctions).
No stubs.
