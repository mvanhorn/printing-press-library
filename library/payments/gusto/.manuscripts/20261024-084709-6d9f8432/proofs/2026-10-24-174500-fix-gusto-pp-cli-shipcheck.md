# gusto-pp-cli shipcheck (run 20261024-084709-6d9f8432)

## Final umbrella run (shipcheck4, 2026-10-24)
| Leg | Result |
|---|---|
| verify (mock) | FAIL: 97% (32/33); only row failing = browser-session-proof |
| validate-narrative | PASS |
| dogfood | PASS (WARN: 9 dead generator helpers) |
| workflow-verify | PASS (no manifest) |
| apify-audit | PASS |
| verify-skill | PASS |
| scorecard | PASS 69/100 Grade B |

verify live mode (--env-var PP_GUSTO_LIVE): 94%; failing rows = browser-session-proof (same cause) and `auth-env:GUSTO_SKIP_TLS_VERIFY` ("required per-call auth env var is not set": verify classifies the TLS-skip override as an auth var).

## Before/after
- verify pass rate: 92% (loop 1) -> 97%
- verify-skill: 8 errors -> 0
- scorecard: 69 -> 69

## Fixes applied
1. Flags declared inline (verify-skill scanner).
2. Generated GraphQL endpoints replaced by hand-built commands; spec resource `session` list endpoint (tail read path).
3. chrome.go: plain http routed to h1 (generator bug).
4. auth.go cookie-tool probe made quiet via cookie_probe_quiet.go (generator bug: Python traceback on stderr broke `doctor --json` combined output).
5. CalculatedReview: re-read review until total debit equals employer cost (live bug found: prep printed a stale $64.93 debit; settled value $73.08).

## Browser-session proof against the real site (ship-threshold exception evidence)
- `gusto-pp-cli auth login --chrome`: session saved (auth_source config, domain app.gusto.com).
- `gusto-pp-cli doctor --json`: browser_session_proof = valid, "GET /api/user_authentication_events verified at 2026-10-24T22:41:39Z".
- `gusto-pp-cli session --json`: live 200 with sign-in events.
- Harness gap: verify (runtime.go) runs the CLI under a fresh scoped HOME with no credential mirror, so its browser-session-proof row cannot see any real session. Live dogfood uses scopeSubprocessHomeWithCredentialMirror. Retro finding.

## Live behavior run (Draft Oct 26 - Nov 1, 2026; restored to empty afterwards)
- history, due, show, reimburse list, reimbursements suggestions, net-pay-preview: correct live data.
- mileage 5.2 x 0.60 -> $3.12, read back; re-run replaced, no duplicate.
- hours set 4 -> read back 4; prep from matching CSV -> no changes; prep after hours 6 -> applied 4, settled totals gross 64, reimb 3.12, net 62.22, debit 73.08.
- verify: matching CSV exit 0; mismatching CSV exit 1 with 4 specific mismatches.
- compare vs Oct 19-25: debit -279.83, payable tax -48.11.
- Cleanup: reimbursement removed, hours 0, review recalculated: gross 0, debit 0.

## Recommendation
ship. The operator approved on 2026-10-24 treating the verify browser-session-proof row as a documented Printing Press harness gap (fresh scoped HOME without credential mirror); the real-site proof passes and the live dogfood gate (Phase 18, credential-mirrored) is the authoritative session check.
