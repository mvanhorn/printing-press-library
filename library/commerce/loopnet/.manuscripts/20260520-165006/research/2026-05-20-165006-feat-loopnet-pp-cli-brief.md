# LoopNet CLI Brief

## API Identity
- **Domain:** Commercial real estate marketplace. LoopNet (loopnet.com, a CoStar Group property) is the most-trafficked CRE listings site in the US — for-sale and for-lease office, retail, industrial, multifamily, land, hospitality, health-care, and special-purpose properties, plus businesses-for-sale (BizBuySell) and auctions (Ten-X).
- **Users:** CRE brokers, investors, analysts, tenants, buyers. For this run: an aspiring CRE AI / market-intelligence developer building data pipelines.
- **Data profile:** Listing-centric. Each property carries price, $/sqft, size, cap rate, NOI, year built, building class, zoning, parking, units, broker, images, description, highlights. Search is geo + property-type + listing-type faceted.
- **No official API.** Confirmed repeatedly across research — CoStar Group does not publish a LoopNet developer API. Every existing tool scrapes the HTML site.

## Reachability Risk
- **Low (mitigated).** `probe-reachability` result: plain stdlib HTTP → **403** (Akamai Bot Manager); Surf with a Chrome TLS fingerprint → **200**, confidence 0.85, `mode: browser_http`, `needs_clearance_cookie: false`. The generated CLI ships **Surf transport** (browser-compatible HTTP) which clears Akamai with no login and no clearance cookie.
- Corroboration: `johnstenner/LoopnetMCP` uses `curl_cffi` impersonating `chrome136` for the same effect, with a `nodriver` headless fallback only for JS-challenge pages (marker `sec-if-cpt-container`). Surf is the printed-CLI equivalent of the curl_cffi layer.
- **ToS note:** LoopNet's terms restrict automated access; CoStar Group is litigious about scraping. Scope is single-user, respectful-rate, public-listing market research feeding the user's own analysis pipeline — not redistribution or bulk extraction. Rate limiting (≥2-3s delay, 1 concurrent) is a hard design requirement, surfaced in the README.

## Top Workflows
1. **Search inventory** — find listings by location + property type + for-sale/for-lease, filtered by price and size.
2. **Pull property detail** — full fact sheet for a listing ID (price, cap rate, NOI, class, zoning, broker).
3. **Sync a submarket into a local store** — repeated syncs accumulate the time series LoopNet itself does not expose.
4. **Detect change** — price cuts, new listings, delistings, aging inventory since the last sync.
5. **Aggregate market intelligence** — supply counts, price/$psf/cap-rate distributions, velocity per submarket.

## Table Stakes (absorbed from LoopnetMCP + Apify scrapers + community crawlers)
- Faceted search: location (city/state, state, zip), property type, for-sale vs for-lease, businesses-for-sale, broker search.
- Price filters (`min-price`, `max-price`, `price-type`: unit/sf/acre) and size filters (`min-size`, `max-size`).
- Per-listing detail extraction (18+ fields).
- Market overview / aggregate stats for an area.
- Pagination across multi-page result sets.
- Bulk extraction with `maxItems` caps.

## Data Layer
- **Primary entities:** `listings` (search-summary grain), `properties` (detail grain), `brokers`, `markets` (location × property-type × listing-type), `price_history` (per-listing observed price over time), `listing_snapshots` (per-sync presence record).
- **Sync cursor:** per-market last-sync timestamp; per-listing `first_seen` / `last_seen` / `last_price`.
- **FTS/search:** full-text over listing name, address, description, highlights, broker — offline `search` + composable `sql`.
- **The historic dimension is synthesized, not fetched.** LoopNet shows a snapshot; the store turns repeated snapshots into days-on-market, price-cut history, supply trend, and delisting detection.

## Codebase Intelligence
- Source: direct source read of `johnstenner/LoopnetMCP` (`src/loopnet_mcp/`).
- **URL structure (ground truth):**
  - Search: `https://www.loopnet.com/search/{type_slug}/{location_slug}/{listing_type}/{page}/` with optional query `min-price`, `max-price`, `price-type`, `min-size`, `max-size`.
  - Detail: `https://www.loopnet.com/Listing/{listing_id}/`.
- **Parsing:** HTML only (no JSON-LD / embedded JSON used by the MCP). Search results = `article.placard` elements (`header h4 a`, `header a.subtitle-beta`, `ul.data-points-2c li`); pagination = `a[data-automation-id="NextPage"]`; total = `.total-results-digits`. Detail = `.profile-hero-main-title`, `td.feature-grid__data[data-fact-type="..."]`, `table.property-data`, `.highlights-wrap .bulleted-list li`, `section.description .sales-notes-text`, `#mosaic-profile img`, `ul.contacts li.contact`.
- **Open question for browser-sniff:** whether current LoopNet pages also carry JSON-LD / an embedded JSON state blob — far more robust than CSS selectors. To be confirmed by a live capture (Phase 1.7).
- **Rate limiting:** MCP defaults — 3s request delay, 1 concurrent, 30s timeout, 3 retries (exponential backoff), 300s cache TTL.
- **Field set:** PropertySummary (17 fields), PropertyDetail (28 fields), MarketOverview (12 fields). Captured in the absorb manifest.

## User Vision
- The user is an aspiring CRE AI / market-intelligence developer. Goal: leverage LoopNet's data surfaces for **current and historic** data to power rich analysis and market intelligence.
- This CLI feeds an existing project — **Seyon Monitor** (`C:\Users\melan\Documents\CRE_Market_Intelligence`): a local-first Python/Streamlit/SQLite pipeline that monitors industrial-CRE demand signals across MA/NH/PA. Pipeline grain: `raw_docs → source_candidates → signal_events`; ingest path `data/raw/{source}/`.
- Seyon Monitor today covers demand/intent signals (EDGAR, news, permits, incentives, WARN) but **lacks asset-level pricing, transaction/listing data, cap-rate/yield, and distress signals** — exactly what LoopNet supplies. The CLI should emit clean JSON/CSV an external pipeline can ingest into `data/raw/loopnet/`.
- Data categories the user explicitly wants: **Supply & Inventory**, **Pricing & Deal Sentiment**, **Market Velocity**, **Demand & Intent Signals**, **Asset Pricing & Yield**, **Distressed Property & Motivation Signals**. Each maps to a transcendence command in the absorb manifest.
- **Reprint directives (this run):** (1) Add `mcp.transport: [stdio, http]` to the spec before generating so the MCP server is reachable by cloud-hosted agents over HTTP streamable transport in addition to stdio. (2) Add a `workflow_verify.yaml` verification manifest covering the `sync → caprate → price-cuts` flow — the core market-intelligence loop — so Phase 4 `workflow-verify` exercises it end-to-end. These are build-configuration directives, not feature requests.

## Product Thesis
- **Name:** `loopnet-pp-cli` (binary). Display name: **LoopNet**. Working product identity: a LoopNet market-intelligence CLI.
- **Headline:** LoopNet shows you today. This CLI remembers — every sync builds the price history, days-on-market, and supply time series LoopNet never gives you.
- **Why it should exist:** Existing LoopNet tools (LoopnetMCP, Apify actors, browser scrapers) are stateless extractors — they hand you a snapshot and forget it. None build a local time series, none compute velocity or price-cut deltas, none expose offline FTS or composable SQL, none are agent-native with `--json`/`--select`/typed exit codes. A SQLite-backed, agent-native CLI that syncs submarkets and diffs them over time turns LoopNet from a search box into a market-intelligence feed.

## Build Priorities
1. **Data layer + sync** (P0): `listings`, `properties`, `brokers`, `markets`, `price_history`, `listing_snapshots` tables; FTS; Surf transport; respectful rate limiting; `sync` accumulating snapshots.
2. **Absorbed surface** (P1): `search`, `property` (detail), `brokers`, `market` (overview/stats) — match LoopnetMCP's 3 tools + Apify's filter surface, beaten with offline store, `--json`/`--select`/`--csv`, `--dry-run`, typed exit codes.
3. **Transcendence** (P2): price-cut detection, days-on-market, new-listing feed (`since`), aged/stale inventory, delisting detection, market velocity, cap-rate/yield analysis, supply trend, comp finder, distress scan, submarket digest — all powered by the accumulated local store.
