Acceptance Report: zimmo
  Level: Full Dogfood (live, no auth — anonymous token minted by the CLI)
  Tests: 172/172 passed (run 2; run 1: 160/170)
  Failures (run 1), all fixed:
    - locality-price / sub-locality-price happy path + json: Zimmo requires startDate → spec marks it required with an example (CLI fix)
    - geocode help: no example → x-pp-example (CLI fix)
    - doctor, sync, workflow archive (json_fidelity) and feedback list, profile list (dry_run_json): framework commands ignored --dry-run under --json → hand-authored dry-run guard hook (CLI fix; Printing Press issue)
  Fixes applied: 4
  Printing Press issues: 3
    - doctor/sync/workflow archive/feedback list/profile list templates do not emit the dry-run envelope under --dry-run --json (fails live dogfood whenever a resource is syncable)
    - x-pp-syncable: false does not turn off a heuristically syncable GET list endpoint
    - OpenAPI response-path inference treats a lone array field (priceHistory) of a single-object response as a list envelope
  Gate: PASS
