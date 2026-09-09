# SEEK CLI Brief

Target: `https://au.seek.com/` — from-website build. `BROWSER_SNIFF_TARGET_URL=https://au.seek.com/`.
No public spec; the runtime surface will come from Phase 1.7 browser-sniff. This brief
drives feature selection, not the contract.

## API Identity
- **Domain:** `au.seek.com` (canonical since the 2024 migration off `www.seek.com.au`; the
  old domain still resolves and still serves the JSON search API). SEEK Australia is the
  country's #1 employment marketplace (ASX:SEK). Sister site: `seek.co.nz` (New Zealand),
  same platform, `siteKey=NZ-Main`.
- **Users:** Job seekers (search, track, apply), salary researchers, recruiters doing
  market/competitor scans, data/analytics folks tracking hiring trends.
- **Data profile:** Job listings (title, advertiser, location, salary label, work type,
  classification taxonomy, listing date, teaser + full description, bullet points),
  advertiser/company profiles + company reviews + salary insights, a large numeric
  classification/subclassification taxonomy, location autocomplete tree.
- **Auth:** None for search and (most) job detail reads — community scrapers hit the JSON
  API unauthenticated. A logged-in browser session would additionally expose saved
  searches, applied-job history, and recommended jobs (cookie/session based, not API key).

## Reachability Risk
- **Search API — Low.** `GET https://www.seek.com.au/api/jobsearch/v5/search` (and the
  older `/api/chalice-search/v4/search`) is used directly by multiple community scrapers
  (`qinscode/SeekSpider`, Apify actors) pure-HTTP with only a browser `User-Agent`, no
  cookie, no token. Returns JSON: `{ data: [...jobs], totalCount, solMetadata: { pageSize } }`.
- **Job detail HTML pages — High.** `https://www.seek.com.au/job/{id}` returns Cloudflare
  `403` / JS challenge for non-browser clients (`SeekSpider` explicitly handles `403` and
  gives up on the description). Phase 1.7 must find the **JSON job-detail endpoint** the
  SPA uses (GraphQL `jobDetails` query on `graphql-dark-prod.cloud.seek.com.au`, or a
  `chalice`/`jobdetails` REST route, or embedded `__APOLLO_STATE__` in the page) rather
  than parsing the challenge-gated HTML.
- **New host names to expect in the sniff:** `api.cloud.seek.com.au`,
  `graphql-dark-prod.cloud.seek.com.au` (both present in the `au.seek.com` homepage HTML;
  Apollo client), alongside the legacy `www.seek.com.au/api/*` routes.
- Tier/permission hints from 4xx body: none observed (no auth tier — it is a challenge
  gate, not an entitlement gate).
- Probe-safe endpoint: `GET https://www.seek.com.au/api/jobsearch/v5/search?siteKey=AU-Main&keywords=developer&page=1` (read-only, unauthenticated, returns bounded JSON).

## Top Workflows
1. **Daily hunt with dedup.** Run a saved query (keywords + location + filters), see only
   listings that are new since the last run, export the delta to JSON/CSV, pipe to an LLM
   for match scoring. Today this needs a scraper + a hand-rolled "seen IDs" file.
2. **Salary research.** Pull every listing for a role/location, extract the salary labels,
   compute a local histogram / percentiles. SEEK shows per-listing salary text and has a
   salary-insights surface; nobody exposes an aggregate over live listings.
3. **Company/advertiser scan.** Given an advertiser, list all their current openings +
   company profile + reviews + rating. Useful for "who is hiring for X" and for
   interview prep.
4. **Hiring-trend monitoring.** Track listing volume by classification / region / work
   type over time in a local store; chart the trend. Recruiters and analysts do this
   manually in spreadsheets.
5. **Full-text recall over jobs you've already seen.** Offline FTS across the descriptions
   of every job the CLI has fetched, so "that Go role in Brisbane with the 4-day week"
   is findable weeks later even after the listing expires.

## Table Stakes
(union of `tomquirk/seek-com-au-api`, `qinscode/SeekSpider`, and the paid Apify actors
`blackfalcondata/seek-scraper`, `parseforge/seek-scraper`, `scrapestorm/...`)
- Keyword + location search
- Filters: classification/subclassification, work type (full/part/contract/casual),
  salary range, date posted, remote/work-from-home, "SEEK-listed only"
- Pagination + bulk/all-pages export
- Job detail: full description, bullet points, work type, suburb, salary, apply URL
- Company/advertiser profile: name, logo, reviews, star rating, salary insights
- Incremental / delta tracking ("new since last run")
- Compact structured output for AI agents (explicitly a selling point of the Apify actors)
- CSV + JSON output
- AU **and** NZ coverage (`siteKey` switch)

## Data Layer
- **Primary entities:** `job` (id, title, advertiser_id, advertiser_name, teaser, content,
  bullet_points, salary_label, salary_min/max parsed, work_type, work_arrangement,
  classification, subclassification, area/suburb, city, listing_date, expiry, apply_url,
  is_seek_listed, source_site AU|NZ); `advertiser` (id, name, logo, review_count,
  overall_rating, salary_rating, culture_rating); `saved_search` (name, query params,
  last_run_at, last_seen_max_listing_date); `job_seen` (job_id, first_seen_at,
  last_seen_at, search_name).
- **Sync cursor:** `listingDate` descending is the natural high-water mark per saved
  search; fall back to a job-ID set for reposts that reuse an old date.
- **FTS/search:** SQLite FTS5 over `job.title + teaser + content + bullet_points`;
  filter columns on classification, work_type, salary_min, area, source_site.

## Codebase Intelligence
- No official SDK. `tomquirk/seek-com-au-api` (Python, unmaintained) exposes only
  `search(keywords, limit)` and `get_listing(id)` — thin.
- `qinscode/SeekSpider` (Scrapy) is the best endpoint reference:
  - Search: `GET https://www.seek.com.au/api/jobsearch/v5/search`
  - Params seen: `siteKey=AU-Main`, `sourcesystem=houston`, `where=<location>`, `page`,
    `seekSelectAllPages=true`, `classification`, `subclassification`, `include=seodata`,
    `locale=en-AU`, plus `keywords`
  - Response: `data[]` job objects with `id`, `title`, `listingDate`,
    `locations[].label`, `advertiser.{id,description}`, `workTypes[]`, `salaryLabel`,
    `classifications[].subclassification.description`; `totalCount`; `solMetadata.pageSize`
  - Detail page `https://www.seek.com.au/job/{id}` — Cloudflare-gated (see Reachability).
- Apify actors confirm the valuable derived surfaces: company profiles, salary
  aggregation, incremental tracking, "compact output for AI agents."

## User Vision
- None provided ("Let's go").

## Product Thesis
- **Name:** `seek-pp-cli` (binary), slug `seek`.
- **Why it should exist:** Every existing option is either a thin unmaintained Python
  wrapper, a Scrapy project you must host and wire to Postgres, or a paid per-run Apify
  actor. There is no local-first, single-binary tool that does keyword/location/filter
  search, resolves the Cloudflare-gated job details via the SPA's own JSON endpoint,
  keeps a local SQLite history of everything it has seen, computes salary distributions
  and hiring trends offline, and emits agent-native output — for both AU and NZ — with
  no API key and no infrastructure.

## Build Priorities
1. **Search** — `seek jobs search` with keywords, location, classification, work type,
   salary range, date-posted, remote, AU/NZ site; pagination + `--all`; JSON/CSV/table.
2. **Job detail** — `seek jobs get <id>` via the SPA JSON endpoint discovered in 1.7
   (never the challenge-gated HTML); full description + bullets + apply URL.
3. **Local store + delta** — persist every job + advertiser to SQLite; `seek jobs new`
   (delta since last run for a saved search); FTS via `seek jobs search --local`.
4. **Saved searches + watch** — `seek search save/list/run`, `seek watch <name>` diffing
   over time.
5. **Salary + trends** — `seek salary <role> --location` (local histogram/percentiles
   over fetched listings); `seek trends --by classification|region` over the local store.
6. **Company** — `seek company <advertiser>` profile + reviews + active listings.
