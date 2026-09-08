# Acceptance Report: pbs

  Level:  Full Dogfood (live, against www.pbs.gov.pk)
  Tests:  92/92 passed, 0 failed, 65 skipped
  Gate:   **PASS**

## Marker
`proofs/phase5-acceptance.json` — status `pass`, level `full`, matrix 92,
tests_passed 92, source_fingerprint `116b2cc330f6a23a...` over 179 files.
Written by the runner, not by hand.

## No hollow coverage
Every one of the eight novel features passed `help`, `happy_path` AND
`json_fidelity` against the live site:
  spread · drift · basket · weights · verify · coverage · movers · revisions
The two Priority-1 infrastructure commands passed the same three checks.

The 65 skips are all legitimate and fall into three groups:
  * `error_path` skipped on 6 of 8 novel features, reason "no positional argument" —
    those commands take no positional, so there is no bad-input probe to run. The two
    that DO take one (`spread`, `drift`) both PASS error_path with typed exit 3.
  * `sync` write path skipped, reason "mutating command dry-run only" — correct
    treatment for the only command that writes.
  * framework commands (feedback, learnings, teach*, api, doctor, file, index).

## Independently verified behaviour, live
- `releases` enumerated 148 weekly + 50 monthly releases, sorted by parsed date,
  and surfaced **4 upstream file collisions** and **3 filename/index date mismatches**.
- `sync` fetched and parsed real releases into the panel: 2,601 price rows and 51
  weight rows per release, resumable, exiting with typed code 5 and a resume command
  when work remained.
- `verify --all` ran 24 checks across 4 releases with **0 failures**: weight totals
  100.0000/100.0000; 864 complete min<=avg<=max triplets per release, 0 violations;
  section counts 17+7+27=51 agreeing; 3 header bands partitioning 17 cities disjointly.
- `spread` reported onions ranging 166.60 (Sargodha) to 252.97 (Islamabad) across 17
  contributing cities on 2026-09-03.
- `movers` recomputed +26.30% week-on-week for onions from stored levels, reproducing
  the Bureau's own published 26.3 for the same item and week.
- `coverage` on a store with no data reported the upstream side correctly: 198 releases
  indexed, 0 stored, 17 weeks absent upstream, 89.7% index coverage.
- `revisions` re-checked 4 stored files, reported 4 unchanged and 0 false positives,
  while still surfacing all 4 genuine upstream collisions.
- `doctor` reported "API: reachable"; the /robots.txt health path avoided the
  HTML-200 doctor defect that needed a hand-written workaround on CDC.
- Generated endpoints both work live: `index` returned real extracted links with
  `source: live`, and `file Annex_03.09.2026.xlsx` fetched successfully.

## Fixes applied during this phase: 4
1. An EMPTY panel is now a distinct state from a MISSING one and from an UNMATCHED
   item. Previously a fresh install answered a legitimate query with `no item matches
   "Onions"` plus a hint naming a `pbs-pp-cli items` command **that does not exist**.
   Root cause: the store FILE is pre-created by the framework's learn loop, so an
   os.Stat check is almost always true and `panelEmpty` (row count) is the real signal.
   Effect: the scorecard's live sample probe went 3/8 -> 7/8.
2. `--max-age` carries a 30-minute default, so the staleness warning fired on every
   read. Now it fires only when the flag is explicitly set — and wiring it removed the
   last dead flag (dead_flags 1 -> 0).
3. `coverage` no longer returns early on a missing store. It answers "what exists
   upstream versus what do I hold", and the upstream half is exactly what a caller
   with no data needs first.
4. Restored the `// pp:data-source` annotation on all eight novel files, lost when the
   generated scaffolds were replaced (reimplementation_check 8 missing -> 0).

## Machine issues for the retro: 4
1. **scorecard HOLDS on a dimension it excludes from its own denominator.**
   `live_api_verification` is listed under "omitted from denominator" alongside
   `auth_protocol` and `mcp_surface_strategy`, yet only it appears in
   `unverified_dimensions` and gates the verdict. `auth_protocol` is equally N/A on a
   no-auth CLI and does NOT gate. Meanwhile every live path was verified by hand:
   `index` returns `source: live`, `file` fetches 55,559 real bytes, `doctor` reports
   API reachable, and live-check itself scored 7 passed / 1 failed.
2. **scorecard --live-check SIGBUS, reproducing MUFAP retro item 5 unchanged.**
   The probe reports `Binary refresh: fresh_fallback (same-name runnable binary is
   newer than Go sources)` and then faults. It moved between features across three
   consecutive runs (drift alone, then drift + revisions), and does NOT reproduce
   standalone: 10 consecutive runs of the same command, 0 failures, and `go test -race`
   clean on both hand-written packages.
3. **dogfood reimplementation_check reports "sync uses generic Upsert only" — FALSE.**
   The CLI never calls `store.Upsert` or `UpsertBatch` anywhere; every write goes to
   typed `pbs_*` tables. Verified by grep across internal/ with no match outside
   internal/store itself.
4. **dogfood dead_functions flags `isDryRunResponseForClient`**, which the CDC run
   already recorded as having three real call sites. Recurs unchanged. Also
   `readSecretFromStdin` (unreachable in a no-auth CLI) and `successfulNoop` are dead
   as generated, both in generated helpers.go where a local deletion would not survive
   regeneration.

## PII
None. This dataset is city-level aggregate consumer prices; no personal data of any
kind passes through the CLI, and no credential exists to leak.
