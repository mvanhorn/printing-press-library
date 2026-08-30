# Flighty CLI — Phase 5 Live Dogfood Acceptance Report

## Acceptance Summary
- **Level:** Full Dogfood
- **Tests:** 89/89 passed, 0 failed, 88 skipped (N/A probes: write-path lifecycle for a read-only CLI, per-source rate-limit rows for a single-source no-auth CLI)
- **Gate: PASS** (acceptance marker: `proofs/phase5-acceptance.json`, status `pass`, level `full`, written by the runner)

## Fixes applied during dogfood (fix-before-ship rule)
1. `airports tv --status MAJOR_ISSUES` — the command's own example advertised `--status` but the generated command lacked the flag (spec example drift). Added the `--status` client-side filter reusing `flightyFilterCatalog`.
2. `airports airline <unknown>` error-path — an unknown airline code cannot be distinguished from "airline not disrupted today" without inventing semantics; the honest contract is empty result + note. Annotated `pp:no-error-path-probe = "true"` per the dogfood error-path opt-out contract.
3. `feedback --help` missing Examples section — added a realistic Example to the generated feedback command.

## First dogfood run (before fixes)
- 86 passed / 4 failed (the three items above + tv json_fidelity tied to the same --status gap). All fixed in one loop; rerun: 89/89.

## Live evidence highlights
- `sync --resources airports --full`: 157-159 records per run, ~1.5s.
- `workflow status`: airports 159 + airports-tv 159 items in store.
- Cross-airport commands read the mirror; `airports worst` ranked LHR top (4828m, MAJOR_ISSUES) from a live catalog fetch in a sandboxed HOME (auto fallback verified).

## Printing Press issues (retro candidates)
- Generated endpoint `example` values are not flag-validated at generate time: the spec's tv example promised `--status` that the generated command didn't emit (params in spec generate flags; example text is not cross-checked). Candidate for a generator example-vs-flags consistency check.
- Framework `feedback` command emitted without an Example section (dogfood help probe requires one on parent commands).
