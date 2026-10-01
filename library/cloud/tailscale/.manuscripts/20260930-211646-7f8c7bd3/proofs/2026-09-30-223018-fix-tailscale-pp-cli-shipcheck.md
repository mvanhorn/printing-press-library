<!-- slop-gate: off -->
# Tailscale CLI shipcheck (loop 1)

Command: `cli-printing-press shipcheck --dir <working> --spec research/tailscale-spec-enriched.yaml --research-dir <run>` with TAILSCALE_API_KEY loaded from a mode-600 env file.

## Result
- Verdict: PASS (7/7 legs: verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard)
- verify: Verification verdict WARN (pass rate 99%, threshold 80%). Running fix loop (max 3 iterations)...
- scorecard:   Total: 94/100 - Grade A
- Sample output probe (live, novel-feature examples): 5/10 passed. The 5 misses were placeholder device names in examples (home-mac, nas) that do not exist in the test tailnet (exit 3, correct not-found behavior) and devices inspect timing out at 10s because the audit-log endpoint takes about 7s for a 7-day window.

## Fixes applied before and during this loop
- Spec: removed DELETE /tailnet/{tailnet} (generator promoted it onto the bare `tailnet` group), marked acl preview/validate as reads, renamed the AWS external ID POST to create-or-get and marked it a mutation, explicit device filter params, 28 operationId renames, `{tailnet}` bound to TAILSCALE_TAILNET with "-" default so sync works.
- Code: search forced local (generator picked POST /dns/searchpaths as a search endpoint), sync stamps tailnet scope on typed rows, novel mutations plan-only over MCP, hujson copy-before-standardize fix.
- After loop 1: novel-feature examples switched to `self` so they run on any tailnet; devices inspect audit window defaults to 24h.

## Known gaps (documented in README)
- Live --yes writes were exercised against an httptest mock, not the real tailnet.
- MCP `tailscale_execute` can run mutating endpoints (permission-prompted by MCP hosts); upstream confirm gate in progress.
- sync reports non-critical errors for `acl` (single document) and `user-invites` (API returns null when empty).

## Recommendation
ship (pending phase 4.95 code review and live dogfood)
