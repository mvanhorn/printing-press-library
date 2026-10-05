# Crowd-sniff report: uber-jobs (2026-10-05)

Command: `cli-printing-press crowd-sniff --api uber-jobs --base-url https://jobs.uber.com --output research/uber-jobs-crowd-spec.yaml --json`. It ran from 09:34:45Z to 09:35:04Z, rc 0, through a local guard proxy (HTTPS_PROXY=127.0.0.1:18080) that logs every CONNECT and refuses uber.com and oraclecloud.com. The proxy log is in crowd-sniff-guardproxy-log.tsv.

## Network audit (owner checkpoint: the ledger must gain no uber.com line)
- Hosts contacted by crowd-sniff: api.github.com (1 CONNECT), registry.npmjs.org (1), api.npmjs.org (1). There was NO uber.com and NO oraclecloud.com connection attempt.
- The only jobs.uber.com line in the proxy log (09:34:17Z, DENIED) is the orchestrator's proxy self-test, run before crowd-sniff. The proxy refused it before dialing, so no TCP connection reached Uber. It is not an Uber request and is not added to uber-request-ledger.tsv.
- The binary does honour the proxy: GitHub and npm traffic went through it, so an Uber attempt would have been logged and refused.

## 1. npm Packages Analyzed
- The npm search returned no Uber careers SDK. stderr shows `crowd-sniff: downloads API returned status 400` (the npm downloads API), so no packages yielded endpoints. This matches the research agent: no `uber-jobs` or `uber-careers` package exists on npm or PyPI.

## 2. GitHub Repos Searched
- GitHub code search was authenticated with the owner's gh token, passed to this one command and never printed or stored. crowd-sniff's internal queries are not echoed. 3 code-search hits came back (tier code-search).
- For broader context, see community-scrapers-survey.md. That subagent found headstart, kalil0321/ats-scrapers and freehire (current site), fetchaller-mcp, jobradar, ApplyKit and techjobsnl (Oracle), plus several dead legacy scrapers.

## 3. Endpoints Discovered
| Method | Path | Source Tier | Source Count | Disposition |
|---|---|---|---|---|
| GET | /api/app/all | code-search | 1 | REJECT: not seen in the owner's HAR or in the site's JS (owner rule: request only URLs from the HAR or JS). Unrelated code. |
| GET | /api/loadSearchJobsResults | code-search | 1 | REJECT: the legacy www.uber.com RPC, 404 since about 6-8 Aug 2026, and it was a POST. Owner rule: never call it. |
| GET | /api/role | code-search | 1 | REJECT: not in the HAR or JS. Unrelated code. |

## 4. Base URL Resolution
- Supplied explicitly as https://jobs.uber.com (from the HAR). crowd-sniff found no competing base URL.

## 5. Auth Patterns Detected
- None (`auth.type: none`). This matches every working community client: no cookie, no token, no CSRF.

## 6. Parameter Name Evidence
- None from crowd-sniff (param_count 0). The parameter evidence comes from the site JS (JOB_QUERY_KEYS) and the community clients (page, pagesize) in community-scrapers-survey.md.

## 7. Coverage Summary
- 3 endpoints found, all rejected, so 0 are merged. The enrichment merge is therefore a no-op. Phase 2 generates from the browser-sniff spec (after noise removal), not from `--spec … --spec uber-jobs-crowd-spec.yaml`.
- Gaps that crowd-sniff did not fill, but research named: `/en/jobs/sitemap.xml` (freehire; NOT in the HAR or JS; requestable only with owner approval, for example if robots.txt lists it), per-job JSON-LD pages (/en/jobs/{id}/; seen in the HAR as RSC payloads), and the Oracle CE finder variants (documented by Oracle; covered in oracle-ce-research.md).
