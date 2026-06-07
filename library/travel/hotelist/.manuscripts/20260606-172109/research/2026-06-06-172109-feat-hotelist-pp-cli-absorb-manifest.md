# Hotelist CLI — Absorb Manifest

**Landscape:** No Hotelist-specific CLI/MCP/wrapper exists. The product to match-and-beat is the **Hotelist.com website itself** (every UI capability). Adjacent hotel MCP servers (hotelzero=Booking.com, jinko, 1Stay, hotel-concierge) inform table-stakes shapes (search/filter/detail) but use entirely different data sources; nothing absorbs Hotelist's AI-rated dataset.

**Our edge over the website:** offline SQLite mirror, `--json`/`--select`/`--csv` agent-native output, typed exit codes, multi-location and chain-vs-chain queries the single-map UI cannot express, honest "scraped from Hotelist / AI-rated, not an official API" labeling.

## Absorbed (match or beat the website + table stakes)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search hotels in a city | Hotelist city dropdown / `/bangkok` | `hotelist-pp-cli search <city-or-area>` | Offline cache, `--json`, sorted by Hotelist Score |
| 2 | Search by country | Hotelist country selector | `(behavior in hotelist-pp-cli search <country>)` `--country` resolution | Country `exact-match` filter; works for 153 countries |
| 3 | Search by region | Hotelist region selector | `(behavior in hotelist-pp-cli search <region>)` region geohash set | Europe/Asia/etc as a single query |
| 4 | Amenity filter (photo-verified) | Hotelist amenity filter | `hotelist-pp-cli filter <location> --amenity <name>` | `contains` over `ai_amenities_json`; multiple AND together |
| 5 | Gym-with-weights filter | Hotelist "Weightlifting gym"/"Squat rack" | `(behavior in hotelist-pp-cli filter <location>)` `--gym-weights` | Maps to real-weights amenity, not "has a gym" |
| 6 | Tennis-court filter | Hotelist "Tennis court" | `(behavior in hotelist-pp-cli filter <location>)` `--tennis` | One-flag verified amenity |
| 7 | Pool filter | Hotelist "Pool" | `(behavior in hotelist-pp-cli filter <location>)` `--pool` | One-flag verified amenity |
| 8 | Best value (rating/$) ranking | Hotelist "Best value (rating/$)" sort | `hotelist-pp-cli value <location-or-chain>` | Uses site best-value sort AND local rating/$ recompute |
| 9 | Sort by Hotelist Score | Hotelist sort | `(behavior in hotelist-pp-cli search <loc>)` `--sort score` (default) | Default ranking |
| 10 | Sort by price | Hotelist sort | `(behavior in hotelist-pp-cli search <loc>)` `--sort price[-desc]` | cheap/expensive |
| 11 | Sort by year built / renovated | Hotelist sort | `(behavior in hotelist-pp-cli search <loc>)` `--sort newest` | newness ordering |
| 12 | Price-range filter | Hotelist price slider | `(behavior in hotelist-pp-cli filter <loc>)` `--min-price/--max-price` | `price` greater/less-than |
| 13 | Rating-range filter | Hotelist rating slider | `(behavior in hotelist-pp-cli filter <loc>)` `--min-rating` | `hotellist_rating` greater-than |
| 14 | Newness filter | Hotelist "years old" filter | `(behavior in hotelist-pp-cli filter <loc>)` `--max-age/--built-after` | `year_built` filter |
| 15 | Chain / sub-brand filter | Hotelist chain dropdown | `(behavior in hotelist-pp-cli filter <loc>)` `--chain <name>` | `parent_chain_code`/`chain_code` |
| 16 | Boutique / collection filter | Hotelist boutique/collection | `(behavior in hotelist-pp-cli filter <loc>)` `--boutique`/`--collection` | independent / small-luxury rollups |
| 17 | Search by hotel name | Hotelist search box | `(behavior in hotelist-pp-cli search <loc>)` `--name <text>` | top-level `search` param |
| 18 | Hotel detail w/ AI breakdown | Hotelist detail modal | `hotelist-pp-cli show <hotel-id-or-slug>` | `/modal/{id}` HTML → structured Score, AI photo/review rating, consensus, amenities, pros/cons |
| 19 | "Exceptional" badge | Hotelist grid badge | `(behavior in hotelist-pp-cli search/show)` exceptional flag | score 8+, high consensus, built <10y surfaced in output |
| 20 | Wifi-speed filter | Hotelist internet filter | `(behavior in hotelist-pp-cli filter <loc>)` `--min-wifi` | `internet_json` greater-than |
| 21 | Currency display | Hotelist currency dropdown | `(behavior in all commands)` `--currency` note | base-currency prices labeled honestly |
| 22 | Dataset stats | Hotelist `/stats` page | `hotelist-pp-cli stats [location]` | counts/coverage; offline summary of cached data |
| 23 | City/chain reference data | Hotelist dropdown `<option>`s | `(generated endpoint)` sync of city→geohash + chain tables | Local FTS lookup; powers location resolution |

**Stubs:** none planned. All 23 absorbed rows are shipping scope.

**`--checkin`/`--checkout` note:** accepted for ergonomic parity but Hotelist has no date-based pricing (`price` is an AI-estimated nightly figure). The flags pass through as display context only and emit an honest "Hotelist prices are AI-estimated nightly averages, not date-specific quotes" note. Not a backend filter. (Disclosed at Phase Gate.)

## Transcendence

Customer model: location-independent professionals (levelsio's core audience) choosing a monthly base with compound requirements (rating floor AND real gym AND price ceiling) across whole countries/regions, not one city map at a time; chain-loyalty auditors who suspect a brand's quality varies by region; repeat users who want to track a place over time. Shared blocker: the single-map UI can't aggregate, rank across locations, compare chains, or remember history.

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|-----------------|
| 1 | Country value leaderboard | `rank-country <country> [--amenities] [--min-rating] [--max-price] [--top]` | hand-code | Single `/api` call with `country` exact-match filter, then local rating/$ recompute + sort across all returned cities. The map UI is bounded to one viewport. | Use for a national top-N-by-value list. Do NOT use for a user-defined route (use `corridor`) or brand comparison (use `chain-compare`). |
| 2 | Chain vs chain value showdown | `chain-compare --chains <c,...> [--country] [--metric] [--min-hotels]` | hand-code | One `/api` call per chain (parent_chain_code + country/bbox), then cross-chain mean rating, median price, rating/$, std-dev computed locally. UI filters one chain at a time with no aggregates. | Head-to-head brand value. Distinct from `chain-consistency` (variance within one brand) and `boutique-vs-brand`. |
| 3 | Nomad corridor scout | `corridor --cities "<City1,City2,...>" [--amenities] [--min-rating] [--max-price] [--top]` | hand-code | One `/api` call per named city (resolved via local geohash table), compound filters, unified per-city-winner table. Needs the local geohash index + multi-city aggregation. | Best hotel per stop on a planned route. Distinct from `rank-country` (exhaustive national scan). |
| 4 | Watch & drift tracker | `watch add <scope>` / `watch diff [--since] [--metric rating\|price\|both]` | hand-code | Stores timestamped scrape snapshots in SQLite; `diff` self-joins on (hotel_id, scrape_date) to surface rating/price movement. The website has zero historical state. | `watch add` registers a saved scope; `watch diff` reports change. Point-in-time search commands can't do this. |
| 5 | Chain consistency score | `chain-consistency --chain <code> [--country] [--metric] [--breakdown city]` | hand-code | mean/median/std-dev/min/max of ratings (or price) across all cached properties of one chain — population stat unavailable on the site. | Is this brand reliably good or full of outliers? Distinct from `chain-compare` (brand vs brand). |
| 6 | Price cliff finder | `price-cliff <city> [--min-rating] [--bin-size]` | hand-code | Bins cached hotels by price, computes mean rating per bin, finds where marginal rating-per-dollar collapses. The site's histogram is visual-only with no extractable breakpoint. | Cheapest price point that's still legitimately good. Distinct from a `--max-price` input filter. |

**Deferred (not shipping):** `amenity-audit` (claimed-vs-verified delta needs `/api` to return claimed amenities, which it does not — filter-only column; data separability unconfirmed). `gym-rank` (subsumed by `rank-country --amenities squat-rack`). `boutique-vs-brand`, `chain-footprint` (lower marginal value vs. the 6 above).

All 6 transcendence rows are `hand-code` and shipping scope.
