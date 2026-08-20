# RideWithGPS CLI — Phase 4.8/4.9/4.95 Review Findings

## Phase 4.95 Local Code Review (8 hand-authored Go files)
Reviewer: general-purpose subagent (correctness + security). go vet clean.

**ERROR (fixed in-place):**
- `export.go exportAsset` (also reached via `event_routes.go`): **path traversal** — `--id` is user-controlled and flowed into the output filename + request path. `--id ../../tmp/evil` escaped `--out`. **Fix:** reject ids containing `/`, `\`, or `..` at the top of `exportAsset` → `ef.Error = "invalid id..."`. Verified: traversal id now rejected, no escape.

**Clean (explicitly ruled out):** SQL injection (all flag values use `?` placeholders; only fixed const exprs are interpolated), nil-deref (track points nil-checked; Gear guarded), goroutine/channel correctness (gear fan-out: buffered channel, defer-release semaphore, separate wait-then-close, no leak/deadlock), resource leaks (db/rows closed on all paths), error handling (partial-failure accounting excludes failed fetches from aggregates), context/timeout (every command wraps boundCtx), NULL-safety (all scans use sql.Null*).

## Phase 4.8/4.9 README/SKILL Correctness Audit
Reviewer: general-purpose subagent vs shipped binary.

**ERROR (fixed at source in research.json + re-rendered):**
- `auth login` was fabricated (3× across README/SKILL) — the CLI has no `login` subcommand (auth has set-token/status/logout; the harvester is top-level `auth-tokens`). Fixed `narrative.auth_narrative` + `troubleshoots[0].fix` to the real flow (`auth-tokens --user-email --user-password` + env vars). Verified: 0 `auth login` refs remain.

**WARNING (fixed):**
- "RideWithGPS itself" prose in export description → "Ride with GPS". Verified: 0 in user-facing docs.

**PASS:** all commands/flags resolve (gear `--due-km`, audit checks, top-level `event-routes`, flat `gear`); Unique Features = exactly the 7 built; no placeholder literals; auth headers/env vars correct; read/write claims accurate (no map-drawing/public-route-discovery claims); anti-triggers present; trigger phrases map to real capabilities.

## Phase 4.85 Agentic Output Review
Covered by the scorecard sample-output probe (7/7 novel features produce plausible output, 100% pass). Deferred deeper output review to Phase 5 live dogfood (no live data available without credentials at review time). Wave B = warnings-only; none to surface.

## Post-fix
Re-ran shipcheck after both fixes: 6/6 legs PASS, scorecard 92/100. No regressions.
