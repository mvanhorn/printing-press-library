# Discogs CLI — Live Acceptance (Full Dogfood, read-only)

Token: valid personal access token (user-provided, in-memory only, never written). Destructive endpoints skipped by runner default → read-only against the real account.

**Result: 179/194 passed.** All 7 novel commands and all core parity reads (collection, wantlist, marketplace stats/listing/fee/price_suggestions, database release/master/artist/label, identity, lists) PASS live.

## 15 failures — all non-defects, classified

**Framework learn-loop commands (8 tests) — RETRO (not this CLI's code):**
- teach, teach-pattern, teach-playbook, playbook amend — happy_path + json_fidelity exit 2 (missing required flags the dogfood matrix didn't synthesize). Generated framework scaffolding; the fix is upstream in the Printing Press, not a local patch.

**Inventory get-by-id (6 tests) — BLOCKED_FIXTURE:**
- inventory export-get, export-download, upload-get — happy_path + json_fidelity exit 3. Correct 404 for a synthesized id; the account has no exports/uploads to fetch, and creating an upload would mutate inventory (out of read-only scope). Verified the 404 handling is correct (helpful hint, typed exit 3).

**database search error_path (1 test) — no-error-path command:**
- search accepts any query string; there is no invalid argument that makes it exit non-zero. `pp:no-error-path-probe` is the correct classification.

## Verdict
Ship-quality. Zero defects in shipping-scope features. Residual failures are a framework matrix gap (retro) + fixture limitations (BLOCKED_FIXTURE) + one no-error-path command. The binary acceptance marker reads `status: fail` on the raw count, but every failure is a documented non-defect.
