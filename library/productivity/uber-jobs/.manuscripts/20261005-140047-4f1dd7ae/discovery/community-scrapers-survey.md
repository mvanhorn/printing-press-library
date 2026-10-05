# Community survey: existing Uber careers clients (5 Oct 2026)

Done by an Opus research subagent: code and text reading only, with zero requests to uber.com or oraclecloud.com. Claims are as reported by that agent, with source URLs. INFERRED marks the agent's own inferences. The orchestrator has not re-verified these.

## Tools
| Tool | URL | Last activity | Target | Status |
|---|---|---|---|---|
| headstart UberScraper | https://github.com/sarthakjain004/headstart (src/headstart/scrapers/uber.py; docs/uber/2026-09-11_api-measurement.md) | pushed 2026-10-04; PR #446 merged 2026-09-12 | jobs.uber.com /api/jobs/search/ | working (measured 11-28 Sep) |
| ats-scrapers UberScraper (PyPI ats-scrapers 0.3.0, released 2026-09-02) | https://github.com/kalil0321/ats-scrapers (168★; PR #271) | Uber fix PR on 2026-08-23 | jobs.uber.com /api/jobs/search/ | working (710/710 unique) |
| freehire uber adapter | https://github.com/strelov1/freehire (816★; PR #2047; internal/ingest/sources/uber.go) | 2026-08-17 | jobs.uber.com /en/jobs/sitemap.xml + per-job JSON-LD | working (684) |
| fetchaller-mcp search_uber_jobs | https://github.com/Averyy/fetchaller-mcp (src/fetchaller/oracle_recruiting) | pushed 2026-09-28 | Oracle ORC iaziqy | working |
| jobradar uber.mjs | https://github.com/adityasingh2400/jobradar (radar/sources/custom/uber.mjs) | pushed 2026-10-05 | Oracle ORC, siteNumber UberCareers | working per comments |
| ApplyKit UberProvider (Swift) | https://github.com/dasautoooo/ApplyKit (BigTechProviders.swift) | pushed 2026-09-26 | Oracle ORC, siteNumber CX_1 | working per comments |
| techjobsnl uber.rs | https://github.com/imhalawa/techjobsnl (src/sources/uber.rs) | pushed 2026-09-05 | Oracle ORC, NL only | working |
| Argus ats/uber.py; ShawnLi14 job-scraper; JobHunt; job-hunter; FAANG-2027 trackers | (see agent report) | Feb-Oct 2026 | legacy www.uber.com/api/loadSearchJobsResults (Playwright or POST) | dead or failing: 404 since about 6-8 Aug 2026 |
| ApplyPilot employers.yaml | https://github.com/Pickle-Pixel/ApplyPilot | n/a | Workday uber.wd5 | INFERRED wrong |

## Reachability evidence
- The legacy `POST www.uber.com/api/loadSearchJobsResults` returns 404 from about 6 Aug 2026 (FAANG-2027-Internships-Tracker issues #6 and the Europe #3-#8; freehire PR #2047 "gone (404)"; kalil0321's docstring "retired … in August 2026"). `uber.com/careers/*` 301s to jobs.uber.com/en/jobs/ (headstart notes, 11 Sep).
- `/api/jobs/search/` gets 403 `cf-mitigated: challenge` from stock curl and python-requests (headstart, 11 Sep 2026). It gets 200 from curl_cffi impersonate="chrome", 18/18 at about 13 req/s with "No rate limit found" (headstart), and from httpcloak, locally and from a VPS (kalil0321 PR #271). freehire found JA3/uTLS alone NOT enough, because "the edge fingerprints the HTTP/2 layer too". It uses bogdanfinn/tls-client with Chrome_144 plus a matching UA.
- freehire PR #2047 says /en/jobs/sitemap.xml "is not behind the challenge". NOTE: this URL is not in the owner's HAR or the site JS, so it is not requestable under owner rules unless the owner approves it, for example via robots.txt's Sitemap line.
- headstart reports robots.txt as `User-Agent: * / Allow: /` with a Sitemap line. UNVERIFIED here; the owner is saving robots.txt from Chrome.
- No 403/Cloudflare issues were filed against headstart, fetchaller-mcp, Argus or jobradar.

## Contract facts for GET /api/jobs/search/ (from working clients)
- `page` starts at 1. The request key is lowercase `pagesize`, and the response echoes `pageSize`. Both working clients send only page and pagesize.
- NO server cap on pagesize: headstart tried 100, 1,000, 10,000 and 100,000, and each returned everything once pagesize >= totalJobs. totalPages recalculates (2 gives 259; 200 gives 3).
- MULTI-PAGE WALKS ARE UNRELIABLE (headstart, 12 Sep): walks at 100 and at 200 per page each got 519 of 520, missing a different job each time, with 1 duplicate. One page of 1000 gave exactly 520 unique, twice. headstart therefore does a probe (pagesize=1) and then one page sized to totalJobs. kalil0321 pages at 1000 and fails hard on a totalJobs change, a repeated id, or unique != totalJobs.
- Past the last page (`page=999`) the reply is `{"jobs": [], "totalJobs": N, …}`.
- Headers used: `Accept: application/json`, plus optional `Referer: https://jobs.uber.com/en/jobs/`. No x-csrf-token, no cookies. headstart sends UA `headstart/0.1` over curl_cffi Chrome impersonation.
- The listing Description is byte-identical to the job page's JSON-LD description (3 of 3), so no detail call is needed.
- Data quirks (headstart): Remote false on 517/517; Salary numeric fields null on 517/517 (Salary.Description prose on 250/517); 57 of 538 Descriptions are whole HTML documents with `<title><p> Cleaned Document </p></title>` (28 Sep); 27.9% of jobs list 2-5 locations (headstart issue #538). DisplayDate is stable, spread 19 Jun to 11 Sep.
- Totals over time (different clients): 710 (23 Aug), 684 sitemap (17 Aug), 640 Oracle (about 2 Aug), 517 (11 Sep), 520 (12 Sep), 535 (22 Sep), 538 (28 Sep), and 578-581 on 5 Oct (this run).
- No client uses search/location/countries/team/subTeam/contractTypes/experience/businessUnits/lat/lng/radius, so there is NO community evidence on filter keys, multi-value encoding or sort keys. Rule 1 must prove them live.

## Oracle ORC facts from clients (context for the fallback)
- `…/recruitingCEJobRequisitions?onlyData=true&expand=requisitionList[.secondaryLocations]&finder=findReqs;siteNumber=…,limit=200,offset=N[,keyword="…"][,location="…"][,sortBy=POSTING_DATES_DESC][,facetsList=NONE]`. Reads items[0].requisitionList[] (Id, Title, PrimaryLocation, secondaryLocations[].Name, PostedDate, Department, JobFamily, WorkplaceType, ShortDescriptionStr) and items[0].TotalJobsCount.
- Page cap is 200 (jobradar; ApplyKit: "caps a page at 200 however large a limit"). `expand=requisitionList` is MANDATORY; without it the list is silently empty with a correct count (fetchaller). The `location` filter takes country names and silently ignores cities ("Canada" 640→13, "Toronto" returns 640) (fetchaller).
- siteNumber conflict in community code (CX_1 vs UberCareers). This run VERIFIED CX_1 from the site page (data-sitenumber) and got 200s with it.
- Detail: `recruitingCEJobRequisitionDetails?expand=all&onlyData=true&finder=ById;Id="…",siteNumber=…`.
