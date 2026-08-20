# SculptOK CLI — Live Acceptance Report

Level: Full Dogfood (live, with user-provided key + approved generation)
Gate: PASS (dogfood --live: 64 passed, 0 failed, 49 skipped)

## Manual full-workflow verification (live)
- doctor: PASS — auth configured, API reachable, env:SCULPTOK_API_KEY source.
- credits balance: PASS — returns balance (free read). NOTE: default serves a cached value; `--no-cache` and `cost` read fresh.
- credits history --json: PASS — returned the account's credit-change history (paged).
- drawings list --json: PASS — returned the account's prior generated images.
- sync --resources credits,drawings: PASS — mirrored credit events + drawings into the local SQLite store.
- analytics --type credits --group-by actionType: PASS — real spend breakdown by action type (draws/downloads/check-ins).
- search "Draw" --type credits: PASS — found draw-spend events with embedded promptIds.
- generate depthmap <local PNG> --out --db (LIVE, ~10 credits, user-approved): PASS — uploaded local image (multipart), submitted draw, polled to completion, returned result URL, downloaded a 1280x1280 depth-map PNG, and persisted the job to the local store.
- search --type jobs (after generate): PASS — the live job appears with full metadata (kind, status, resultUrls, creditCost).
- draw status --uuid <promptId>: PASS — returned terminal status.
- Credit deduction confirmed (balance dropped by the draw cost; verified via fresh read).

## Fixes applied during Phase 5
- draw status references corrected from positional to `--uuid` (generate output, README, research.json).
- generate {depthmap,stl,threed,restore}: added pp:no-error-path-probe (error-path probe is unfittable for a harness-guarded paid file-input mutation; happy path + JSON checks still run).
- (pre-Phase-5 review) URL-encode query params; preload reconcile prompt-ids; README account/auth-status wording.

## Known minor gaps (non-blocking)
- `credits balance` (generated command) serves a cached GET; the live balance is fresher via `--no-cache` or `cost`. Candidate for polish (disable cache on the balance endpoint).
- Generated absorbed list/object commands emit the full {code,msg,data} envelope (response_path not applied); data is correct and agent-parseable. Hand-built commands unwrap cleanly.

## Printing Press issues (retro)
- Generated list commands referenced `truncateJSONArray` without emitting it (compile failure).
- No SQLite store/sync emitted for a list-only (no get-by-id) spec; novel-feature commands scaffolded as TODO stubs with no store foundation.

PII: account identifiers, balances, and history values described generically; no literal user/account data, tokens, or emails recorded.
