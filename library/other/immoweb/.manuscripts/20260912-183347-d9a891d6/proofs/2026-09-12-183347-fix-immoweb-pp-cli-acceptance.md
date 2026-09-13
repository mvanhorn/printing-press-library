# Acceptance Report: immoweb

  Level: Full Dogfood (user-selected)
  Tests: 139/139 passed (114 help/structure rows reported skipped/unverified by the runner)
  Auth: none (public JSON endpoints)
  Failures (first run): 1
    - dump: `dump --format csv --postcode 1050 --json` printed CSV; expected JSON because --json was given.
  Fixes applied: 1
    - CLI fix: --json/--agent now win over --format csv/jsonl in `dump` (geojson, already JSON, is kept).
  Printing Press issues: 0 new in this phase (see phase-4.95 retro candidates)
  Earlier in-session live verification (pre-dogfood): watch, triage (live and local), dump, deal, yield, market multi-commune, drops empty-area hint, province/district validation, large-area harvest fallback.
  Gate: PASS
