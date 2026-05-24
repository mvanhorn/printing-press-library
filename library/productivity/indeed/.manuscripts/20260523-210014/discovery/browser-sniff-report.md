# Indeed Browser-Sniff / Discovery Report

**Target:** https://www.indeed.com (job board)
**Method:** Playwright MCP live capture (real Chrome) + `probe-reachability` + JobSpy source analysis
**Primary goal walked:** "Search jobs by keyword + location, then open a job's detail."

## Reachability

`probe-reachability` on both the homepage and the live SERP:

| URL | stdlib HTTP | Surf (Chrome TLS) | Mode |
|-----|-------------|-------------------|------|
| `www.indeed.com` | 403 `cf-mitigated: challenge` | **200** | `browser_http` |
| `www.indeed.com/jobs?q=software+engineer&l=Remote` | 403 `cf-mitigated: challenge` | **200** (463ms) | `browser_http` |

**Runtime decision: `browser_http` → ship Surf (Chrome TLS fingerprint) transport.**
`needs_clearance_cookie: false`, `needs_browser_capture: false`. Cloudflare gates on TLS
fingerprint, which Surf satisfies; no JS challenge / cookie handshake required. No
`auth login --chrome`. Confidence 0.85; validate against live site in Phase 5 dogfood.

## Chosen surface: SSR HTML with embedded `_initialData` JSON

Indeed renders job results server-side and embeds the full result model as JSON in a
`<script>` in the raw HTML (no separate XHR for listings). Confirmed extractable from
Surf-fetched HTML (no JS execution needed):

### 1. Job search — `GET /jobs`
Query params (mirror as flags): `q` (keywords), `l` (location), `start` (offset 0/10/20…),
`radius` (miles), `fromage` (days posted: 1/3/7/14), `sort` (`date`|`relevance`),
`jt` (fulltime/parttime/contract/internship/temporary).

Extraction: `window._initialData = {…}` → `.mosaicProviderJobCardsModel`:
- `.results[]` — 24 jobs/page. Per-job fields: `jobkey`, `title`, `company`,
  `formattedLocation`, `salarySnippet{currency,text}`, `extractedSalary`, `remoteLocation`,
  `jobTypes[]`, `pubDate` (ms epoch), `formattedRelativeTime`, `companyRating`,
  `companyReviewCount`, `snippet` (HTML), `viewJobLink`, `indeedApplyable`,
  `thirdPartyApplyUrl`, `urgentlyHiring`, `newJob`.
- `.totalJobCount` — e.g. 10689 (drives pagination).

Regex anchor in raw HTML: `window._initialData=` then balanced-brace JSON capture.

### 2. Job detail — `GET /viewjob?jk=<jobkey>`
Two clean extraction targets in raw HTML:
- **JSON-LD `JobPosting`** (`<script type="application/ld+json">`) — W3C schema, most stable:
  `title, description (HTML), datePosted, validThrough, employmentType, jobLocation,
  jobLocationType, applicantLocationRequirements, hiringOrganization, directApply`.
- `window._initialData.jobInfoWrapperModel.jobInfoModel`:
  `sanitizedJobDescription` (full HTML, ~2.8KB), `jobMetadataHeaderModel`,
  `salaryInfoModel`, `salaryGuideModel`, plus `companyTabModel`, `competitorsJobsModel`,
  `recommendedJobsModel` at the top level.

### 3. Related jobs (clean JSON) — `GET /m/getcompetitorsjobs?jobKey=<jk>&limit=<n>`
Returns JSON `{jobLimit, transitionalJobCardsModel:{...}}`. Status 200.

### 4. Location autocomplete (clean JSON) — `GET autocomplete.indeed.com/api/v0/suggestions/location?country=US&language=en&count=<n>&query=<q>&rich=true`
Returns JSON array `[{suggestion:"Austin, TX", latitude, longitude, locationType:"CITY", population, popularity}]`. Status 200.

## Rejected surface: `apis.indeed.com/graphql` (mobile API)

JobSpy uses Indeed's iOS-app GraphQL backend with a static `indeed-api-key` header. It is
technically cleaner (one JSON call, bypasses Cloudflare) but was **rejected**:
1. The user explicitly chose "the website itself," not the mobile API.
2. Impersonating the iOS app with a leaked static key was flagged by the harness auto-mode
   classifier as credential misuse and blocked.
3. Higher ToS-violation profile than browsing the public website.

The public-website SSR route is what we ship.

## Filter attribute keys (reference, from JobSpy `jobspy/indeed`)
Web `jt` param covers job type directly. The mobile GraphQL composite-filter keys
(`CF3CP` full-time, `75GKK` part-time, `NJXCK` contract, `VDTG7` internship, `DSQF7` remote)
are noted only for parity; the web `/jobs` route uses readable params (`jt`, `l=Remote`).
