# Scryfall CLI Brief

## API Identity
- Domain: Magic: The Gathering card database (api.scryfall.com). Card data, pricing, rulings, sets, symbology, bulk exports.
- Users: MTG players, deck builders, collectors, traders, store owners, app developers. Scryfall is the de facto canonical card search engine for the game.
- Data profile: read-only JSON REST. ~100k cards with full printings and daily-refreshed TCGplayer/Cardmarket prices, 1000+ sets, rulings, mana symbol catalog, 7 daily bulk files.

## Reachability Risk
- None. `probe-reachability` = standard_http (confidence 0.95), both stdlib and Surf probes returned 200 JSON. No auth on any read endpoint. Rate limits are polite-use (10 req/s guidance, no key).

## Top Workflows
1. Card lookup + price check: exact/fuzzy name -> current USD/EUR/foil prices across printings.
2. Fulltext search: Scryfall query syntax (`c:red f:modern`, oracle text, set filters) with pagination.
3. Collection valuation: batch-fetch up to 75 identifiers in one call (`/cards/collection`) -> price a binder/deck list.
4. Rulings pull: official rulings for any printing (by set/collector number, multiverse, mtgo, or Scryfall ID).
5. Set browsing + symbology: set codes/dates/card counts; mana cost parsing.

## Table Stakes
- Named/exact/random card fetch, search, autocomplete
- All identifier routes: Scryfall ID, multiverse, mtgo, arena, tcgplayer, cardmarket, set+collector number
- Rulings per identifier route
- Sets list/detail, catalog endpoints, mana symbology + parse-mana

## Data Layer
- Primary entities: cards, rulings, sets, bulk_data, catalog/symbology
- Sync cursor: none upstream; sync is full-pagination per resource (bulk-data listing is cheap, /cards is huge - default sync targets sets/catalog/bulk metadata)
- FTS/search: local store over synced entities + live passthrough for card search

## Codebase Intelligence
- Source: smgoller/scryfall-openapi (community OpenAPI 3 spec, MIT) as base spec; extended locally with 4 endpoints verified live against api.scryfall.com: POST /cards/collection, GET /cards/tcgplayer/{id}, GET /cards/cardmarket/{id} (spec lacked them; all probed successfully).
- Auth: none. No keys, no headers. CLI ships keyless.
- Rate limiting: self-imposed politeness; generated client defaults from community provenance.

## Product Thesis
- Name: scryfall-pp-cli
- Why it should exist: every MTG workflow starts at Scryfall, but there is no first-class terminal surface for it. This CLI puts card search, live pricing, batch collection valuation, and rulings into one agent-native tool with a local store for offline analysis. Zero auth friction means zero onboarding drop-off.

## Build Priorities
1. Cards resource complete (search/named/all identifier routes/collection batch/rulings) - the daily driver
2. Prices surfaced compactly (--compact prints name + usd/eur) - collector value loop
3. Sets + bulk-data listings - collection management context
