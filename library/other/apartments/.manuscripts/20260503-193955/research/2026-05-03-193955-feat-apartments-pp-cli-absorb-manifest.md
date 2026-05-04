# Apartments.com CLI Absorb Manifest

## Source landscape

| Tool | Stack | Status | Features it informs |
|------|-------|--------|---------------------|
| [adinutzyc21/apartments-scraper](https://github.com/adinutzyc21/apartments-scraper) | Python + Selenium | Broken since 2021 (bot-blocked) | 21 listing fields, Google Maps distance integration |
| [johnludwigm/PyApartments](https://github.com/johnludwigm/PyApartments) | Python | Stale | Property tracking, SQLite storage idea |
| [shilongdai/Apartment_Scraper](https://github.com/shilongdai/Apartment_Scraper) | Python + Selenium + bs4 | Stale | JSON + CSV output |
| [davidhuang620/Apartments.com-web-Scrapping](https://github.com/davidhuang620/Apartments.com-web-Scrapping) | Python | Stale | Bulk listing extraction |
| [cccdenhart/apartments-scraper](https://github.com/cccdenhart/apartments-scraper) | Python + R | Stale | schema.org microdata selectors, URL builder |
| Apify "Apartments.com Scraper" (parseforge) | Hosted SaaS | Deprecated | Field schema reference |
| Apify "Apartments.com Property Data Extractor" (epctex) | Hosted SaaS, paid | Active (paid) | Field schema reference |
| Apify "Apartments Search Scraper" (powerai) | Hosted SaaS, paid | Active (paid) | Search filters reference |
| MCP servers | — | None exist | — |
| Claude skills / plugins | — | None exist | — |

**No tool in the public landscape produces a free, agent-native, working CLI.** Every existing scraper is broken or paid SaaS. This is the differentiator.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Search by city + state | apartments.com path `/{city}-{state}/`; cccdenhart Site.py URL builder | `search --city austin --state TX` builds slug via Surf | Path validated, `--json` + `--select` for agents, offline-cached |
| 2 | Search by zip code | apartments.com path `/{zip}/` | `search --zip 78704` | Numeric coercion + sanity check |
| 3 | Filter: studio | path filter `studio` | `--beds 0` or `--studio` | Mapped to slug fragment |
| 4 | Filter: exact beds | path filter `1-bedrooms`, `2-bedrooms` | `--beds N` | Validated 0–4+ |
| 5 | Filter: min beds | path filter `min-N-bedrooms` | `--beds-min N` | |
| 6 | Filter: exact baths | path filter `1-bathrooms` | `--baths N` | |
| 7 | Filter: min baths | path filter `min-N-bathrooms` | `--baths-min N` | |
| 8 | Filter: price max | path filter `under-PRICE` | `--price-max 2500` | |
| 9 | Filter: price range | path filter `MIN-to-MAX` | `--price-min 1500 --price-max 2500` | |
| 10 | Filter: pet | path filter `pet-friendly`, `pet-friendly-dog`, `pet-friendly-cat-or-dog` | `--pets dog`, `--pets cat`, `--pets both` | Human-friendly enum |
| 11 | Filter: property type | path prefix `/houses/`, `/condos/`, `/townhomes/` | `--type house\|condo\|townhome\|apartment` | Default apartments |
| 12 | Pagination | path suffix `/{N}/` | `search --page N` and `--all` | Auto-paginate option |
| 13 | Listing detail extraction | apartments.com listing page; cccdenhart Page.py schema.org parsing | `get <url-or-id>` | Stable `meta[itemprop=...]` + `data-*` selectors |
| 14 | Address fields | schema.org microdata (streetAddress, addressLocality, addressRegion, postalCode) | Native struct fields | Validated USPS-style |
| 15 | Beds / baths / max-rent | placard `data-beds` / `data-baths` / `data-maxrent` | Native typed fields | JSON-typed |
| 16 | Available date | listing availability table | Parsed to ISO date | Normalized |
| 17 | Pet policy details | listing page section | `pet_policy` struct: cats/dogs allowed, monthly fees, deposits, weight limits | Powers `value` ranking |
| 18 | Lease terms | listing page section | `lease_info` struct: min/max months | |
| 19 | Amenities list | listing page bullets | `[]string` field | FTS5-indexed for `must-have` |
| 20 | Photos | listing page gallery URLs | `[]string` field, `--no-photos` flag | |
| 21 | Contact phone | listing page contact section | E.164 normalized | |
| 22 | Floor plans | floor-plan section | `floor_plans[]` child rows: rent / sqft / beds / availability | Powers `floorplans` rank |
| 23 | CSV export | adinutzyc21 + cccdenhart pandas DataFrame | `--csv` global flag | Built-in everywhere |
| 24 | JSON export | shilongdai/Apartment_Scraper | `--json` (default for agents) | Native everywhere |
| 25 | Sync to local store | (none of the scrapers) | `sync <saved-search>` writes to SQLite | Foundation for transcendence |
| 26 | Search local store | (none) | `search-local "term"` over FTS5 | Offline + regex |
| 27 | SQL access | (none) | `sql "SELECT ..."` SELECT-only | Power-user composition |
| 28 | Deduplication | (none — every Python scraper re-emits dupes) | Listing URL is the natural key | Survives re-syncs |

Every absorbed row is mandatory shipping scope. No stubs.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Persona | Why Only We Can Do This |
|---|---------|---------|-------|---------|------------------------|
| 1 | Watch saved searches with diff | `watch <saved-search>` | 10/10 | Priya, Maya | Surf-fetched snapshots in SQLite, diffs latest vs previous on listing URL; apartments.com only emails new listings, not removals or price drops |
| 2 | Multi-slug union ranking | `nearby <slug1> <slug2> ...` | 9/10 | Maya, Jordan | apartments.com URL is single-slug per query; `cliutil.FanoutRun` unions N searches, dedupes by listing URL |
| 3 | Total-cost-of-occupancy ranking | `value --budget N --pet dog --months 12` | 8/10 | Jordan | Local SQL: `(maxrent * months) + (pet_rent * months) + pet_deposit + pet_fee` — site has no total-cost sort |
| 4 | $/sqft and $/bed ranking | `rank --by sqft\|bed` | 8/10 | Jordan | Local SQL ratio compute over sqft/beds; site sort options omit ratio metrics |
| 5 | Side-by-side compare | `compare <url-or-id> ...` | 7/10 | Maya, Jordan | 2–8 listings pivoted into wide-table columns with $/sqft and amenity-overlap; site has no compare view |
| 6 | Price-drop alerts | `drops [--since N] [--min-pct N]` | 8/10 | Priya, Jordan, Maya | Snapshot history table: latest `maxrent` vs earliest within window, ≥N% threshold |
| 7 | Stale-listing flag | `stale [--days N]` | 8/10 | Priya, Jordan | Snapshot history: `(maxrent, available_date)` unchanged for ≥N days — signals phantom or stuck unit |
| 8 | Phantom-listing detector | `phantoms` | 7/10 | Priya, Maya | Three-signal join: `last_fetch_status = 404` ∪ `not_in_latest_sync_for(saved_search)` ∪ `unchanged_for_days >= 45` |
| 9 | Neighborhood market summary | `market <city-state>` | 7/10 | Maya, Sam | Aggregation SQL: median, p10, p90 of rent and rent/sqft, pet-friendly share, by `(addressLocality, addressRegion, beds)` |
| 10 | Listing history | `history <url-or-id>` | 6/10 | Priya, Jordan | Time-series read from snapshot table for one listing |
| 11 | Weekly digest | `digest --saved-search S [--since 7d]` | 8/10 | Priya, Maya | Composes #1, #4, #6, #7, #8 into one structured digest output for Priya's Monday email ritual |
| 12 | Floor-plan-level value report | `floorplans --rank price-per-sqft` | 6/10 | Jordan, Maya | Joins listings to `floor_plans` child rows, ranks per-plan rent/sqft |
| 13 | Amenity must-have intersect | `must-have "in-unit washer" "parking"` | 5/10 | Jordan | FTS5 AND-join over amenities array; site filter is fixed curated list |
| 14 | Local shortlist | `shortlist add\|show\|remove` | 5/10 | Maya, Jordan | Tag-based local shortlist table, joins to listings; feeds `compare` and `digest --shortlist` |

All 14 transcendence features are mandatory shipping scope. No stubs.

## Reachability + transport

- Runtime: **Surf with Chrome TLS fingerprint** (`mode: browser_http` per `printing-press probe-reachability`).
- Auth: **none for v1.** All listed features work anonymously. Saved-search login (cookie session) is explicitly out of scope and will be flagged as a `## Known Gaps` item only if the user later asks for it.
- Surface: **HTML/SSR with schema.org microdata.** No XHR JSON endpoints relied upon. The internal CLI uses `response_format: html` extractors.

## Stubs

None. Every listed feature is shipping scope.

## Source provenance for README credits

Tools listed in the source-landscape table will be credited in `research.json` `alternatives[]` and rendered as the README's "Why this CLI" comparison block.
