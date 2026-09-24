# Immovlan CLI Brief

## API Identity
- Domain: immovlan.be — Belgium's third real-estate portal (Rossel group; ~100 000 listings; strong in Brussels and Wallonia, print cross-visibility via Le Soir / Sudinfo supplements). Ranked #5 by traffic in BE real estate (Immoweb #1, Zimmo #2).
- Users: buyers, renters, investors, and property traders who want listings Immoweb does not carry (agencies exclusive to Rossel's network), and agencies publishing there.
- Data profile: server-rendered HTML search pages (`/fr/immobilier?transactiontypes=a-vendre&propertytypes=maison,appartement&towns=1030-schaerbeek&epcratings=bad,poor&minprice=…&maxprice=…&page=N`, 20 cards/page, `data-value-id=VBE…` reference per card) and detail pages (`/fr/detail/<type>/<transaction>/<postcode>/<slug>/<ref>`) with JSON-LD (`price`, `surface`), meta `cXenseParse:rbf-immovlan-*` (price, PEB as `BrusselsC`, garden, garage), structured feature table (surface habitable, terrain, année, état, chambres). No JSON search API: robots.txt disallows `*/api*` and `*SearchPartial*`; the XHR list endpoint is internal. Images on `api-image.immovlan.be`. No auth for search/detail. Cloudflare Turnstile is loaded only for forms (contact, login); plain GETs return 200 from a Mac and from a Linux VM.

## Reachability Risk
- Low. Direct HTTP `200` with full content from two networks (2026-09-22). Four Apify actors scrape it with plain HTTP today (solidcode, studio-amba, lexis-solutions, stealth_mode — the last advertises "fast, no browser"). Risk: HTML markup changes (v3 templates dated 2026); robots.txt is strict about internal endpoints, which the CLI must not use.
- Filters confirmed live: `epcratings` (bands unknown|bad|poor|good|excellent — not letters; letter is on the detail page), `minprice`, `maxprice`, `minbedrooms`, `maxbedrooms`, `towns=<postcode>-<slug>` (repeatable), `municipals=`, `propertytypes`, `transactiontypes`, `page`. Sort is a JS dropdown (param to confirm during capture).

## Top Workflows
1. Search a commune / postcode band for sale or rent with price, bedrooms and PEB band, newest first, and get a clean table or JSON (the #1 workflow: "find what is for sale in Schaerbeek under 2.5 M with a bad PEB").
2. Open one listing and read everything the site knows: price, surface, land, PEB letter, year, condition, rooms, agency vs private, publication date, photos.
3. Cross-portal coverage for a trader's daily sourcing: the same listing on Immoweb and Immovlan, or exclusive to one — dedupe by address/photos.
4. Watch a saved search and report new / cheaper / gone since last run (local store).
5. Export CSV/GeoJSON for a map or a spreadsheet.

## Table Stakes (from the Apify actors and immoweb-pp-cli)
- Every search filter as a flag; pagination; `--json/--csv`; detail with PEB letter, surface, price, contact, photos; agency vs private seller; price history from a local store; commune resolution from a name; agent envelope for MCP.

## Data Layer
- Primary entities: listing (ref VBE…, type, transaction, postcode, locality, street, price, surface, land, bedrooms, PEB letter + kWh, year, condition, agency, private flag, published/modified dates, photos, lat/lng when present), search (saved criteria), price observation.
- Sync cursor: none server-side; harvest by search pages, `first_seen/last_seen` locally.
- FTS/search: title + street + locality + description.

## User Vision
- Sibling of `immoweb-pp-cli` for a Brussels property trader's (marchand de biens) daily sourcing job `brussels-peb-hunter`: same field names where possible (`id`, `url`, `price`, `surface_m2`, `price_per_m2`, `epc`, `postal_code`, `street`, `private_seller`, `created_at`, `condition`) so the job can merge both portals; `--epc` accepting letters mapped to Immovlan bands (F/G → `bad`), `--postcode`, `--sort newest`, `--pages`, `--agent`.

## Product Thesis
- Name: immovlan-pp-cli
- Why it should exist: Immovlan carries listings that never reach Immoweb (Rossel-network agencies, Wallonia/Brussels private sellers); no CLI exists, the only tooling is paid Apify actors returning raw dumps. A local store adds days-on-market and price cuts the site does not show, and the field parity with immoweb-pp-cli makes cross-portal deduplication a one-liner.

## Build Priorities
1. `find` with every confirmed filter + newest sort + pagination, HTML card extraction, store upsert.
2. `show <ref|url>` detail extraction (JSON-LD + meta + feature table), PEB letter, dates.
3. Local store + `watch` (new/cheaper/gone) + `drops`.
4. `dump` CSV/GeoJSON and `locations` (postcode → town slug).
5. Cross-portal helper: `same-as` by normalized address to an immoweb id (novel).
