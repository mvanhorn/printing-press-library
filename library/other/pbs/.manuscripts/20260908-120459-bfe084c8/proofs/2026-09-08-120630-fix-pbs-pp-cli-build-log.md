Manifest transcendence rows: 8 planned, 8 built. Phase 3 will not pass until all 8 ship.

# PBS CLI — Phase 3 build log

## Transcendence rows: 8 planned, 8 BUILT, 0 stubs
spread · drift · basket · weights · verify · coverage · movers · revisions
Zero `pp:novel-scaffold` markers remain in any of the eight files. Nothing ships as a
placeholder, and nothing was downgraded from shipping scope mid-build.

## Priority 1 infrastructure, also hand-built
`releases` (index enumerator) and `sync` (resumable two-file-per-release backfill).
Registered through `registerNovelCommand` in its own `internal/cli/pbs_register.go`, not by
editing root.go, so `generate --force` preserves the wiring.

## Priority 0: the parser (internal/pbsparse, 7 files, 30 test funcs)
values.go   three-state cell classifier (present / zero / blank / N.A. / unparseable)
index.go    JS-literal release index, sorted by parsed date, collision + mismatch detection
xlsx.go     dependency-free xlsx reader (archive/zip + encoding/xml), merges + inline strings
annexure.go stacked-row-block Appendix-A parser with group-aware header resolution
pdf.go      PDF annexure parser on a learned column model
pdfgrid.go  glyph -> line -> cell reconstruction from text-run geometry
report.go   executive-summary parser: quintiles, income bands, weights, impacts, sections

## Phase 3 Completion Gate: ALL THREE PASS
GATE 1 per-row Cobra resolution: 10/10 approved command paths resolve as real leaves
  (Usage names the leaf and shows [flags], never a `[command]` parent fall-through).
GATE 2 dogfood novel_features_check: planned 8, found 8, missing none, skipped false.
GATE 3 pure-logic test presence: internal/pbsparse 4 files / 30 test funcs;
  internal/pbsfetch 1 file / 9 test funcs (written when the gate caught it at zero).

## Verified against the DATABASE, not a log
198 releases indexed (148 weekly + 50 monthly), 510 release files.
4 releases synced -> 10,404 price rows (4 x 2,601), 17 cities, 51 items,
204 weight rows, 24 index rows, 12 section totals, 1,711 national/derived series rows.
NULL DISCIPLINE PERFECT: 0 non-present rows carry a value; 0 present rows are NULL.
Typed exit code 5 on an incomplete sync, printed AFTER the resumable summary.

## Cross-validations that passed
PDF vs XLSX, same release, two independent encodings and two independent parsers:
  2,601 cells compared, coverage 100.00%, **0 value disagreements**,
  21 cells (0.81%) unrecoverable from the PDF and ALL of them flagged by the parser
  itself as column-overflow suspects; 0 silent label corruptions.
Report vs annexure national price: **50 of 50 items agree**.
`verify --all` over 4 releases x 6 checks = 24 checks, **0 failures**:
  weight totals 100.0000/100.0000 on every release; 864 complete min<=avg<=max triplets
  per release with 0 violations; section counts 17+7+27=51 agree; 3 bands partition
  17 cities disjointly (7/7/3).
`movers` recomputed +26.30% WoW for Onions, reproducing the Bureau's own published 26.3.
`verify` national-average median deviation 0.419-0.453%, independently reproducing the
  0.45% measured during discovery by a different route.

## Defects I introduced and the tests caught
1. HEADER-GROUP CONFLATION (highest consequence). Collected every column whose header
   matched /national/ into one list and kept the last, returning **153.98 (LAST WEEK's
   national average) where 154.00 is correct**. Same magnitude, different variable; no
   range check could catch it. Fixed by resolving each merge span to its own group with
   its own sub-labels. The fix also recovered four series the first pass silently discarded.
2. APPENDIX-B LEAK. After Appendix-A's rows ended, Appendix-B's tables (which restart
   numbering at 1) were absorbed into the last city band, so **an electrician's per-point
   wage was emitted as the price of beef in Bannu**. Fixed with two independent guards: an
   explicit section-marker reset and a monotonic item-number check.
3. NUMBERING-ROW INGESTION. The `1 2 3 ... 24` column-index row parsed as item 1, which
   then made the monotonic guard close the band on the first real data row and drop the
   whole panel. Fixed by requiring a description to contain a letter.
4. DEGENERATE TUKEY FENCES. With 27 of 51 items unchanged, the IQR collapses and every
   mover was flagged. Fixed by fencing over the items that actually moved and stating the
   method in the output.
5. MISLABELLED MEASURE. `basket` without --rebase returned a weighted mean PRICE IN RUPEES
   and called it an "index". Fixed with an explicit `measure` field on every point.
6. REVISIONS FALSE POSITIVE. pbs_coverage did not record WHICH url was fetched, so a join
   picked one of the pdf/xlsx pair arbitrarily and every release reported "re-pointed".
   Fixed by recording the url and comparing against the live URL SET.

## Intentionally deferred (approved at the Phase 1.5 gate)
Appendix-B surfaces — wage rates, fertilizer, cement, CNG, wheat — need a second
city-resolution path because Appendix-B covers a different, smaller city set, and CNG
carries a per-region unit split (per litre in Punjab, per kg elsewhere) needing a
unit-aware aggregation guard. Their cells ARE counted in the state census so a release's
accounting is complete rather than silently partial.

## Machine findings for the retro
1. dogfood reimplementation_check reports "sync uses generic Upsert only" — FALSE. The CLI
   never calls store.Upsert or UpsertBatch anywhere; every write goes to typed pbs_* tables.
   Verified by grep across internal/ with no matches outside internal/store itself.
2. dogfood dead_functions flags `isDryRunResponseForClient`, which the CDC run already
   recorded as having three real call sites. Recurs unchanged.
3. Generated dead code persists in a no-auth CLI: `readSecretFromStdin` cannot be reached
   without auth, and `successfulNoop` is dead as generated. Both live in generated
   helpers.go, so a local deletion would not survive regeneration.
