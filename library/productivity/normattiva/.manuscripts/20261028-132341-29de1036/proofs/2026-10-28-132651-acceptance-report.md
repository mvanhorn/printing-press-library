Acceptance Report: normattiva
  Level: Full Dogfood (runner: cli-printing-press dogfood --live --level full)
  Tests: 183/183 passed (138 skipped by the runner: positional-less error paths, generated commands without dry-run, mutating-command guards)
  Loop 1 failures (5): cerca/fonti error_path on nonsense input (valid empty results), dettaglio urn happy-args split on ';', local-data purge dry-run JSON lacked "action".
  Fixes applied: 4
    - CLI fix: pp:no-error-path-probe on cerca and fonti (zero matches is a valid result)
    - CLI fix: spec happy_args escaped as \; for the URN
    - Printing Press issue (patched locally): local-data purge --dry-run --json emits no "action" key
    - (earlier phases) 10+ review fixes, re-verified live
  Extra live check (user-approved, once): esporta --avvia --attendi on L. 241/1990 → token, stato 3, 38 KB ZIP containing exactly LEGGE_19900807_241 JSON.
  Printing Press issues: 3
    - local-data purge dry-run envelope missing "action" (template)
    - ricerca group Short "Get and create ricerca" for POST read-only search (template)
    - 2 dead generated helpers retainCLIQueryParams/retainExplicitQueryParams
  Gate: PASS
