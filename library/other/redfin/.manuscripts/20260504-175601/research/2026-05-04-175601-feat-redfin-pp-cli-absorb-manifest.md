# Redfin CLI Absorb Manifest

## Source landscape

| Tool | Stack | Status | Features it informs |
|------|-------|--------|---------------------|
| [reteps/redfin](https://github.com/reteps/redfin) | Python | Stale (403 issues since 2021) | Stingray API endpoint reference |
| [dreed47/redfin](https://github.com/dreed47/redfin) | Python | Stale (403 issues) | Stingray endpoint reference |
| [wang-ye/redfin-scraper](https://github.com/wang-ye/redfin-scraper) | Python + proxies | Stale | Sold homes filter, proxy-rotation pattern |
| [alientechsw/RedfinPlus](https://github.com/alientechsw/RedfinPlus) | Docs only | Reference | Most complete public Stingray endpoint catalog |
| [Apify crawlerbros/redfin-scraper](https://apify.com/crawlerbros/redfin-scraper) | Hosted SaaS, paid | Active | Field schema reference |
| [Apify sovereigntaylor/redfin-scraper](https://apify.com/sovereigntaylor/redfin-scraper) | Hosted SaaS, paid | Active | Field schema |
| [Apify automation-lab/redfin-scraper](https://apify.com/automation-lab/redfin-scraper/api) | Hosted SaaS, paid | Active | Search filter shapes |
| [Scrapfly Redfin guide](https://scrapfly.io/blog/posts/how-to-scrape-redfin) | Docs only | Reference | Polygon search params, anti-block guidance |
| MCP servers | — | None exist | — |
| Claude skills / plugins | — | None exist | — |

**No public free tool produces a working agent-native Redfin CLI.** The Python scrapers are bot-blocked; Apify SaaS works but is paid and not CLI-shaped.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Search by city + state | redfin.com path slug + `/stingray/api/gis` | `homes --city austin --state TX` | Path-validated, --json + --select for agents |
| 2 | Region autocomplete | `/do/region-search-autocomplete?location=` | `region resolve <query>` | Cached locally |
| 3 | Polygon-bounded search | gis `poly` param | `--polygon "lat,lng;lat,lng;..."` | Composes with all filters |
| 4 | Status filter | gis `status` | `--status for-sale\|sold\|pending\|coming-soon` | Enum validated |
| 5 | Property type | gis `uipt` | `--type house\|condo\|townhouse\|multi\|land` | Maps human enum to int code |
| 6 | Beds / baths min | gis `num_beds`/`num_baths` | `--beds-min N --baths-min N` | Numeric coercion |
| 7 | Price range | gis `min_price`/`max_price` | `--price-min --price-max` | |
| 8 | Sqft range | gis params | `--sqft-min --sqft-max` | |
| 9 | Year-built range | gis params | `--year-min --year-max` | |
| 10 | Lot size min | gis param | `--lot-min` | |
| 11 | School quality filter | gis params | `--schools-min N` | |
| 12 | Sold-only search | gis `sf` flag | `sold --city ... --year 2024` | Convenience subcommand |
| 13 | Pagination | gis `page_number` + `num_homes` | `--page N --limit N --all` | Auto-paginate option |
| 14 | Listing detail (combined) | initialInfo + aboveTheFold + belowTheFold | `listing get <url>` | Three Stingray calls behind one command |
| 15 | Photos | aboveTheFold | `listing.photos[]` | Native field |
| 16 | Price history | belowTheFold | `listing.price_history[]` | Time-series in JSON |
| 17 | Tax history | belowTheFold | `listing.tax_history[]` | |
| 18 | School assignments | belowTheFold | `listing.schools[]` | With ratings |
| 19 | Redfin Estimate (AVM) | belowTheFold | `listing.estimate` | With confidence range |
| 20 | Aggregate market trends | `region/<type>/<id>/<period>/aggregate-trends` | `market <region>` | Single-call summary |
| 21 | CSV export | `gis-csv` (350 cap) | `--csv` global flag | Per-search export |
| 22 | JSON export | (none of the scrapers) | `--json` (default for agents) | Native everywhere |
| 23 | RSS new-listings feed | `/newest_listings.xml` | `feed new --region <slug>` | Filterable by region |
| 24 | RSS updated feed | `/sitemap_com_latest_updates.xml` | `feed updates` | |
| 25 | Sync to local store | (NONE of the scrapers do this) | `sync-search <slug>` writes to SQLite | Foundation for transcendence |
| 26 | Search local store | (NONE) | `search "term"` over FTS5 | Offline + regex |
| 27 | SQL access | (NONE) | `sql "SELECT ..."` SELECT-only | Power-user composition |
| 28 | Deduplication | (NONE — Apify SaaS dupes) | Listing URL = natural key | |

Every absorbed row is mandatory shipping scope. No stubs.

## Transcendence (only possible with our approach)

| # | Feature | Command | Score | Persona | Why Only We Can Do This |
|---|---------|---------|-------|---------|------------------------|
| 1 | Saved-search watch with diff | `watch <slug> [--since N]` | 10/10 | Maya, Raj | Re-runs the synced gis query, diffs against the previous `listing_history_event` snapshot in local SQLite, emits NEW / REMOVED / PRICE-CHANGED / STATUS-CHANGED. Redfin's saved-search emails show only new listings, no diffs. |
| 2 | $/sqft net-HOA ranking | `rank --by price-per-sqft --net-hoa --region <slug>` | 9/10 | Raj | SQL: `(price - hoa*12*5) / sqft` ranked over local store. Apify actors lack net-HOA $/sqft; Redfin UI sort is by price only. |
| 3 | Side-by-side compare | `compare <url-a> <url-b> ...` | 8/10 | Maya, Tom | initialInfo + aboveTheFold + belowTheFold per URL, aligned columnar output (price, $/sqft, beds, baths, lot, year, school avg, AVM delta, last sale, taxes). Redfin has no compare view. |
| 4 | Sold-comp recipe | `comps <subject-url> [--radius 0.5mi --sqft-tol 15 --months 6 --bed-match]` | 9/10 | Tom, Raj | Resolves subject, derives circular polygon from radius, runs sold-status gis search, filters by sqft tolerance + bed match, emits ranked comp set. Redfin's polygon UI is clumsy and unsavable. |
| 5 | Stale + price-drop scan | `drops --region <slug> [--since 7d --min-pct 3 --dom-min N]` | 8/10 | Raj, Maya | Reads `listing_history_event` for the region, returns active listings with downward price events in window OR DOM exceeding threshold. Redfin buries DOM inside individual pages. |
| 6 | Multi-region union ranking | `rank --regions a,b,c --by price-per-sqft` | 7/10 | Raj, Priya | UNIONs synced rows across N region searches, ranks across the union, dedupes on listing URL. Redfin UI is single-region; Apify costs per-region run. |
| 7 | Trends overlay across regions | `trends --regions a,b,c --metric median-sale --period 24m` | 8/10 | Priya | One aggregate-trends call per region, joined into tidy long table (region × month × metric), CSV/JSON-shaped. Redfin's trends page is one region at a time. |
| 8 | Bulk export past 350 cap | `export --region <slug> --status sold --year 2024` | 7/10 | Raj | Slices price space into bands, page-walks gis-csv per band until each band returns < 350 rows, dedupes on URL across bands, single output. The 350-cap is the most-cited Redfin scraper limitation. |
| 9 | Neighborhood market summary | `summary --region <slug>` | 7/10 | Priya, Raj | One trends call + local store aggregates: active count, pending count, sold-90d count, median list, median sold, median DOM, median $/sqft, % with price drops. Redfin spreads these stats across 4 pages. |
| 10 | Region appreciation ranker | `appreciation --parent <metro> --period 12m` | 6/10 | Priya | Enumerates child neighborhood regions under a metro, fans out aggregate-trends, ranks by YoY median-sale % change. Redfin shows trends per region but never ranks neighborhoods within a metro. |

All 10 transcendence features are mandatory shipping scope. No stubs.

## Reachability + transport

- Runtime: **Surf with Chrome TLS fingerprint** (`mode: browser_clearance_http`, confidence 0.6 — both stdlib and Surf got 200 on homepage; per-IP rate limiting reported by community scrapers at scale).
- Pacing: `cliutil.AdaptiveLimiter` with conservative default (1 req/s), exponential backoff on 429.
- Auth: **none for v1**; account-only features (saved homes, alerts) explicitly out of scope.
- Geo: Stingray is US-only; non-US callers will get 403 regardless.
- Surface: **Stingray JSON + CSV** with the `{}&&` prefix-stripping wrapper. Some commands (compare, listing) make multiple Stingray calls per invocation.

## Stubs

None. Every listed feature is shipping scope.

## Source provenance for README credits

Listed in `research.json` `alternatives[]`; rendered as the README's source comparison block.
