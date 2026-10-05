# jobs.uber.com backing API: confirmed contract (Go stdlib, honest UA, no auth)

Measured 2026-10-05 from 11:14:40Z to 11:16:45Z with discovery/contract/plan-01.tsv: 36 requests, sequential, >= 3.5 s apart. Identity: `User-Agent: uber-jobs-pp-cli/0.1.0`, `Accept: application/json` (`text/html` for the facets page), no cookies. Raw bodies and headers are in contract/out-01/. The analysis script output is contract/analysis-01.txt (uber-jobs-prep/analyze_contract.py). There were 0 refusals: no 403, 429 or challenge. One 500 (see NOT WORKING).

Base URL: https://jobs.uber.com. ALWAYS send the trailing slash; the form without it answers 308 (HAR).

## Endpoint: GET /api/jobs/search/?<query>
Envelope: `{jobs[], totalPages, totalJobs, page, pageSize}`. An error adds `error: "Failed to search jobs"` (seen only on the 500).

### CONFIRMED (server-side; every returned row matches; nonsense → 0)
- `page` (1-based) and `pagesize`: pagesize is honoured far above 10. `pagesize=1000` returned all 584 rows, unique 584 == totalJobs 584, totalPages 1. `pagesize=1` returned totalJobs 584 with 1 row (the size probe). The default with no params is pageSize 10, totalPages 59. No result cap was seen at 1000.
- Past the end: `page=7&pagesize=100` gave `jobs:[]` with totalJobs 584; `page=999&pagesize=10` gave `{"jobs":[],"totalPages":59,"totalJobs":584,"page":999,"pageSize":10}`. Stop on an empty page.
- Pagination advances. The 100-row walk p1-p7 returned [100,100,100,100,100,84,0] = 584, unique 584, 0 dups, 0 missing vs the single page, with distinct page hashes 11494483, 7649bd16, e0059b55, 1ffd6f01, f9c33273, a906c85f, e3b0c442 (empty). But order differs by page size: walk order != single-page order (page 1 matched). The single page sized to totalJobs is therefore the canonical full read, and dedupe by Id is required regardless (community measurements saw drift on other days).
- `search=<kw>`: zzqqxvnonsense gave 0; engineer gave 221 (100/100 returned rows contain "engineer"); operations gave 427 (it matches more fields than title and description: Azure AI Search, team labels). Server-side keyword search. It maps to `--base-query`.
- `countries=<Country name>` (the name from the countries facet, NOT ISO): Zzqqland gave 0; Germany gave 22 (22/22 match; the corpus has primary-location Germany on 21 and any-location on 22, so it MATCHES ANY LOCATION); United States gave 289 (100/100). REPEATED KEY = OR: `countries=Germany&countries=Canada` gave 36 (36/36, the union).
- `team=<Team>`: Zzqq gave 0; Engineer gave 111 (== corpus 111); Finance gave 14 (== 14).
- `subTeam=<SubTeam>`: Zzqq gave 0; Business Operations gave 19 (19/19); Customer Support gave 22 (exact subteam, == the corpus AdditionalText suffix count of 22).
- `contractTypes=<value>`: Zzqq gave 0; "Full time" gave 577 (== corpus 577; 7 rows have an empty ContractType).
- `location=<text>`: Zzqqville gave 0; Berlin gave 12 (12/12).
- `lat`/`lng`/`radius` (miles; the UI default is 100): lat=52.52&lng=13.4&radius=50 gave 12 (12/12, the same set as location=Berlin). Never call Google Places; take explicit coordinates only.

### NOT WORKING / NOT EXPOSED
- `countries[]=Germany` (bracket form) is SILENTLY IGNORED: it returned 584 (4/100 rows match), so the spec must use the plain repeated key.
- `businessUnits=Zzqq` gave HTTP 500 `{"error":"Failed to search jobs","jobs":[],"totalJobs":0}`. There are no known valid values (no facet), so do NOT expose it. It is a 5xx and never retried.
- `experience=Zzqq` gave 0. The key is applied, but the experienceLevels facet is empty and no row has an ExperienceLevel, so do not expose it.
- There is NO sort key (JOB_QUERY_KEYS has none). The default order is by `@search.score`, monotonic non-increasing on 584/584, which is relevance with freshness, NOT date: only 309 of 583 adjacent pairs are DisplayDate-non-increasing, and the first inversion is at row 150. `--sort recent` MUST be a client-side sort on DisplayDate over the full read.

### Caching (record, never assume totals agree)
- A non-default page size is fresh: `pagesize=1000`, `pagesize=1` and `pagesize=100` all came back `cache-control: public`, `x-vercel-cache: MISS`, `age: 0`, cf-cache-status DYNAMIC, with totalJobs 584.
- The default-size URL `?page=2` came back STALE: `x-vercel-cache: STALE`, `age: 14841` (about 4.1 h), totalJobs **578**. That is the same stale object the owner's HAR saw (578 vs 581). Hours-old totals sit behind the default-size URLs.
- Design rule: always send an explicit, non-default `pagesize` (the size probe uses 1 and the full read uses the probed total). Even so, new-since must tolerate hours of lag on any surface.
- The facets HTML (/en/jobs/) returned `x-vercel-cache: HIT`, age 144, totalJobs 584.

## Row shape (584 rows, live)
- `Id` is a string and equals `Reference` on 584/584. Use it as the posting key.
- `DisplayDate`: ONE shape, `YYYY-MM-DDTHH:MM:SSZ`, on 584/584 rows; range 2026-06-19T07:30:00Z → 2026-10-05T10:05:14Z.
- **DATE FLOOR:** 63 of 584 rows (10.8%) have exactly `2026-06-19T07:30:00Z`. All of them are old-series ids (146988-160351); the next-earliest real value is 2026-07-06T17:43:38Z. This is a migration floor, not the posting date. The CLI must flag these rows (for example `posted_date_is_floor: true`) and keep posted_raw verbatim. They are always outside short recency windows. To decide in build: posted_on for floor rows (the owner rule says never invent).
- DisplayDate is the posting date: JSON-LD `datePosted` == the date part of DisplayDate on 9/9 detail pages (HAR), and Oracle `ExternalPostedStartDate` == DisplayDate exactly on 1/1 (302906, 5 Oct). Rule 6 is satisfied for non-floor rows.
- `Locations[]`: 160 of 584 rows (27.4%) have more than one location. `CountryCode` is null on all 842 location entries, and `Country` is a name. The corpus has 38 countries, one of them (Saudi Arabia) NOT in the HAR's 37-entry facet list but present in today's live facets (38). The ISO3 map must cover the LIVE facet list, and unmapped names must be reported.
- `Teams[]` is empty on 17 rows. WorkPattern: Regular 549, Intern 10, Fixed Term 7, Direct NCG Hire 1, empty 17. ContractType: "Full time" 577, empty 7. Remote: true on 0 rows.
- `Description` is never empty. 64 rows wrap it in a whole HTML document (`<title><p> Cleaned Document </p></title>`), so the HTML stripper must handle full documents.
- Salary: `Description` is non-empty on 301; MinValue/MaxValue are null on 584/584. The US sentence "The base salary range for this role is USD $X per year - USD $Y per year" matches on 228 of 301. Parse only that shape; otherwise null plus salary_text.

## Endpoint: POST /api/jobs/recently-viewed/ (a read; mutation: false)
- Body `{"jobIds":["302906","160425","999999999"],"locale":"en"}` returned 200 `{"jobs":[{id,title,url,locations,teams}]}` with **2 rows**. The fake id was silently dropped, so missing ids are absent rather than errors. The `url` has no trailing slash (`/en/jobs/302906`). Cache: `private, max-age=60, stale-while-revalidate=30`.
- It is usable as a batch existence check. get-by-id must still assert id equality and exit not-found when absent.

## Endpoint: POST /api/jobs/recommendations/ (JS only)
- `{"viewJobIds":["302906"],"maxJobs":5}` returned 200 `{"jobs":[]}`: empty for a valid id. No value was shown, so it stays out of the CLI (killed C14 stands).

## Page: GET /en/jobs/ (text/html), the facet source (rule 3)
- 200, 207 KB, `x-vercel-cache: HIT`. The RSC payload carries countries **38**, teams 18, subTeams 53, contractTypes 1, workPatterns 4, totalJobs 584. No JSON-LD on the list page.
- No CSRF token and no custom header were needed on any endpoint over the runtime transport (rule 3 recheck done).

## Detail fetch needed? No.
Search rows carry the full HTML Description. The community found it byte-identical to the job page JSON-LD (headstart 3/3). The detail-only fields (UpdateDate, SubTeams, ApplicationUrl) are either derivable (subteam from AdditionalText; apply URL from the pattern `…/UberCareers/jobs/preview/<Id>/apply/email?mode=location`) or not needed.

## Request budget implication
A full read is 2 requests (the pagesize=1 probe, then one page sized to the total). The tracker's per-country call is 2 requests (probe plus full, with the countries filter). Today's total is 3 diagnosis + 36 contract = 39 of the 300/day cap.

## Owner decision (2026-10-05, about 11:20Z)
- Date-floor rows (DisplayDate == 2026-06-19T07:30:00Z): posted_date = null, posted_on = null, posted_date_is_floor = true, posted_raw = DisplayDate verbatim. Non-floor rows: posted_on = the UTC date of DisplayDate, and posted_date = "Month D, YYYY" with no zero padding.
