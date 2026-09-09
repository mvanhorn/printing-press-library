# SEEK CLI — Absorb Manifest

Run: 20260908-143250-21abe609 · Target: `au.seek.com` (from-website) · Spec source: browser-sniffed

## Ecosystem surveyed

| Tool | Kind | Contributed |
|------|------|-------------|
| `tomquirk/seek-com-au-api` | Python wrapper (unmaintained) | search, get_listing |
| `qinscode/SeekSpider` | Scrapy project | classification/region search, job detail, salary normalization |
| `blackfalcondata/seek-scraper` (+ `parseforge`, `easyapi`, `unfenced-group` Apify actors) | Hosted scraper actors (paid) | all search filters, incremental NEW/UPDATED/EXPIRED tracking, content hashing, repost detection, structured salary, company intelligence, screening questions, compact AI mode, JSON/CSV/Excel, extracted emails, applicant-demand signals |
| Apify MCP wrappers (`automation-lab/seek-scraper`, `websift/seek-job-scraper`) | MCP servers over the actors | same feature set via MCP |
| `patrickvicente/job-hunter-mcp` | MCP | application tracking, candidate fit scoring |
| `Sakshee5/jobapply-mcp-server` | MCP | posting analysis, résumé/cover-letter optimization |
| SEEK native (browser-sniff) | — | saved searches, saved jobs, applied/saved status, recommended jobs, "how you match", salary nudge |

No dedicated SEEK CLI or native SEEK MCP server exists. Every option is a thin Python
wrapper, a self-hosted Scrapy project, or a paid per-run Apify actor.

## Absorbed (match or beat everything that exists)

| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Keyword + location job search | tomquirk/seek-com-au-api `search`; Apify seek-scraper | `seek-pp-cli listings search --keywords --where` | Offline SQLite cache, `--json`/`--agent`/`--select`, typed exit codes |
| 2 | Multi-filter search (work type, salary band, date posted, remote/hybrid, classification, sort) | Apify blackfalcondata/parseforge seek-scraper | `(generated endpoint) listings search` | All filters one command; entity-alias resolution (`full-time`→242, `remote`→2) |
| 3 | Full job detail (description, salary, apply URL, bullets) | qinscode/SeekSpider; Apify | `seek-pp-cli listings get <id>` | Uses the SPA GraphQL `jobDetails` endpoint, never the Cloudflare-gated HTML |
| 4 | Result count for a filter set without fetching rows | (novel to CLIs) | `seek-pp-cli listings count` | Cheap probe before a full search |
| 5 | AU + NZ coverage | Apify seek-scraper (6 countries) | `(behavior in seek-pp-cli listings search)` `--site AU-Main\|NZ-Main` on every command | Single binary, no per-country config |
| 6 | Pagination / bulk export | Apify (max 25 pages) | `(behavior in seek-pp-cli listings search)` `--page`/`--page-size` + framework `sync` | CSV + JSON + agent output |
| 7 | Structured salary (min/max/currency/type parsed from text) | Apify seek-scraper `salaryMin/Max/...` | `(behavior in seek-pp-cli listings search)` parse `salaryLabel` into structured SQLite columns | Queryable salary columns, not just a label |
| 8 | Company / advertiser profile (industry, size, website, phone, reviews rating + count, verified flag) | Apify company intelligence; SEEK `companyProfile` | `seek-pp-cli listings get <id>` (hand-built: generator GraphQL path cannot bind a positional to a query variable) companyProfile block | Bundled with job detail, one call |
| 9 | Incremental / delta tracking (NEW / UPDATED / EXPIRED, content hash) | Apify seek-scraper incremental mode | `(behavior in seek-pp-cli listings search)` local `job_seen` table, first/last-seen + content hash | Runs locally, free, no hosted actor |
| 10 | Repost detection (relisted same advertiser + title) | Apify `isRepost/repostOfId` | `(behavior in seek-pp-cli listings search)` dedupe pass over local store | Local, deterministic |
| 11 | Compact / AI-agent output mode | Apify "compact mode" | `(behavior in seek-pp-cli listings search)` `--agent`/`--compact`/`--select` | Framework-standard, dotted-path field selection |
| 12 | Saved searches / job alerts list | SEEK native `viewer.apacSavedSearches` | `seek-pp-cli me saved-searches` (cookie auth) | First tool to expose these outside the website |
| 13 | Saved jobs list | SEEK native `viewer.savedJobs` | `seek-pp-cli me saved-jobs` (cookie auth) | — |
| 14 | Saved / applied status for a set of job IDs | SEEK native `viewer.searchSavedJobs/searchAppliedJobs` | `seek-pp-cli me job-status` (cookie auth) | Bulk "have I applied to any of these?" |
| 15 | "How you match" — skills/credentials extracted from a JD | SEEK native `GetMatchedQualities` | `(generated endpoint) listings matched-qualities <id>` | — |
| 16 | Recommended / similar jobs for a listing | SEEK native `JobDetailsRecommendedJobs` | `(generated endpoint) listings recommended <id>` | — |
| 17 | Application tracking (applied / interviewing / offer / rejected) | patrickvicente/job-hunter-mcp | `(behavior in seek-pp-cli applications)` local `applications` table with status; framework `analytics --type application --group-by status` for funnel stats | Local, private, no account needed |

**Stubs:** none. All 17 absorbed rows ship fully.

## Transcendence (only possible with our approach)

Minimum 5 required — 6 survived the adversarial cut. All `hand-code` (each ~50–150 LoC
Go + `root.go` wiring). Full audit trail (customer model, 14 candidates, 8 kills) in
`2026-09-08-143524-novel-features-brainstorm.md`.

| # | Feature | Command | Buildability | Score | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|-------|------------------------|------------------|
| 1 | Salary distribution over live listings | `salary <role> --where <loc> [--classification <id>] [--site] [--job <id>]` | hand-code | 9/10 | Local SQLite aggregation (p10/25/50/75/90, histogram, disclosure rate) over parsed `salary_min`/`salary_max`; SEEK exposes no aggregation endpoint | Use `salary` for percentile/histogram statistics on disclosed pay from listings in the local store. Do NOT use it for a live count of jobs in a pay band — that is `listings facets`. Do NOT use it for listing-volume trends over time — that is `trends`. |
| 2 | Hiring-volume trend series | `trends --by classification\|subclassification\|region\|worktype\|workarrangement [--since 90d] [--keywords <kw>]` | hand-code | 9/10 | Period-over-period series from retained `job_seen.first_seen_at` history joined to `job`; the API returns only point-in-time counts | Use `trends` for how listing volume changes over time from local `job_seen` history. For a point-in-time breakdown of a live query with no sync, use `listings facets`. For flat group-by over the whole local store, use framework `analytics --type job`. |
| 3 | Company hiring scan | `company <advertiser> [--site] [--active]` | hand-code | 9/10 | Enumeration + GraphQL `companyProfile` + `advertiser`×`job`×`job_seen` local join in one call vs many | Use `company` to profile an advertiser and enumerate all their current openings. For the company block attached to a single posting, use `listings get <id>`. |
| 4 | Live facet breakdown | `listings facets --keywords <kw> --where <loc> [--group-by classification\|worktype\|salary\|region]` | hand-code | 8/10 | Surfaces the `facets`/`sortModes` block every scraper discards; zero rows fetched, no sync | Use `listings facets` for a live, no-sync breakdown of a query across classification / work type / pay band / region. For a single total, use `listings count`. For trends or percentile stats, sync first and use `trends` or `salary`. |
| 5 | Classification taxonomy resolver | `classifications [list \| resolve <term>]` | hand-code | 7/10 | Curated reference of SEEK's numeric classification/subclassification tree (`// pp:novel-static-reference`), refreshed from search-response facets | Use `classifications` to discover the numeric IDs for `listings search --classification`. Work-type, salary and date filters are resolved inline by `listings search`. |
| 6 | New listings across every saved search | `me new-jobs [--search <name>] [--since <7d>]` | hand-code | 9/10 | Orchestrates cookie-auth `viewer.apacSavedSearches` + per-search query + local `job_seen` anti-join + agent output; no single endpoint does this | Use `me new-jobs` to run every SEEK-native saved search and return only postings not yet in the local store. To list saved-search definitions only, use `me saved-searches`. |

**Hand-code commitment:** 6 features, all `hand-code`. Auto-emitted from the spec:
6 endpoint commands (`listings search/get/count`, `me saved-searches/saved-jobs/job-status`)
plus `matched-qualities` / `recommended` if added at generate.

**Risks flagged for the gate:**
- Company reviews payload was not captured in the sniff (`shouldDisplayReviews` /
  `companyProfile` confirmed present; the ratings sub-shape is inferred). `company` and
  `listings get` degrade gracefully if the field set differs.
- `me/*` commands need `auth login --chrome`; cookie replay outside the browser is
  unverified until first login / Phase 5 live smoke.
- Hand-written GraphQL queries are leaner than SEEK's persisted queries; if SEEK tightens
  its schema, `listings get` / `me *` queries may need a field trim.
