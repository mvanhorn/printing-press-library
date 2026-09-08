# PBS CLI — HANDOFF (8 Sep 2026, ~15:1x PKT)
# A new session can resume from this file alone.

RUN_ID:   20260908-120459-bfe084c8
RUN DIR:  ~/printing-press/.runstate/personal-74f0f171/runs/20260908-120459-bfe084c8
CLI:      $RUNDIR/working/pbs-pp-cli   (builds clean, gofmt clean, all tests green)
SPEC:     $RUNDIR/pbs-spec.yaml        (internal YAML, hand-authored)
PRESS:    ~/go/bin/cli-printing-press  v4.31.7  sha256 39a10b8d523b2579… (PATCHED — never upgrade)
LOCK:     RELEASED by promote. Refresh or release:
            cli-printing-press lock update  --cli pbs-pp-cli --phase <p>
            cli-printing-press lock release --cli pbs-pp-cli

## ============ STATE: WHERE EXACTLY THIS STOPPED ============
DONE:  preflight, live registry screen, 6-agent landscape, target decision, Phase 0,
       brief, sniff/crowd gates + marker, Phase 1.5 absorb gate (APPROVED by owner),
       Phase 1.9 reachability, Phase 2 generate, Phase 3 build + gate (ALL 3 PASS),
       Phase 4 shipcheck (6/7 legs PASS), Phase 4.9 doc audit (PASS),
       Phase 4.95 code review (9 findings, ALL 9 VERIFIED AND FIXED),
       Phase 5 FULL LIVE DOGFOOD (92/92 PASS) — but see the marker warning below.

## >>> STATUS: PROMOTED. NEXT STEP IS PUBLISH. <<<
The CLI is PROMOTED to ~/printing-press/library/pbs and manuscripts are ARCHIVED and
PATH-SCRUBBED. The Phase 5 marker was re-run AFTER the nine code-review fixes and is
byte-identical in all three locations (embedded / archived / runstate), fingerprint
dee68ef79043c839... over 180 files, 0 mismatches against the promoted source.

NEXT: `/printing-press-publish pbs`, then drive Greptile to >=4/5 with zero open threads.

DO NOT edit any .go file before publish without re-running live dogfood and re-syncing
the marker to BOTH the embedded and archived proofs paths (lookup is EMBEDDED-FIRST).

If you ever need to re-run live dogfood:

  cd /tmp && cli-printing-press dogfood --live \
    --dir  ~/printing-press/.runstate/personal-74f0f171/runs/20260908-120459-bfe084c8/working/pbs-pp-cli \
    --level full \
    --research-dir ~/printing-press/.runstate/personal-74f0f171/runs/20260908-120459-bfe084c8/research \
    --json \
    --write-acceptance ~/printing-press/.runstate/personal-74f0f171/runs/20260908-120459-bfe084c8/proofs/phase5-acceptance.json

Expect: verdict PASS, matrix ~92, tests_failed 0, ~65 skips (all legitimate — see
proofs/<stamp>-fix-pbs-pp-cli-acceptance.md for why each skip is correct).
DO NOT hand-edit that marker. DO NOT edit any .go file after writing it.

Then, in order:
  1. DONE — promoted (Path A, first print). promoted:true, library_dir library/pbs.
  2. DONE — manuscripts archived to ~/printing-press/manuscripts/pbs/<RUN_ID>/ and
     scrubbed (local-path occurrences 2 -> 0).
  3. DONE — marker synced to embedded AND archived, verified by CONTENT HASH (identical),
     and re-verified against the promoted library source: 180 files, 0 mismatches.
  4. TODO — /printing-press-publish pbs  → Greptile >=4/5, zero open threads.
  5. TODO — file the retro (4 machine findings below, NONE filed).

## ============ PUBLISH VALIDATE: 11/13 PASS — READ THIS BEFORE FIXING THE 2 ============
Run: cli-printing-press publish validate --dir ~/printing-press/library/pbs   (exit 5)

  PASS: manifest, transcendence, **phase5**, govulncheck, go vet, go build, --help,
        --version, verify-skill, patches, manuscripts
  FAIL: go mod tidy      "go.mod or go.sum is not tidy"
  FAIL: module path      module is "pbs-pp-cli"; needs the canonical prefix
                         github.com/mvanhorn/printing-press-library/library/other/pbs

phase5 PASSING is the important part — that is the gate that blocked CDC and MUFAP.

**DO NOT hand-fix these two.** Both go.mod AND go.sum are inside the marker's
source_files fingerprint (verified). Running `go mod tidy`, or rewriting the module path,
CHANGES those files and therefore INVALIDATES the Phase 5 marker — which then fails
`publish package` with the fingerprint error, and you would have to re-run live dogfood
again. `/printing-press-publish pbs` performs the module-path rewrite as part of its own
packaging flow and is the correct owner of both fixes. Let it do them.

Correct order: `/printing-press-publish pbs`  ->  let it rewrite the module path and tidy
->  if it asks for a fresh marker, re-run live dogfood ONCE at that point  ->  sync the
marker to BOTH embedded and archived proofs  ->  push, open PR, drive Greptile to >=4/5.

## ============ WHAT THIS CLI IS ============
pbs-pp-cli — Pakistan Bureau of Statistics price panel. Category `other`, auth NONE,
transport plain Go net/http (no Cloudflare, no cookies, no browser). spec_source `sniffed`.
26 commands. 8 novel + 2 hand-built infrastructure + framework.

  releases   enumerate the release index (the ONLY enumerator)
  sync       fetch+parse into the local panel; resumable, TWO files per release
  spread     cross-city dispersion for one item, optionally as a series
  drift      one item over time for named cities
  basket     custom weighted index over any item subset
  weights    read/diff the weight vector; --check-total asserts the 100.0000 invariant
  verify     six arithmetic invariants, measured from OUTSIDE the parser
  coverage   upstream-vs-local reconciliation with every hole named
  movers     rank items by change RECOMPUTED FROM LEVELS
  revisions  detect what PBS rewrote, re-pointed, deleted, or collided

## ============ SETTLED FACTS — DO NOT RE-DERIVE ============
GOVERNANCE (best in the candidate pool): robots.txt is `User-agent: *` / `Disallow:`
  (empty = allow all). NO AI-crawler directives at all. The Data Dissemination Policy
  grants an open government licence permitting COMMERCIAL reuse with attribution. This
  is the only Pakistani source found with affirmative written permission.
TRANSPORT: ~60 probes, every content URL HTTP 200 via plain Go net/http, 30–900ms, zero
  Cloudflare. Works on macOS LibreSSL curl too (unlike SBP). No browser, ever.
DOCTOR: health_check_path is /robots.txt (text/plain) — deliberately chosen and it WORKS,
  dodging upstream #4612 which forced a hand-written health command on CDC and MUFAP.
THE INDEX: /price-statistics/ serves TWO JavaScript literal arrays in the HTML —
  `data` (148 weekly SPI releases) and `cpidata1` (50 CPI months). There is no JSON API.
  The WordPress REST API is a PARTIAL SLICE (17 of 148, x-wp-total: 17) and MUST NOT be
  used as an enumerator. /spi and /spi-base-2007-08 both 404, so history starts 2023-07-13.
DEPTH: weekly 2023-07-13 .. 2026-09-03, 148 of 165 weeks = 89.7%. 17 weeks absent
  UPSTREAM, incl. a 7-week hole 2024-09-12..2024-11-07. CPI monthly Jul-2022 .. Aug-2026.
THE ARRAY IS NOT SORTED. array[-1] is 2024-08-08 while the true minimum is 2023-07-13.
  Reading the tail as oldest understates depth by two years. Sort by parsed date.
48 of 148 weekly releases have an .xlsx annexure (44 via the `annexureExcel` key plus 4
  more whose plain `annexure` key points at an .xlsx). The other 100 are PDF-ONLY.
35 DISTINCT annexure filename shapes, incl. one `Annex.pdf` with NO DATE. URLs are
  UNCONSTRUCTIBLE — always scrape the index. A wrong guess returns a clean 404.
THE `date` FIELD IS AUTHORITATIVE, THE FILENAME IS NOT. 3 rows disagree (2026-02-19,
  2025-08-21, 2025-06-01). Use the index date; use the FILENAME only to tell annexure
  from report (the annexure/report KEYS are swapped on several rows).
ALL 202 FILES SIT IN ONE CONSTANT DIRECTORY /wp-content/uploads/2020/07/ — the upload
  path carries ZERO date information. Unlike CDC there is no dated tree to walk.
RELEASES ARE NOT ALWAYS THURSDAY: 130 Thu, 9 Wed, 5 Sat, 2 Sun, 1 Tue, 1 Fri. A weekday
  gap detector emits 18 FALSE gaps. Gap detection must be cadence-based with tolerance.
A RELEASE IS TWO FILES. The annexure carries the city x item price grid; the REPORT
  carries the weight vector, quintile indices with income bands, and section counts.
  `TOTAL` occurs ZERO times in annexure text. A one-file design drops half the dataset.
THE ITEM BASKET IS NOT CONSTANT. `Gas Charges for Q1` is absent in 2023, present in 2026.
  Never assume 51 items; carry the observed set per release.
PURE-GO PDF EXTRACTION WORKS on every vintage (ledongthuc/pdf: 7 files 2023..2026, all
  pages, 0 errors, 0 panics, 4–23ms). NO PDFKit/swift bridge — this is the NEPRA outcome,
  not the CDC one. The 4MB annexures DO carry 11–15 decorative image objects; ignore them.
PDF TEXT RUNS ARE PER-GLYPH. Rebuild rows by Y-banding then split cells at font-relative
  X gaps. Header-midpoint column bounds alone are NOT enough: a left-aligned description
  under a centred header pulls its first letters into the previous column. HYBRID —
  gap-tokenize the label region, column-assign the numeric region.
THREE NULL STATES, NEVER COLLAPSED: numeric 0 = UNCOLLECTED (not free), blank = absent,
  "N.A." = not available (the file documents its own sentinel). Cost of collapsing,
  measured: Rice IRRI-6/9 true national avg 154.00; averaging the zeros gives 128.36
  (-16.6%); excluding them gives 155.86.
`Sr.` IS NOT A KEY. It restarts at 1 in each of three sections and the sections are
  re-ranked weekly. Item DESCRIPTION is the only stable join key.
THE PERCENT/IMPACT COLUMNS ARE PUBLISHED AS ~ALL ZERO (measured: only 1–3 of 51 impact
  values non-zero per release). Derive every percentage from LEVELS. Verified equivalent:
  `movers` recomputed +26.30% WoW for onions, reproducing the Bureau's own published 26.3.
CITY WEIGHTS ARE NOT PUBLISHED. A rebuilt unweighted national average diverges from the
  published one by 0.419–0.453% median. `verify --check national-average` reports this as
  EXPECTED-DIVERGENCE, never a failure. Do NOT build any provincial/regional aggregate —
  it would look official and be wrong. (This is why `provincial` was cut at the gate.)
4 UPSTREAM FILE COLLISIONS, each losing one release's data:
  CPI_Monthly_Prices_Annex_0.pdf -> Feb-2024 + Mar-2024 (Feb 2024 is GONE)
  Annex_13.08.2025.pdf           -> 2025-08-13 + 2025-08-21
  4.-Annex-03-07-2025.pdf        -> cpi 2025-06 + spi 2025-07-03   (CROSS-SERIES)
  Monthly-Review-June-2025.pdf   -> cpi 2025-06 + spi 2025-07-10   (CROSS-SERIES)
THE STORE FILE IS PRE-CREATED BY THE FRAMEWORK'S LEARN LOOP, so an os.Stat missing-store
  check is almost always FALSE. `panelEmpty` (row count) is the real signal. Treat
  missing / empty / unmatched as THREE states. Getting this right took the scorecard's
  live sample probe from 3/8 to 7/8.
BACKFILL: a full sync walks ~300 files in one invocation bounded by root --timeout
  (default 1m). ALWAYS pass --timeout 6h for --full, or bound with --max-releases.

## ============ HARD NEGATIVES (answered, stop asking) ============
- /spi and /spi-base-2007-08 are BOTH HTTP 404. The 2007-08 base series is unreachable.
  History starts 2023-07-13. There is no deeper panel.
- The WP REST API covers 17 of 148 releases. Not an enumerator.
- Wayback is NOT a backfill: earliest SPI upload capture is 20251022150101, while the live
  index reaches 2023-07. THE LIVE SITE IS DEEPER THAN THE ARCHIVE. Nobody archives this.
- Appendix-B (wage rates, fertilizer, cement, CNG, wheat) is DEFERRED, not forgotten. It
  covers a DIFFERENT, SMALLER city set needing a second resolution path, and CNG is priced
  per litre in Punjab and per kg elsewhere needing a unit-aware guard. Its cells ARE
  counted in the state census. Disclosed in SKILL.md anti-triggers. Strongest reprint item:
  real wage in food units (`wage --in-terms-of`).
- SPI<->CPI item crosswalk was CUT on verifiability: the description-match width between
  the 51 weekly and 66 monthly items is UNMEASURED. Measure it before shipping that.

## ============ VERIFICATION ALREADY DONE (do not repeat) ============
DB-VERIFIED (not log-grepped): 198 releases / 510 files indexed; 4 releases synced ->
  10,404 price rows (4 x 2,601), 17 cities, 51 items, 204 weight rows, 24 index rows,
  12 section totals, 1,711 national/derived rows, 8 coverage rows.
  NULL DISCIPLINE PERFECT: 0 non-present rows carry a value; 0 present rows are NULL.
CROSS-ENCODING: PDF vs XLSX, same release, two parsers — 2,601 cells, 100% coverage,
  **0 value disagreements**; 21 cells (0.81%) unrecoverable from the PDF and ALL flagged
  by the parser as column-overflow suspects; 0 silent label corruptions; 82.4% exact labels.
CROSS-FILE: report vs annexure national price — 50 of 50 items agree.
INVARIANTS: `verify --all` 24 checks over 4 releases, 0 failures. Weight totals
  100.0000/100.0000. 864 complete min<=avg<=max triplets/release, 0 violations.
  Sections 17+7+27=51 agree. 3 bands partition 17 cities disjointly (7/7/3).
TESTS: internal/pbsparse 30 funcs; internal/pbsfetch 11 funcs; internal/cli spearman
  regressions 4 funcs. `go test -race` clean. gofmt clean. 0 novel scaffolds remain.

## ============ THE 9 CODE-REVIEW FINDINGS — ALL FIXED, ALL VERIFIED FIRST ============
1. pdf.go — triplets were zipped to cities BY INDEX with no cardinality check, so one
   untokenized MIN header shifted every later city's prices onto its neighbour, plausibly
   and undetectably. Now the band is REFUSED and the parse fails.
2. annexure.go — when the MIN/AVG/MAX sub-label row was unreadable, all three columns got
   StatSingle and COLLIDED ON ONE PRIMARY KEY, so Go map iteration order decided which of
   min/avg/max survived — a different answer every run, while sync reported 3 rows written.
   Now multi-column city spans with unresolved stats are refused.
3. verify.go — a `skipped` check was counted as FAILED and exited 5 claiming a parse
   failure. Now skipped is its own tally.
4. pbs_sync.go — a report PARSE failure never cleared `ok`, so the release counted as
   complete, `incomplete:false`, exit 0, and loadCompleted skipped it forever, silently
   losing weights and indices. Now it marks the release incomplete.
5. pbs_query.go — spearman differenced ranks built over DIFFERENT city sets (1..17 vs
   1..3), producing rho = -146. Now ranks are recomputed over the intersection with tied
   ranks; 4 regression tests pin it.
6. pbs_sync.go — every recordCoverage error was discarded with `_ =`, so prices could
   commit while provenance vanished. Now surfaced via orFirst.
7. index.go — extractArray brace-matched without string awareness, so a `]` inside any
   quoted URL truncated the index and the truncation reported success. Now string-aware.
8. xlsx.go / fetch.go — unbounded decompression and body reads. Now 64 MiB per worksheet
   (with an UncompressedSize64 pre-check) and 128 MiB per body.
9. fetch.go — scraped URLs were fetched with no host check. Now a constructor-set
   AllowedHosts allowlist (set from DefaultAllowedHosts, never from scraped input), with
   regression tests for localhost, an off-origin host, file://, and a suffix-spoof host.
CLEARED BY THE REVIEW (do not re-investigate): nullable-column scans, SQL injection,
  *sql.Rows leaks, NaN/Inf into JSON, nested queries on open Rows.

## ============ MACHINE FINDINGS FOR THE RETRO — 4, NONE FILED ============
1. **scorecard HOLDS on a dimension it excludes from its own denominator.**
   `live_api_verification` is listed under "omitted from denominator" alongside
   `auth_protocol` and `mcp_surface_strategy`, yet only it appears in
   `unverified_dimensions` and gates the verdict. `auth_protocol` is equally N/A on a
   no-auth CLI and does NOT gate. This alone is why shipcheck reads HOLD, not 0.
   Verified by hand that every live path works: `index` returns source:live, `file`
   fetches 55,559 real bytes, `doctor` reports API reachable, live-check 7 pass / 1 fail.
2. **scorecard --live-check SIGBUS — MUFAP retro item 5 reproducing unchanged.** The probe
   prints `Binary refresh: fresh_fallback (same-name runnable binary is newer than Go
   sources)` then faults. It MOVED BETWEEN FEATURES across three consecutive runs and does
   NOT reproduce standalone: 10 consecutive runs of the same command, 0 failures,
   `go test -race` clean.
3. **dogfood reimplementation_check reports "sync uses generic Upsert only" — FALSE.**
   The CLI never calls store.Upsert or UpsertBatch anywhere; all writes go to typed pbs_*
   tables. Verified by grep with no match outside internal/store itself.
4. **dogfood dead_functions flags `isDryRunResponseForClient`**, which the CDC run already
   recorded as having 3 real call sites. Recurs unchanged. `readSecretFromStdin` is also
   unreachable in a no-auth CLI and `successfulNoop` is dead as generated — both in
   generated helpers.go, where a local deletion would not survive regeneration.

## ============ RESEARCH.DB — PROPOSED, NOT PERFORMED ============
Nothing was written to ~/psx-research/data/research.db. Proposed ADDITIVE tables (new
tables only, owner approval required first):
  pbs_spi_price   (as_of, city, city_code, item_desc, stat, value, value_state)
  pbs_spi_weight  (as_of, item_desc, weight_lowest, weight_combined, impact_*)
  pbs_spi_index   (as_of, quintile, band_low, band_high, index, prev_week, cor_week)
  pbs_spi_cov     (as_of, role, state, sha256, present/zero/blank/na counts)
Load by SHELLING OUT to the CLI (the MUFAP mufap_load.py pattern) so the three-state null
decode and the item-description key have ONE implementation and cannot drift.
JOIN VALUE: weekly city-level prices against mufap_netsales_cov and policy_rates_clean —
the first sub-national Pakistani price series available to that engine.

## ============ THE CANDIDATE PIPELINE (scored, for the next target) ============
Full table in scratchpad/scored-table-20260908.md. Ready to build, in order:
  SBP auctions  5/5/3/4  sovereign yield curve 1998+, bid-level demand ladders. JOINS to
                         research.db. Exclude KIBOR (Refinitiv-licensed).
  NEPRA         5/5/4/3  DISCO performance (T&D loss vs target, SAIFI, SAIDI) + 134-plant
                         generation as CLEAN HTML. Pure-Go PDF verified 93–97% vs MuPDF.
  PPRA          5/4/5/3  tender+award+winner join. BUILD ON EPMS HTML ONLY — the EPADS
                         JSON API uses a hardcoded Basic key (value withheld) from the
                         public JS bundle (decoded and confirmed); EPMS needs no credential
                         and carries both the awards and the Tender No join key.
  OGRA          5/4/4/3  geocoded per-pump prices back to 2017-02-16.
SHELVED ON OWNER'S CALL: PakWheels and PTA (ToS conflict). DEAD: PMEX (managed CF
challenge 26/26, robots unreadable), SECP, OLX, PriceOye, Sastaticket, Bookme, Graana.
NEPRA/OGRA carry the same Cloudflare stock ClaudeBot Disallow + ai-train=no block as CDC;
owner already ruled these get the CDC treatment.
