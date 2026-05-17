# Google Reviews CLI Brief

## API Identity
- Domain: Business reviews on Google Maps — star ratings, text reviews, reviewer profiles, owner responses, place metadata
- Users: Business owners monitoring reputation, competitive analysts, marketers, researchers, developers building review-monitoring pipelines, agents doing local SEO work
- Data profile: Place entities (name, address, phone, website, rating, review_count, category, coordinates), Reviews (text, rating, date, author_name, author_url, review_count, photos, owner_response), Sorting modes (most_relevant, newest, highest, lowest)

## Reachability Risk
- **Low** — Google Maps page loads via standard HTTP (200 OK, stdlib probe). Internal review XHR endpoint `/maps/preview/review/listentitiesreviews` discovered via community reverse-engineering; returns JSON after stripping `)]}'\n` anti-XSSI prefix. Place search via `google.com/search?tbm=map` with `pb=` protobuf-encoded parameter also accessible via HTTP.
- Key risk: Google's DOM structure changes without notice; CSS selectors in scrapers break. XHR endpoints are more stable but request parameter construction is complex.
- Mitigation: Target the XHR endpoints directly (not HTML parsing), capture with browser-sniff to get exact request shape, implement with standard HTTP transport.

## Top Workflows
1. **Business review monitoring** — check latest reviews for your business, alert on new 1-star reviews
2. **Competitive analysis** — compare ratings and review sentiment across competing businesses in a category
3. **Lead/prospect research** — fetch place details + reviews for a business before outreach
4. **Review export** — pull all reviews for a place into JSON/CSV for analysis
5. **Rating trend tracking** — sync reviews over time, detect rating drift, spot sudden drops

## Table Stakes
- Search a business by name/location/query
- Get place details (rating, review count, hours, phone, website, address, coordinates)
- Get all reviews for a place (paginated, no limit cap)
- Filter by star rating (1★ through 5★)
- Sort by: most relevant, newest, highest rated, lowest rated
- Export to JSON and CSV
- Show owner responses alongside reviews

## Data Layer
- Primary entities: places, reviews
- Sync cursor: review `sort_by=newest` + `updated_at` for incremental sync
- FTS/search: full-text search on review bodies, author names
- Compound queries: "all 1-star reviews from last 30 days", "businesses with rating < 3.5 in category"

## Codebase Intelligence
- Internal endpoint: `https://www.google.com/maps/preview/review/listentitiesreviews`
  - Response: JSON after stripping `)]}'\n` prefix
  - Sort modes: `sort_by=1` (relevant), `2` (newest), `3` (highest), `4` (lowest)
  - Pagination: token-based (next_page_token in response)
- Place search: `https://www.google.com/search?tbm=map` with `pb=` protobuf param
- Place details: `https://www.google.com/maps/place/` page + XHR for structured data
- Auth: None required for public reviews; browser session needed to see review history for your own business
- Rate limiting: Google throttles aggressive crawlers; adaptive limiter needed

## Competitors
| Tool | Language | Stars | Approach | Limitations |
|---|---|---|---|---|
| gosom/google-maps-scraper | Go | ~3.4k | Playwright browser automation | Needs Chrome, slow, full browser overhead |
| georgekhananaev/google-reviews-scraper-pro | Python | ~200 | SeleniumBase UC Mode | Python runtime, browser-based |
| Petey1337/google-review-scraper | JavaScript | ~50 | Puppeteer + stealth | Node.js runtime, browser |
| cablate/mcp-google-map | TypeScript MCP | ~100 | Google Maps API (with key) | Requires paid API key, ≤5 reviews |
| SerpAPI | SaaS | N/A | Managed scraping service | Paid, external dependency |
| omkarcloud scraper | Python | ~500 | Playwright | Python runtime |

**The gap:** No key-free, single-binary Go CLI that uses the internal XHR endpoints directly (not a full browser). gosom comes closest in Go but uses Playwright (full browser overhead). Our CLI targets the HTTP endpoints directly with optional browser-sniff auth.

## Product Thesis
- Name: `google-reviews-pp-cli`
- Why it should exist: The only Go CLI that pulls Google Maps reviews via the internal JSON endpoints — no API key, no Playwright, no external service. Single binary, offline-capable SQLite store, agent-native output. Works for monitoring your own business reviews, competitive analysis, or feeding reviews into an AI pipeline.

## Build Priorities
1. Place search (find a business by name/query, return place_id + metadata)
2. Reviews fetch (paginated, all sort modes, all star filters)
3. SQLite sync + FTS search
4. Transcendence: rating drift detection, sentiment window, competitor comparison, review velocity alerts
