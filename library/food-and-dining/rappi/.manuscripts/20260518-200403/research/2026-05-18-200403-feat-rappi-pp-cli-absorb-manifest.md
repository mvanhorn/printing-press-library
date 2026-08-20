# Rappi CLI Absorb Manifest

## Source tools surveyed

| Tool | Type | Maintained | Stars | Provides |
|------|------|-----------|-------|----------|
| `parseforge/rappi-scraper` (Apify) | Paid scraper, 9 countries | Active | n/a | Field set, MX coverage |
| `yasmany.casanova/rappi-restaurant-scraper` (Apify) | Paid scraper, restaurants | Active | n/a | Menu + restaurant field set |
| `luminati-io/rappi-price-tracker` (Bright Data) | Tutorial-grade scraper | Stale | low | Field schema |
| `ingmpesca/rappi-webScraping` | Selenium tutorial | Abandoned | low | HTML extraction pattern |
| `n0shake/dash` (DoorDash CLI) | Delivery CLI | Archived 2020 | 100s | Command shape inspiration |
| `dan-v/golang-ubereats` | Unofficial Go client | Stale 2018-2019 | low | lat/lng pattern |
| `dev-portal.rappi.com` | Official merchant API | Active | — | Auth pattern (Bearer x-authorization), but merchant-scoped (out of scope) |
| `rappi-pp-cli` MCP servers / Claude skills | — | — | — | **None exist** — this CLI is the first |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value | Status |
|---|---------|-------------|--------------------|-------------|--------|
| 1 | List restaurants by city | rappi.com.mx SSR + Apify | `restaurants list-city <city>` | --json, --select, --csv, agent-native | shipping |
| 2 | List restaurants by category | rappi.com.mx SSR + Apify | `restaurants list-category <city> <category>` | Same | shipping |
| 3 | Restaurant detail | rappi.com.mx SSR JSON-LD | `restaurants get <id-slug>` | Parses schema.org Restaurant block | shipping |
| 4 | List stores by type | rappi.com.mx SSR | `stores list-by-type <type>` | --json, --select | shipping |
| 5 | Store detail | rappi.com.mx SSR | `stores get <id-slug>` | Same | shipping |
| 6 | Restaurant + store catalog snapshot | Apify parseforge | `sync` populates SQLite per city | Offline, deterministic, repeatable | shipping |
| 7 | Cross-entity text search | Bright Data tutorial | `search <query>` via FTS5 | Works offline | shipping |
| 8 | Restaurant category enumeration | Apify + UI | `categories list` | Closed enum baked in | shipping |
| 9 | Store type enumeration | Apify + UI | `stores types` | Closed enum baked in | shipping |
| 10 | City + default coordinates | manual | `cities list` | Offline lookup with coords | shipping |
| 11 | Browse alphabetized product catalog | rappi.com.mx SSR | `catalog browse-letter` | --limit, --json | shipping |
| 12 | Promotions landing | rappi.com.mx SSR | `promotions list` | --json | shipping |
| 13 | Bulk export | Apify dataset | --json / --csv / --select on every command | Built-in framework | shipping |

## Transcendence (only possible with our approach)

10 features survived the adversarial cut, all scoring 6-8/10.

| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|-------------------------|
| 1 | Restaurants newcomers/closures diff | `restaurants diff --city <city> --category <cat> --since <date>` | 8/10 | Local SQLite snapshot diff; Rappi UI has no diff view |
| 2 | Top-rated with rating + review-count floor | `restaurants top --city <city> --category <cat> --min-rating 4.5 --min-reviews 100` | 8/10 | Rappi UI cannot filter by review-count floor |
| 3 | Cross-city store coverage matrix | `stores coverage --cities cdmx,gdl,mty` | 8/10 | Cross-city aggregation Rappi has no view for |
| 4 | Coverage delta over time | `stores coverage-diff --since <date>` | 7/10 | Temporal snapshot diff requires local SQLite |
| 5 | Restaurants open at a specific time | `restaurants open --city <city> --at "23:30" --category <cat>` | 7/10 | Parses schema.org openingHours, evaluates arbitrary time |
| 6 | Cuisine-by-neighborhood breakdown | `restaurants by-neighborhood --city <city> --category <cat>` | 7/10 | Address-component extraction + local aggregation |
| 7 | Cross-category restaurant overlap | `restaurants multi-category --city <city>` | 6/10 | Self-join across category-list snapshots in SQLite |
| 8 | Geo-radius restaurant filter | `restaurants near --lat <> --lng <> --radius-km 2` | 7/10 | Haversine over local geo data; UI is point-and-list, no radius |
| 9 | Brand presence across cities | `restaurants brand --name "Sushi Itto"` | 6/10 | Fuzzy-name match across all city+category snapshots |
| 10 | Cross-store-type geo adjacency | `stores adjacency --type pharmacy --within-km 1 --of-type market` | 6/10 | Cross-store-type Haversine join in SQLite |

## Source priority
Single-source (rappi-mx). No combo CLI, no inversion concerns.

## Stub list
None. All shipping-scope features are buildable on the SSR + JSON-LD surface verified during browser-sniff.

## Known gaps (NOT in v1, documented upfront)
- Menu items + prices (requires XHR API, geo-blocked from datacenter IPs without residential proxies)
- Cart / orders / order tracking (requires login)
- RappiPay / RappiCard / RappiTravel (adjacent products, not delivery catalog)
