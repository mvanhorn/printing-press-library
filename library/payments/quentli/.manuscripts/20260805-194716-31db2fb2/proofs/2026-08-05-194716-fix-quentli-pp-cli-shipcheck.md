# Quentli CLI Shipcheck Report

> Final score derived from .printing-press.json persisted by shipcheck.

## Command output summary (shipcheck umbrella)
- verify: PASS
- validate-narrative (--strict --full-examples): PASS
- dogfood: PASS
- workflow-verify: PASS
- verify-skill: PASS
- scorecard: Grade A, 96/100

## Sample Output Probe (live command sample)
- Passed: 6/6 (100% pass rate, 0 skipped)
- All six novel-feature commands behave (dunning, subs at-risk, reconcile, revenue, webhooks health, customer balance).

## Scorecard dimensions
- Output Modes, Auth, Error Handling, Terminal UX, README, Doctor, Agent Native: 10/10 each
- MCP Quality 8/10; MCP Remote Transport, Tool Design, Surface Strategy 10/10
- Local Cache 10/10; Cache Freshness 5/10; Insight 6/10; Agent Workflow 9/10
- Domain Correctness: Path Validity, Auth Protocol, Data Pipeline, Sync all 10/10
- Type Fidelity 5/5, Dead Code 5/5
- Unverified: live_api_verification (no API key available)

## Top blockers found
1. webhooks health was registered with no --since flag but narrative/example referenced it -> added --since + time-window filter.
2. customer balance dry-run output omitted the customer id -> custom dry-run envelope includes customer_id; example dropped --select that stripped the token.
3. novel scatter: subs at-risk initially skipped active subs (logic bug inverted) -> fixed to flag active subscriptions.

## Fixes applied
- Added --since flag + filter to webhooks health.
- Rewrote customer balance dry-run to include customer_id; simplified its research.json example.
- Corrected subs at-risk active-subscription logic.
- Added internal/cli/money.go + money_test.go (table-driven pure-logic tests).
- Added registerNovelCommand hook to attach webhooks health under the generator-owned webhooks command.

## Before/After
- verify: PASS (no failures after fixes)
- verify-skill: FAIL -> PASS
- validate-narrative: FAIL -> PASS
- scorecard: 96/100 Grade A

## Final ship recommendation
- ship (with documented, non-blocking gap: live API smoke testing skipped because no QUENTLI_SECRET_KEY available).
- MCP readiness: full (54 tools, all auth-required).
