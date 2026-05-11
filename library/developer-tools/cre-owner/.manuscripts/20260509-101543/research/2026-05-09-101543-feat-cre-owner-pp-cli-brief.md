# CRE Owner CLI Brief

## API Identity
- Domain: Commercial real estate owner lookup, deal sourcing, and cold outreach
- Users: CRE investors, wholesalers, brokers seeking off-market deals and building owner contact info
- Data profile: Multi-source aggregation — parcels, owners, entity chains, listings, sales history, tax records, code violations, deed transfers
- Priority market: Lake County, Indiana (Merrillville, Gary, Hammond, Crown Point, Hobart)

## Reachability Risk
- [Medium] Crexi uses Cloudflare protection — direct HTTP gets 403/JS challenges. Browser-clearance cookie auth is required. Multiple Apify/scraping services exist for Crexi, confirming it's scrapeable with the right approach.
- [Low] SEC EDGAR — free public API, 10 req/sec with User-Agent header
- [Low] OpenCorporates — free tier works but only 200 req/month. Usable for spot lookups, not bulk traversal.
- [Low] Lake County assessor portals — public web, standard HTML scraping
- [Low] Indiana SoS/INBiz — public web search, no API

## Source Architecture (Revised)

### Foundation Tier (free, always works)
1. **County Assessor / Recorder** — Lake County IN first. Three portals:
   - Beacon/Schneider GIS (beacon.schneidercorp.com) — CAMA, assessment data, parcel viewer
   - publicaccessnow.com — tax search, delinquency records
   - ArcGIS Experience Builder — parcel boundaries, spatial queries
   - Data: assessed values, tax records, owner of record, parcel boundaries, sales history
2. **Indiana Secretary of State / INBiz** (inbiz.in.gov) — business entity search by name/number. No API, web scraping only. Data: entity name, type, status, formation date, registered agent, principal office.
3. **SEC EDGAR** (data.sec.gov) — free REST API, no auth. Endpoints: submissions history, XBRL financials, full-text search. Data: REIT 10-K/10-Q/8-K filings, beneficial ownership (Schedule 13D/G), insider transactions.
4. **OpenCorporates** (api.opencorporates.com/v0.4.8) — free tier: 200 req/month. REST API with token auth. Endpoints: company search, company detail, officers, filings. Data: company name, jurisdiction, status, officers, registered agent (jurisdiction-dependent).
5. **archive.today / Wayback Machine** — fallback for dead listing pages and removed assessor records.

### Enrichment Tier (user has Crexi account)
6. **Crexi** (api.crexi.com) — no public API. Internal REST endpoints discovered via browser-sniff. Cloudflare protected, requires Chrome session cookie. Data: active listings, sold comps, broker contacts (name, phone, email), property details (price, sqft, cap rate, NOI, zoning).

### Future Hooks (paid, stubs only)
7. **Regrid** — nationwide parcel data, $500/mo minimum. No free tier.
8. **ATTOM** — AVMs, comps, transactions, parcel-level data. Has MCP server (launched Jan 2026).
9. **Trepp** — CMBS loan maturity data.
10. **PropertyShark** — ownership records, permits, comps.

## Top Workflows
1. **Owner Lookup** — Given address/parcel → find owner of record → pierce LLC → find beneficial owner → get contact info
2. **Portfolio Discovery** — Given entity name → find all properties owned by same beneficial owner across multiple LLCs
3. **Motivated Seller Screening** — Filter market for distress signals: tax delinquent, long hold (15y+), out-of-state LLC, no recent refinance, code violations
4. **Cold Outreach List Building** — Rank owners by outreach-worthiness, produce mailing/contact lists with registered agent addresses
5. **Deal Sourcing** — Compound queries: "tax-delinquent parcels owned by entities with 3+ buildings" or "held >15y by out-of-state LLC with no refinance"

## Table Stakes (from competing SaaS)
- Owner of record lookup (Reonomy, PropertyShark, PropStream)
- LLC piercing / true owner identification (Reonomy — ML-based)
- Tax delinquency filters (PropertyShark, PropStream, BatchLeads)
- Comp analysis / sold history (CoStar, Crexi, ATTOM)
- Skip tracing / contact enrichment (DealMachine 96.5% hit rate, BatchLeads)
- Bulk list building with 140+ filters (PropStream/BatchLeads)
- Driving-for-dollars mobile capture (DealMachine)

## Data Layer
- Primary entities: parcels, owners, entities (LLCs/corps), entity_officers, listings, sales, tax_records, code_violations, deed_transfers, contacts
- Entity chain: parcel → owner_name → entity_search → officers → cross-reference → beneficial_owner
- Sync cursor: per-source last-sync timestamp
- FTS/search: full-text on owner names, entity names, addresses, parcel IDs
- Spatial: lat/lng on parcels for geographic queries (Lake County IN bbox)

## Codebase Intelligence
- No existing CLI tools for CRE owner lookup — pure whitespace
- MCP servers exist for ATTOM (WAV Group, Jan 2026), BatchData, SEC EDGAR (stefanoamorelli/sec-edgar-mcp), Flexmls
- SEC EDGAR MCP: Python, uses data.sec.gov, company search + filing retrieval
- No Crexi SDK/wrapper exists anywhere (npm, PyPI, GitHub)
- OpenCorporates: thin wrappers on PyPI (opyncorporates) and npm (opencorporates), both stale

## User Vision
- Priority market: Lake County, Indiana
- User has Crexi account (cookie auth via Chrome)
- All other sources free-tier — no paid keys required
- Leave hooks for ATTOM, Trepp, PropertyShark
- Goal: cold outreach to CRE building owners AND deal sourcing (motivated-seller signals)
- Compound SQLite queries are the moat — hero commands must be first-class

## Source Priority
- Foundation: County assessor (Lake County IN) — public web portals, scraping — free
- Foundation: Indiana SoS/INBiz — public web search — free
- Foundation: SEC EDGAR — official REST API — free
- Foundation: OpenCorporates — REST API — free (200 req/mo)
- Enrichment: Crexi — reverse-engineered REST — cookie auth (user has account)
- Hooks only: Regrid ($500/mo), ATTOM, Trepp, PropertyShark
- **Economics:** Foundation tier is entirely free. Crexi enrichment requires user's existing account. No paid API keys required for core functionality.
- **Inversion risk:** Crexi has the richest listing data but is enrichment-only. County assessor + SoS are the actual foundation — don't let Crexi's data richness invert the tiering.

## Product Thesis
- Name: CRE Owner CLI (cre-owner-pp-cli)
- Skill: /pp-cre-owner
- Why it should exist: No CLI tool exists for CRE owner lookup. The SaaS incumbents (Reonomy $thousands/yr, PropStream $99/mo, CoStar $thousands/yr) all require subscriptions. This CLI stitches free public data sources into a local SQLite database where compound queries surface insights none of the individual sources can produce alone. The entity-chain traversal (parcel → LLC → officers → other LLCs → portfolio) across county records + SoS + OpenCorporates + SEC EDGAR creates a "poor man's Reonomy" that costs nothing to run.

## Build Priorities
1. Data layer: SQLite schema for parcels, owners, entities, officers, listings, sales, tax records
2. County assessor scrapers: Lake County IN (Beacon, publicaccessnow, ArcGIS) with adapter pattern for future counties
3. Indiana SoS/INBiz scraper: entity search, officer lookup, registered agent
4. SEC EDGAR client: REIT filings, beneficial ownership schedules
5. OpenCorporates client: company search, officer traversal (rate-limited)
6. Crexi client: browser-clearance cookie auth, listings, comps, broker contacts
7. Hero commands: search, owner, portfolio, motivated, outreach, sync, enrich
8. Compound queries: the six moat queries from the spec
9. Future hooks: ATTOM, Trepp, PropertyShark, Regrid stubs
