# Zameen CLI — Absorb Manifest

Greenfield: no existing Zameen CLI, no official API. "Best sources" are the property-portal CLI reference (afspies/homehunt) and the hobby community scrapers (IIvexII, osmanrkhan, etc.). We match every table-stakes feature over a robust credential-free surface and add local-store transcendence no scraper offers.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search listings with real filters (purpose, type, city/area, price, beds, baths, area) | homehunt `search` / all scrapers | `zameen-pp-cli find --city Islamabad --purpose buy --type Homes --min-price --max-price --min-beds --max-beds --min-area --max-area` | Live Zameen data, client-side filters over scanned pages, `--json`/`--select`/`--csv` |
| 2 | Sort results (price asc/desc, newest, area) | property portals | `(behavior in zameen-pp-cli find --sort price-asc|price-desc|newest|area)` | Client-side sort over scanned/filtered set |
| 3 | Get a single listing by id | zillow-api-scraper detail | `zameen-pp-cli get <external-id>` | Full listing detail (store-first, live fallback) |
| 4 | Open a listing in the browser | portal tools | `zameen-pp-cli open <external-id>` | Reconstructs canonical Zameen URL; print-by-default, `--launch` to open |
| 5 | Local SQLite mirror of listings (auto-dedup) | homehunt `list` | `(behavior in zameen-pp-cli pull)` | Offline mirror, upsert by `external_id` (dedup for free) |
| 6 | Offline full-text search of synced listings | framework | `(behavior in zameen-pp-cli search "term" --db)` framework FTS | Search offline once synced |
| 7 | SQL over local listings | framework | `(behavior in zameen-pp-cli sql)` | Arbitrary local analytics |
| 8 | Bulk export result set to CSV/JSON | homehunt `export-csv/json`, all scrapers | `(behavior in zameen-pp-cli find --csv)` + `--json` | One-command bulk export of any search |
| 9 | Sync a query into the store | homehunt `run-config` | `zameen-pp-cli pull --city --purpose --type --max-pages` | Populate mirror for offline + cross-run diffing |
| 10 | Analytics / group-by aggregates | homehunt `stats` | `(behavior in zameen-pp-cli analytics --type listings --group-by city)` | Framework rollups over the store |
| 11 | Health check / reachability | framework | `(behavior in zameen-pp-cli doctor)` | Confirms Zameen reachable + store status |
| 12 | Learn loop (aliases: isb→Islamabad, plot→Plots, etc.) | — (framework, Zameen-seeded) | `(behavior in zameen-pp-cli teach/recall)` | City/category alias resolution from the spec `learn:` block |

Every absorbed row ships. Core search/get/open/sync are hand-built Go over the `internal/zameen` client (window.state extraction); store/search/sql/analytics/doctor/learn are framework.

## Transcendence (only possible with our approach) — 5 features, all hand-code

| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|--------------------------|------------------|
| 1 | Saved-search cross-run diff (new listings + price drops) | `watch <name>` | hand-code | Stores per-run result snapshots keyed by `external_id` in local SQLite and diffs the current scan against the last — impossible without the store; Zameen has no API and no alerts | none |
| 2 | Area price research (median price, price-per-Marla, inventory) | `comps --city <c> --area <a>` | hand-code | Aggregates synced listings by area/society into Marla-normalized medians locally — not echoed from any API call | Use this command for area/society-level rollups (medians, inventory, price-per-Marla). Do NOT use it to rank individual below-market listings; use 'deals' instead. |
| 3 | Below-market plot-file detector | `deals --city <c> --area <a> --type Plots` | hand-code | Computes each listing's price-per-Marla, joins to the area median across the scanned set, ranks/flags under-median files (PK plot-flipping culture) | Use this command to rank individual listings by how far under the area price-per-Marla they sit. Do NOT use it for area summary stats; use 'comps' instead. |
| 4 | Days-on-market / aging inventory | `aging --city <c> --days <n>` | hand-code | Derives days-on-market from each synced listing's `updated_at` unix timestamp from local SQLite — negotiation leverage the HTML never surfaces | none |
| 5 | Agency inventory leaderboard | `agencies --city <c>` | hand-code | Joins synced listings to their agency and rolls up count + median asking price per agency — a join the raw page never exposes | none |

Minimum 5 transcendence features: met (5). No stubs planned. Hand-code count: 5 transcendence + core hand-built client/commands (search/get/open/sync).
