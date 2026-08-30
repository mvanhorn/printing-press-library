# The Lancet CLI — Phase 5 Acceptance Report

  Level: Full Dogfood (live OpenAlex)
  Tests: 68/68 passed (0 failed, 45 skipped non-applicable)
  Gate: PASS

## Coverage
- doctor, journals, works search/get, authors search/get, sources get — pass
- cited-by (live citation lookup) — pass
- refresh (Lancet-scoped OpenAlex sync) — pass
- rank-authors, mesh, affiliation-growth, drift, curate, visibility-gap — pass
  (all return real data; analytics auto-sync a flagship sample on first run)
- sync, workflow archive — pass (dogfood-bounded pagination)
- JSON fidelity + output modes across all leaf commands — pass

## Fixes applied inline (all fixed before ship, none deferred)
- Lancet-default search scoping; curate live fallback; auto-sync on empty store;
  OpenAlex `per-page` pagination; dogfood page caps for sync/workflow archive;
  research.json/README `refresh` command alignment.

## Printing Press issues for retro
- Windows: dogfood executes `<cli>.exe`; building with `-o <cli>` (no extension)
  leaves a stale `.exe` shadowing fresh code → 12 spurious failures. Standardize
  the Windows binary name in generate/dogfood.
- Phase 1.9 reachability validated connectivity but not field coverage; the CrossRef
  affiliation gap (0% coverage) should surface pre-generation, not at Phase 3.

  Polish (Phase 5.5): attempted 4x, blocked by transient API 529 Overloaded
  (server-side). CLI meets ship threshold independently; polish can be re-run via
  /printing-press-polish thelancet when capacity returns.

Gate: PASS — proceed to promote.
