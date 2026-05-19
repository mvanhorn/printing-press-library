# Tennis Warehouse Absorb Manifest

## Source landscape

No competing tool found in any search:
- No CLI for tennis-warehouse.com (`site:github.com tennis-warehouse cli` → empty)
- No MCP server (`tennis-warehouse mcp` → empty)
- No Claude Code plugin (`tennis-warehouse claude-plugins-official` → empty)
- No npm SDK (`site:npmjs.com tennis-warehouse` → empty)
- No PyPI client (`site:pypi.org tennis-warehouse` → empty)
- No SKILL.md (`site:github.com tennis-warehouse SKILL.md` → empty)

This is greenfield. There is nothing agent-shaped that touches Tennis Warehouse's catalog today. The "absorbed" features below mirror table-stakes the website itself provides (browse, filter, view detail), with our value-add being agent-native output, offline-via-SQLite, and composability.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Browse used racquets by brand | tennis-warehouse.com web UI (per `ccode`) | `used list --brand <name>` over local SQLite snapshot | Filterable + sortable + `--json --select` + multi-brand |
| 2 | Browse new racquets by brand | tennis-warehouse.com web UI | `racquets list --brand <name>` over local SQLite snapshot | Filter + sort + `--json --select` |
| 3 | View full spec sheet for a racquet | web detail page (`descpageRC...html`) | `racquets get <sku>` and `used get <pcode>` | Same data, available as JSON for agents; no HTML re-parsing per query |
| 4 | View individual used units for a model | web order page (`/orderusedproduct.html`) | `used units <pcode>` | Per-unit stock code, grade, grip size, price; filterable; `--json` |
| 5 | Sort by price / spec | web sort UI | `--sort price-asc\|price-desc\|swingweight\|strung-weight\|head-size` | Multi-key composable sorting; agent-friendly |
| 6 | Filter by spec | web filters (limited) | `racquets list --head-size 98 --string-pattern 16x19 --max-strung-weight 11.5` | More dimensions than the web exposes; ranged values supported |
| 7 | Local snapshot of catalog | n/a — site has no API | `sync` builds full SQLite snapshot of new + used inventory | Offline browsing; reproducible queries; agent-context friendly |
| 8 | Full-text search across catalog | web search box | `search "wilson blade 98"` over FTS5 | Composable with `--json --select`; works offline |
| 9 | Raw SQL access | n/a | `sql "SELECT * FROM racquets WHERE ..."` | Power-user query path that bypasses the curated commands |
| 10 | View the grading legend | static help page on usedracquets.html | `used grades` static-reference | Embedded in CLI; offline; `--json` |
| 11 | Health probe | n/a | `doctor` verifies network reach to tennis-warehouse.com and DB consistency | Standard agent-onboarding signal |

11 absorbed features. Every row will ship; none are stubs.

## Transcendence (only possible with our approach)

These eight features come from the novel-features subagent's adversarial-cut output (run on 2026-05-19, see `2026-05-19-104537-novel-features-brainstorm.md` for the full audit trail including customer model and killed candidates).

| # | Feature | Command | Buildability | Why Only We Can Do This |
|---|---------|---------|--------------|-------------------------|
| 1 | Substitute finder by spec similarity | `racquets similar <sku> [--tolerance loose\|tight] [--limit N]` | hand-code | Joins target SKU against all current racquets, scores by absolute distance across head_size, strung_weight, swingweight, and exact string_pattern match. The website has no "find racquets similar to this one" surface. |
| 2 | Side-by-side spec compare | `racquets compare <sku> <sku> [<sku>...]` | hand-code | SELECT across 2–5 SKUs, aligned spec-by-spec, diff highlighting. `--json` emits row-per-spec matrix. Web requires 5 browser tabs and a spreadsheet. |
| 3 | Used-vs-new value gap | `used deals [--brand <name>] [--min-discount-pct 30] [--grade A]` | hand-code | LEFT JOIN `used_listing` against `racquet` MSRP to compute discount per listing. No web page joins used inventory against new MSRP. |
| 4 | Price drop tracking | `used drops --since 7d [--brand <name>] [--min-drop-pct 10] [--watchlist-only]` | hand-code | Joins each `used_listing` to its two latest `price_snapshot` rows, computes delta. Site has no price history at all. |
| 5 | New-arrival feed | `used new --since 7d [--brand <name>] [--grade A]` | hand-code | SQLite scan of `used_listing` where `first_seen_at >= now - window`; sync sets first_seen_at on insert. Site has no "new since" view. |
| 6 | Inventory depth by model | `used depth [--brand <name>] [--min-units 3] [--grade A]` | hand-code | Aggregates per-physical-unit rows grouped by (pcode, grade), returns per-model unit counts. Site exposes units only on a model's own order page — no aggregate. |
| 7 | Watchlist + drops | `used watch <pcode>` / `used watchlist` / `used watchlist drops --since 7d` | hand-code | Adds a `watchlist` table; integrates with #4. No daemon — strictly "list current state on demand." |
| 8 | Grip-size availability | `used grip-availability --size 4_3/8 [--grade A] [--brand <name>]` | hand-code | Scans used unit rows by grip, groups by (brand, model, grade), counts available. Grip is critical (wrong grip = unplayable) and the site shows it only per-detail-page. |

**Hand-code commitment:** all 8 transcendence rows are `hand-code`. The generator will scaffold the HTTP client, store, sync skeleton, and search/sql commands — every novel command above requires a hand-authored `internal/cli/<command>.go` file (~80–150 LoC each) plus `root.go` AddCommand wiring. The HTML extractors that populate the store from live pages are also hand-authored (~200 LoC total under `internal/scraper/`).

## Anti-reimplementation note

All transcendence commands operate on the local SQLite store populated by `sync` (which calls live Tennis Warehouse HTTP pages and parses HTML). No hand-rolled response builders. No canned constants. The `used grades` reference command (absorbed #10) is curated static content under `// pp:novel-static-reference` (the grading rubric is published by Tennis Warehouse and not expected to change).
