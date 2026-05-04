# Apartments.com CLI Brief

## API Identity
- Domain: US apartment rental listings; metropolitan & local-market search; one of the top three rental aggregators (alongside Zillow Rentals and Realtor.com Rentals).
- Owner: CoStar Group (publicly traded, ~$1B+ revenue from CoStar's residential portfolio that includes Apartments.com, ApartmentFinder, ApartmentHomeLiving, Doorsteps).
- Users: prospective renters comparing 5–50 candidate units; power users include relocators, students, agents, data-driven hunters.
- Data profile: HTML/SSR-rendered listing pages with rich `schema.org` Apartment microdata + `data-*` attributes; no public REST/GraphQL spec; URL surface is path-based (`/{location-slug}/{filter-segment}/{page}/`).

## Reachability Risk
- **Low.** `printing-press probe-reachability https://www.apartments.com` returned `mode: browser_http, confidence: 0.85`. Plain stdlib HTTP is 403 (Akamai-style bot rejection), but Surf with Chrome TLS fingerprint cleared 200 OK. The printed CLI ships Surf transport at runtime — no clearance cookie capture, no resident browser sidecar.
- All five existing Python scrapers in the public landscape are non-functional or stale (last updates 2018–2021). They were broken precisely by the protection that Surf now clears. This is not a "broken-API" risk row — it is the differentiator.

## Top Workflows
1. **Search rentals by city + filters** — pick a city, narrow by beds, price, pets, square footage, page through results.
2. **Inspect a specific listing** — fetch one listing, read rent ranges, address, beds/baths, square footage, amenities, pet policy, contact.
3. **Watch a saved search over time** — re-run the same query daily, surface NEW listings, removed listings, and price changes.
4. **Compare a shortlist** — pull 3–8 candidate listings, render a side-by-side table by $/bed, $/sqft, amenities, distance.
5. **Export to CSV / JSON for spreadsheet or downstream analysis** — agents and humans alike need the data flat.

## Table Stakes
- Search by city, state, zip, neighborhood (path-based slug).
- Beds filter (`studio`, `1-bedrooms`, `2-bedrooms`, …, `min-N-bedrooms`).
- Baths filter (`1-bathrooms`, `min-N-bathrooms`).
- Price range filter (`under-PRICE`, `over-PRICE`, `MIN-to-MAX`).
- Pet filter (`pet-friendly`, `pet-friendly-dog`, `pet-friendly-cat-or-dog`).
- Property-type prefixes (`/apartments/`, `/houses/`, `/condos/`, `/townhomes/`).
- Pagination.
- Listing detail: address, rent, beds, baths, sqft, available date, amenities, pet policy, lease terms, contact phone, photos.

## Codebase Intelligence
- No DeepWiki source: there is no canonical apartments.com SDK repo. Intelligence below is synthesized from five Python scraper repos that each successfully captured the surface before Akamai-style protection broke them.
- Selectors / extraction (cccdenhart/apartments-scraper Page.py — schema.org microdata, stable):
  - `meta[itemprop=streetAddress]`, `meta[itemprop=addressLocality]`, `meta[itemprop=addressRegion]`, `meta[itemprop=postalCode]` for address.
  - `data-beds`, `data-baths`, `data-maxrent` on listing markers.
  - Listing-card link: `a.placardTitle.js-placardTitle` (still valid as of community reports; site has not redesigned this card class).
  - Pagination: `a[data-page]`.
- URL construction (cccdenhart/Site.py + community URL examples):
  - `https://www.apartments.com/{city}-{state}/`
  - `…/{N}-bedrooms/`, `…/min-{N}-bedrooms/`, `…/{N}-bedrooms-{M}-bathrooms/`
  - `…/under-{PRICE}/`, `…/{MIN}-to-{MAX}/`
  - `…/pet-friendly/`, `…/pet-friendly-dog/`, `…/pet-friendly-cat-or-dog/`
  - Pagination: `…/{page}/` (1-indexed, omitted on page 1).
- Auth: none required for read-only search and listing inspection. Logged-in features (saved searches, applications) require cookie session — out of scope for v1.

## Source Priority
- Single-source CLI; no priority ordering needed.

## Product Thesis
- Name: `apartments-pp-cli` (binary), `apartments` (slug).
- Tagline: *"The apartment hunt CLI that actually works in 2026."*
- Why it should exist:
  1. Every existing apartments.com tool is broken. The five Python scrapers we found were all bot-blocked. Surf transport plus printing-press's HTML/microdata extraction makes this the first working agent-native tool for the site.
  2. SQLite local store unlocks the workflows the website itself doesn't offer: watch a saved search, diff over time, rank by $/sqft, compare apartments side-by-side, export to CSV for downstream agents.
  3. Agent-native by default. Every command emits `--json` and `--select` for narrow context budgets. MCP-ready for Claude Desktop / agent hosts.

## Build Priorities
1. **Foundation (P0)** — Surf-backed client, SQLite store keyed on listing URL/property-id, schema.org microdata extractor, location-slug builder.
2. **Absorb (P1)** — search (city + bed + bath + price + pet + property-type + page), get (single listing detail), list (paginate + filter), location autosuggest (best-effort from path-based responses), export (CSV / JSON).
3. **Transcend (P2)** —
   - `watch` saved searches: re-run, diff against last sync, surface new/removed/price-changed listings.
   - `compare` shortlist: side-by-side table for 2–8 listings.
   - `value` ranking: $/bed, $/sqft, listings under user budget, pet-fee-aware total cost-of-occupancy.
   - `nearby` aggregation: union of multiple city/zip slugs into one ranked list (the apartments.com UI does not support multi-city queries).
   - `stale` flag on listings unchanged for N days (signal that rent likely won't drop further or unit might be a phantom listing).
