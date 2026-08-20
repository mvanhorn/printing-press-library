# Zillow CLI Brief

## API Identity
- Domain: U.S. residential housing-market data from Zillow Group.
- Data profile: official Zillow Research CSV time series plus optional, separately authorized Bridge datasets. Zillow Research exposes downloadable regional metrics without an API key. Bridge exposes public records, Zestimates, and MLS data only after dataset approval.

## Users
- Mortgage professionals preparing borrower and referral-partner market briefs.
- Buyer agents tracking supply, demand, pricing, and affordability shifts.
- Homebuyers and residential investors comparing metros before narrowing a search.
- Housing analysts maintaining repeatable regional time-series datasets.

## Reachability Risk
- **Public CSV surface: Low.** Eight tested `files.zillowstatic.com/research/public_csvs/...` endpoints returned HTTP 200 on 2026-07-29. July files report `Last-Modified: Thu, 16 Jul 2026`.
- **Zillow.com listing surface: High / excluded.** Zillow Terms of Use section 5 prohibits automated queries, scraping, CAPTCHA bypass, and access-control bypass. This run will not replay hidden Zillow.com search endpoints or ship anti-bot workarounds.
- **Bridge surface: Permissioned.** Zillow says MLS, public-record, and Zestimate access is invite/approval based. Bridge uses `Authorization: Bearer <token>` and caps ordinary responses at 200 rows. Bridge commands must remain optional and token-scoped.
- Probe: `probe-reachability https://www.zillow.com/` returned `standard_http` with HTTP 200 for stdlib and Surf, but raw fetches of `/research/data/` and `/corporate/terms-of-use/` returned HTTP 403. Direct CSV endpoints remained reachable.

## Reachability Gate
- Decision: PASS for official Zillow Research surface.
- Evidence: all eight spec endpoints returned HTTP 200 on 2026-07-29.
- Bridge: expected auth gate; no live probe until user supplies approved access.

## Top Workflows
1. Mortgage professional refreshes metro/ZIP market data, then produces a borrower- or realtor-ready brief with current value, rent, inventory, sales, heat, affordability, and year-over-year movement.
2. Buyer agent compares several regions on appreciation, rent pressure, inventory, days to pending, and forecast direction before advising a relocating client.
3. Investor screens metros for value/rent divergence, demand, inventory normalization, and affordability pressure, then exports a shortlist.
4. Analyst syncs official releases into SQLite, queries historical series, and detects revisions or new release dates without maintaining spreadsheets.
5. Authorized Bridge user looks up Zestimates, assessments, transactions, or MLS listings without mixing credentials into public-data commands.

## Table Stakes
- Download and cache official CSV datasets.
- Resolve regions by name, type, state, and RegionID.
- Show latest value plus month-over-month and year-over-year change.
- Compare regions and metrics over a common time window.
- Export JSON, CSV, compact agent output, and selected fields.
- Sync to SQLite, search locally, inspect freshness, and run SQL.
- Retry transient failures; preserve source URL, release date, and Zillow attribution.
- Optional Bridge token support for approved datasets only.

## Data Layer
- Primary entities: dataset, region, observation, release, source provenance.
- Optional Bridge entities: parcel, assessment, transaction, Zestimate, listing.
- Sync cursor: HTTP `Last-Modified`/ETag plus latest observation date and content hash.
- FTS/search: region name, state, geography type, metric aliases.
- Storage boundary: Zillow Research metrics may be cached and analyzed; Bridge data retention follows the user's dataset agreement and is not cached by default.

## Ecosystem Findings
- Printing Press Redfin CLI proves demand for region comparison, trends, summaries, appreciation ranking, local SQL, and saved-state diffs.
- `sap156/zillow-mcp-server` advertises search, property details, Zestimate, market trends, and mortgage math, but its hard-coded `api.bridgeinteractive.com/v1/properties/...` contract does not match current Bridge documentation; treat it as feature evidence, not endpoint evidence.
- Zillapi skills expose address/URL/ZPID lookup, listing search, Zestimate, price history, photos, schools, and agent contact behind a paid third-party key. They are not an official Zillow source.
- `node-zillow` exposes legacy ZWSID methods such as search, Zestimate, comps, charts, demographics, region children, and mortgage calculations; it was last published eight years ago and is contract-stale.

## Product Thesis
- Name: `zillow-pp-cli`
- Thesis: official-data Zillow market intelligence, not a scraper. It should beat the Redfin CLI on regional analytics, reproducibility, provenance, and mortgage-professional workflows while refusing prohibited Zillow.com automation.
- Headline: Sync official Zillow housing datasets, compare markets, detect releases, and generate decision-ready housing briefs from terminal or MCP.

## Build Priorities
1. No-key Zillow Research sync, latest, trends, compare, rank, and brief commands.
2. Local SQLite, release/revision tracking, provenance, attribution, JSON/select output, and MCP parity.
3. Mortgage-oriented affordability and rent-vs-buy scenario commands using published metrics plus explicit user inputs.
4. Optional Bridge commands behind `BRIDGE_ACCESS_TOKEN`, disabled cleanly without approved access.
5. Explicit `open` command that prints Zillow links by default and launches only with `--launch`; no website scraping.
