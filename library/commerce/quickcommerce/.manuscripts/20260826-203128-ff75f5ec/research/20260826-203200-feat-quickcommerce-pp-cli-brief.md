# QuickCommerce API CLI Brief

## API Identity
- Domain: Real-time Indian quick-commerce and marketplace product data.
- Canonical product: QuickCommerce API, operated by QuickCompare AI Technologies Pvt Ltd.
- Base URL: `https://api.quickcommerceapi.com`.
- Users: Developers, price-intelligence teams, D2C/FMCG operators, shopping assistants, and delivery analytics teams.
- Data profile: Location-sensitive product search, price, inventory, item details, delivery ETA, platform support, credit balance, and response/request metadata.
- Platform coverage: Search/item across BlinkIt, Zepto, Swiggy, BigBasket, DMart, JioMart, Minutes, Amazon, Nykaa, Myntra, and Flipkart. ETA is limited to the seven quick-commerce platforms.

## Reachability Risk
- Low: official docs and machine-readable `llms-full.txt` are reachable; safe unauthenticated `/v1/supported-platforms` probe returned 200.
- Authenticated probes returned 200 for `/v1/credits`, `/v1/search`, `/v1/item`, `/v1/eta`, `/v1/groupsearch`, and `/v1/groupeta` using the user-supplied credential at runtime. The credential value is intentionally absent from all artifacts.
- Documented errors: 401 invalid/missing key, 402 no credits, 404 item/platform unavailable, 422 invalid params/platform, 429 rate limit (100 requests/min), 500/502/504 upstream failures.
- Cost: one credit per platform for standard calls; group calls cost one credit per requested platform; credits and supported-platforms are free.

## Top Workflows
1. Search a product at a location on one platform, then inspect the returned item IDs and prices.
2. Compare price, stock, pack size, rating, and deeplink across multiple platforms with one group search.
3. Compare delivery ETAs and store availability for a location across quick-commerce platforms.
4. Monitor credit balance, expiry, and usage before spending paid API calls.
5. Build a local searchable mirror and compare repeated observations over time without re-querying the API.

## Table Stakes
- All seven documented REST endpoints with accurate query/header handling.
- API-key configuration using `X-API-Key`, with optional `x-geolocation-pincode` support for DMart, JioMart, and Minutes.
- JSON, compact, field-selected, CSV, and agent-native output.
- Clear handling of 401/402/404/422/429/5xx responses and request/credit headers.
- Location and platform validation, realistic examples, dry-run requests, and a doctor/health command.
- Hosted MCP parity: search, item, ETA, group search, group ETA, credits, and platform discovery.

## Data Layer
- Primary entities: products, item variants, platform observations, delivery ETA observations, credit packs, and supported-platform capabilities.
- Persist product observations keyed by platform/item ID/location/query with fetched timestamp, offer price, MRP, inventory, availability, rating, quantity, store, and deeplink.
- Persist ETA observations keyed by platform/location/pincode with ETA, open status, store IDs, timestamp, and request ID.
- Persist credit snapshots and supported-platform capability metadata.
- Sync cursor: timestamp plus query/location/platform tuple; refresh is manual because every paid read consumes credits.
- FTS/search: local product name, brand, quantity, platform, and query fields; support SQL for price, stock, availability, and recency analysis.

## Ecosystem and Competitive Surface
- The provider offers a hosted MCP server at `https://api.quickcommerceapi.com/mcp` with seven endpoint-mirror tools and no published local package.
- Web research found no official CLI, npm SDK, PyPI SDK, or established public wrapper. The dominant incumbent is therefore direct REST/MCP usage or custom scraper code.
- Adjacent public-library CLIs cover general retail and marketplace search, but not this API's unified location-aware 11-platform contract.
- DIY scrapers are fragile against upstream site changes; the provider positions this API as a stable unified alternative. The CLI should avoid resident browser transport and use replayable HTTP.

## Product Thesis
- Name: QuickCommerce CLI.
- Headline: Compare live Indian product prices, stock, and delivery ETAs from the terminal, then keep the history locally.
- Why it should exist: The hosted MCP is conversational but ephemeral, while direct API calls force agents to repeat location/platform parameters and lose historical context. A CLI can preserve the full API surface, expose clean agent-native commands, enforce credit-aware read behavior, and make cross-platform comparison and price/availability history composable offline.

## Build Priorities
1. Generate the seven documented endpoints with canonical `QUICKCOMMERCE_API_KEY` support and pincode header handling.
2. Add local persistence and FTS for product/item/ETA/credit observations with explicit manual sync.
3. Add `compare` and `fastest` local/live workflows that normalize prices and ETA values without phantom rows.
4. Add credit-aware request planning, cost estimates, and low-balance warnings before paid fan-out.
5. Add resilient, bounded output for agents: `--json`, `--agent`, `--select`, `--compact`, `--csv`, and honest empty/partial results.

## Discovery Decisions
- Browser-sniff gate: declined by user; official documented API and Chrome DevTools MCP review are the source of truth.
- Crowd-sniff gate: skipped; research found no community SDK/code surface that would add endpoint evidence.
- Auth context: user supplied a credential explicitly for full read-only live testing; use only at runtime and never persist its value.
 
## Reachability Gate
- Decision: PASS
- Evidence: `GET https://api.quickcommerceapi.com/v1/supported-platforms` returned HTTP 200 without credentials; authenticated read-only probes for `/v1/credits`, `/v1/search`, `/v1/item`, `/v1/eta`, `/v1/groupsearch`, and `/v1/groupeta` returned HTTP 200.
