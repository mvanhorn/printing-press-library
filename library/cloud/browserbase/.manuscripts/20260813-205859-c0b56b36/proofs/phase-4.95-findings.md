# Phase 4.95 — Local Code Review

## Review path
Direct subagent dispatch (correctness + security + maintainability reviewers, parallel).

## Autofix summary
13 findings autofixed in-place across 1 round; see working tree (uncommitted).

## Correctness fixes applied
1. **sessions_run.go**: hold now governed by session lifetime (`holdCtx`), not the 60s root `--timeout` — was releasing sessions ~60s early on every run. Added [60s, 6h] API range validation on `--timeout`. Release path uses `replacePathParam`.
2. **projects_digest.go**: agent-runs resource_type fixed to `agents-runs` (hyphenated, as sync writes); downloads now joined via typed `downloads` table → sessions by sessionId (was always 0).
3. **usage_trend.go**: queries typed `usage` table on `projects_id` + `synced_at` (was querying nonexistent `resources` rows with `id` match and `recordedAt` field — always empty).
4. **web_history.go**: fetch branch queries typed `fetch` table (synced_at/status_code); fixed scan-arity bug (4-col SELECT vs 5-col Scan — would crash on every fetch row).
5. **agents_runs_diff.go**: `GetNoCache` for fresh reads; result comparison canonicalized; messages paginated (3 pages × 100).
6. **sessions_orphans.go**: status filter RUNNING/PENDING (was CREATED — dead enum); release-failure message includes HTTP status.
7. **fetch_batch.go**: global pacing via shared ticker (was per-goroutine sleep = ~3× rate); deterministic indexed result order; counts derived from single post-pass; 3xx classified as failure.

## Maintainability fixes applied
- Duplicated RFC3339 parse blocks → `cliutil.ParseStoredTime` (4 files).
- Dead `SessionMins` field removed; dead `Type == "search"` self-assignment removed.
- gofmt alignment on all 9 novel files.

## Template-shape retro candidates
- None identified.

## Out-of-scope retro candidates
- None (cliutil/cobratree untouched).

## Surface-to-user findings
- None (all fixes mechanical and behavior-preserving).

## Convergence outcome
Findings cleared at round 1.

## Security review
PASS — no security findings (no injection, no secret leakage, no path traversal, parameterized SQL).
