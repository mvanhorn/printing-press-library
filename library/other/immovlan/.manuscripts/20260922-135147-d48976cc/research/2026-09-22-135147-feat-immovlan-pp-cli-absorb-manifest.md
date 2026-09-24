# Immovlan CLI — Absorb Manifest (2026-09-22)

Sources surveyed: immovlan.be itself (search sidebar, advanced filters, detail page, JSON-LD), four Apify actors (solidcode/immovlan-be-scraper, studio-amba/immovlan-scraper, lexis-solutions/immovlan, stealth_mode/immovlan-property-search-scraper), the sibling immoweb-pp-cli. No npm/PyPI SDK, no MCP server, no GitHub CLI exists for Immovlan.

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search by transaction, property type, postcodes/towns | site, Apify actors | immovlan-pp-cli find | Postcodes or commune names, multi-town, offline store of every result |
| 2 | EPC band filter with letter mapping (F/G → bad, D/E → poor, B/C → good, A → excellent) | site | (behavior in immovlan-pp-cli find --epc) | Accepts letters like immoweb-pp-cli; letter per card from the watermark |
| 3 | Price and bedroom ranges | site, Apify | (behavior in immovlan-pp-cli find --min-price/--max-price/--min-bedrooms/--max-bedrooms) | Same flag names as immoweb-pp-cli |
| 4 | Sort by price, bedrooms, surface, postcode; newest by default | site | (behavior in immovlan-pp-cli find --sort) | `newest` alias for the default order |
| 5 | Pagination / all pages | site, Apify | (behavior in immovlan-pp-cli find --pages/--limit) | Page count from pagination, stable `total_matching` estimate |
| 6 | Listing detail: price, surface, PEB letter, year, condition, cadastral income, rooms, agency/private, phone, geo, datePosted, rented flag | site, Apify (32+ fields) | immovlan-pp-cli show | JSON-LD + meta + feature table merged into immoweb-pp-cli field names |
| 7 | Photo URLs and download | Apify | immovlan-pp-cli photos | `--list`, `--dir`, host-pinned to api-image.immovlan.be |
| 8 | Notary public sales feed | site (transactiontypes=en-vente-publique) | (behavior in immovlan-pp-cli find --deal public-sale) | Second notary source next to Biddit for the sourcing job |
| 9 | Rentals and colocation | site | (behavior in immovlan-pp-cli find --deal rent / colocation) | — |
| 10 | Agency / seller contact | site, Apify | (behavior in immovlan-pp-cli show) | Seller type (agency/private) from dataLayer, phone from card |
| 11 | JSON / CSV / GeoJSON export | Apify | immovlan-pp-cli dump | Same columns as immoweb-pp-cli dump; csv formula-safe |
| 12 | Postcode ↔ town slug resolution | site autocomplete | immovlan-pp-cli locations | Offline table of the 19 Brussels communes + stored localities; `municipals=` needs no slug |
| 13 | Saved searches + alerts | site (login) | immovlan-pp-cli saved / watch | No account; new / cheaper / gone diff in the local store |
| 14 | Favourites | site (login) | immovlan-pp-cli shortlist | Local, with notes |
| 15 | Price history / price cuts | immoweb-pp-cli parity | immovlan-pp-cli drops | Price observations per run |
| 16 | Days on market | immoweb-pp-cli parity | (behavior in immovlan-pp-cli show / find) | From JSON-LD datePosted |
| 17 | Hide listings | immoweb-pp-cli parity | immovlan-pp-cli hide | — |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | Buildability | Why Only We Can Do This | Persona | Long Description |
|---|---------|---------|-------|--------------|-------------------------|---------|------------------|
| 1 | Cross-portal match | same-as <ref\|--all> [--unmatched] [--price-gap] | 9/10 | hand-code | Joins the local store with immoweb-pp-cli's SQLite (address, photo hash, price+surface+agency) — no portal knows the other | Samuel, Arnaud | Use this command to find the Immoweb counterpart of a stored Immovlan listing, or with --all --unmatched to list Immovlan-only listings. It reads immoweb-pp-cli's local store only and never fetches Immoweb. Run `enrich` first so street addresses and photos are present; use `show` to read one listing's own fields. |
| 2 | Re-listing detector | relisted [--since 90d] [--postcode …] | 7/10 | hand-code | Links a "Nouveau" listing to its earlier reference by address/photos; true days on market across refs | Arnaud, Karim | Use this command to see listings that came back under a new reference. Do NOT use it for price cuts on the same reference; use `drops`. `watch` reports new and gone listings per saved search but does not link a gone listing to its re-listing. |
| 3 | PEB trap | peb-trap [--postcode …] [--max-price …] | 9/10 | hand-code | F/G letter (detail meta) AND currently rented (feature table): owners who can no longer re-let since 2026 | Arnaud | Use this command to list PEB F/G properties that are currently rented. It needs the detail letter and rented flag, so run `enrich --missing epc,rented` first. `find --epc F` only returns the search-card band (bad) without the rented flag. |
| 4 | Bulk detail enrichment | enrich [--top N] [--missing epc,geo,rented,cadastral,software] [--rate 1/s] | 9/10 | hand-code | Resumable, only-missing detail fetches into the store; the primitive the other commands depend on | Samuel | Use this command to fill missing detail fields for many stored listings without printing them. To read one listing, use `show`. |
| 5 | Division candidates | split-candidates [--min-surface 200] [--min-bedrooms 4] [--postcode …] | 8/10 | hand-code | Surface / €/m² ranking the site cannot do (no surface filter on Immovlan) | Arnaud | none |
| 6 | Agency leaderboard | agencies [--postcode …] [--epc F,G] | 6/10 | hand-code | Group-by on the store + the CRM `software` field from dataLayer (syndicated feed vs Rossel-exclusive) | Arnaud, Samuel | none |

Hand-code count: 6 of 6 transcendence rows (plus the promoted find/show/photos/locations/saved/watch/shortlist/hide/drops/dump commands, which mirror immoweb-pp-cli and parse HTML by hand). No stubs.
