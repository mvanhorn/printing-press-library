Acceptance Report: lacourt-holidays
  Level: Full Dogfood (live, public no-auth API)
  Tests: 98/98 run checks passed (77 skipped by the runner: framework commands needing fixtures, mutating-only probes)
  Failures (fixed across 2 loops):
    - ics: hollow (happy path ran with --out-dir --dry-run) -> pp:preview-happy-path + year=2027 fixture, stdout-only first example
    - holidays: no derivable example -> spec example 2027, Example + happy-args
  Fixes applied: 2 (CLI fixes)
  Printing Press issues: 3
    - generate aborts at govulncheck gate before writing .printing-press.json/manifest.json/spec.yaml/CHANGELOG.md
    - string-wrapped JSON array response yields no syncable resource (no sync/search) and no ID field
    - generated docs carry sync/search/credential boilerplate for read-only no-auth CLIs without a sync resource
  Gate: PASS
