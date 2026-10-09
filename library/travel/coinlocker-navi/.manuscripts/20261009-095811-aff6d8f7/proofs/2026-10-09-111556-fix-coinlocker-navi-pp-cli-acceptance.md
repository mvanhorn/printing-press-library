Acceptance Report: coinlocker-navi
  Level: Full Dogfood (orchestrator answer, 2026-10-09; ~1 req/s per host, on demand, no bulk crawl)
  Tests: 46/46 passed (44 skipped by the runner: framework commands with free-text positionals, mutating framework commands such as profile/feedback without --allow-destructive, and 2 no-error-path-probe annotations)
  Commands exercised live: near (4), locker (5), vacancy (4), source search/locker/ekicube (3 each), doctor, which, api, profile, feedback, workflow status
  Failures in earlier loops:
    - loop 1 (5/42 failed): source commands had no Examples; near/vacancy unknown keyword gave exit 0 (valid empty source result)
    - loop 2 (4/46 failed): source search/locker HTTP 500 from the site
  Fixes applied: 4
    - CLI fix: Examples on source search/locker/ekicube
    - CLI fix: pp:no-error-path-probe on near and vacancy (a keyword with no matches is a valid empty result); near now adds a "no records for <keyword>" note
    - CLI fix: spec happy_args used wire names (--service_type) and a positional id; now kebab-case flags and --id
    - CLI fix: HTML source commands now send Accept: text/html (site returns 500 for Accept: application/json)
  Printing Press issues: 2
    - Generated HTML-response endpoints default to Accept: application/json (client.go doInternal); should default to text/html for X-Printing-Press-HTML-Response endpoints
    - Generated dry-run JSON for HTML endpoints nests dry_run under results and prints the request text after the JSON on stdout
  Gate: PASS
  Marker: <runstate>/printed-cli-9cb29709/runs/20261009-095811-aff6d8f7/proofs/phase5-acceptance.json (status pass, level full, 46/46)
