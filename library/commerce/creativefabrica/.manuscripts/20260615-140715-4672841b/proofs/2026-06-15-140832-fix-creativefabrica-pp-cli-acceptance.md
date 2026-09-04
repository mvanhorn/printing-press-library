# Acceptance Report: creativefabrica

Level: Full Dogfood
Tests: 49/49 passed
Failures: none
Auth context: no user auth (public catalog); public Algolia search key configured via cache.
Gate: PASS

## Coverage
Mechanical full matrix across every leaf command (find, free, pod, deals, designer,
designer-stats, designer-compare, new-since, tags, categories, types, product, auth,
doctor): help text, happy-path live calls, JSON-parse validation, output-mode fidelity
(--json/--csv/--select/--compact), and error paths. All 49 checks passed against the
live Creative Fabrica Algolia catalog.

## Notes
- Earlier scorecard live-sample flagged new-since for "no query token in output" — that is
  correct first-run seed behavior (seeds the snapshot, reports nothing new), not a bug.
- Fixes applied this session: flexString/flexFloat tolerance for mixed price types,
  CleanText for HTML entities, AdaptiveLimiter + typed 429 handling on the Algolia client,
  quoteFacet backslash-escaping (filter-injection hardening), Retry-After honoring.
