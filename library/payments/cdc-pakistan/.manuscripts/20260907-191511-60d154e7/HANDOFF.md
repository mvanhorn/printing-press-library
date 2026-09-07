# CDC PAKISTAN CLI — HANDOFF (7 Sep 2026, ~22:5x PKT)

RUN_ID: 20260907-191511-60d154e7
PROMOTED: library/cdc-pakistan   scorecard 80/100 A, verify 100% PASS, 8 novel features
MARKER:   status pass, level full, 107/107, coverage_hollow ABSENT, 191 files fingerprinted
LOCK:     released. Manuscripts archived to manuscripts/cdc-pakistan/<run>/ (6.4MB, secret-scan clean)

## ============ SETTLED FACTS — DO NOT RE-DERIVE ============
TRANSPORT: whole site Cloudflare JS-challenged; /robots.txt is the ONLY fetchable path.
  NOT the TLS trap (curl_cffi chrome124/120/110 all 403). probe-reachability says
  browser_clearance_http and its surf-chrome probe ALSO 403s -- Surf is neither sufficient
  nor necessary. ONE headed real-Chrome visit mints cf_clearance, which then replays over
  PLAIN Go stdlib HTTP (verified with LibreSSL curl, the weakest stack here) on HTML AND PDFs.
  => spec is http_transport: standard + auth.type cookie. Do NOT "upgrade" it to browser-chrome.
CLEARANCE TTL: **HARD ~30 MINUTES FROM MINT, regardless of traffic.** Measured twice
  (alive t+25m / dead t+30m with traffic every <=5min; dead t+31m after 23min idle -- idle
  timeout FALSIFIED). The cookie's own `expires` claims 365 DAYS and is MEANINGLESS.
  Store (cookie, User-Agent, minted_at) as ONE unit -- the cookie is UA-bound.
MINIMAL HEADERS: User-Agent + Cookie only. Unlike MUFAP, X-Requested-With/Origin/Referer
  are NOT needed.
THE ONLY VALID ENUMERATOR: POST /wp-admin/admin-ajax.php with
  action=update_posts_by_year year=<2007..2026> paged=<N> cpt=downloads
  taxonomy=downloads_category term=<slug>. Unauthenticated, NO nonce. 10 items/page.
  /downloads-category/<cat>/page/N/ URL pagination is DECORATIVE -- byte-identical item
  block for every N (sha256 constant across pages 1-8). Never walk it.
YEAR FILTER keys on the UPLOAD date, NOT the document's as-of date (Security List Report:
  meta "30 April", upload 2026/05, filed under year_param 2026). Sweep ALL years and key on
  the parsed as-of date. Persist all three dates; derive none.
CORPUS: 7,956 distinct documents, all 300 (category,year) pairs settled, 0 transport errors.
  notices 4,332 + circulars 3,307 = the deep 20-year asset. 12 other categories total 317.
  1,150 docs have NO date-bearing path; 1,950 were bulk-uploaded in 2015 -> upload_year is
  useless as an as-of date for ~a third of the corpus.
FREE-FLOAT PANEL: ONE live vintage only (2025-11-30, split A/B). CDC REMOVES prior months.
  Wayback holds ~7 more (2022-12..2024-02) and stops. NOT a time series.
A/B SPLIT: by INSTRUMENT CLASS, zero ISIN overlap. A = "ORDINARY, PREFERENCE SHARES &
  MODARABA CERTIFICATES" (29,350). B = "OPEN END / ETF FUNDS / SAVING CERTIFICATES" (66).
  B has NO capital and NO percentage columns -- funds have units. NULL there means
  "not published for this instrument class", 0 means "published as zero" (CDC writes `-`).
PDF EXTRACTION: CDC switched producer from "Microsoft Excel 2016" (PDF 1.5) to
  "Microsoft: Print To PDF" (PDF 1.7). NO pure-Go reader handles the LIVE vintage:
  ledongthuc fails ("stream not present"), dslipak HANGS, pdfcpu extracts content streams
  not text. No system extractor installed. SOLUTION: macOS PDFKit via `swift` subprocess
  bridge (the SKILL-sanctioned pattern) -- 659 pages / 3.56M chars in 3.5s. Pure-Go kept as
  a fallback for Excel-era vintages. PDF features are macOS-only in practice; documented.
THREE ROW LAYOUTS + a debt variant, all derived empirically from real headers:
  equity-2025 (4 numerics) | equity-2024 (6, has Market Value, NO status) |
  funds-2025 (1) | debt (REORDERED: status BEFORE the live date, plus a DD-Mon-YY maturity)
NO ACCOUNTING NEGATIVES in CDC PDFs (full 21-page scan, zero `(digits)`). `-` means ZERO
  (736 cells). The MUFAP "(4.97) = -4.97" trap does NOT apply here.
FIVE date encodings incl. an AMBIGUOUS numeric one: both 10/24/2013 (only MM/DD) and
  13/03/2025 (only DD/MM) occur in ONE report. Raw preserved, ambiguity FLAGGED, never normalised.
"LISTED-" concatenation: status fuses with a zero dash with no separator in part B.
UN-LISTED normalises to UNLISTED (28,689 rows) or a group-by splits.
EXTRACTION QUALITY, measured FROM OUTSIDE the parser (ISINs in raw text vs rows emitted):
  2025-11-30 A 99.80% | B 100.00% | 2024-01-31 50.41% (Excel-era column interleaving).
INVARIANTS on the live vintage: excl<=incl 0/29,060 violations; pct==shares/capital*100
  0/29,060, worst 0.0050pp; ISIN check digit 29,291/29,291. Data is clean to rounding.

## ============ HARD NEGATIVES (answered, stop asking) ============
- HOLDER-LEVEL POSITIONS ARE NOT PUBLIC. cdcaccess.com.pk is a per-investor Angular login.
- THE 9-CLASS INVESTOR TAXONOMY IS NOT PUBLISHED. CDC has only Individual/Corporate/RDA
  (+Sahulat until deleted). NOT a third joinable source with NCCPL/MUFAP. Checked the
  statistics page, the pre-WordPress /UserPanel/ section, and the ~20k-URL archived corpus.
- WP REST API is a dead end: /wp-json/ root answers but every wp/v2/* is 401 (iThemes), and
  all 446 routes are plugin ADMIN surfaces. No CDC data namespace.
- cdcsrsl.com (Share Registrar) is separately challenged and NOT mapped.

## ============ GOVERNANCE ============
robots.txt: `User-agent: *` -> Allow: / with Content-Signal search=yes, **ai-train=no**,
use=reference, under an Article 4 EU DSM reservation; **ClaudeBot/GPTBot/CCBot/Bytespider/
Google-Extended/meta-externalagent/Amazonbot are each Disallow: /**. Surfaced to the owner,
who elected to proceed. The CLI must not identify as a named AI crawler and this data must
not feed model training.

## ============ NEXT STEPS ============
1. OPTIONAL publish: /printing-press-publish cdc-pakistan  (nothing blocks it; publish
   validate not run mid-pipeline by design).
2. RETRO IS UNFILED and has real material -- see the list below.
3. If you want the statistics history: only ~42 non-contiguous archived months exist
   (2015-09..2024-06, nothing after). Ship as provenance='archive', never as a series.

## ============ RETRO CANDIDATES (none filed) ============
1. FLEET-WIDE: generated printJSONFiltered never calls SetEscapeHTML(false), so EVERY
   printed CLI HTML-escapes & < > in JSON (& in titles and copy-paste hints).
2. command_mirror_capabilities truncates at the first clause boundary with no ellipsis --
   left the verify-rows MCP description dangling on "...its own share and".
3. dogfood dead-function FP: isDryRunResponseForClient has 3 call sites (helpers.go:455,520,570).
   Only successfulNoop is genuinely dead.
4. dogfood novel-feature depth resolver ambiguates DUPLICATE LEAF NAMES (top-level `stats`
   vs `learnings stats`) and reports a parent that isn't the registration.
5. dogfood config-consistency check emits regex captures (", token, ") as field names --
   fires on every cookie-auth CLI.
6. generated dead code: maxAge persistent flag (root.go), successfulNoop (helpers.go).
7. pp:typed-exit-codes honoured by the live matrix but NOT by scorecard's sample probe.
8. scorecard --live-check runs features concurrently against one SQLite file -> transient
   "database disk image is malformed (11)", not reproducible standalone.
9. 52 gosec findings in generated files (G202 in platform/migration.go which is
   byte-identical across mufap/nccpl/psx; G112 missing ReadHeaderTimeout in cmd/*-pp-mcp).
10. #4612 doctor HTML-200 recurred as predicted -- worked around with cdc_health pattern.

## ============ MY OWN ERRORS THIS RUN (recorded so they aren't repeated) ============
- Claimed a 365-day clearance TTL from the cookie's own field. Measure, don't read.
- Claimed the year filter "silently under-reports" from a 2-year sample. It was MY sampling.
- Reported "154 invariant violations" from a generic regex that compared a CAPITAL column
  against a PERCENTAGE column. Data was clean all along.
- Two substring bugs in statistics labels (Unlisted collapsing onto listed; Share Registrar
  keyed as a securities total).
- FOUR broken verification harnesses of my own (zsh word-splitting twice, a sed pattern, a
  grep -A6 that truncated a command list) that reported failures against working code.
- A silent python str.replace with no assert that no-op'd a fix I then reported as applied.
