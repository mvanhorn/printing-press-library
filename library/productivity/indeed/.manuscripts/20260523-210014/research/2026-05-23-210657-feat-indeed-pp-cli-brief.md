# Indeed CLI Brief

## API Identity
- Domain: World's largest job board. This CLI is read-only job *search & research* for job seekers.
- Users: Job seekers running repeated searches, salary/company research, recency sweeps; agents pulling job data into pipelines.
- Data profile: Jobs (key, title, company, location, salary, remote, dates, description), companies (rating, industry, size), salary snippets. SSR-embedded JSON.

## Reachability Risk
- **Low.** `probe-reachability` shows Surf (Chrome TLS fingerprint) clears Cloudflare on both
  the homepage and the live `/jobs` SERP (200; stdlib gets 403 `cf-mitigated: challenge`).
  Runtime = `browser_http`, no clearance cookie. Residual risk: Cloudflare could escalate to a
  JS challenge on the SERP path; validated live in Phase 5 dogfood. Chosen surface (public
  website SSR) is lower-risk and user-authorized vs JobSpy's iOS-app GraphQL endpoint (rejected;
  see discovery report).

## Top Workflows
1. **Keyword + location search** with filters (radius, date posted, sort, job type, remote) and pagination.
2. **Saved-query watch** — re-run a named search, surface only postings new since last run.
3. **Recency sweep** — `--posted 1 --sort date` for the freshest roles.
4. **Salary-floor filter** — parse extracted salary, drop anything under `--min-salary`.
5. **Multi-location fan-out** — same keyword across several cities, dedup by job key.
6. **Job detail + pipe-to-apply** — `job get <key>` for full description; print the apply URL for the human to open.

## Table Stakes (from JobSpy 3.4k★, indeed-scraper, linkedin-jobs-scraper)
- Multi-filter search: keywords, location, distance/radius, job type, remote, date posted, sort, pagination/limit.
- Salary parsing (min/max/unit/currency/source).
- Structured output: JSON + CSV export.
- Cross-run dedup by stable job key.
- Full job description retrieval.

## Data Layer
- Primary entities: `job` (key PK), `company` (name/slug PK), `saved_search` (named query), `search_run` (run cursor + which jobs).
- Sync cursor: dedup on stable `job.key`; `first_seen_at` is the durable delta key for "new since last run" (Indeed's `nextCursor` is per-session only, not durable).
- FTS/search: FTS5 over synced job title + company + description for offline search.

## Codebase Intelligence
- Source: JobSpy (`speedyapply/JobSpy`) — canonical scraper, confirms field shapes and filter semantics.
- Auth: none for public job search. (Saved jobs / applications need login — out of scope; read-only CLI.)
- Data model: job → embedded company (employer) + compensation + location + attributes.
- Rate limiting: Indeed web is lenient for reasonable use; CLI sets a conservative default delay between paged requests.
- Architecture: results are SSR-embedded JSON (`_initialData.mosaicProviderJobCardsModel`), not a JSON API — parsing is hand-built on top of the generated Surf client.

## User Vision
- "Search for jobs" is the core. User asked about submitting resumes; we explained auto-apply is
  not viable/advisable (ATS redirects, hardened Indeed Apply, ToS, irreversible action) and scoped
  to read-only search + research. `apply` only prints/opens the listing URL — never auto-submits.

## Product Thesis
- Name: **jobhunt** (binary `indeed-pp-cli`) — a job-seeker's command line for Indeed.
- Why it should exist: every existing tool is a one-shot scraper that dumps a CSV. None keeps a
  local history, so none can answer "what's *new* since I last looked?" or do offline full-text
  search across everything you've already pulled. Local SQLite makes those first-class.

## Build Priorities
1. **Foundation + search**: Surf client (browser-chrome), `_initialData` parser, `search` command with all filters, `--json/--select/--csv`, pagination, local store of every job seen.
2. **Absorb**: `job get` (full description via JSON-LD + sanitizedJobDescription), `company` info, `related` jobs, salary parsing, CSV export, dedup.
3. **Transcend**: saved searches, `new` (jobs since last run of a saved search), offline FTS `find`, `--min-salary` floor, multi-location fan-out dedup.
