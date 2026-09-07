# SUMMARY (loop complete, 7 Sep 2026 ~21:10 PKT)
ALL 7 BLOCKERS SETTLED. Decisive facts: (B1) cf_clearance has a HARD ~30-min lifetime from
mint regardless of traffic -- its own 365-day `expires` field is meaningless; every command
must check a ~27-min margin up front and the backfill must chunk, checkpoint and resume.
(B2/B6) The free-float panel is a RICH CROSS-SECTION (29,416 securities, Nov-2025, A+B clean
partition) but NOT a time series: 1 live vintage, ~7 archived, and the two comparable vintages
have incompatible schemas and a 24x universe gap. (B3) There is exactly ONE valid enumerator,
admin-ajax; URL /page/N/ pagination is decorative, and `year_param` keys on upload date not
as-of date. (B5) No accounting negatives; ISINs verify 1,206/1,206. (B7) Statistics history is
~42 non-contiguous archived months, best-effort only. TWO OF MY OWN EARLIER CLAIMS WERE WRONG
AND ARE CORRECTED IN PLACE: the "365-day TTL" and the "year filter under-reports" trap.
The deep asset is the 7,639-document, 20-year notices+circulars event stream, not the panel.
CONSEQUENCE FOR SCOPE: build the snapshot + event-stream features; do NOT promise
delta/point-in-time/time-series features on the free-float panel.

# CDC Pakistan CLI — BLOCKER LEDGER
Run 20260907-191511-60d154e7. Updated by the blocker loop. One row per blocker.
RULES: never fabricate a result; record a method as TRIED only after it actually ran;
never run CDC requests while another CDC stream is active; pace >=1s; treat 403
challenge-HTML as a transport error, NEVER as an empty result.

---
## B1 — cf_clearance real server-side TTL  [STATUS: RESOLVED]
ANSWER: **HARD ~30-MINUTE LIFETIME FROM MINT, regardless of traffic.**
Measurement (run 2, minted 14:56:34 UTC): t+0/60/180/360/600/900/1200/1500s all http=200;
t+1800s http=403. Run 1 (minted 14:19:25 UTC) was touched for ~6 min then left IDLE 23 min
and was dead by t+31m. Both runs fit a hard TTL from mint.
TRIED:   [a] full decay schedule -> DIED_AT=t+1800s. [b] hypothesis discrimination:
         H1 hard-TTL CONFIRMED; H2 idle-timeout FALSIFIED (run 2 was touched every <=5 min
         and still died at 30 min); H3 challenge-rotation not needed to explain either run.
         The cookie's own `expires` field claims 365 days and is therefore MEANINGLESS --
         do not trust it, and do not let the generated CLI trust it either.
UNTRIED (deferred, not blocking): [c] whether a 403 can be revived by re-fetching the HTML
         root with the same cookie instead of a full re-mint; [d] whether two cookies minted
         in one browser session are independently valid (parallel-safety).
DESIGN CONSEQUENCES (binding on the build):
  * Store (cookie, User-Agent, minted_at) as ONE unit. cf_clearance is UA-bound.
  * Every command must check `now - minted_at < ~27min` BEFORE starting work and refuse early
    with a typed exit code, rather than failing mid-fan-out.
  * A ~450-request corpus sweep at 1s pacing (~9 min) FITS one window. A full document+PDF
    backfill (thousands of fetches) DOES NOT -- it must chunk by (category, year),
    checkpoint to the store, and re-derive what is missing from the DB each iteration
    rather than trusting a hardcoded list.
  * Phase 5 live dogfood (~50s measured on MUFAP) fits trivially.
  * An unattended multi-year backfill is IMPOSSIBLE without automated re-minting, which needs
    a browser. So `backfill` exits cleanly with "N chunks remaining, re-run auth login" and is
    RESUMABLE -- it must never present a partial run as complete.


## B2 — archive depth of the free-float series  [STATUS: RESOLVED — and the answer is BAD]
ANSWER: **ONE live vintage. Nov-2025, split A/B.** Established by the AUTHORITATIVE full
enumeration: 1,216 requests, all 300 (category,year) pairs settled, 917 productive buckets,
299 clean empty-terminators, **7,956 distinct documents**, ZERO transport errors.
CDC publishes this report monthly but REMOVES prior months from the live site.
Wayback holds ~7 more (2022-12 .. 2024-02). Total obtainable: ~8 NON-CONTIGUOUS vintages.
The adversarial reviewer set >=24 months as the threshold for the time-series features.
WE HAVE 1 LIVE + ~7 ARCHIVED. The threshold is NOT met.
WORSE — THE VINTAGES ARE NOT SCHEMA-COMPARABLE (see B6): Jan-2024 has 1,206 rows / 21 pages
WITH a Market Value column and NO status/% columns; Nov-2025 part A has 29,350 rows /
659 pages WITHOUT Market Value and WITH STATUS + % columns. A 24x universe expansion and an
incompatible column set between the only two vintages available for comparison.
=> Snapshot/cross-section features: SUPPORTED. Delta / point-in-time / time-series
   features: NOT SUPPORTED. Ship them as empty tables with recorded gaps, or not at all.
TRIED:   [a] full paced AJAX sweep, 2 clearance windows, resumable, status-verified per bucket.
         [b] direct /assets/uploads/ probing folded into [a]. [c] Wayback CDX (archived only
         to 2024-02). [d] checked publications + list-of-securities + miscellaneous: the
         series lives ONLY in `miscellaneous`.


## B3 — enumeration authority  [STATUS: RESOLVED — and my earlier claim was WRONG]
CORRECTION: I previously recorded "the AJAX year filter silently under-reports" as a verified
trap, citing list-of-securities returning 0 items for year=2024 AND year=2026. That was a
SAMPLING ERROR ON MY PART, not a site defect. The full sweep found both of that category's
files under **year_param=2025**. The two years I happened to probe are genuinely empty for
that category. The year filter is sound.
TRIED:   [c] AJAX union-over-all-years vs the unfiltered page-1 set, via the 300-pair sweep.
         RESULT: 791 productive buckets, 96 empty-terminators, **ZERO repeat-block
         terminators**, ZERO transport errors in 887 requests. The AJAX paged path is CLEAN.
         The repeat-block defect is CONFINED to the /downloads-category/<cat>/page/N/ URL path
         (still proven: N=1..8 identical, sethash 115323c67793). So there is exactly ONE valid
         enumerator and it is admin-ajax.
STANDS: URL pagination is decorative and must never be walked.
NEW REAL TRAP (replaces the false one): **year_param keys on the UPLOAD/POST date, not the
         document's as-of date.** Evidence: "Security List Report" has meta='30 April',
         upload path 2026/05, and appears under year_param=2026 -- the as-of month (April)
         and the upload month (May) DISAGREE. Consequence: to assemble all documents ABOUT a
         period you must sweep ALL years and key on the PARSED as-of date; year_param alone
         will misfile any document uploaded after its own as-of date. Persist all three dates
         (meta day-month, upload YYYY/MM, year_param) and derive as-of separately, never
         collapsing them.
UNTRIED: [a] whether omitting `year` returns the unfiltered set (moot -- per-year union is
         proven complete and cheap); [b] paged= past the last real page (the empty-terminator
         at 96 buckets already establishes the terminator shape); [d] posts_per_page widening
         (nice-to-have, not needed).


## B4 — 403-challenge conflated with empty result  [STATUS: RESOLVED BY DESIGN]
Resolution: client must classify challenge-HTML (`cf-mitigated: challenge` header OR
`<title>Just a moment...` body) as a typed transport error with its own exit code, never as
an empty page. Encode in spec + a store-level guard that refuses to record a zero-row bucket
whose HTTP status was not 200.

## B5 — PDF numeric conventions  [STATUS: RESOLVED]
* NO accounting negatives. Zero `(digits)` matches in 332k chars across 21 pages. The MUFAP
  "(4.97) = -4.97" convention DOES NOT APPLY to CDC. `-` means zero (736 cells). Standard
  en-US grouping, `.` decimal. Trap retired for this source.
* ISO-6166 check digit passes 1,206/1,206 on the Jan-2024 vintage and the ISIN column
  extracts cleanly in both vintages -- the identity spine rests on solid ground.
* The `excl_GoP <= incl_GoP` invariant test was INCONCLUSIVE and the fault was MINE: a generic
  row regex reported 154 "violations" whose examples (KELSC5, KELSTS15) showed incl=-27, i.e.
  it folded a date/tenor fragment into a numeric column. Debt instruments carry a DIFFERENT
  layout inside the SAME table. Do NOT report those 154 as a data finding. A column-aware
  parser is a BUILD task; re-run the invariant then.
* 104 of 1,206 rows carry FREEZE / `*` / `***` markers INSIDE the name string, not a column.


## B8 — pure-Go PDF text extraction  [STATUS: RESOLVED via sanctioned Swift/PDFKit bridge]
Discovered during Phase 3, not during discovery. CDC CHANGED ITS PRODUCTION PIPELINE:
  2024 vintage: /Producer "Microsoft Excel 2016", PDF 1.5, 55 ObjStm + XRef stream.
  2025 vintage: /Producer "Microsoft: Print To PDF", PDF 1.7, classic xref, 0 ObjStm.
Three pure-Go libraries tested against the LIVE (2025) vintage:
  * github.com/ledongthuc/pdf  -> works on 2024 (97 rows p1, 62ms) but FAILS on both 2025
    files: "malformed PDF: reading at offset 0: stream not present".
  * github.com/dslipak/pdf     -> HANGS (>120s, killed). Worse than an error.
  * github.com/pdfcpu/pdfcpu   -> ExtractContent extracts raw CONTENT STREAMS to files, not
    text. Wrong tool; it is a manipulation library.
No system extractor exists on this machine: pdftotext, mutool, qpdf, gs, pdftk, pdfinfo all
absent; no poppler/mupdf/qpdf/ghostscript brew formula installed.
RESOLUTION: macOS ships PDFKit (Quartz) and `swift` is present, which is EXACTLY the
Swift-subprocess-bridge pattern SKILL.md mandates for macOS framework APIs (precedent:
agent-capture-pp-cli/internal/capture/cgwindow.go). Measured: part B 5,345 chars OK;
**part A 659 pages / 3,562,556 chars in 3.5 SECONDS**. Zero install, first-party, testable here.
DESIGN: two-backend extractor. PDFKit-via-swift is primary (handles every vintage); pure-Go
ledongthuc is a fallback that works on Excel-era vintages and keeps non-macOS builds partly
useful. Record WHICH backend produced the text as row provenance. Non-macOS without a fallback
hit must error honestly naming the limitation -- never return empty text.

## B6 — A/B split semantics  [STATUS: RESOLVED]
ANSWER: **CLEAN PARTITION, ZERO ISIN OVERLAP.** A=29,350 distinct ISINs (659 pages, 16.5MB);
B=66 distinct (2 pages, 372KB); intersection=0; union=29,416. BOTH must be loaded; neither
part alone is the month. Naive concatenation does NOT double-count -- but it DOES mis-align,
because the parts have DIFFERENT COLUMN SETS:
  A: S.No | Security Id | Name | Short Name | Live Date | STATUS | Shares Available in CDS |
     Paid-up incl GoP | % of shares w.r.t. paid-up incl GoP | Paid-up excl GoP
  B: S.No | Security Id | Name | Security Symbol | Live date | Status | Units Available in CDS
B is MUTUAL FUND UNITS (JS Large Cap, Alhamra Islamic Stock, Pakistan Income Fund, ...) and
has NO paid-up capital and NO percentage columns. So part B rows can never carry a
free-float percentage; emitting them with null capital fields is correct, inventing one is not.
SPLIT KEY — ANSWERED DEFINITIVELY from the PDF titles (PDFKit extraction, Phase 3):
  Part A = "DETAILS OF ORDINARY, PREFERENCE SHARES & MODARABA CERTIFICATES"
  Part B = "DETAILS OF OPEN END / ETF FUNDS / SAVING CERTIFICATES"
The partition is by INSTRUMENT CLASS, not alphabetical and not size. That is why the ISIN sets
are disjoint AND why B has no paid-up-capital column: funds have units, not share capital.
TOKENISATION TRAP found in the same pass: Status and a `-` zero value CONCATENATE with no
separator ("... 19/Jan/2007 LISTED-"). Must split `LISTED-` into `LISTED` + `-`.

RECONCILIATION NOTE (open, not an error): part A holds 29,350 securities while the statistics
page reports 732 listed + 59,217 unlisted = 59,949. The report covers a SUBSET. Flag the
residual; do not conclude either source is wrong.

---
## EVENT CORPUS (bonus finding, sized during B2)
`notices` 4,332 + `circulars` 3,307 = **7,639 dated regulatory documents spanning 2007-2026**.
THIS is the deep, genuinely backfillable asset -- not the free-float panel.
14 title patterns classify 4,716 (61.7%). The eligibility lifecycle is a real 6-state machine:
  declared (1,281) -> intention-to-suspend (127) -> suspended (961) -> extension (644)
  -> removal-of-suspension (82) | revoked (385) | admission-terminated (55)
plus corporate-action events: bonus 576, rights 472, new issue/IPO 158, symbol/name change 145,
sub-division 24, merger 54, restrictions 390. Residual is operational notices
(Remote Education via NMS 314, RTA blocking 100, PCM appointments 64) -- legitimately NOT
security events. A second pattern pass would clear ~80%.
CAVEAT: 1,150 documents have NO /assets/uploads/YYYY/MM/ path (legacy /assets/uploads/misc/,
/assets/uploads/publications/, and plain http://), and 1,950 were bulk-uploaded in 2015, so
upload_year is USELESS as an as-of date for a third of the corpus. As-of must come from the
title/meta, with URL normalisation for the legacy shapes.
