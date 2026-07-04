# ImmoScout24 Mobile CLI Brief

## API Identity
- Domain: German real-estate listing search and expose details.
- Users: apartment hunters, property researchers, relocation agents, and agents monitoring new listings.
- Data profile: listing cards, expose detail sections, contact metadata, map markers, location data, prices, room counts, and publication recency.

## Reachability Risk
- Medium. The public website returns CloudFront 401 to direct curl from this host, but the mobile JSON host answered HTTP 200 for unauthenticated test calls.
- Probe-safe endpoint used: `GET /search/total?searchType=region&realestatetype=apartmentrent&pricetype=calculatedtotalrent&geocodes=/de/berlin/berlin`.
- Mobile host: `https://api.mobile.immobilienscout24.de`.
- User-Agent evidence: Fredy and ImmoScout wrappers send `ImmoScout_27.12_26.2_._` or `ImmoScout_27.3_26.0_._iOS`.
- Boundary: do not implement proxy rotation, CAPTCHA bypass, browser fingerprint evasion, or paywall bypass. Use low rate limits and expose honest failures.

## Top Workflows
1. Search Berlin, city, region, radius, or shape queries and return fresh listing cards.
2. Retrieve a listing expose by ID and extract detail sections/contact metadata.
3. Convert ordinary ImmoScout web search URLs into mobile API queries.
4. Track a saved search over time and identify newly seen exposes.
5. Sync listing cards/details locally for offline filtering, dedupe, and alerts.

## Table Stakes
- `search total`, `search list`, `search map`, and `expose get` over the mobile JSON API.
- Region, radius, price, room, living-space, sorting, and equipment filters.
- JSON/agent output with `--select` for large responses.
- Respectful defaults: bounded pagesize, timeout, and rate limit.

## Data Layer
- Primary entities: listing cards and expose details.
- Sync cursor: search query plus page number and first-seen expose ID.
- FTS/search: title, address line, attributes, expose text sections.

## Source Priority
- Primary: ImmoScout24 mobile JSON API, discovered through Fredy reverse-engineering notes and live probes.
- Secondary: official ImmoScout24 Business/API docs for terminology only, not as the main transport.
- Economics: no API key required for the confirmed mobile read endpoints; official Business API credentials are not required for the first CLI.
- Inversion risk: do not switch to the official OAuth business API just because it is documented. The user explicitly wants the website/mobile app surface.

## Product Thesis
- Name: ImmoScout24 Mobile CLI.
- Why it should exist: The website blocks simple HTTP access, but the mobile JSON API exposes clean listing/search/expose data. A CLI can make that surface understandable, scriptable, searchable, and agent-safe without running a browser.

## Build Priorities
1. Generate endpoint commands for `search total`, `search list`, `search map`, and `expose get`.
2. Add a URL translator command from ImmoScout web search URL to mobile query.
3. Add saved-search/watch workflows only after endpoint generation is stable.
