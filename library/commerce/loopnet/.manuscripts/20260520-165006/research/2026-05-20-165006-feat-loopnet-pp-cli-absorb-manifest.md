# LoopNet CLI — Absorb Manifest

Every absorbed row is a feature we MUST build. Every transcendence row is hand-built in Phase 3.

> **Reprint, run 20260520-165006.** Transcendence table reconciled by the
> novel-features subagent against the prior v4.9.0 CLI's 9 features:
> 7 kept, 2 dropped (`assessment-gap`, `comps`), 1 new (`digest`).

## Ecosystem scan (Step 1.5a)

| Tool | Type | Contribution |
|------|------|--------------|
| `johnstenner/LoopnetMCP` | Python MCP server | **Primary source** — 3 tools (search_properties, get_property_details, get_market_overview), Akamai bypass via curl_cffi, full field set, exact URL structure. Source read directly. |
| `crawlerbros/loopnet-scraper` (Apify) | Commercial scraper | Field set + search-type enum (for-sale, for-lease, businesses-for-sale, brokers), propertyType enum. |
| `getdataforme/loopnet-scraper` (Apify) | Commercial scraper | Bulk extraction patterns. |
| `BenNormann/commercial-realestate-crawler-v3` | Python crawler | Smart scheduling, new-listing email notifications. |
| `Spareo/ln-scraper` | Python scraper | Property pull (Slack output). Low signal. |
| Browse AI / Thunderbit / ScrapingBee / ZenRows | Scraper templates | Per-listing field coverage (position, title, address, link, image, description, broker). |

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-------------|--------------------|-------------|
| 1 | Search listings by location | LoopnetMCP `search_properties` | Surf-HTTP fetch + CollectionPage JSON-LD parse | Offline store, FTS, `--json`/`--select`, typed exit codes |
| 2 | Filter by property type | LoopnetMCP / Apify `propertyType` | `--type` flag (office-buildings/retail/industrial-properties/multifamily/land/hospitality/health-care/special-purpose) | Validated enum, agent-native |
| 3 | Filter for-sale vs for-lease | Apify `searchType` | `--listing` flag | — |
| 4 | Filter by price (min/max) | LoopnetMCP `price_min/max` | `--min-price`/`--max-price` flags | — |
| 5 | Filter by size (min/max) | LoopnetMCP `size_min/max` | `--min-size`/`--max-size` flags | — |
| 6 | Pull full property detail (28+ fields) | LoopnetMCP `get_property_details` (source) | Detail-page JSON-LD + `data-fact-type` table parse | Parcels, tax assessments, FAR — beyond any scraper |
| 7 | Market overview / aggregate stats | LoopnetMCP `get_market_overview` (source) | Local SQLite aggregation | Offline, historical |
| 8 | Auto-pagination across result pages | Apify pagination / `maxItems` | Auto-paginate with `--limit` | — |
| 9 | Bulk extraction of a submarket | Apify bulk scrapers | `sync` command into local store | Idempotent, resumable |
| 10 | Broker contact data | All scrapers | broker name / company / phone fields | — |
| 11 | Businesses-for-sale listing type | Apify `searchType` businesses-for-sale | `--listing businesses-for-sale` | — |
| 12 | Broker search / directory | Apify `searchType` brokers | `brokers` command | — |
| 13 | Listing images | All scrapers | image fields in `--json` | — |
| 14 | Description + highlights extraction | LoopnetMCP detail | description / highlights fields | — |
| 15 | Cap rate / NOI / yield-field extraction | LoopnetMCP detail | cap_rate / noi / price_per_sqft fields | — |
| 16 | CSV / structured export | Apify / Browse AI | `--csv` / `--json` / `--select` | Composable, pipe to `jq` |
| 17 | New-listing monitoring | BenNormann crawler (email alerts) | `sync` + `since` window | Offline diff, no mail server |
| 18 | Scheduled / repeated monitoring | BenNormann smart scheduling | cron-friendly `sync`, typed exit codes | — |

## Transcendence (only possible with our SQLite-backed, agent-native approach)

| # | Feature | Command | Buildability | Score | Why Only We Can Do This |
|---|---------|---------|--------------|-------|-------------------------|
| 1 | Price-cut detection | `price-cuts <location>` | hand-code | 9/10 | Requires `price_history` diff across syncs — LoopNet shows only the current price; no tool builds the time series |
| 2 | Days-on-market | `dom <location>` | hand-code | 9/10 | Requires `first_seen` from `listing_snapshots` — LoopNet hides days-on-market entirely |
| 3 | Cap-rate / yield distribution | `caprate <location>` | hand-code | 9/10 | Local quartile distribution over cap-rate / NOI / $psf; gains an outlier-flag column (absorbs the cut `mispriced` idea) |
| 4 | Market velocity | `velocity <location>` | hand-code | 8/10 | Requires new/delisted counts + median DOM aggregated across snapshot history |
| 5 | Delisting detection | `delisted <location>` | hand-code | 8/10 | Requires presence diff of `listing_snapshots` between syncs |
| 6 | Distress / motivation scan | `distress <location>` | hand-code | 8/10 | Deterministic keyword match over synced description/highlights + Ten-X auction flag + recent cuts — needs the local corpus |
| 7 | Submarket digest | `digest <location>` | hand-code | 8/10 | One-command roll-up joining `listings`, `price_history`, `listing_snapshots` — supply, cuts, median DOM, new/delisted, distress counts |
| 8 | Pipeline feed export | `feed <location>` | hand-code | 8/10 | Emits run-stamped JSON/CSV mapped to the six CRE data categories, shaped for `data/raw/loopnet/` ingest |

**Dropped prior features (override at the gate if you want them kept):**
- `assessment-gap` — dropped: depends on detail-page tax-assessment fields the brief flags as unconfirmed (HTML-only source, open browser-sniff question). The valuation-screen role moves into `caprate`.
- `comps` — dropped: single-listing comp lookup is a per-deal/occasional task, not a weekly ritual for any current persona; `caprate`'s submarket-wide screen covers the recurring need.

**Hand-code commitment:** all 8 transcendence rows are `hand-code` — Phase 3 hand-writes 8 Cobra command files (~50-150 LoC each) plus `root.go` wiring, on top of the hand-built data layer (`listings`, `properties`, `brokers`, `markets`, `price_history`, `listing_snapshots` tables + FTS). The generator emits the 2 absorbed endpoint commands (`inventory listings`, `property detail`) and the framework commands (`search`, `sql`, `sync`, `doctor`, etc.).

**Stubs:** none. Every transcendence row ships fully implemented.

## Reprint build-configuration directives (from User context)
- **MCP transport:** add `mcp.transport: [stdio, http]` to `spec.yaml` before `generate` so the MCP server exposes HTTP streamable transport (cloud-hosted agents) alongside stdio.
- **Workflow manifest:** author `workflow_verify.yaml` covering the `sync → caprate → price-cuts` flow so Phase 4 `workflow-verify` exercises the core market-intelligence loop end-to-end (the prior run shipped with no manifest — `workflow-verify` reported "no workflow manifest found, skipping").

## Notes / risks for the gate
- LoopNet has no API; the CLI ships **Surf transport** (Chrome TLS fingerprint) which `probe-reachability` re-confirmed this run clears Akamai (stdlib 403 → surf-chrome 200, `mode: browser_http`, no clearance cookie).
- Extraction is JSON-LD-first (schema.org `RealEstateListing`/`CollectionPage`), with the `data-fact-type` HTML table as a supplement. JSON-LD is far more stable than CSS selectors.
- ToS: LoopNet restricts automated access. Scope is single-user, respectful-rate market research. Rate limiting (≥2-3s delay, 1 concurrent) is built in and documented in the README.
- The transcendence set depends on repeated `sync` runs accumulating snapshots — value compounds over time. A fresh install has no history until the second sync; `price-cuts`, `dom`, `velocity`, `delisted`, `digest` return empty until then.
