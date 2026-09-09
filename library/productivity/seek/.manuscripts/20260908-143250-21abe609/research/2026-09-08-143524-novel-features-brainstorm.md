# SEEK CLI — Novel Features Brainstorm (subagent audit trail)

Run: 20260908-143250-21abe609 · first print (no prior research) · subagent: general-purpose

## Customer model

Four personas, grounded in the brief's **Users** (job seekers, salary researchers, recruiters doing market scans, data/analytics folks) and **Top Workflows** 1-5.

### Priya — active job seeker (software engineer, Sydney; also runs NZ searches)
- **Today:** SEEK website in a browser + a hand-kept spreadsheet of where she's applied. Re-scrolls the same keyword/location search every day looking for anything unfamiliar.
- **Weekly ritual:** Mon/Wed/Fri she runs 3-4 saved searches, opens ~15 listings, pastes the promising ones into a doc, tweaks her résumé per role, updates the applied-to spreadsheet.
- **Frustration:** Can't tell new listings from ones she saw yesterday; listings expire before she's decided; no bulk way to ask "have I already applied to any of these advertisers?"; a role she liked three weeks ago is now un-findable.

### Dan — independent recruiter / talent sourcer
- **Today:** Paid Apify actor runs + LinkedIn. Bills clients for "who is hiring for X in this market" snapshots.
- **Weekly ritual:** Monday scan of 5-6 classifications across AU **and** NZ; assembles a per-client competitor-hiring picture.
- **Frustration:** Per-run Apify cost; output is a flat job list with no company-level rollup; reposts inflate the counts and he dedupes them by hand.

### Mara — compensation analyst at a mid-size employer
- **Today:** Manually samples 30-40 SEEK listings per benchmark role each quarter, eyeballs the salary text into Excel, computes a median.
- **Weekly ritual:** Pulls salary labels for 2-3 benchmark roles, hand-parses ranges, maintains a spreadsheet.
- **Frustration:** Salary text is unstructured and often absent; no percentile/distribution view; the sample is stale the moment she stops collecting; no sense of what share of postings even disclose pay.

### Sam — labour-market analyst / data journalist
- **Today:** Ad-hoc scrapes into notebooks; every pull is a fresh snapshot with no history.
- **Weekly ritual:** Tracks ICT and healthcare listing volume by region; watches the remote/hybrid share of postings move over time.
- **Frustration:** No back-history, so no trend line; work-arrangement and classification breakdowns aren't retained between runs.

## Candidates (pre-cut)

- **A `salary`** — percentiles/histogram/disclosure-rate over local salary columns. Persona: Mara. Source (b)+(c). KEEP.
- **B `trends`** — time-series listing counts by classification/region/worktype with period deltas. Persona: Sam, Dan. Source (c). KEEP.
- **C `company`** — advertiser profile + reviews + all current openings + local count history. Persona: Dan, Priya. Source (b)+(c). KEEP (reviews payload not captured in sniff — minor data risk).
- **D `search "text" --type job`** — offline FTS recall. Persona: Priya. Source (a). KILL — framework `search` already emits this.
- **E `listings facets`** — per-facet counts from the search response, no rows/sync. Persona: Dan, Mara. Source (b). KEEP.
- **F `classifications`** — browse/resolve SEEK's numeric classification IDs. Persona: all. Source (b). KEEP.
- **G `me new-jobs`** — run every SEEK-native saved search, return only postings not in local job_seen. Persona: Priya. Source (b)+(c). KEEP.
- **H `me saved-jobs --check`** — flag expired/changed saved jobs. Persona: Priya. Source (c). KILL — thin, `isActive` already returned.
- **I `analytics --type application --group-by`** — funnel stats over local applications. Persona: Priya. Source (c). KILL — framework `analytics` covers it.
- **J `listings compare`** — side-by-side of 2-5 listings. Persona: Priya. Source (c). KILL — speculative, no evidence.
- **K `listings market <id>`** — one job's salary percentile. Persona: Priya, Mara. Source (b)+(c). KILL — fold into `salary --job`.
- **L `me salary-benchmark`** — surface `viewer.salaryNudge`. Persona: Mara. Source (b). KILL — thin wrapper.
- **M `watch <name>`** — background poll of a saved search. Persona: Priya. Source (a). KILL — persistent process = scope creep.
- **N `company reviews <advertiser>`** — standalone rating breakdown. Persona: Dan. Source (b). KILL — bundled into `company` and `listings get`.

## Survivors and kills

### Survivors

| # | Feature | Command | Persona | Score | Buildability | How It Works | Evidence | Long Description |
|---|---------|---------|---------|-------|--------------|--------------|----------|------------------|
| 1 | Salary distribution over live listings | `seek salary <role> --where <loc> [--classification <id>] [--site AU-Main\|NZ-Main] [--job <id>]` | Mara | 9/10 | hand-code | Computes p10/25/50/75/90, histogram buckets and disclosure rate in local SQLite from synced `job.salary_min`/`salary_max`; no API aggregation endpoint exists. `--job <id>` ranks one job in the distribution. | Brief workflow 2 + Build Priority 5 ("nobody exposes an aggregate over live listings"); Apify seek-scraper parses salary but has no aggregate (2 sources) | Use `salary` for percentile/histogram statistics on disclosed pay from listings in the local store. Do NOT use it for a live count of jobs in a pay band — that is `listings facets`. Do NOT use it for listing-volume trends over time — that is `trends`. |
| 2 | Hiring-volume trend series | `seek trends --by classification\|subclassification\|region\|worktype\|workarrangement [--since 90d] [--keywords <kw>]` | Sam, Dan | 9/10 | hand-code | Buckets `job_seen.first_seen_at` by week/month joined to `job`, emits counts per bucket with period-over-period deltas; framework `analytics` gives only flat point-in-time group-by. | Brief workflow 4 + Build Priority 5; Apify incremental mode retains snapshots but has no trend view (2 sources) | Use `trends` for how listing volume changes over time from local `job_seen` history. For a point-in-time breakdown of a live query with no sync, use `listings facets`. For flat group-by over the whole local store, use framework `analytics --type job`. |
| 3 | Company hiring scan | `seek company <advertiser> [--site AU-Main\|NZ-Main] [--active]` | Dan, Priya | 9/10 | hand-code | Filters the REST search to one advertiser, attaches GraphQL `jobDetails.companyProfile`, joins `advertiser`×`job`×`job_seen` for local opening-count history. | Brief workflow 3 + Apify company-intelligence surface (2 sources) | Use `company` to profile an advertiser and enumerate all their current openings. For the company block attached to a single posting, use `listings get <id>`. |
| 4 | Live facet breakdown | `seek listings facets --keywords <kw> --where <loc> [--group-by classification\|worktype\|salary\|region]` | Dan, Mara | 8/10 | hand-code | Reads the `facets`/`sortModes` block already returned by `GET /api/jobsearch/v5/search`; per-facet counts, zero rows fetched, no sync. | Discovery report §7 confirms `facets` in the search response; no competitor exposes facet counts (1 source) | Use `listings facets` for a live, no-sync breakdown of a query across classification / work type / pay band / region. For a single total, use `listings count`. For trends or percentile stats, sync first and use `trends` or `salary`. |
| 5 | Classification taxonomy resolver | `seek classifications [list \| resolve <term>]` | all personas | 7/10 | hand-code | Curated reference of SEEK's numeric classification/subclassification IDs (`// pp:novel-static-reference`), seeded/refreshed from search-response facets. | Brief data profile ("a large numeric classification/subclassification taxonomy") + SeekSpider documents the params with no lookup (2 sources) | Use `classifications` to discover the numeric IDs for `listings search --classification`. Work-type, salary and date filters are resolved inline by `listings search`. |
| 6 | New listings across every saved search | `seek me new-jobs [--search <name>] [--since <7d>]` | Priya | 9/10 | hand-code | Reads cookie-auth `viewer.apacSavedSearches`, runs each saved query, anti-joins against local `job_seen`, tags each hit with its saved-search name. | Brief workflow 1 (the top workflow) + SEEK-native `apacSavedSearches` with `newToYouCountLabel` (2 sources) | Use `me new-jobs` to run every SEEK-native saved search and return only postings not yet in the local store. To list saved-search definitions only, use `me saved-searches`. |

### Killed candidates

| Feature | Kill reason | Closest surviving sibling |
|---------|-------------|---------------------------|
| D. Offline recall (`search "text" --type job`) | Framework `search --type <resource>` already emits offline FTS; table-stakes plumbing, not novel. | G `me new-jobs` |
| H. Saved-job drift (`me saved-jobs --check`) | `viewer.savedJobs` already returns `isActive`; drift already computed by the delta; no evidence of need. | G `me new-jobs` |
| I. Application funnel stats (`analytics --type application`) | Framework `analytics --group-by` covers it over local `applications`; nothing SEEK-specific. | B `trends` |
| J. Listing side-by-side (`listings compare`) | Speculative — no community complaint or competitor gap. | C `company` |
| K. One-job salary percentile (`listings market <id>`) | Duplicates `salary`; folded in as `salary --job <id>`. | A `salary` |
| L. Salary nudge (`me salary-benchmark`) | Single authenticated field, thin wrapper — fails wrapper-vs-leverage. | A `salary` |
| M. Saved-search watch (`watch <name>`) | Persistent background process = scope creep; poll-once form collapses into `me new-jobs`. | G `me new-jobs` |
| N. Company reviews (`company reviews <advertiser>`) | Bundled into the `company` scan and `listings get` companyProfile block. | C `company` |
