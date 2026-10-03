# Bland CLI Phase 5 Acceptance

- Level: Full live dogfood, read-only mutations policy.
- Tests: 105/105 passed, 0 failed. The runner reports 132 skipped/unverified samples out of 237 total observations; the machine-readable acceptance marker records matrix size 105 and status `pass`.
- Auth: API key was available and authenticated read-only call-list requests succeeded.
- Fixes: Corrected `active` to filter the supported `GET /v1/calls?completed=false` endpoint after `/v1/active` returned 404; added missing stop/event-stream help examples; annotated local task search to skip the invalid generic error-path probe because an unmatched search correctly returns an empty list; made jobs list/prune honor dry-run.
- Mutating calls: Skipped by default. No outbound call was placed and no call was stopped.
- Hollow features: `calls task run` was exercised only in dry-run mode because live invocation places a real call. `calls search-task` could not receive a live sample because the dogfood sandbox has no locally cached CLI-originated task history and its generic positional-word classifier skipped that probe. Output plausibility review therefore remains SKIP (no eligible passing samples).
- Privacy: Live call-history output samples were redacted from dogfood result artifacts; scans found no original test-call ID or phone number in the run tree.

Gate: PASS. See `phase5-acceptance.json` for the machine-readable marker and `2026-09-29-fix-bland-pp-cli-dogfood-results-retry2.json` for the redacted runner results.
