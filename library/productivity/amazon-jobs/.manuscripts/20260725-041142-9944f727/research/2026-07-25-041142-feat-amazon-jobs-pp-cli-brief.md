# Amazon Jobs CLI Brief

## API Identity
- Domain: Amazon's public careers site (amazon.jobs) backing JSON API.
- Users: job seekers targeting Amazon/AWS roles, recruiters, and agents doing labor-market research.
- Data profile: ~10k+ open reqs (hits capped at 10000 for broad queries), each a rich record with
  title, full description, basic/preferred qualifications, location(s), category, team, schedule,
  intern/manager flags, and posted/updated dates.
- Auth: NONE. Clean unauthenticated GET returns JSON. Immediately usable.

## Reachability Risk
- None. `GET https://www.amazon.jobs/en/search.json` -> HTTP 200, application/json, no cookies/key.
- Go encoding/json parses responses cleanly (verified). No bot-protection/challenge observed.
- Probe-safe endpoint used: GET /en/search.json (read-only).

## Confirmed Contract (see discovery/api-contract-notes.md)
- Search: GET /en/search.json
  - Server-side: base_query, normalized_country_code[], normalized_state_name[],
    normalized_city_name[], offset, result_limit, sort(recent|relevant).
  - hits = total (capped 10000 broad; accurate filtered). facets[] returns empty (no server aggregation).
- Get by id: base_query=<id_icims>&result_limit=1 returns the single full record.
- Category / schedule-type / intern-manager filters do NOT work as .json params -> CLIENT-SIDE filter
  over returned records (records carry job_category, business_category, is_intern, is_manager,
  job_schedule_type; these can be null -> NULL-safe filtering).
- No separate JSON detail endpoint; search results already carry the full description.

## Top Workflows
1. Keyword search for roles ("software engineer", "data scientist", "operations manager").
2. Filter by location (city/state/country) — the #1 job-seeker need.
3. Read a specific job's full description + qualifications by id.
4. "What's new" — detect newly-posted roles for a saved search since last check.
5. Bulk pull + local analysis (which cities/teams have the most SDE roles right now).

## Table Stakes (from competitors: shubhtoy & marcogdepinto scrapers, Parse.bot wrapper)
- Keyword search with pagination.
- Location filter (city/state/country).
- Fetch full job details by id.
- Sort recent vs relevant.
- Export to JSON/CSV.
- Category filter (client-side here, since server slugs don't work).

## Data Layer
- Primary entity: job (keyed by id_icims). Store full record + synced_at.
- Saved searches: named query + filters, last-synced cursor for diffing.
- Sync cursor: posted_date / updated_time + set of known id_icims per saved search -> enables "new since".
- FTS/search: FTS over title + description + qualifications for offline keyword search.

## Codebase Intelligence
- No official SDK/repo; competitors are thin ad-hoc scrapers. No amazon.jobs-specific MCP or CLI exists.
- General job-search MCPs (JobSpy, JobDataLake, Servation/job-search-mcp) are multi-board aggregators,
  NOT amazon.jobs-specific — no direct competitor to beat, only to exceed.

## Product Thesis
- Name: amazon-jobs — the agent-native Amazon careers CLI.
- Why it should exist: every existing tool is a one-off scraper or a paid wrapper. None offer a local
  store, offline FTS, "new since last check" diffing, or agent-native JSON/select output. A native Go
  CLI with SQLite turns amazon.jobs from a website you refresh into a queryable, watchable dataset —
  no auth, no scraping fragility (uses the same JSON the site does).

## Build Priorities
1. Core: search (keyword + location + sort + pagination), get-by-id, full-record + clean-text output.
2. Data layer: sync a search into SQLite, offline FTS search, SQL access.
3. Transcendence: "new since" diff for saved searches, local aggregate stats, saved-search tracking,
   client-side category/intern/manager/schedule filters over synced or live data.
