Acceptance Report: edeka
  Level: Full Dogfood (binary-owned live matrix, without --allow-destructive)
  Tests: 183/183 passed (134 skipped: mutating dry-run JSON rows and commands that do not honour --dry-run)
  Session: the test account's cookie session injected through EDEKA_SESSION_FILE (owner-only file outside the run dir)
  Live rows that ran for real: markets search/current, offers, products search/show, categories, suggest, deals check, compare markets, price history, slots list/earliest/selected, basket show, order list, order place (preview only: slot, payment, items, total from the shop's review page; placed=false).
  Mutating rows ran as dry runs: basket add/set/remove/clear, list add, slots reserve/release, markets select, payment options.
  Hollow: basket add, list add (live-happy-path needs --allow-destructive, which this run did not use so that no checkout path could run destructively). Both were exercised live by hand during the build: basket add by text and by id, weight article with --grams, set, remove through the basket form, all reverted.
  Failures (first run): 5 error-path rows (products search, deals check, compare markets, markets search, price history exited 0 for a nonsense term because the shop answers unknown words with unrelated fallback products).
  Fixes applied: 1
    - CLI fix: unmatched queries now exit 3 with the JSON envelope (word-level match check WordMatch; BestMatch requires it too).
  Printing Press issues: 2
    - A --force regeneration merged a stale copy of generated cliutil tests (credentials_test.go referenced helpers that were no longer there).
    - generate writes .printing-press.json only after its go-test gate passes; on Windows two generated cliutil tests always fail, so the manifest is missing until a --validate=false regeneration.
  Gate: PASS
