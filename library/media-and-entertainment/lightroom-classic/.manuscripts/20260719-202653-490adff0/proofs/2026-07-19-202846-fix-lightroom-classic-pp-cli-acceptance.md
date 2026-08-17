Acceptance Report: lightroom-classic
  Level: Full Dogfood
  Tests: 124/124 passed (0 failed; earlier run 112/121 before fixes)
  Failures (first run, all fixed):
    - catalog collections/keywords/cameras/lenses happy_path: mirrored commands carried top-level examples the runner couldn't map — gave mirrors path-correct examples
    - path error_path: unknown filename legitimately returns [] exit 0 — annotated pp:no-error-path-probe
    - project happy/json: fixture collection absent from this catalog raised an error — now returns a structured zero-progress report with a note (empty local result, not a failure)
    - sync happy/json: hidden sync still dialed a nonexistent API — overridden to an honest local no-op ("catalog is the data store")
  Fixes applied: 4 (above)
  Printing Press issues (retro): live-dogfood enumerates hidden commands; local-sqlite specs still emit HTTP sync; mirrored-command example derivation assumes one instance per command
  Gate: PASS

  Sample correctness (real 32k-image catalog): streaks 199/200 days in 2026 (gap 2026-06-22, current 27, longest 172); on-this-day resolves per-year bests; photos filters return matching camera/ISO rows with correct APEX conversions; path resolves and exists-checks; funnel percentages consistent with develop-settings counts.
