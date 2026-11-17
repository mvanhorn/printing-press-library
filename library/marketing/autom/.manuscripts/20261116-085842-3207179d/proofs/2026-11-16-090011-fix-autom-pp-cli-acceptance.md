Acceptance Report: autom
  Level: Full Dogfood (run by the user with their own key; the agent never read the key)
  Tests: 153/153 passed (final run on the final source tree, marker proofs/phase5-acceptance.json)
  Runs:
    - Run 1: 152/154; failures: ground error_path, visibility error_path; hollow: news watch
    - Run 2 (after fixes): 153/153
    - Run 3 (after the Google redirect fix): 153/153
  Failures (final): none
  Fixes applied: 4
    - Domain validation on every domain argument and flag (exit 2 for non-domains)
    - ground: pp:no-error-path-probe (any text is a valid question)
    - news watch and rank check: pp:preview-happy-path with --peek / --no-save, so the live matrix runs them for real
    - Google redirect links (google.com/goto?...&gsessionid=...) compared by domain + title in serp diff, competitors, movers and ground
  Observations from live output:
    - Bing answered HTTP 500 on one call during run 2 (Autom-side); engines compare reported it in fetch_failures and compared the remaining engine.
    - Generated endpoint commands without a spec example for --query skipped their live happy path (help, dry-run and JSON checks ran). The hand-written commands that call those endpoints ran live.
  Printing Press issues: 3
    - mcp:local-write makes the live runner treat a local-only write as mutating (dry-run only, hollow novel feature)
    - dogfood's host scan flags literal host names in pure-logic code (bing.com in a redirect detector)
    - generate --force on the same spec deleted hand-authored files (see build log)
  Gate: PASS
