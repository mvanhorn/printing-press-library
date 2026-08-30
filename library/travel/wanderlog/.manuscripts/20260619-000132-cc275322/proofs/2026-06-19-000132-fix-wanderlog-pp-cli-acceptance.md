Acceptance Report: wanderlog
  Level: Full Dogfood, anonymous public tier
  Tests: 85/85 passed
  Skipped: cookie-only personal trip commands without WANDERLOG_COOKIE
  Failures: none
  Fixes applied: 4
    - Added shared-plan clone/fill/preview commands for shared Wanderlog plans.
    - Added focused unit tests for plan key parsing, copyability reports, and ShareDB fill op construction.
    - Treated Wanderlog HTTP 200 responses with success:false as command failures.
    - Replaced placeholder place endpoint examples with live Eiffel Tower fixtures.
  Credential-gated gap:
    - plan clone/fill --apply and trips home require WANDERLOG_COOKIE and were not run against a disposable authenticated trip.
  Gate: PASS
