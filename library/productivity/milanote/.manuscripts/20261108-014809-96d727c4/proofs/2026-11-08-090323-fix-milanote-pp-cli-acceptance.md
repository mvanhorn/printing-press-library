# Acceptance Report: milanote

Level: Full Dogfood
Tests: 216/216 passed (300 rows skipped by the runner: no portable fixture, or mutating without --allow-destructive)
Failures: none
Write lifecycle: 24 checks on disposable boards, recorded in milanote-lifecycle.md, boards trashed afterwards
Gate: PASS

Runner notes:
- 10 novel commands covered by milanote-lifecycle.md (pp:verified-by-proof); none hollow.
- The Chrome login moved from `auth chrome` to top-level `chrome-login` because the runner has no matrix rows for `auth` subcommands. It runs for real in milanote-lifecycle.md with a throwaway Chrome and temporary config.

Fixes found during live testing:
- layout grid: a wide heading stretched every column; columns now use the common width and wide cards span.
- layout check: side-by-side gaps are now measured.
- share get: failed while a read-only link was on.
- boards list: printed breadth-first, so indentation was wrong.
- images: height estimate was one unit too tall.

Not tested for real: share invite and share remove (dry-run only; a real run emails a person).
Column heights remain estimates.
Printing Press issues for retro: novel commands under `auth` can never pass the Phase 5 coverage gate; dogfood's "reimplemented" heuristic misses API calls made through helpers; phase-receipt has no entry path for a run resumed mid-pipeline.
