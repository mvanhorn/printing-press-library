# Shipcheck Proof — jobber-pp-cli

Run ID: `20260522-234028`
Printing Press version: `4.9.0`
Date: 2026-05-24

## Final Verdict: SHIP-WITH-DOCUMENTED-GAPS

Phase 4/4.8/4.85/4.9/4.95 shipcheck (structural) passed; Phase 5 live
dogfood was skipped per operator directive ("Skip live dogfood, promote
to GitHub now") because no production tenant credential is available for
this print and the CLI is strictly read-only against Jobber's GraphQL
API. The 5 unimplemented novel commands listed below are intentional
v0.2 gaps, not blockers.

| Leg | Result | Notes |
|-----|--------|-------|
| Phase 4 — structural shipcheck | PASS | command_tree 45/45 registered, config_consistency consistent, MCP runtime_walking pass |
| Phase 4.8 — example coverage | PASS (advisory) | 8/10 commands carry examples; `ar aging` and `snapshot save` missing |
| Phase 4.85 — pipeline check | PASS | sync uses domain-specific Upsert methods; 1 domain table |
| Phase 4.9 — dead-code sweep | WARN | 1 dead flag (`allowPartialFailure`), 3 dead helpers (`detectPartialFailure`, `dryRunOK`, `partialFailureErr`) — generator scaffolding for a feature not used by this read-only print |
| Phase 4.95 — novel-features audit | WARN | 3/8 planned novel commands implemented (`ar aging`, `invoices trace`, `snapshot save/diff`); 5 deferred to v0.2 (see Known Gaps) |
| Phase 5 — live dogfood | SKIP | `auth_required_no_credential`; recorded in `phase5-skip.json` alongside this artifact |

## Top Findings Fixed In This Cycle

1. **Greptile P0 — RefreshAccessToken race.** The 4-worker sync pool shares one `*Client`; concurrent 401s could each call `RefreshAccessToken` and the second would post an already-invalidated refresh token, permanently breaking the rotation chain. Fixed by adding `sync.Mutex` to `Client`, introducing `refreshIfStale(authHeaderBefore)` which acquires the lock and short-circuits when `Config.AuthHeader()` has already changed, and routing the 401 retry path through it. `RefreshAccessToken()` still exists as the public single-caller entry point and now also takes the lock.
2. **Greptile P1 — Cache file permissions.** `writeCache` was creating `0o755` directories and `0o644` files containing raw API response bodies that can include customer rows. Tightened to `0o700`/`0o600`.
3. **Greptile P1 — Unconditional PowerShell exec on non-Windows hosts.** `oauth.go` was calling `powershell -NoProfile -Command [Environment]::SetEnvironmentVariable(...)` on every refresh regardless of OS, producing noisy stderr on Linux/macOS. Guarded with `runtime.GOOS == "windows"`; process-env `os.Setenv` calls above the guard still rotate tokens for the current process on all platforms.
4. **Greptile P2 — SQL validator missed `WITH`.** `sqlBannedTokenRE` rejected the DML/DDL/PRAGMA set but a CTE-wrapped write like `WITH x AS (...) DELETE FROM t` bypassed it. SQLite read-only mode still caught the actual write, but defense-in-depth required the validator to reject. Added `|WITH` to the alternation.

## CI / Verifier Status

- `go build ./...` clean.
- `go vet ./...` clean (per prior in-session run).
- `verify-library-conventions.yml → Publish package completeness` previously failed PR #816 with two annotations:
  - Missing `.goreleaser.yaml` — addressed by adding dual-build (CLI + MCP) goreleaser config matching the sales-and-crm/gohighlevel template.
  - Missing acceptance/shipcheck artifact in `proofs/` — addressed by this file (`shipcheck` substring in the filename satisfies `verify_publish_package.py:295`).

## Known Gaps (v0.2 Backlog)

The following novel commands are listed in `.printing-press.json:novel_features` but not yet implemented. They were dropped from in-session scope to keep the publish lane unblocked:

- `payouts reconcile`
- `jobs pnl`
- `jobs stale`
- `funnel`
- `clients 360`

## Ship Recommendation

Ship. All four Greptile findings are addressed in `.printing-press-patches.json:jobber-greptile-pr816-hardening`. Phase 5 live dogfood remains skipped pending an authorized tenant credential; until then, the structural shipcheck above is the highest-fidelity validation available.
