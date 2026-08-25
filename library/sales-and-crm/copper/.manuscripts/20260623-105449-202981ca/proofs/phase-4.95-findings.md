# Phase 4.95 — Local Code Review (7 novel command files)

Reviewer: correctness persona. 5/7 files clean (forecast, stale, dedupe, who, upsert — injection-safe, NULL-safe, mutation-gated). bulk + log concurrency/mutation gates confirmed correct.

## Autofixed in-place (2 warnings, both security)
1. bulk.go — opportunity ids from --ids/--query were concatenated raw into PUT /opportunities/{id}; a crafted id (../people/5, ?/#) could redirect the write. FIX: added isNumericID() allowlist in resolveBulkIDs; non-numeric ids rejected with usageErr. Regression test TestNovelBulkRejectsNonNumericID added.
2. log.go — flagActivity concatenated raw into DELETE /activities/{id}. FIX: numeric validation before delete in the log-fix branch.

Build + all novel tests green after fixes.

## Residual risks (documented, not blocking)
- log fix is non-atomic (delete then recreate; if recreate fails the original activity is lost) — inherent to Copper's immutable-activities API, no rollback possible.
- log fix always recreates as Note(0) even if the original was a call/meeting — minor type downgrade; future enhancement could add --activity-type to log fix.
- who.go / upsert.go minor edge cases (corrupt-row skip, idless-2xx) — low likelihood on real Copper responses.

## RETRO CANDIDATE (generator-level, out of scope here)
internal/cliutil/credentials_test.go: 4 failing tests reproduce in a pristine generation (TOML written into a JSON-parsed config path). One is a secret-scrub security assertion. File against the Printing Press generator.
