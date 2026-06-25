# Google Play CLI — Phase 5 Acceptance Report

  Level: Full Dogfood (live, no API key — public store)
  Tests: 61/61 passed (60 error-path probes correctly skipped via pp:no-error-path-probe; 0 failed)
  Gate: PASS

## What was tested (binary-owned live matrix)
- help + happy-path + JSON-fidelity for every leaf command against the live Play Store
- output-mode fidelity (--json/--select/--compact)
- error paths where applicable

## First run: 7/68 error-path failures -> fixed
The first full matrix failed 7 error_path probes: permissions, reviews, search-store, suggest, rank-history, watch-listing, review-digest all returned exit 0 for an `__printing_press_invalid__` argument. Root cause: Play returns HTTP 200 + an empty success envelope for unknown appIds, search/suggest accept any term, and the local-store commands return a valid empty-state view. None can distinguish bad input from a valid empty result without inventing API-specific not-found semantics.

Fix (the documented opt-out, not a fake heuristic): added `cmd.Annotations["pp:no-error-path-probe"] = "true"` to those 7 commands. app/similar/developer/datasafety were NOT annotated — they correctly error (exit 5) on a missing listing because the HTML parse finds no title.

Re-run: 61/61 pass.

## Auth context
type: none (public store, no credential). Live testing ran without any key.

## Printing Press issues for retro
- Generated framework `import` command + `--idempotent` flag ship on a read-only no-auth CLI (see phase-4.95-findings.md). Retro candidate.
