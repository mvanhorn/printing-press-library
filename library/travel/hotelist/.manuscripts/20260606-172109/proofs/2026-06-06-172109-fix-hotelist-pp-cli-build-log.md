# Hotelist CLI — Build Log

Manifest transcendence rows: 6 planned, 6 built. Phase 3 will not pass until all 6 ship.

## Built

### Priority 0 — foundation
- Custom `/api` client layer (`hotelist_api.go`): `apiFilter` grammar + `buildAPIParams` (jQuery nested-array serialization `filters[N][target]=...`), `fetchHotels`, `adaptiveFetch` (geohash-precision widening), `politeClient` (2 req/s default), `hlHotel` type, output model, value/exceptional logic.
- Location resolution (`hotelist_locations.go`): city→geohash table scraped from homepage `<option>` list, full chain code map (EM=Marriott…), region bboxes, country detection, `resolveLocation`.
- SQLite store: reuses generated `resources` table (resource_type `city`/`hotel`) + hand-authored `watch_scopes` / `hotel_snapshots` tables (`ensureWatchTables`).
- `sync` / `sync cities` (replaces generic sync stub).

### Priority 1 — absorbed (the 4 user commands + stats)
- `search <city-or-area>` — geohash/country/region resolution, --min-rating/--max-price/--name/--sort/--limit/--checkin/--checkout/--exceptional.
- `filter <location>` — --gym-weights/--tennis/--pool/--amenity (photo-verified `contains`), --min-rating/--max-price/--min-price/--chain/--boutique/--collection/--built-after/--sort/--limit.
- `value <location-or-chain>` — best-value sort + local rating/$ recompute (priced hotels lead, price-0 sink); --country/--min-rating/--max-price.
- `show <hotel-id>` — parses `/modal/{id}` HTML into Hotelist Score, AI photo/review ratings, per-source ratings, verified amenities, pros/cons.
- `stats` — local mirror summary.

### Priority 2 — transcendence (all 6, hand-code)
1. `rank-country <country>` — national value leaderboard (single country-filter call + local value sort).
2. `chain-compare --chains` — head-to-head chain mean rating / median price / value / stdev.
3. `corridor --cities` — best hotel per stop on a multi-city route.
4. `watch add|diff|list` — timestamped snapshots + rating/price drift diff (the only history-aware feature).
5. `chain-consistency --chain` — mean/median/stdev/range + verdict.
6. `price-cliff <city>` — price binning + cliff detection (min-bin-population guard) + value recommendations.

## Key implementation findings
- `/api` returns `parent_chain_code: null` in list results; chain display is mapped from the request code, not read back.
- Chain filter uses 2-char codes (EM, EH, FS…), not display names — full map embedded.
- City `<option>` geohashes vary in precision; `adaptiveFetch` widens the prefix (min 3 chars) when a query returns <5 hotels (fixes Tulum `d59f`→0 vs `d59`→20).
- Homepage `<option>` markup spans multiple lines; regex uses `(?s)` + `\s+`.
- `/modal` GET works identically to POST (id is in the path); spec uses GET for html response_format support.
- `--checkin/--checkout` accepted but pass-through only (Hotelist has no dated pricing) with an honest note.

## Tests
- Hand-authored `hotelist_test.go`: 9 table-driven tests (nested param serialization, chain/amenity/slug resolution, value sort, exceptional logic, modal parser, stats helpers, sort resolution). All pass.

## Deferred (not shipped, per Phase 1.5)
- `amenity-audit` (claimed-vs-verified delta — `/api` does not return claimed amenities; data separability unconfirmed).
- `gym-rank` (subsumed by `rank-country --amenities`), `boutique-vs-brand`, `chain-footprint`.

## No stubs. All 6 transcendence rows shipping.
