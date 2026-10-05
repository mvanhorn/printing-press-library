# Uber Jobs CLI Brief

Run 20261005-140047-4f1dd7ae · press v4.33.0 · skill 3.0.0 · written 2026-10-05 (UTC). Labels: VERIFIED (measured, with a source), INFERRED, UNVERIFIED.

## API Identity
- Domain: Uber's public careers postings. There are two surfaces that serve the same requisitions:
  - **Primary: the Uber careers site, jobs.uber.com.** Next.js App Router on Vercel behind Cloudflare. The backing JSON read is `GET https://jobs.uber.com/api/jobs/search/?<query>` → `{jobs[], totalPages, totalJobs, page, pageSize}`, 10 rows per page by default (VERIFIED from the owner's DevTools HAR, 5 Oct 2026). The JS also calls `POST /api/jobs/recently-viewed/` `{jobIds, locale}` → `{jobs:[{id,title,url,locations,teams}]}` (a batch lookup), and `POST /api/jobs/recommendations` `{viewJobIds, maxJobs}` (JS only, not captured). The search index behind it is Azure AI Search, index `uber-careers-v2-production` (VERIFIED: an `@odata.context` in a detail payload).
  - **Fallback: Uber's public Oracle Recruiting Cloud candidate-experience API, site UberCareers.** Tenant `iaziqy.fa.ocs.oraclecloud.com`, siteNumber `CX_1`. These are documented Oracle REST resources: `recruitingCEJobRequisitions` (finder `findReqs`) and `recruitingCEJobRequisitionDetails` (finder `ById`) under `/hcmRestApi/resources/11.13.18.05/` (VERIFIED, with 3 owner-approved requests on 5 Oct, all 200 over Go stdlib).
- Users: job seekers tracking Uber openings across markets, career coaches and recruiters, and agents driving a job-tracker pipeline. The owner's job-tracker script already drives an equivalent CLI for another employer (amazon-jobs-pp-cli) and needs the same contract here.
- Data profile: about 578-581 live public postings (5 Oct 2026: HTML total 578, API total 581, Oracle TotalJobsCount 578). Each posting has an id that is a 6-digit requisition number shared by both surfaces, title, an HTML description, an ISO UTC DisplayDate, Teams and SubTeams, ContractType, WorkPattern, Remote, a Salary block (pay transparency text in HTML), and Locations[] (City, Region, Country as a name, CountryCode null, a GeoJSON point). Multi-location postings exist (2 of 10 sampled). Facet lists are embedded in the /en/jobs/ RSC payload: 37 countries, 18 teams, 53 subTeams, teamToSubTeamMappings, contractTypes, workPatterns. A small, slowly changing corpus with daily churn (Oracle postingDatesFacet on 5 Oct: <7 days 127, <30 days 314, >30 days 264).

## Reachability Risk
- **High for non-browser clients on jobs.uber.com (VERIFIED, one data point).** On 5 Oct 2026, a plain `curl` GET of https://jobs.uber.com/en/ with browser headers got HTTP 403, `cf-mitigated: challenge`, `<title>Just a moment...</title>` (a Cloudflare managed challenge). The owner's Chrome loaded the same site with no visible challenge, and the HAR has no challenge-platform request. `critical-ch` asks for the full Sec-CH-UA set. The runtime transport is still undecided; Phase 1.9 (probe-reachability) settles it.
- **Phase 1.9 reachability gate (2026-10-05): PASS.**
  - 09:52:18Z: the plain macOS curl check (LibreSSL, curl UA) on GET /api/jobs/search/ got HTTP/2 403, `cf-mitigated: challenge`, "Just a moment..." (5,409 B; cf-ray a45b88635e60516f-KHI). The 4xx body had no tier/permission keywords.
  - 09:56:42Z: probe-reachability `--probe-only stdlib` on the same URL got **200 application/json** in 2,438 ms, mode standard_http 0.95. Its headers were the probe default `User-Agent: printing-press/<version> (probe-reachability)` and `Accept: text/html,application/json;q=0.9,*/*;q=0.8`, over Go stdlib TLS.
  - The Surf rung was NOT run, because stdlib passed and the owner asked for the minimum number of requests. The runs were split to keep the owner's 3 s spacing.
  - Reading: the challenge keys on the client stack (macOS curl), not on a non-browser UA (S5). This is a single probe, so S6 applies: sustained-volume evidence comes from the discovery contract tests.
- **Search replies are edge-cached:** `cache-control: public`, `x-vercel-cache: STALE`, `age: 15081` (about 4.2 h) on the HAR's search call. Totals differ across surfaces in one session (HTML 578 vs API 581). New-since logic must tolerate hours of lag.
- **Low-to-medium on the Oracle fallback (VERIFIED, 3 requests):** Go stdlib got 200 on all 3. Akamai fronts the tenant (`ak_bmsc` cookie, `Akgrn` header), so it is Akamai Bot Manager, unproven at volume.
- Community evidence (discovery/community-scrapers-survey.md):
  - The two maintained scrapers of `/api/jobs/search/` reach it only with Chrome TLS+HTTP/2 impersonation. headstart uses curl_cffi impersonate="chrome" (18/18 at 200 on 11 Sep 2026); kalil0321/ats-scrapers uses httpcloak (PR #271, 23 Aug 2026).
  - Stock curl and python-requests get 403 `cf-mitigated: challenge` (headstart, 11 Sep).
  - freehire (PR #2047) reports that uTLS/JA3 alone is NOT enough because "the edge fingerprints the HTTP/2 layer too".
  - No client uses cookies or a CSRF token.
  - The legacy `www.uber.com/api/loadSearchJobsResults` has returned 404 since about 6-8 Aug 2026 (FAANG-2027-Internships-Tracker issues; freehire PR #2047). It is never used.
- Oracle fallback (discovery/oracle-ce-research.md):
  - Some tenants show intermittent Akamai 403s on valid requests (jobseek #6394, Aug 2026), 503 bursts (#2217) and transient 302s (#5715).
  - A browser User-Agent is required.
  - Oracle's REST reference labels the Recruiting CE endpoints "only for Oracle internal use". This is a TERMS FLAG for the owner, not a stop; Uber's own candidate site calls them anonymously.
- **Terms flag (owner-reviewed 2026-10-05T11:31:47Z):** the Uber General Terms of Use (Uber B.V., PK jurisdiction, modified 8/12/2026) limit use to personal noncommercial use and prohibit "programs or scripts for the purpose of scraping, indexing, surveying, or otherwise data mining" and "mirror or frame". robots.txt is `Allow: /` with no AI-crawler block. Owner rule: a flag, not a stop. Revisit at Phase 6 (see discovery/terms/terms-review.md).
- Owner rule, which overrides community practice: never retry a 403, 429 or challenge on any host.

## Top Workflows
1. **Tracker pull:** list recent postings for one ISO3 market, optionally keyword-scoped, paged with --limit/--offset, sorted by posting date, as one top-level JSON envelope (`postings --country GBR --limit 100 --offset 0 --sort recent --base-query <kw> --json --data-source live`).
2. **What's new since I last looked:** saved searches, plus a new-since report that baselines on the first run and only advances after a complete, uncapped scan.
3. **Market and team scan:** sync the whole corpus locally (about 58 search pages), then answer stats by category, team or country offline.
4. **Screen out disqualifiers:** description contains / not-contains filters (sponsorship, relocation, language) and posted-within, applied with no HTML noise.
5. **Look up one posting by id** and confirm it still exists. Apply links point at the Oracle candidate-experience site, and the CLI never applies.

## Table Stakes
These come from the existing Uber clients (headstart, ats-scrapers, freehire, fetchaller-mcp, openings-mcp) and general job-search tools (JobSpy, py-linkedin-jobs-scraper).
- A full-corpus pull deduped by id that fails loudly when the unique count != totalJobs (ats-scrapers). headstart measured that 100- and 200-row page walks each dropped one job and duplicated another, while a single page sized to totalJobs (pagesize has no server cap) was exact.
- Keyword search; location/country filter; team/category filter; remote/workplace filter; job type (contract type, work pattern).
- Posted-within, in hours or days (JobSpy hours_old; LinkedIn time filters).
- limit + offset paging; results_wanted.
- The full description with a text/HTML choice; the absolute job URL from `Urls[IsDefault]`; all locations for multi-location postings (27.9% of Uber postings list 2-5).
- Facet listing (openings-mcp `facets`) and get-by-id/detail with a typed not-found (openings-mcp ErrJobNotFound).
- JSON and CSV output.
- Not table stakes anywhere: saved searches, new-since/alerts and offline stats. Only app-level repos have them, so they are our differentiators.

## Data Layer
- Primary entities: `postings` (one row per requisition id; Id == Reference); `posting_locations` (every location, primary first); teams and subteams; a `facets` snapshot (countries, teams, subTeams, mappings) as the source of filter values and of the country-name → ISO3 map.
- Sync cursor: none on the API side. Full sync pages /api/jobs/search/ to the declared total, dedupes by id within and across pages, and records first_seen/last_seen per posting plus per-saved-search baselines. Postings not seen in a complete sync are marked closed, not deleted.
- FTS/search: FTS5 over title, team, subteam, location and HTML-stripped description.

## Codebase Intelligence
- Source: reading the community source code (not DeepWiki): headstart src/headstart/scrapers/uber.py and docs/uber/2026-09-11_api-measurement.md; kalil0321/ats-scrapers src/ats_scrapers/scrapers/uber.py; freehire internal/ingest/sources/uber.go; fetchaller-mcp src/fetchaller/oracle_recruiting.
- Auth: none. No cookie, no CSRF, no custom header. Headers used: `Accept: application/json`, plus optional `Referer: https://jobs.uber.com/en/jobs/`.
- Data model: one row per `Id` (== `Reference`), with `Locations[]`, `Teams[]`, `Urls[]` and `Salary{…}`. The listing Description is byte-identical to the job page's JSON-LD description (headstart 3/3), so no detail call is needed. 57 of 538 Descriptions are whole HTML documents titled "<p> Cleaned Document </p>", so the HTML strip must handle full documents. Remote is false and Salary numbers are null on every sampled row.
- Rate limiting: none observed on jobs.uber.com once past Cloudflare (headstart 18/18 at about 13 req/s). None observed on Oracle either (headstart 6,351 requests, no ratelimit headers). The owner's ≥3 s pacing applies regardless.
- Architecture: Next.js App Router over an Azure AI Search index (uber-careers-v2-production). Postings originate in Oracle Recruiting Cloud (the same requisition numbers; the apply URL is `…/UberCareers/jobs/preview/<Id>/apply/email`). Pages are edge-cached up to hours (`x-vercel-cache: STALE`, age about 4.2 h).

## User Vision
Primary source: the Uber careers site, jobs.uber.com. Fallback: Uber's public Oracle Recruiting Cloud candidate-experience API (site UberCareers), used automatically only when jobs.uber.com refuses a request; both serve the same requisition ids, and meta.source names the source that answered. Public postings, read-only, no login. A job-tracker script already drives an equivalent CLI for another employer, so uber-jobs must match its contract:
- a postings command taking --country <ISO3> --limit --offset --sort recent --base-query --json --data-source live;
- output is always a top-level JSON object, never a bare array, with one envelope for every command and data source: {meta:{source}, hits, returned, scanned, scan_cap_hit, results:[...]};
- per posting: id, title, posted_date in Month D, YYYY form (what the tracker parses), posted_on (ISO date), posted_raw (the site's own value), job_category, country_code (ISO alpha-3), location, normalized_location, locations[], basic_qualifications, preferred_qualifications, description (HTML stripped), is_intern, university_job, job_path, url (absolute), employer=uber. Null where the site has no such field, never invented;
- doctor prints "API: reachable" only after a positive content check;
- DNS or transport failure has its own documented exit code, outside 2, 3, 4, 5, 7 and 10;
- --agent output keeps description.
Candidate ideas, not requirements: saved searches with a new-since command; sync plus stats by category; posted-within; description contains and not-contains filters; get by id.

## Source Priority
- Primary: **jobs-uber-com** (the Uber careers site, jobs.uber.com). Spec state: no official spec, so it comes from browser-sniff of the owner's HAR (`--har`). Auth: free, none.
- Secondary (automatic fallback only): **oracle-ce-ubercareers** (Oracle Recruiting Cloud CE API, site UberCareers). Spec state: official vendor REST documentation (docs.oracle.com, REST API for Oracle Fusion Cloud HCM) plus 3 verified live responses. Auth: free, none.
- **Economics:** both are free and keyless, so there is no paid-key scoping.
- **Inversion risk:** the fallback has formal vendor docs, and the primary has only a sniffed spec. Do NOT let documentation completeness invert the order. jobs.uber.com owns every headline command, and Oracle is used only when jobs.uber.com refuses. Cost asymmetry also favours the primary: jobs.uber.com search rows carry the full description at 10 per page (about 58 requests for the corpus). Oracle search rows carry no description and no category (0 of 25), so it needs one detail call per posting (about 24 + 578 requests).

## Product Thesis
- Name: uber-jobs-pp-cli (slug uber-jobs; display name "Uber Careers" for prose)
- Why it should exist: no maintained tool gives agents a stable, contract-tested feed of Uber's public postings. This CLI matches the job-tracker contract the owner already depends on (one envelope, ISO3 countries, tracker-parsable dates), keeps a local store for new-since and stats, and survives Cloudflare refusals by falling back to Uber's own Oracle candidate-experience API instead of returning empty results.

## Build Priorities
1. `postings` list on /api/jobs/search/ with the tracker contract: country (ISO3, mapped through the facet list), offset/limit, sort recent proven against posting date, base-query → search, one envelope. Paging strategy follows the community measurements, to be re-proven live under owner rule 2. pagesize is uncapped and multi-page walks drift, so the likely design is a size probe (pagesize=1, reading totalJobs) plus ONE page sized to the total, with offset/limit applied client-side and unique-count == totalJobs asserted. That is about 2 requests per full scan instead of about 58.
2. Typed reachability: positive content check on every 200, Cloudflare challenge → exit 7 (never 4), a distinct DNS/transport exit code, `doctor` printing "API: reachable" only after a content check, cross-process ≥3 s pacing, no retry on 403/429/challenge, and the automatic Oracle fallback with meta.source.
3. Local store and sync with first_seen/last_seen, closed-not-deleted, saved searches and new-since.
4. get-by-id with an id-equality assertion (recently-viewed/ batch lookup, or a search fallback), plus description filters and posted-within over HTML-stripped text.
5. stats by category, team and country from the store.
