# Acceptance Report: awwwards
  Level: Full Dogfood (binary-owned live matrix, --level full)
  Tests: 74/74 passed (55 skipped: help-only probes + annotated error-path opt-outs)
  Failures: none (final run)

## Fix loop (1 iteration)
First run: 74/78 — 4 error_path failures (websites browse, directory browse, elements, elements-top): probes expected non-zero exit for `__printing_press_invalid__` arguments, but awwwards.com soft-200s every unknown filter segment (verified live: /websites/, /directory/, /elements/ all return HTTP 200 for garbage segments). Bad input is genuinely indistinguishable from a valid empty/generic result upstream.
  Fixes applied: 4 — `pp:no-error-path-probe` annotations per the documented opt-out (no local "empty means not found" heuristics invented).
  Tagged: CLI fix (annotation) — the underlying behavior is an upstream API contract, not a bug.

## Earlier in-session live verification (Phase 3/4.95, real mirror of 93 sites)
- mirror: 2 pages + details + hero elements -> 62 cards, 8+40 details, 31 elements
- find AND-intersection asserted (every result carries all queried tags); find --tech three-js 0 -> 14 matches after fix
- top ranked by design correctly (depo-luxe 7.81 > monolog 7.61 > units 7.52)
- inspect live + local-cache paths return identical profiles (overall 7.43 = the site's own value)
- trends tag/tech/color with --vs deltas and coverage notes
- context-pack: 18-23 matches with benchmarks (avg 7.41, p90 7.62), palettes, tech
- palette-match near-black: monolog #080807 at distance 4.12
- elements-top: 15/31 joined, jury/votes score_source labels
- studio: credited maker aggregated (1 win, SOTD, avg 7.35)

  Printing Press issues (retro candidates): writeNoop dead emission; HeyGen Beacon template leak; scorecard live-check vacuous [] passes without seeded fixture; analyzer CAPTCHA false-positive on `.legal-recaptcha` CSS class.

  Gate: PASS
