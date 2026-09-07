Manifest transcendence rows: 8 planned, 8 BUILT. Phase 3 gate PASSED both halves.

# Phase 3 Build Log — cdc-pakistan-pp-cli

Planned rows (all `hand-code` except `stats history` = spec-emits):
  1. verify rows          2. identity ledger     3. float triangulate   4. coverage map
  5. eligibility state    6. verify schema       7. stats history       8. gop stake
Zero stubs approved. Zero absorbed rows (manifest empty by measurement).

## Priority 0 — foundation

### internal/cdcpdf/ — DONE (extractor + 3-layout parser + 7 passing tests)
BLOCKER FOUND AND SOLVED IN-PHASE (recorded as B8 in BLOCKERS.md): no pure-Go PDF reader
handles the LIVE vintage. CDC switched producer from "Microsoft Excel 2016" (PDF 1.5, object
streams) to "Microsoft: Print To PDF" (PDF 1.7, classic xref). ledongthuc/pdf fails
("stream not present"), dslipak/pdf HANGS, pdfcpu extracts content streams not text, and no
system extractor (pdftotext/mutool/qpdf/gs) is installed. Resolved with the SKILL-sanctioned
Swift subprocess bridge to macOS PDFKit: 659 pages / 3.56M chars in 3.5s. Pure-Go retained as
a fallback so non-Darwin builds stay useful on Excel-era vintages; backend recorded as row
provenance so a backend swap can never masquerade as a schema change.

THREE row layouts derived EMPIRICALLY from real headers, not assumed:
  equity-2025 (4 numerics): seq ISIN NAME [markers] SHORT DATE STATUS shares incl pct excl
  equity-2024 (6 numerics): ... DATE shares mktval incl pct_incl excl pct_excl   [NO status]
  funds-2025  (1 numeric) : seq ISIN NAME SYMBOL DATE STATUS units  [no capital, no pct]
  debt        (1-3)       : REORDERED -- status BEFORE the live date, plus a maturity date

TRAPS HANDLED (each verified against real rows):
  * `-` decodes to ZERO, not missing (736 such cells in one vintage).
  * NO accounting negatives exist in CDC PDFs -- confirmed by full scan -- but `(x)` is still
    decoded defensively because undecoded it inverted a whole market in MUFAP.
  * "LISTED-" concatenation: status and a zero dash fuse with no separator in part B.
  * UN-LISTED normalises to UNLISTED so a group-by cannot split (28,689 rows affected).
  * FREEZE / `*` / `***` / U+25D8 glyph markers live INSIDE the name string, not a column.
  * FOURTH and FIFTH date encodings found: numeric D/M/YYYY (AMBIGUOUS -- both 10/24/2013 and
    13/03/2025 occur, so order cannot be inferred; raw preserved and ambiguity FLAGGED, never
    normalised) and DD-Mon-YY maturity dates on debt rows.
  * A numeric-field-count mismatch is an ERROR-severity finding, never a silent coercion.

EXTRACTION QUALITY, MEASURED FROM OUTSIDE THE EXTRACTOR (ISINs present in raw text vs rows
emitted) -- the standing project lesson:
  2025-11-30 part A (LIVE): 29,291 / 29,350 = 99.80%   23 findings
  2025-11-30 part B (LIVE):      66 /     66 = 100.00%   0 findings
  2024-01-31 (archived)   :     609 /  1,208 = 50.41%   44 findings
  The 2024 shortfall is Excel-era PDFs rendering the S.No column as a SEPARATE text block,
  which interleaves records. It is an archived vintage, the live one is at 99.8%, and the
  number is reported by `verify rows` rather than hidden.

INVARIANTS, both 0 violations on the live vintage:
  excl_GoP <= incl_GoP        : 29,060 tested, 0 violations
  pct == shares/incl * 100    : 29,060 tested, 0 violations, worst delta 0.0050pp
  => This RETIRES the "154 invariant violations" recorded earlier under B5. That was a
     generic-regex artefact comparing a CAPITAL column against a PERCENTAGE column. The
     source data is clean to rounding precision.

Manifest transcendence rows: 8 planned, 8 BUILT. Phase 3 gate PASSED both halves.

### internal/cdcparse/ — DONE (downloads + statistics + event classifier, 8 passing tests)
* ParseDownloads walks the DOM (x/net/html) rather than regex, and uses cliutil.CleanText so
  HTML entities decode ("&#8211;" -> en dash) instead of leaking as the &#39; bug class.
* IsChallenge / ErrChallenge are EXPORTED and every parser returns ErrChallenge for an
  interstitial. This is the direct fix for the defect that made a 300-bucket sweep record
  zeros against a dead clearance cookie: a 403 challenge body is HTML, and a selector-based
  parser reads it as "no items". A genuinely empty fragment is still empty; the two are now
  distinguishable, with a regression test for all three challenge signatures.
* Documents persist THREE dates and derive none: meta day-month (no year), upload YYYY/MM
  (the upload date, which can differ from as-of), and year_param. 14% of the corpus has NO
  date-bearing path and is flagged legacy_path rather than guessed.
* Statistics label normalisation is drift-resistant, and TWO SUBSTRING BUGS were found and
  fixed by testing against the real page:
    - "Securities-Unlisted" CONTAINS "listed", so it collapsed onto securities_listed,
      producing a duplicate key and silently losing the 59,217 unlisted count.
    - "Total Number of Securities under Share Registrar" CONTAINS "number of securities",
      so the registrar count was keyed as a securities total.
  Both now have explicit ordering + `excludes` guards and dedicated regression tests.
  Verified on the live page: 15 UNIQUE keys, as_of "July-2026", 0 unknown labels.
* Unknown labels are KEPT with Known=false and surfaced in UnknownLabels. A dropped row is an
  invisible loss, and CDC has already renamed rows and deleted one.
* Event classifier: 8-state eligibility lifecycle. Rule ORDER is load-bearing and tested --
  "extension of suspension" must beat "suspension", "removal of intention" must beat
  "intention". Measured over the real 7,639-document corpus: 76.3% classified
  (was 61.7% before the ordering fixes), 5,300 high-confidence / 527 low / 1,812 unclassified.
  States: declared 1,281 | extension 644 | revoked 385 | suspended 366 |
  removal-of-suspension 250 | intention-to-suspend 194 | removal-of-intention 101 |
  terminated 86. Unclassified is a first-class outcome, not a silent default.

### internal/store/cdc_migrations.go — DONE (8 tables, own file so regen preserves it)
cdc_documents (3 dates, none derived) | cdc_coverage_buckets (TRI-state incl. not-probed)
cdc_vintages (layout + producer + backend + coverage_pct) | cdc_penetration_rows (paid-up and
pct NULLABLE because part B funds genuinely have no share capital -- NULL means "not published
for this instrument class", 0 means "published as zero") | cdc_stats_snapshots (append-only,
provenance live|archive) | cdc_identity_events | cdc_data_quality_findings (the shared
substrate every verify command writes to, with an acknowledged flag that --strict honours).

Manifest transcendence rows: 8 planned, 8 BUILT. Phase 3 gate PASSED both halves.

## Priority 2 — all 8 transcendence features BUILT
Each was behaviourally tested against REAL CDC data, not just compiled:
  coverage map        live probe: 48 requests, 464 real documents stored, page-2 correctly
                      persisted as not-found-at-source (a real negative), pairs_settled 2/2
  eligibility state   390 real eligibility events; no-match case reports "no notice TITLE
                      contained that text", never "was never suspended"
  identity ledger     parsed "Calcorp Limited -> ARM Green Industries Limited"; an unparseable
                      rename is KEPT with parsed=false rather than dropped
  stats history       live snapshot captured July-2026, 15 metrics, provenance=live
  verify rows         part A 29,291 rows @ 99.80% coverage, part B 66 @ 100%; ALL THREE checks
                      0 violations (pct_matches_ratio, excl_le_incl, isin_check_digit 29,291/29,291)
  verify schema       fingerprints both parts, detects cross-layout drift and partial months
  gop stake           real result: KEL GoP 6,726,912,277 shares = 24.36% of paid-up capital
  float triangulate   three legs reported separately with denominators named; a missing leg is
                      available=false with a reason, never imputed or blended

PHASE 3 COMPLETION GATE:
  part 1 per-row Cobra resolution: all 8 approved leaves resolve with a correct Usage line
  part 2 dogfood novel_features_check: planned=8 found=8 missing=none skipped=none
Parent-group Shorts were generic (three shared "Local state that compounds" from the
research.json group field) and were rewritten to be distinct and meaningful.
