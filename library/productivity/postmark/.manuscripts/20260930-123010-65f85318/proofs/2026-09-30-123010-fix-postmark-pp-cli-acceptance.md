<!-- slop-gate: off -->
# Acceptance Report: postmark

  Level: Full Dogfood (user-selected)
  Credentials: the test account's account token plus one server token, from a password manager; mutations dry-run only (no --allow-destructive)
  Tests: 372/372 passed (pass rate 100%), 388 skipped by policy, 0 failed
  Novel feature happy paths (live, non-dry-run): pulse, email send-once, recipient-domains, templates check, servers bootstrap all PASS; coverage_hollow: none

  Run history:
    Run 1: 43/411 failed
      - get commands ran fake spec example IDs (explicit positional happy-args disabled the list-companion ID resolver)
      - bulk email and data removals not enabled on the test account
      - hand command first examples named servers that do not exist on the account
      - templates validate example had no body
      - suppressions list error-path: Postmark returns 200 [] for unknown streams
    Run 2: 2/370 failed (templates validate); hollow: email send-once, servers bootstrap (classified mutating, dry-run only)
    Run 3: 5/373 failed (novel examples named nonexistent servers; bootstrap error-path has no invalid name)
    Run 4: 0 failed, PASS

  Fixes applied: 9
    - CLI fix: flag-only x-happy-args on GET operations with path IDs so the runner resolves real IDs
    - CLI fix: x-live-dogfood-requires-tier on bulk email (bulk-approved) and data removals (data-removals-enabled)
    - CLI fix: templates validate x-pp-example with subject/html/test model
    - CLI fix: first help examples of diagnose, suppressions check, streams health, webhooks health, templates check, email send-once, servers bootstrap no longer name a server
    - CLI fix: pp:no-error-path-probe on suppressions list (upstream 200 for unknown stream) and servers bootstrap (any name is valid)
    - CLI fix: email send-once and servers bootstrap withhold --send/--apply/--env-file from MCP and are read-only there; CLI keeps explicit --send/--apply
    - CLI fix: research.json novel examples runnable against any account
  Printing Press issues: 4
    - Spec path examples become explicit positional happy-args, which disables the live runner's list-companion ID resolver
    - Plan-by-default novel commands (the recommended side-effect pattern) are always hollow unless marked read-only; no annotation for "default invocation is plan-only"
    - live_check isGracefulEmptyResponse counts OS "no such file or directory" usage errors as passes
    - Runtime verify mock returns bare arrays for paths ending in "s"
  Gate: PASS
