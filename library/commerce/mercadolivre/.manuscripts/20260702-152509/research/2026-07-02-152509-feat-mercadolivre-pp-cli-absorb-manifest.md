# Absorb Manifest — mercadolivre-pp-cli

## Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Product search w/ price | Menegueli/WebscrapingML, Unwrangle mercado_search | Paginated JSON-LD @graph extraction (name/brand/price/rating/url) | Stable JSON-LD (not fragile CSS), --json/--csv, offline persist, agent-native |
| 2 | Product detail + specs | linces/MercadoScraper, Unwrangle product-details | JSON-LD Product + spec-table (~40 attrs) extraction | Structured attribute rows, offline, --select fields |
| 3 | Query autosuggest | (frontend endpoint) | http2.mlstatic autosuggest (no auth) | Query expansion for search |
| 4 | Text->category resolution | (official domain_discovery) | api.mercadolibre domain_discovery (no auth) | Maps free text to category_id for schema/compare |
| 5 | Category attribute schema | official categories/{id}/attributes | Fetch + persist attribute schema (no auth) | Normalizes comparison columns |
| 6 | Local persistence + sync | (none do this well) | SQLite: product/attribute/price_snapshot/search_result + FTS | Offline re-query, history, SQL passthrough |
| 7 | Offline FTS search | (none) | FTS5 over name+attributes | Re-query synced data without hitting the site |
| 8 | SQL passthrough | (none) | Read-only SELECT over local store | Composable procurement queries |
| 9 | CSV/JSON export | scraper CSVs | --json/--csv/--select on every command | Spreadsheet + agent pipelines |
| 10 | Health/doctor | (none) | doctor: cookie validity + reachability probe | Diagnoses captcha-wall/clearance state |
| 11 | Chrome clearance auth | (Playwright scrapers) | auth login --chrome (cookie import) + Surf transport | Clears captcha wall without a resident browser |

## Transcendence (only possible with our approach)
| # | Feature | Command | Score | Why Only We Can Do This |
|---|---------|---------|-------|------------------------|
| 1 | Aligned spec matrix | `compare <ids…> [--diff]` | 10 | Local join product+attribute against category schema -> one normalized column per attribute + price row; --diff hides identical rows. No single API call gives this. |
| 2 | Cheapest meeting spec floor | `cheapest --query <q> --spec "voltagem=220V" --spec "potencia>=700W"` | 9 | Mechanical =/>=/<= predicate eval over local attribute table, then sort by price. |
| 3 | Cross-seller price dispersion | `dispersion MLB<id>` | 9 | min/max/median/stddev over the multiple seller listings an ML catalog product aggregates. Exploits ML's catalog structure. |
| 4 | Price drift over time | `price-history <product> [--since 30d]` | 9 | Delta over repeated local price_snapshot captures. Pure local-data. |
| 5 | Cotação quotation bundle | `cotacao <ids…> [--format md\|csv]` | 8 | Cross-entity join product+attribute+seller+latest price with captured_at provenance -> purchase-request-shaped doc. |
| 6 | Freshness gate | `stale [--older-than 7d]` | 7 | Local captured_at query; blocks cotações built on cold prices. |

Notes: spec-diff ships as `compare --diff`. Transport = browser_clearance_http (Chrome cookie import + Surf). Primary extraction = JSON-LD.
