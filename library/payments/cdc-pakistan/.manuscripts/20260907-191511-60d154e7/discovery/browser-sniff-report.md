# CDC Pakistan — browser-sniff discovery report
Run: 20260907-191511-60d154e7 · 7 Sep 2026 · anonymous capture, isolated Chrome profile

## Consent / method
Owner approved driving a browser (scope gate). Capture used the system Google Chrome
BINARY with a FRESH throwaway profile dir — the operator's own Chrome profile, cookies
and logins were never opened or read. No login was performed anywhere; CDC needs none.

## Transport verdict — REPLAYABLE (browser_clearance_http, cookie-only)
- Every path on www.cdcpakistan.com is Cloudflare-challenged (403, `cf-mitigated: challenge`)
  EXCEPT /robots.txt. Static /assets/uploads/*.pdf are challenged too.
- NOT the TLS-fingerprint trap: curl_cffi chrome124/120/110 all 403. Genuine JS challenge.
- press `probe-reachability` -> mode browser_clearance_http, needs_clearance_cookie true;
  its surf-chrome probe ALSO 403s, so Surf alone is insufficient.
- ONE headed real-Chrome visit clears it in <5s. cf_clearance then replays over
  PLAIN stdlib HTTP (LibreSSL curl) -> 200 on every page AND on the PDFs.
  => spec http_transport: standard. NO Surf, NO browser-chrome, NO resident browser.
- Minimal replay header set: User-Agent (must match the minting browser) + Cookie: cf_clearance.
  X-Requested-With / Origin / Referer are NOT required (differs from MUFAP).
- cf_clearance stated TTL = 365 days (issued 2026-09-07, expires 2027-09-07). MEASURED
  AND REFUTED: the cookie's own `expires` field is untrustworthy. Server-side validity is
  a HARD ~30 minutes from mint, and it is NOT an idle timeout -- the cookie was in
  continuous use throughout. Evidence in `cf-ttl-measurement.log` (same directory):
  HTTP 200 at t+0/60/180/360/600/900/1200/1500s, then 403 at t+1800s. Treat 30 minutes
  from mint as the usable window and re-mint past it.

## The real API (the find)
POST /wp-admin/admin-ajax.php   (unauthenticated; NO nonce)
  action=update_posts_by_year  year=<2007..2026>  paged=<N>
  cpt=downloads  taxonomy=downloads_category  term=<category-slug>
Returns a rendered HTML fragment of `.download_list` items. 10 items/page.

Item shape:
  div.download_list > div.row > div > h4              -> title
                                  div.meta            -> "DD Month"  <-- NO YEAR
                            > a[download][href]       -> /assets/uploads/YYYY/MM/<file>.pdf

## WordPress REST API — DEAD END
/wp-json/ root answers 200 (512KB route doc, 446 routes) but every /wp-json/wp/v2/* is
401 `itsec_rest_api_access_restricted` (iThemes Security). All 446 routes are plugin ADMIN
surfaces (iThemes, Wordfence, WPML, SiteGround, Pods, Popup Maker, Redirection, Slider Rev).
There is NO custom CDC data namespace. Not probed further; admin routes deliberately untouched.

## TRAPS FOUND (all verified, not inferred)
1. URL PAGINATION IS BROKEN. /downloads-category/<cat>/page/N/ returns the IDENTICAL
   10-item set for N=1..8 (sethash 115323c67793). The "335 pages" on circulars and
   "48 pages" on notices are decorative. Naive walking yields ~3,350 duplicates of 10 items.
   ONLY admin-ajax `paged` advances (verified: 5 distinct hashes, descending dates).
2. YEAR FILTER UNDER-REPORTS. list-of-securities returns 0 items for year=2024 AND
   year=2026, yet its unfiltered page demonstrably holds 2 files. Year filter is NOT a
   complete enumeration. The two enumeration paths DISAGREE -> reconciliation gate required.
3. ITEM DATES CARRY NO YEAR. div.meta is "30 December". Year must come from the `year`
   request param and/or the /assets/uploads/YYYY/MM/ path. These can disagree (a doc dated
   31 Aug may be uploaded in Sep) -- record both, never silently pick one.
4. STATISTICS SCHEMA DRIFTS. Apr-2024 vs Jul-2026 row labels differ:
   "Number of Shares" -> "Number of Shares/Debt Instruments";
   "Number of Securities" SPLIT into Listed / Unlisted;
   "Total Number of Sahulat Accounts" REMOVED.
   Hardcoding the label set silently drops rows. Key on normalized label + unknown bucket.
5. FILE-NAME SUFFIX VARIANTS. The monthly free-float report is now split into TWO files
   (`...-30-November-2025-A.pdf` and `-B.pdf`); Security List Report carries a WordPress
   dedup suffix (`-30-April-2026-1.pdf`). Treating -A as the whole month is a partial slice.
6. FREEZE/status markers are embedded in the company-name string inside the PDFs
   ("... - FREEZE ***", "(FREEZE)", trailing "*"), not a separate column.
7. `-` in PDF numeric cells means zero, and must not be conflated with missing.
   Accounting-negative "(x)" form NOT yet checked in these PDFs -- must verify before use.

## Data surfaces
DATE-PARAMETERISED (backfillable via admin-ajax year x paged):
  circulars, notices (CDS-eligibility declarations/suspensions, symbol+name changes),
  disciplinary-registers, guidelines, procedures, newsletter, publications,
  quarterly-accounts, annual-reports, forms, designated-time-schedule,
  tariff-fee-structure, miscellaneous, sustainability-report  (15 categories, 2007..2026)

CURRENT-STATE ONLY (no date param):
  /about-us/statistics/  -- 16-row HTML table, monthly-refreshed, "Facts (As of July-2026)"

MONTHLY DOCUMENT SERIES (the prize; inside `miscellaneous`, live):
  Share-Percentage-in-CDS-with-respect-to-paid-up-Capital-as-of-<DD Month YYYY>.pdf
    per-security: ISIN | company | symbol | date | available shares in CDS | market value
                  | paid-up capital incl. GoP | paid-up capital excl. GoP
    21 pages, ~1.7MB, text extracts cleanly. Newest live: 30 Nov 2025 (split A/B).
  ISIN-with-CFI-Code-as-of-<date>.pdf   (newest live: 23 Dec 2025)
  Security-List-Report-<date>.pdf       (newest live: 30 Apr 2026)

## NEGATIVE RESULTS (report these, do not paper over)
- HOLDER-LEVEL POSITIONS ARE NOT PUBLIC. cdcaccess.com.pk is an Angular SPA login keyed to
  an individual investor's own credentials. No public holder-level feed exists at any depth.
- THE 9-CLASS INVESTOR TAXONOMY IS NOT PRESENT. CDC exposes only Individual / Corporate
  (+ RDA; Sahulat until it was dropped). It is NOT a third joinable source on the
  NCCPL/MUFAP taxonomy. Checked: statistics page, pre-WordPress /UserPanel/ section
  (static Statistics.html, no data endpoints), and the ~20k-URL archived corpus.
- cdcsrsl.com (CDC Share Registrar Services) is Cloudflare-challenged as well; not mapped.

## GOVERNANCE (operator's own machine-readable statement)
/robots.txt: `User-agent: *` -> `Allow: /` with
  `Content-Signal: search=yes, ai-train=no, use=reference`,
under an express Article 4 EU DSM reservation of rights. ClaudeBot, GPTBot, CCBot,
Bytespider, Google-Extended, meta-externalagent, Amazonbot are each `Disallow: /`.
Read strictly: generic automated access is Allowed and use is "reference"; AI TRAINING is
refused; the named AI crawlers are refused. Surfaced to the owner, who elected to proceed.
The printed CLI must not identify as ClaudeBot and this data must not feed model training.
