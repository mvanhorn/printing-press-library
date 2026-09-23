# Zimmo CLI — Absorb Manifest (2026-09-23)

Sources surveyed: zimmo.be itself (JS bundle: 130 service methods; public ones replayed live), Apify actors solidcode/zimmo-scraper, studio-amba/zimmo-scraper, stealth_mode/zimmo-property-search-scraper, dz_omar/zimmo-scraper, studio-amba/property-listing-lifecycle-tracker, GitHub Joffreybvn/bot-scrape-zimmo, siblings immoweb-pp-cli / immovlan-pp-cli. No npm/PyPI SDK, no official MCP, no Go/Node CLI for Zimmo.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search by status, type, commune/postcode, price, bedrooms, surface, EPC, text, sort | site, Apify actors | zimmo-pp-cli find | Commune names or postcodes resolved to place ids; all pages into the local store; same flag names as immoweb/immovlan CLIs |
| 2 | Paste a zimmo.be search URL (base64 `search=` param) | dz_omar actor | (behavior in zimmo-pp-cli find --url) | Decodes the site's filter JSON, so any UI search becomes a CLI search |
| 3 | Full listing detail: exact address, GPS, rooms, EPC letter + kWh, flood zone, planning, documents, dealer, price history | dz_omar actor | zimmo-pp-cli show | By zimmo code or URL; flattened field names shared with sibling CLIs |
| 4 | Photo URLs / download | Apify actors | zimmo-pp-cli photos | --list / --dir, host-pinned to files.zimmo.be |
| 5 | JSON / CSV / GeoJSON export | Apify actors | zimmo-pp-cli dump | Same columns as immoweb/immovlan dump, formula-safe CSV |
| 6 | Agency info + review score + stats | solidcode actor | zimmo-pp-cli agency | Listings count, for sale / to rent, reviews |
| 7 | Postcode / commune → place id | site | zimmo-pp-cli places | Offline cache of resolved places |
| 8 | Price indicator €/m² per commune with monthly history | site (indicateur-de-prix) | zimmo-pp-cli prices | By type, history as table/JSON |
| 9 | Saved searches + alerts (new / gone) | site (login) | zimmo-pp-cli watch | No account; diff against the local store |
| 10 | Favourites with notes | site (login) | zimmo-pp-cli shortlist | Local, with notes |
| 11 | Price-drop tracking | studio-amba lifecycle tracker | zimmo-pp-cli drops | Observations per sync + Zimmo's own priceHistory |
| 12 | Rentals | site | (behavior in zimmo-pp-cli find --deal rent) | — |
| 13 | Sold / rented listings | site | (behavior in zimmo-pp-cli find --status sold) | Historical comps no other portal exposes |
| 14 | Geocode an address | site | (generated endpoint) places geocode | — |
| 15 | Property count | site | (generated endpoint) listings property-count | — |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Persona | Long Description |
|---|---------|---------|-------|--------------|-------------------------|---------|------------------|
| 1 | Bulk detail enrichment | enrich [--top N] [--missing epc,flood,planning,history,gps] [--rate 1/s] | 9/10 | hand-code | Resumable, only-missing detail fetches into the store; primitive for peb-trap/motivated | the tool builder | Use this command to fill detail fields for many stored listings without printing them. To read one listing, use 'show'. |
| 2 | Cross-portal match | same-as <zimmoCode\|--all> [--unmatched] [--price-gap] [--export-address] | 9/10 | hand-code | Joins with immoweb-pp-cli / immovlan-pp-cli SQLite stores read-only; Zimmo's exact address becomes the reference | the tool builder, the investor client | Use this command to find the Immoweb/Immovlan counterpart of a Zimmo listing, or with --all --unmatched to list Zimmo-only listings. It reads the sibling CLIs' local stores only and never fetches those portals. |
| 3 | Sold comps | comps <zimmoCode\|address> [--radius 800m] [--months 24] [--status sold,rented] | 9/10 | hand-code | SOLD/RENTED search around rooftop GPS + €/m² stats vs subject | the investor client, Karim | Use this command for comparable SOLD/RENTED listings around one property. Do NOT use this command to rank many stored listings against commune prices; use 'underpriced' instead. |
| 4 | PEB trap | peb-trap [--postcode …] [--min-kwh 400] [--max-price …] | 9/10 | hand-code | Numeric EPC + renovation obligation (Zimmo-only fields) + take-over/rented signal | the investor client | Use this command to list energy-poor listings (EPC F/G or high kWh/m²) with a renovation obligation. It needs detail fields, so run 'enrich' first. `find --epc F,G` only filters by label. |
| 5 | Underpriced vs commune | underpriced [--postcode …] [--type house\|apartment] [--below 15%] | 8/10 | hand-code | Store × score-api €/m² per commune/sub-locality and type | the investor client | Use this command to rank stored for-sale listings by discount to their commune's €/m². Do NOT use this command for one property's sold comparables; use 'comps' instead. |
| 6 | Gross rental yield | yield <zimmoCode\|--all> [--postcode …] [--min 5%] | 7/10 | hand-code | Median rent €/m² of RENTED/TO_RENT comps × sale surface vs asking price | Léa, the investor client | Use this command to rank sale listings by estimated gross rental yield. Do NOT use this command to list the rental comparables themselves; use 'comps --status rented' instead. |
| 7 | Motivated seller score | motivated [--postcode …] [--min-days 120] | 7/10 | hand-code | Days on market + full priceHistory cut + re-listing at the same exact address + EPC | the investor client | Use this command to rank stored listings by seller-pressure signals (age, total cut, re-listing, EPC). Do NOT use it for price cuts observed between your own syncs; use 'drops' instead. |

Hand-code count: 7 of 7 transcendence rows (plus the promoted find/show/photos/dump/agency/places/prices/watch/shortlist/drops commands). No stubs.
